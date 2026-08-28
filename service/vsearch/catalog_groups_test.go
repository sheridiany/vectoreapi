package vsearch

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListCatalogGroupsCountsDistinctToolsInsteadOfHealthyRoutes(t *testing.T) {
	openRuntimeTestDB(t)
	pool := &model.SearchUpstreamPool{Name: "catalog"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	accounts := make([]*model.SearchUpstreamAccount, 0, 2)
	for _, name := range []string{"primary", "secondary"} {
		encrypted, err := EncryptUpstreamSecret("ak_live_" + name)
		require.NoError(t, err)
		account := &model.SearchUpstreamAccount{
			PoolID: pool.Id, Name: name, BaseURL: DefaultAgentKeyMCPURL,
			SecretCiphertext: encrypted.Ciphertext, SecretNonce: encrypted.Nonce,
			SecretVersion: encrypted.Version, SecretPrefix: UpstreamSecretPrefix("ak_live_" + name),
			Status: model.SearchUpstreamAccountStatusHealthy,
		}
		require.NoError(t, model.CreateSearchUpstreamAccount(account))
		accounts = append(accounts, account)
	}

	capabilities := make([]*model.SearchCapability, 0, 2)
	for index, toolName := range []string{"Brave/getWebSearch", "Brave/getImageSearch"} {
		publicID, err := model.GenerateSearchCapabilityPublicID(toolName)
		require.NoError(t, err)
		capability := &model.SearchCapability{
			PublicID: publicID, Name: "Brave Search", Category: "搜索", Description: "Search public web data",
			InputSchema: `{"type":"object"}`, Status: model.SearchCapabilityStatusEnabled,
			PriceMicros: int64(index+2) * 100_000,
		}
		require.NoError(t, model.CreateSearchCapability(capability))
		capabilities = append(capabilities, capability)
		for accountIndex, account := range accounts {
			if index == 1 && accountIndex == 1 {
				continue
			}
			require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
				CapabilityID: capability.Id, UpstreamAccountID: account.Id, ToolName: toolName,
				InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
			}))
		}
	}

	runtime := NewExecutionRuntime(nil)
	groups, err := runtime.ListCatalogGroups(context.Background(), Principal{UserID: 7, EnterpriseID: 11})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "Brave Search", groups[0].Name)
	assert.Equal(t, int64(2), groups[0].InterfaceCount)
	assert.Equal(t, int64(200_000), groups[0].PriceMinMicros)
	assert.Equal(t, int64(300_000), groups[0].PriceMaxMicros)
	assert.True(t, strings.HasPrefix(groups[0].ID, "vr_grp_"))

	adminCatalog, err := runtime.ListCatalog(context.Background(), Principal{}, true)
	require.NoError(t, err)
	require.Len(t, adminCatalog, 2)
	for _, item := range adminCatalog {
		assert.Equal(t, int64(1), item.InterfaceCount)
	}
	routesByID := make(map[string]int64, len(adminCatalog))
	for _, item := range adminCatalog {
		routesByID[item.ID] = item.HealthyRouteCount
	}
	assert.Equal(t, int64(2), routesByID[capabilities[0].PublicID])
	assert.Equal(t, int64(1), routesByID[capabilities[1].PublicID])

	payload, err := common.Marshal(groups)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "getWebSearch")
	assert.NotContains(t, string(payload), "getImageSearch")
	assert.NotContains(t, string(payload), "primary")
	assert.NotContains(t, string(payload), "secondary")

	require.NoError(t, model.ReplaceSearchCapabilityGrants(capabilities[1].Id, []model.SearchCapabilityGrant{{EnterpriseID: 12}}))
	groups, err = runtime.ListCatalogGroups(context.Background(), Principal{UserID: 7, EnterpriseID: 11})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, int64(1), groups[0].InterfaceCount, "unauthorized tools must not affect a group count")
}

func TestCatalogGroupIdentitySplitsBrokerPlatforms(t *testing.T) {
	youtubeFromTikHub, _ := catalogGroupFromToolName("TikHub/get_video_info_api_v1_youtube_web_get")
	youtubeFromJustOneAPI, _ := catalogGroupFromToolName("JustOneAPI/getApiYoutubeSearchV1")
	reddit, _ := catalogGroupFromToolName("TikHub/fetch_post_comments_api_v1_reddit_app_get")
	douyinShop, _ := catalogGroupFromToolName("TikHub/douyin_shop_product_search")
	douyinSocial, _ := catalogGroupFromToolName("TikHub/douyin_video_detail")
	unknown, label := catalogGroupFromToolName("TikHub/internal_operation")

	assert.Equal(t, "provider:youtube", youtubeFromTikHub)
	assert.Equal(t, youtubeFromTikHub, youtubeFromJustOneAPI)
	assert.Equal(t, "provider:reddit", reddit)
	assert.Equal(t, "provider:douyin-ecommerce", douyinShop)
	assert.Equal(t, "provider:douyin", douyinSocial)
	assert.Empty(t, unknown)
	assert.Empty(t, label)
}

func TestListCatalogGroupsUsesBoundedQueries(t *testing.T) {
	openRuntimeTestDB(t)
	pool := &model.SearchUpstreamPool{Name: "catalog-query-count"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	encrypted, err := EncryptUpstreamSecret("ak_live_catalog_query_count")
	require.NoError(t, err)
	account := &model.SearchUpstreamAccount{
		PoolID: pool.Id, Name: "catalog-query-count", BaseURL: DefaultAgentKeyMCPURL,
		SecretCiphertext: encrypted.Ciphertext, SecretNonce: encrypted.Nonce,
		SecretVersion: encrypted.Version, SecretPrefix: UpstreamSecretPrefix("ak_live_catalog_query_count"),
		Status: model.SearchUpstreamAccountStatusHealthy,
	}
	require.NoError(t, model.CreateSearchUpstreamAccount(account))
	for index := range 24 {
		toolName := fmt.Sprintf("Brave/search_%02d", index)
		publicID, generateErr := model.GenerateSearchCapabilityPublicID(toolName)
		require.NoError(t, generateErr)
		capability := &model.SearchCapability{
			PublicID: publicID, Name: toolName, Category: "搜索", Description: "Search public web data",
			InputSchema: `{"type":"object"}`, Status: model.SearchCapabilityStatusEnabled,
		}
		require.NoError(t, model.CreateSearchCapability(capability))
		require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
			CapabilityID: capability.Id, UpstreamAccountID: account.Id, ToolName: toolName,
			InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		}))
	}

	var queryCount int64
	const callbackName = "vsearch:catalog-query-count"
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register(callbackName, func(*gorm.DB) {
		atomic.AddInt64(&queryCount, 1)
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Query().Remove(callbackName))
	})

	runtime := NewExecutionRuntime(nil)
	groups, err := runtime.ListCatalogGroups(context.Background(), Principal{UserID: 7, EnterpriseID: 11})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, int64(24), groups[0].InterfaceCount)
	assert.LessOrEqual(t, atomic.LoadInt64(&queryCount), int64(6), "catalog queries must not grow with capability count")
}
