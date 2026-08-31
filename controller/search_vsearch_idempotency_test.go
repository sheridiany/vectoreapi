package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExecuteSearchCapabilityReturnsHTTP409ForReusedIdempotencyKey(t *testing.T) {
	previousDB := model.DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, db.AutoMigrate(
		&model.SearchAgentKey{}, &model.SearchExecutionIdempotency{}, &model.SearchCapability{},
		&model.SearchCapabilityGrant{}, &model.SearchUsageEvent{},
	))

	key := &model.SearchAgentKey{
		UserId: 7, EnterpriseID: 11, Name: "dashboard", KeyHash: strings.Repeat("a", 64),
		KeyPrefix: "vr_live_test", Status: model.SearchAgentKeyStatusActive,
	}
	require.NoError(t, key.SetScopes(nil))
	require.NoError(t, db.Create(key).Error)
	idempotencyKey := "dashboard-idempotency-key"
	digest := sha256.Sum256([]byte(idempotencyKey))
	_, state, err := model.BeginSearchExecutionIdempotency(
		key.Id, hex.EncodeToString(digest[:]), strings.Repeat("b", 64), common.GetTimestamp(), common.GetTimestamp()+86_400,
	)
	require.NoError(t, err)
	require.Equal(t, model.SearchExecutionIdempotencyAcquired, state)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", key.UserId)
	c.Set("enterprise_id", key.EnterpriseID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/search/execute", bytes.NewBufferString(`{"service_id":"vr_svc_0123456789abcdef","params":{"query":"news"}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", idempotencyKey)

	ExecuteSearchCapability(c)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var payload struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "IDEMPOTENCY_KEY_REUSED", payload.Code)

	tooLongRecorder := httptest.NewRecorder()
	tooLongContext, _ := gin.CreateTestContext(tooLongRecorder)
	tooLongContext.Set("id", key.UserId)
	tooLongContext.Set("enterprise_id", key.EnterpriseID)
	tooLongContext.Request = httptest.NewRequest(http.MethodPost, "/api/search/execute", bytes.NewBufferString(`{"service_id":"vr_svc_0123456789abcdef","params":{"query":"news"}}`))
	tooLongContext.Request.Header.Set("Content-Type", "application/json")
	tooLongContext.Request.Header.Set("Idempotency-Key", strings.Repeat("x", 129))

	ExecuteSearchCapability(tooLongContext)

	require.Equal(t, http.StatusBadRequest, tooLongRecorder.Code)
	require.NoError(t, common.Unmarshal(tooLongRecorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "IDEMPOTENCY_KEY_TOO_LONG", payload.Code)

	capability := &model.SearchCapability{
		PublicID: "vr_svc_0123456789abcdef", Name: "Search", Category: "search",
		InputSchema: `{"type":"object","required":["query"],"properties":{"query":{"type":"string"}}}`,
		Status:      model.SearchCapabilityStatusEnabled,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	invalidParamsRecorder := httptest.NewRecorder()
	invalidParamsContext, _ := gin.CreateTestContext(invalidParamsRecorder)
	invalidParamsContext.Set("id", key.UserId)
	invalidParamsContext.Set("enterprise_id", key.EnterpriseID)
	invalidParamsContext.Request = httptest.NewRequest(http.MethodPost, "/api/search/execute", bytes.NewBufferString(`{"service_id":"vr_svc_0123456789abcdef","params":{"query":42}}`))
	invalidParamsContext.Request.Header.Set("Content-Type", "application/json")
	invalidParamsContext.Request.Header.Set("Idempotency-Key", "invalid-params-request")

	ExecuteSearchCapability(invalidParamsContext)

	require.Equal(t, http.StatusBadRequest, invalidParamsRecorder.Code)
	require.NoError(t, common.Unmarshal(invalidParamsRecorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "INVALID_TOOL_PARAMS", payload.Code)

	missingKeyRecorder := httptest.NewRecorder()
	missingKeyContext, _ := gin.CreateTestContext(missingKeyRecorder)
	missingKeyContext.Set("id", key.UserId)
	missingKeyContext.Set("enterprise_id", key.EnterpriseID)
	missingKeyContext.Request = httptest.NewRequest(http.MethodPost, "/api/search/execute", bytes.NewBufferString(`{"service_id":"vr_svc_0123456789abcdef","params":{"query":"news"}}`))
	missingKeyContext.Request.Header.Set("Content-Type", "application/json")

	ExecuteSearchCapability(missingKeyContext)

	require.Equal(t, http.StatusBadRequest, missingKeyRecorder.Code)
	require.NoError(t, common.Unmarshal(missingKeyRecorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Equal(t, "IDEMPOTENCY_KEY_REQUIRED", payload.Code)
}
