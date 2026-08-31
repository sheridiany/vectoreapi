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
			PoolID: pool.Id, Name: name, BaseURL: DefaultTikHubBaseURL,
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

func TestProductCatalogShowsCanonicalDraftWithDeclaredPlatformsWithoutMakingItExecutable(t *testing.T) {
	openRuntimeTestDB(t)
	pool := &model.SearchUpstreamPool{Name: "product-catalog"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	accounts := make([]*model.SearchUpstreamAccount, 0, 2)
	for _, provider := range []string{model.SearchUpstreamProviderJustOneAPI, model.SearchUpstreamProviderTikHub} {
		encrypted, err := EncryptUpstreamSecret("secret_" + provider)
		require.NoError(t, err)
		baseURL := DefaultTikHubBaseURL
		if provider == model.SearchUpstreamProviderJustOneAPI {
			baseURL = DefaultJustOneAPIBaseURL
		}
		account := &model.SearchUpstreamAccount{
			PoolID: pool.Id, Name: provider + "-account", Provider: provider,
			BaseURL: baseURL, SecretCiphertext: encrypted.Ciphertext,
			SecretNonce: encrypted.Nonce, SecretVersion: encrypted.Version,
			SecretPrefix: UpstreamSecretPrefix("secret_" + provider),
			Status:       model.SearchUpstreamAccountStatusHealthy,
		}
		require.NoError(t, model.CreateSearchUpstreamAccount(account))
		accounts = append(accounts, account)
	}
	publicID, err := model.GenerateSearchCapabilityPublicID("social.account.get@v1")
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID: publicID, OperationKey: "social.account.get", ContractVersion: "v1",
		Name: "社交账号详情", Category: "社交媒体", Description: "按平台获取标准化账号资料。",
		InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
		Status: model.SearchCapabilityStatusDisabled, PriceMicros: 500_000,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	for index, platform := range []string{"instagram", "tiktok"} {
		require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
			CapabilityID: capability.Id, UpstreamAccountID: accounts[index].Id,
			ToolName: "private-provider-operation-" + platform, Platform: platform,
			ProviderOperationID: "private-provider-operation-" + platform,
			MappingKey:          []string{justOneAPIDirectMappingKey, tikHubDirectMappingKey}[index],
			InputSchema:         capability.InputSchema, OutputSchema: capability.OutputSchema,
			Status: model.SearchCapabilityBindingStatusEnabled,
		}))
	}
	require.NoError(t, model.ReplaceSearchCapabilityGrants(capability.Id, []model.SearchCapabilityGrant{{EnterpriseID: 12}}))

	runtime := NewExecutionRuntime(nil)
	unauthorized, err := runtime.ListCatalogGroups(context.Background(), Principal{EnterpriseID: 11})
	require.NoError(t, err)
	assert.Empty(t, unauthorized)
	wrongCategory, err := runtime.ListCatalogGroups(context.Background(), Principal{EnterpriseID: 12, Scopes: []string{"finance"}})
	require.NoError(t, err)
	assert.Empty(t, wrongCategory)

	userCatalog, err := runtime.ListCatalogGroups(context.Background(), Principal{EnterpriseID: 12})
	require.NoError(t, err)
	require.Len(t, userCatalog, 1)
	assert.Equal(t, "社交账号详情", userCatalog[0].Name)
	assert.Equal(t, "catalog", userCatalog[0].Status)
	assert.False(t, userCatalog[0].Enabled)
	assert.Equal(t, int64(2), userCatalog[0].InterfaceCount)
	assert.Zero(t, userCatalog[0].AvailableInterfaceCount)
	assert.Equal(t, []string{"instagram", "tiktok"}, userCatalog[0].SupportedPlatforms)
	assert.Empty(t, userCatalog[0].RequestParameters)
	assert.Empty(t, userCatalog[0].InformationFields)
	assert.Empty(t, userCatalog[0].CostLabel, "draft upstream cost must not be presented as a user price")
	assert.Zero(t, userCatalog[0].PriceMinMicros)
	assert.Zero(t, userCatalog[0].PriceMaxMicros)

	publicCatalog, err := runtime.ListPublicCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, publicCatalog, 1)
	payload, err := common.Marshal(publicCatalog)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "private-provider-operation")
	assert.NotContains(t, string(payload), "JustOneAPI-account")
	assert.NotContains(t, string(payload), "input_schema")
	assert.NotContains(t, string(payload), "upstream_cost")

	discovery, err := runtime.Discover(context.Background(), Principal{EnterpriseID: 12}, "社交账号")
	require.NoError(t, err)
	assert.Empty(t, discovery.Tools, "draft product entries must remain unavailable to execution discovery")
}

