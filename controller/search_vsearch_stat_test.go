package controller

import (
	"encoding/json"
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

func openSearchUsageStatControllerDB(t *testing.T) {
	t.Helper()
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})
	require.NoError(t, db.AutoMigrate(&model.SearchUsageEvent{}))
}

func TestAdminSearchUsageStatReturnsExactMicros(t *testing.T) {
	openSearchUsageStatControllerDB(t)
	require.NoError(t, model.CreateSearchUsageEvent(&model.SearchUsageEvent{
		RequestID: "req-stat", UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		CapabilityID: 31, ServiceID: "vr_search", ServiceName: "Search",
		Action: model.SearchUsageActionExecute, Status: model.SearchUsageStatusSucceeded,
		HTTPStatus: 200, LatencyMs: 12, UpstreamCostMicros: 1, ChargeMicros: 3,
	}))
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/usage-logs/stat?range=30", nil)

	writeSearchLogStat(c, true)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                  `json:"success"`
		Data    searchLogStatResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, int64(3), payload.Data.QuotaMicros)
	assert.Equal(t, int64(3), payload.Data.RevenueMicros)
	assert.Equal(t, int64(1), payload.Data.UpstreamCostMicros)
	assert.Equal(t, int64(2), payload.Data.ProfitMicros)
}
