package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		&model.WalletPreConsumeRecord{}, &model.SearchExecutionIdempotency{}, &model.Log{},
	))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 7, Username: "usage-user", Password: "test-password", AffCode: "usage-aff", Status: common.UserStatusEnabled, Quota: 1_000,
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
		Id: 31, PoolID: 1, Provider: model.SearchUpstreamProviderTikHub, Name: "usage-account",
		BaseURL: "https://api.tikhub.io", SecretCiphertext: "ciphertext", SecretNonce: "nonce",
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
	assert.Equal(t, "usage-key", adminItem.KeyName)
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
	assert.Equal(t, "usage-key", userPayload.Data.Items[0].KeyName)
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

func TestUnresolvedSearchUsageDoesNotReportRealizedRevenue(t *testing.T) {
	seedSearchUsageLogControllerTest(t)
	require.NoError(t, model.CreateSearchUsageEvent(&model.SearchUsageEvent{
		RequestID: "usage-request-unresolved", UserID: 7, EnterpriseID: 11, AgentKeyID: 21, UpstreamAccountID: 31,
		ServiceID: "vr_search", ServiceName: "Usage Search", Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusIndeterminate, HTTPStatus: 504, LatencyMs: 42,
		ExecutionPhase: model.SearchUsagePhaseDispatching, BillingState: model.SearchUsageBillingReserved,
		ReservedQuota: 125, PlannedUpstreamCostMicros: 100_000, PlannedChargeMicros: 250_000,
		UpstreamCostMicros: 100_000, ChargeMicros: 250_000,
	}))
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs?range=30&status=indeterminate", nil)
	writeSearchLogs(c, true)

	var payload struct {
		Data struct {
			Items []searchLogResponse `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	item := payload.Data.Items[0]
	assert.Zero(t, item.ChargeMicros)
	assert.Zero(t, item.UpstreamCostMicros)
	assert.Zero(t, item.ProfitMicros)
	assert.Equal(t, int64(250_000), item.PlannedChargeMicros)
	assert.Equal(t, model.SearchUsageBillingReserved, item.BillingState)

	csvRecorder := httptest.NewRecorder()
	csvContext, _ := gin.CreateTestContext(csvRecorder)
	csvContext.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs/export?range=30&status=indeterminate", nil)
	AdminExportSearchUsageLogs(csvContext)
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(csvRecorder.Body.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, formatSearchMoney(0), rows[1][10])
	assert.Equal(t, formatSearchMoney(0), rows[1][11])
	assert.Equal(t, formatSearchMoney(0), rows[1][12])
}

func TestAdminSearchUsageShowsKnownVendorCostOnRefundedFailure(t *testing.T) {
	seedSearchUsageLogControllerTest(t)
	require.NoError(t, model.CreateSearchUsageEvent(&model.SearchUsageEvent{
		RequestID: "usage-request-contract-failure", UserID: 7, EnterpriseID: 11, AgentKeyID: 21, UpstreamAccountID: 31,
		ServiceID: "vr_search", ServiceName: "Usage Search", Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusFailed, HTTPStatus: 502, LatencyMs: 42,
		ExecutionPhase: model.SearchUsagePhaseDispatching, BillingState: model.SearchUsageBillingRefunded,
		UpstreamCostMicros: 100_000, ChargeMicros: 0,
	}))
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs?range=30&status=failed", nil)
	writeSearchLogs(c, true)
	var payload struct {
		Data struct {
			Items []searchLogResponse `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	assert.Equal(t, int64(100_000), payload.Data.Items[0].UpstreamCostMicros)
	assert.Zero(t, payload.Data.Items[0].ChargeMicros)
	assert.Equal(t, int64(-100_000), payload.Data.Items[0].ProfitMicros)

	csvRecorder := httptest.NewRecorder()
	csvContext, _ := gin.CreateTestContext(csvRecorder)
	csvContext.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs/export?range=30&status=failed", nil)
	AdminExportSearchUsageLogs(csvContext)
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(csvRecorder.Body.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, formatSearchMoney(100_000), rows[1][10])
	assert.Equal(t, formatSearchMoney(0), rows[1][11])
	assert.Equal(t, formatSearchMoney(-100_000), rows[1][12])
}

func TestAdminSearchUsageReconciliationEndpointSettlesOnceAndAuditsDecision(t *testing.T) {
	seedSearchUsageLogControllerTest(t)
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = previousLogConsumeEnabled })
	const requestID = "usage-request-manual-settle"
	require.NoError(t, model.PreConsumeUserWallet(requestID, 7, 125))
	event := &model.SearchUsageEvent{
		RequestID: requestID, UserID: 7, EnterpriseID: 11, AgentKeyID: 21, UpstreamAccountID: 31,
		ServiceID: "vr_search", ServiceName: "Usage Search", Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusIndeterminate, HTTPStatus: 504, LatencyMs: 42,
		ExecutionPhase: model.SearchUsagePhaseDispatching, BillingState: model.SearchUsageBillingReserved,
		BillingSource: "wallet", ReservedQuota: 125,
		PlannedUpstreamCostMicros: 100_000, PlannedChargeMicros: 250_000,
		ChargeMicros: 250_000, ErrorCode: "VSEARCH_EXECUTION_INDETERMINATE",
	}
	require.NoError(t, model.CreateSearchUsageEvent(event))
	digest := sha256.Sum256([]byte("controller-reconciliation-key"))
	requestHash := strings.Repeat("c", 64)
	now := common.GetTimestamp()
	idempotency, state, err := model.BeginSearchExecutionIdempotency(21, hex.EncodeToString(digest[:]), requestHash, now, now+86_400)
	require.NoError(t, err)
	require.Equal(t, model.SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, model.AttachSearchExecutionUsage(idempotency.Id, requestHash, idempotency.ClaimToken, requestID))

	invoke := func(action, note string) (*httptest.ResponseRecorder, searchUsageReconciliationResponse) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set("id", 99)
		c.Set("username", "root-operator")
		c.Set("role", common.RoleRootUser)
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(event.Id, 10)}}
		body := `{"action":"` + action + `","note":"` + note + `"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/search/admin/usage-logs/1/reconcile", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		AdminReconcileSearchUsage(c)
		var payload struct {
			Data searchUsageReconciliationResponse `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		return recorder, payload.Data
	}

	firstRecorder, first := invoke(model.SearchUsageReconciliationSettle, "upstream invoice confirmed")
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	assert.True(t, first.Started)
	assert.Equal(t, "indeterminate", first.Status)
	assert.Equal(t, model.SearchUsageBillingCommitted, first.BillingState)
	assert.Equal(t, model.SearchUsageReconciliationSettle, first.ReconciliationAction)
	assert.Equal(t, 99, first.ReconciledBy)
	assert.Equal(t, "upstream invoice confirmed", first.ReconciliationNote)

	secondRecorder, second := invoke(model.SearchUsageReconciliationSettle, "retry")
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	assert.False(t, second.Started)
	var auditCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ? AND content LIKE ?", model.LogTypeManage, "Reconciled vSearch usage%").Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount)

	conflictRecorder, _ := invoke(model.SearchUsageReconciliationRefund, "opposite")
	assert.Equal(t, http.StatusConflict, conflictRecorder.Code)
	invalidRecorder, _ := invoke(model.SearchUsageReconciliationSettle, "")
	assert.Equal(t, http.StatusBadRequest, invalidRecorder.Code)
}
