package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDashboardPerfMetricsTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.PerfMetric{}, &model.EnterprisePerfMetric{}))
	for _, enterprise := range []model.Enterprise{
		{Id: 11, Name: "Alpha", Code: "alpha-perf", Status: model.EnterpriseStatusEnabled},
		{Id: 12, Name: "Beta", Code: "beta-perf", Status: model.EnterpriseStatusDisabled},
	} {
		require.NoError(t, db.Create(&enterprise).Error)
	}
	bucketTs := time.Now().Add(-time.Hour).Unix()
	require.NoError(t, db.Create(&model.PerfMetric{
		ModelName: "gpt-dashboard", Group: "default", BucketTs: bucketTs,
		RequestCount: 10, SuccessCount: 8, TotalLatencyMs: 2_300,
	}).Error)
	for _, metric := range []model.EnterprisePerfMetric{
		{EnterpriseID: 11, ModelName: "gpt-dashboard", Group: "default", BucketTs: bucketTs, RequestCount: 3, SuccessCount: 3, TotalLatencyMs: 600},
		{EnterpriseID: 12, ModelName: "gpt-dashboard", Group: "default", BucketTs: bucketTs, RequestCount: 5, SuccessCount: 4, TotalLatencyMs: 1_500},
	} {
		require.NoError(t, db.Create(&metric).Error)
	}
}

func performDashboardPerfMetricsRequest(t *testing.T, role int, query string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", role)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/performance?"+query, nil)
	GetDashboardPerfMetricsSummary(ctx)
	return recorder
}

func decodeDashboardPerfMetricsResponse(t *testing.T, recorder *httptest.ResponseRecorder) perfmetrics.SummaryAllResult {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                         `json:"success"`
		Message string                       `json:"message"`
		Data    perfmetrics.SummaryAllResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	return payload.Data
}

func TestDashboardPerfMetricsEnterpriseScopeValidation(t *testing.T) {
	setupDashboardPerfMetricsTest(t)

	tests := []struct {
		name       string
		role       int
		query      string
		wantStatus int
	}{
		{name: "admin cannot select enterprise", role: common.RoleAdminUser, query: "enterprise_id=11", wantStatus: http.StatusForbidden},
		{name: "enterprise must be numeric", role: common.RoleRootUser, query: "enterprise_id=invalid", wantStatus: http.StatusBadRequest},
		{name: "enterprise must be positive", role: common.RoleRootUser, query: "enterprise_id=0", wantStatus: http.StatusBadRequest},
		{name: "enterprise must exist", role: common.RoleRootUser, query: "enterprise_id=999", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := performDashboardPerfMetricsRequest(t, test.role, test.query)
			assert.Equal(t, test.wantStatus, recorder.Code)
		})
	}
}

func TestDashboardPerfMetricsReturnsSelectedEnterpriseAndGlobalHistory(t *testing.T) {
	setupDashboardPerfMetricsTest(t)

	tenantResult := decodeDashboardPerfMetricsResponse(t,
		performDashboardPerfMetricsRequest(t, common.RoleRootUser, "enterprise_id=11&hours=2"))
	require.Len(t, tenantResult.Models, 1)
	assert.Equal(t, int64(200), tenantResult.Models[0].AvgLatencyMs)
	assert.Equal(t, 100.0, tenantResult.Models[0].SuccessRate)

	disabledTenantResult := decodeDashboardPerfMetricsResponse(t,
		performDashboardPerfMetricsRequest(t, common.RoleRootUser, "enterprise_id=12&hours=2"))
	require.Len(t, disabledTenantResult.Models, 1, "disabled enterprises retain access to historical dashboard data")
	assert.Equal(t, int64(300), disabledTenantResult.Models[0].AvgLatencyMs)
	assert.Equal(t, 80.0, disabledTenantResult.Models[0].SuccessRate)

	globalResult := decodeDashboardPerfMetricsResponse(t,
		performDashboardPerfMetricsRequest(t, common.RoleAdminUser, "hours=2"))
	require.Len(t, globalResult.Models, 1)
	assert.Equal(t, int64(230), globalResult.Models[0].AvgLatencyMs)
	assert.Equal(t, 80.0, globalResult.Models[0].SuccessRate)
}
