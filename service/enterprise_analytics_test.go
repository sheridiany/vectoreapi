package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEnterpriseAnalyticsTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_analytics_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.Log{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestEnterpriseAnalyticsReturnsDailyAndModelBreakdowns(t *testing.T) {
	db := setupEnterpriseAnalyticsTest(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-analytics")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	now := time.Now().Unix()
	start := now - 48*60*60
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, EnterpriseID: enterprise.Id, Username: "alice", ModelName: "gpt-5",
		CreatedAt: now - 24*60*60, Type: model.LogTypeConsume, Quota: 100,
		PromptTokens: 10, CompletionTokens: 20,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, EnterpriseID: enterprise.Id, Username: "alice", ModelName: "gpt-5",
		CreatedAt: now - 24*60*60 + 10, Type: model.LogTypeRefund, Quota: 25,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 102, EnterpriseID: enterprise.Id, Username: "bob", ModelName: "grok-4",
		CreatedAt: now - 100, Type: model.LogTypeConsume, Quota: 60,
		PromptTokens: 5, CompletionTokens: 5,
	}).Error)

	analytics, err := GetEnterpriseAnalytics(enterprise.Id, "custom", formatInt64(start), formatInt64(now))
	require.NoError(t, err)
	assert.Equal(t, enterprise.Id, analytics.EnterpriseID)
	assert.GreaterOrEqual(t, len(analytics.Daily), 2)
	require.Len(t, analytics.Models, 2)
	assert.Equal(t, "gpt-5", analytics.Models[0].ModelName)
	assert.Equal(t, int64(75), analytics.Models[0].NetQuota)
	assert.Equal(t, int64(60), analytics.Models[1].NetQuota)
}

func TestEnterpriseBudgetStatusWarnsAndClampsRemainingQuota(t *testing.T) {
	db := setupEnterpriseAnalyticsTest(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-budget")
	require.NoError(t, err)
	enterprise.MonthlyQuotaBudget = 100
	enterprise.BudgetAlertThreshold = 80
	require.NoError(t, db.Create(enterprise).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, EnterpriseID: enterprise.Id, CreatedAt: time.Now().Unix(),
		Type: model.LogTypeConsume, Quota: 90,
	}).Error)

	status, err := GetEnterpriseBudgetStatus(enterprise.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(90), status.ConsumedQuota)
	assert.Equal(t, int64(10), status.RemainingQuota)
	assert.Equal(t, "warning", status.AlertLevel)

	require.NoError(t, db.Model(&model.Log{}).Where("enterprise_id = ?", enterprise.Id).Update("quota", 120).Error)
	status, err = GetEnterpriseBudgetStatus(enterprise.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(0), status.RemainingQuota)
	assert.Equal(t, "exceeded", status.AlertLevel)
}

func TestEnterpriseAnalyticsRejectsRangesOverNinetyDays(t *testing.T) {
	setupEnterpriseAnalyticsTest(t)
	_, err := GetEnterpriseAnalytics(1, "custom", "1", formatInt64(91*24*60*60+2))
	require.Error(t, err)
}
