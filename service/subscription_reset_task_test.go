package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubscriptionMaintenanceContinuesWhenUsageLogRecoveryFails(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "maintenance.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SearchUsageEvent{},
	))
	failedLogDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "failed-log.db")), &gorm.Config{})
	require.NoError(t, err)
	failedSQLDB, err := failedLogDB.DB()
	require.NoError(t, err)
	require.NoError(t, failedSQLDB.Close())

	model.DB = db
	model.LOG_DB = failedLogDB
	common.LogConsumeEnabled = true
	subscriptionResetRunning.Store(false)
	subscriptionCleanupLast.Store(time.Now().Unix())
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.LogConsumeEnabled = previousLogConsumeEnabled
		subscriptionResetRunning.Store(false)
		assert.NoError(t, sqlDB.Close())
	})

	now := common.GetTimestamp()
	plan := &model.SubscriptionPlan{
		Title: "maintenance", PriceAmount: 1, Currency: "CNY", Enabled: true,
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1_000,
		QuotaResetPeriod: model.SubscriptionResetCustom, QuotaResetCustomSeconds: 60,
	}
	plan.NormalizeDefaults()
	require.NoError(t, db.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	subscription := &model.UserSubscription{
		UserId: 31, PlanId: plan.Id, AmountTotal: 1_000, AmountUsed: 250,
		StartTime: now - 3_600, EndTime: now + 3_600, Status: "active",
		LastResetTime: now - 120, NextResetTime: now - 60,
	}
	require.NoError(t, db.Create(subscription).Error)
	usage := &model.SearchUsageEvent{
		RequestID: "maintenance-log-recovery", UserID: 31, AgentKeyID: 41,
		ServiceID: "vsearch:test", ServiceName: "Search", Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusSucceeded, HTTPStatus: 200,
		ExecutionPhase: model.SearchUsagePhaseCompleted, BillingState: model.SearchUsageBillingLogPending,
	}
	require.NoError(t, model.CreateSearchUsageEvent(usage))

	runSubscriptionQuotaResetOnce()

	require.NoError(t, db.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed, "log outbox failure must not block subscription maintenance")
	assert.Equal(t, int64(2), subscription.QuotaResetVersion)
}