func TestProductCatalogExplainsCanonicalRequestAndInformationFields(t *testing.T) {
	inputSchema, err := common.Marshal(objectInputSchema(map[string]any{
		"platform": stringSchema([]any{"douyin"}),
	}, []any{"platform"}))
	require.NoError(t, err)
	outputSchema, err := common.Marshal(trendListOutputSchema())
	require.NoError(t, err)
	publicID, err := model.GenerateSearchCapabilityPublicID("social.trend.list@v1")
	require.NoError(t, err)
	groups := productCatalogGroups([]catalogSnapshotItem{{
		capability: &model.SearchCapability{
			PublicID: publicID, OperationKey: "social.trend.list", ContractVersion: "v1",
			Name: "平台趋势榜", Category: "社交媒体", InputSchema: string(inputSchema), OutputSchema: string(outputSchema),
		},
		declaredBindings: []*model.SearchCapabilityBinding{{Platform: "douyin", Status: model.SearchCapabilityBindingStatusEnabled}},
	}})

	require.Len(t, groups, 1)
	assert.Equal(t, []string{"platform"}, groups[0].RequestParameters)
	assert.Equal(t, []string{"id", "platform", "rank", "score", "title", "type", "url"}, groups[0].InformationFields)
	payload, err := common.Marshal(groups)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "input_schema")
	assert.NotContains(t, string(payload), "output_schema")
}

func TestProductCatalogCountsOnlyExecutableNonJustOneAPIInterfaces(t *testing.T) {
	openRuntimeTestDB(t)
	pool := &model.SearchUpstreamPool{Name: "product-availability"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	capabilityID, err := model.GenerateSearchCapabilityPublicID("social.content.search@v1")
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID: capabilityID, OperationKey: "social.content.search", ContractVersion: "v1",
		Name: "社交内容搜索", Category: "社交媒体", Description: "按平台搜索公开内容。",
		InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
		Status: model.SearchCapabilityStatusEnabled, PriceMicros: 300_000,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	for index, provider := range []string{model.SearchUpstreamProviderJustOneAPI, model.SearchUpstreamProviderTikHub} {
		encrypted, encryptErr := EncryptUpstreamSecret("secret_" + provider)
		require.NoError(t, encryptErr)
		baseURL := DefaultTikHubBaseURL
		if provider == model.SearchUpstreamProviderJustOneAPI {
			baseURL = DefaultJustOneAPIBaseURL
		}
		account := &model.SearchUpstreamAccount{
			PoolID: pool.Id, Name: provider + "-available", Provider: provider, BaseURL: baseURL,
			SecretCiphertext: encrypted.Ciphertext, SecretNonce: encrypted.Nonce,
			SecretVersion: encrypted.Version, SecretPrefix: UpstreamSecretPrefix("secret_" + provider),
			Status: model.SearchUpstreamAccountStatusHealthy,
		}
		require.NoError(t, model.CreateSearchUpstreamAccount(account))
		platform := []string{"douyin", "tiktok"}[index]
		require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
			CapabilityID: capability.Id, UpstreamAccountID: account.Id,
			ToolName: "provider-operation-" + platform, Platform: platform,
			ProviderOperationID: "provider-operation-" + platform,
			MappingKey:          []string{justOneAPIDirectMappingKey, tikHubDirectMappingKey}[index],
			InputSchema:         capability.InputSchema, OutputSchema: capability.OutputSchema,
			CostCurrency: "CNY", ContractEquivalent: true, BillingReady: true,
			Status: model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 100_000,
		}))
	}

	runtime := NewExecutionRuntime(nil)
	catalog, err := runtime.ListCatalogGroups(context.Background(), Principal{EnterpriseID: 12})
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	assert.Equal(t, "available", catalog[0].Status)
	assert.Equal(t, int64(2), catalog[0].InterfaceCount)
	assert.Equal(t, int64(1), catalog[0].AvailableInterfaceCount)
	assert.Equal(t, []string{"douyin", "tiktok"}, catalog[0].SupportedPlatforms)

	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).
		Where("mapping_key = ?", tikHubDirectMappingKey).
		Update("status", model.SearchCapabilityBindingStatusDisabled).Error)
	catalog, err = runtime.ListCatalogGroups(context.Background(), Principal{EnterpriseID: 12})
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	assert.Equal(t, "unavailable", catalog[0].Status)
	assert.Equal(t, int64(1), catalog[0].InterfaceCount, "retired bindings are not declared interfaces")
	assert.Zero(t, catalog[0].AvailableInterfaceCount)
	_, err = runtime.Describe(context.Background(), Principal{EnterpriseID: 12}, capability.PublicID)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_UNAVAILABLE", publicErr.Code, "JustOneAPI must remain outside execution routing")
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
		PoolID: pool.Id, Name: "catalog-query-count", BaseURL: DefaultTikHubBaseURL,
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
