package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/vsearch"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var searchPublishTestDatabaseID atomic.Uint64

func TestAdminPublishSearchCatalogPublishesRequestedCapabilitiesGlobally(t *testing.T) {
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	databaseName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), searchPublishTestDatabaseID.Add(1))
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(
		&model.SearchUpstreamPool{}, &model.SearchUpstreamAccount{},
		&model.SearchCapability{}, &model.SearchCapabilityBinding{}, &model.SearchCapabilityGrant{},
	))

	pool := &model.SearchUpstreamPool{Name: "default"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	account := &model.SearchUpstreamAccount{
		PoolID: pool.Id, Provider: model.SearchUpstreamProviderAgentKeyMCP, Name: "primary",
		BaseURL: vsearch.DefaultAgentKeyMCPURL, SecretCiphertext: "fixture", SecretNonce: "fixture",
		SecretVersion: 1, SecretPrefix: "fixture", Weight: 1,
		Status: model.SearchUpstreamAccountStatusHealthy,
	}
	require.NoError(t, model.DB.Create(account).Error)
	publicID, err := model.GenerateSearchCapabilityPublicID("private/controller-publish")
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID: publicID, Name: "Publish test", Category: "搜索", InputSchema: `{"type":"object"}`,
		Status: model.SearchCapabilityStatusDisabled, UpstreamCostMicros: 100_000, PriceMicros: 50_000,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.Id, ToolName: "private/controller-publish",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.ReplaceSearchCapabilityEnterpriseGrants(capability.Id, []int{7}))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/search/admin/catalog/publish", bytes.NewBufferString(
		fmt.Sprintf(`{"service_ids":[%q],"access_mode":"all_enterprises"}`, capability.PublicID),
	))
	c.Request.Header.Set("Content-Type", "application/json")

	AdminPublishSearchCatalog(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                  `json:"success"`
		Data    vsearch.PublishResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, 1, payload.Data.Published)
	assert.Zero(t, payload.Data.Skipped)
	stored, err := model.GetSearchCapabilityByID(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusEnabled, stored.Status)
	assert.Equal(t, int64(100_000), stored.PriceMicros)
	grants, err := model.ListSearchCapabilityGrants(capability.Id)
	require.NoError(t, err)
	assert.Empty(t, grants)
}

func TestAdminPublishSearchCatalogReturnsStructuredValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"service_ids":`},
		{name: "empty ids", body: `{"service_ids":[],"access_mode":"all_enterprises"}`},
		{name: "invalid access mode", body: `{"service_ids":["vr_svc_1234567890abcdef"],"access_mode":"selected_enterprises"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/search/admin/catalog/publish", bytes.NewBufferString(test.body))
			c.Request.Header.Set("Content-Type", "application/json")

			AdminPublishSearchCatalog(c)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			var payload struct {
				Success bool   `json:"success"`
				Code    string `json:"code"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.False(t, payload.Success)
			assert.Equal(t, "CATALOG_PUBLISH_INVALID", payload.Code)
		})
	}
}
