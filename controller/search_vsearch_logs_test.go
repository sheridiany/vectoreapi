package controller

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSearchUsageLogControllerTest(t *testing.T) {
	t.Helper()
	openSearchUsageStatControllerDB(t)
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{}, &model.Enterprise{}, &model.SearchAgentKey{}, &model.SearchUpstreamAccount{},
	))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 7, Username: "usage-user", Password: "test-password", AffCode: "usage-aff", Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Enterprise{
		Id: 11, Name: "Usage Enterprise", Code: "usage-enterprise", Status: model.EnterpriseStatusEnabled,
		RegistrationMode: model.EnterpriseRegistrationModeOpen,
	}).Error)
	key := &model.SearchAgentKey{
		Id: 21, UserId: 7, EnterpriseID: 11, Name: "usage-key", KeyHash: strings.Repeat("a", 64),
		KeyPrefix: "vr_live_usage", Status: model.SearchAgentKeyStatusActive,
	}
	require.NoError(t, key.SetScopes(nil))
	require.NoError(t, model.DB.Create(key).Error)
	require.NoError(t, model.DB.Create(&model.SearchUpstreamAccount{
		Id: 31, PoolID: 1, Provider: model.SearchUpstreamProviderAgentKeyMCP, Name: "usage-account",
		BaseURL: "https://api.agentkey.app/v1/mcp", SecretCiphertext: "ciphertext", SecretNonce: "nonce",
		SecretVersion: 1, SecretPrefix: "ak_live_usage", Status: model.SearchUpstreamAccountStatusHealthy,
	}).Error)
	require.NoError(t, model.CreateSearchUsageEvent(&model.SearchUsageEvent{
		RequestID: "usage-request", UserID: 7, EnterpriseID: 11, AgentKeyID: 21, UpstreamAccountID: 31,
		ServiceID: "vr_search", ServiceName: "Usage Search", Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusSucceeded, HTTPStatus: 200, LatencyMs: 42,
		UpstreamCostMicros: 100_000, ChargeMicros: 250_000,
	}))
}

func TestSearchUsageLogEndpointsReturnBulkEnrichedNames(t *testing.T) {
	seedSearchUsageLogControllerTest(t)
	gin.SetMode(gin.TestMode)

	adminRecorder := httptest.NewRecorder()
	adminContext, _ := gin.CreateTestContext(adminRecorder)
	adminContext.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs?range=30", nil)
	writeSearchLogs(adminContext, true)

	require.Equal(t, http.StatusOK, adminRecorder.Code)
	var adminPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Items []searchLogResponse `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminPayload))
	require.True(t, adminPayload.Success)
	require.Len(t, adminPayload.Data.Items, 1)
	adminItem := adminPayload.Data.Items[0]
	assert.Equal(t, "usage-key", adminItem.AgentKeyName)
	assert.Equal(t, "usage-user", adminItem.UserName)
	assert.Equal(t, "Usage Enterprise", adminItem.EnterpriseName)
	assert.Equal(t, "usage-account", adminItem.Account)

	userRecorder := httptest.NewRecorder()
	userContext, _ := gin.CreateTestContext(userRecorder)
	userContext.Set("id", 7)
	userContext.Request = httptest.NewRequest(http.MethodGet, "/api/search/logs?range=30", nil)
	writeSearchLogs(userContext, false)

	var userPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Items []searchLogResponse `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &userPayload))
	require.True(t, userPayload.Success)
	require.Len(t, userPayload.Data.Items, 1)
	assert.Equal(t, "usage-key", userPayload.Data.Items[0].AgentKeyName)
	assert.Empty(t, userPayload.Data.Items[0].UserName)
	assert.Empty(t, userPayload.Data.Items[0].EnterpriseName)
	assert.Empty(t, userPayload.Data.Items[0].Account)
	assert.Zero(t, userPayload.Data.Items[0].UpstreamCostMicros)
}

func TestAdminSearchUsageCSVContainsBulkEnrichedNames(t *testing.T) {
	seedSearchUsageLogControllerTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs/export?range=30", nil)

	AdminExportSearchUsageLogs(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(recorder.Body.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "usage-user", rows[1][3])
	assert.Equal(t, "Usage Enterprise", rows[1][5])
	assert.Equal(t, "Usage Search", rows[1][6])
	assert.Equal(t, "usage-account", rows[1][7])
}
