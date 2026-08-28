package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSearchDataTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previous := DB
	DB = db
	t.Cleanup(func() {
		DB = previous
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		assert.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(
		&User{},
		&SearchUpstreamPool{},
		&SearchUpstreamAccount{},
		&SearchAgentKey{},
		&SearchCapability{},
		&SearchCapabilityBinding{},
		&SearchCapabilityGrant{},
		&SearchUsageEvent{},
		&WalletPreConsumeRecord{},
		&SubscriptionPreConsumeRecord{},
		&UserSubscription{},
	))
	require.NoError(t, ensureSubscriptionPlanTableSQLite())
	return db
}

func TestSearchUpstreamPoolAndAccountLifecycle(t *testing.T) {
	openSearchDataTestDB(t)
	pool := &SearchUpstreamPool{Name: "default"}
	require.NoError(t, CreateSearchUpstreamPool(pool))
	assert.Equal(t, SearchUpstreamPoolStrategyWeighted, pool.Strategy)
	assert.Equal(t, SearchUpstreamPoolStatusEnabled, pool.Status)

	account := &SearchUpstreamAccount{
		PoolID:           pool.Id,
		Name:             "primary",
		BaseURL:          "https://api.agentkey.app/v1/mcp",
		SecretCiphertext: "ciphertext",
		SecretNonce:      "nonce",
		SecretVersion:    1,
		SecretPrefix:     "ak_live_••••",
		Status:           SearchUpstreamAccountStatusHealthy,
	}
	require.NoError(t, CreateSearchUpstreamAccount(account))
	assert.Equal(t, SearchUpstreamProviderAgentKeyMCP, account.Provider)
	assert.Equal(t, 1, account.Weight)

	available, err := ListAvailableSearchUpstreamAccounts(pool.Id)
	require.NoError(t, err)
	require.Len(t, available, 1)
	assert.Equal(t, account.Id, available[0].Id)
	require.ErrorContains(t, DeleteSearchUpstreamPool(pool.Id), "still has accounts")

	require.NoError(t, UpdateSearchUpstreamAccountHealth(account.Id, SearchUpstreamAccountStatusWarning, 123_000_000, 1, "UPSTREAM_RATE_LIMITED", "sanitized"))
	stored, err := GetSearchUpstreamAccountByID(account.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(123_000_000), stored.BalanceMicros)
	assert.Equal(t, SearchUpstreamAccountStatusWarning, stored.Status)
	assert.Positive(t, stored.LastCheckedAt)

	missingPool := *pool
	missingPool.Id = 999
	assert.ErrorIs(t, UpdateSearchUpstreamPool(&missingPool), gorm.ErrRecordNotFound)
	missingAccount := *account
	missingAccount.Id = 999
	assert.ErrorIs(t, UpdateSearchUpstreamAccount(&missingAccount), gorm.ErrRecordNotFound)
}

func TestSearchUpstreamAccountJSONNeverExposesEncryptedSecret(t *testing.T) {
	account := &SearchUpstreamAccount{
		Id:               9,
		Name:             "primary",
		SecretCiphertext: "sensitive-ciphertext",
		SecretNonce:      "sensitive-nonce",
		SecretVersion:    1,
		SecretPrefix:     "ak_live_abc1••••",
	}

	payload, err := common.Marshal(account)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "sensitive-ciphertext")
	assert.NotContains(t, string(payload), "sensitive-nonce")
	assert.NotContains(t, string(payload), "secret_version")
	assert.Contains(t, string(payload), "ak_live_abc1••••")
}

func TestSearchCapabilityDiscoveryPreservesAdminConfiguration(t *testing.T) {
	openSearchDataTestDB(t)
	publicID, err := GenerateSearchCapabilityPublicID("private/provider-tool")
	require.NoError(t, err)
	capability := &SearchCapability{
		PublicID:           publicID,
		Name:               "Search",
		Category:           "web-search",
		InputSchema:        `{"type":"object"}`,
		Status:             SearchCapabilityStatusEnabled,
		UpstreamCostMicros: 100,
		PriceMicros:        500,
	}
	require.NoError(t, CreateSearchCapability(capability))

	discovered := &SearchCapability{
		PublicID:           publicID,
		Name:               "Search updated",
		Category:           "research",
		Description:        "dynamic description",
		InputSchema:        `{"type":"object","required":["q"]}`,
		Status:             SearchCapabilityStatusDisabled,
		UpstreamCostMicros: 250,
		PriceMicros:        999,
	}
	require.NoError(t, UpsertDiscoveredSearchCapability(discovered))
	stored, err := GetSearchCapabilityByPublicID(publicID)
	require.NoError(t, err)
	assert.Equal(t, "Search updated", stored.Name)
	assert.Equal(t, int64(250), stored.UpstreamCostMicros)
	assert.Equal(t, SearchCapabilityStatusEnabled, stored.Status, "sync must not disable configured capability")
	assert.Equal(t, int64(500), stored.PriceMicros, "sync must not overwrite configured price")

	binding := &SearchCapabilityBinding{
		CapabilityID:       stored.Id,
		UpstreamAccountID:  9,
		ToolName:           "private/provider-tool",
		UpstreamCostMicros: 250,
	}
	require.NoError(t, UpsertSearchCapabilityBinding(binding))
	binding.UpstreamCostMicros = 300
	require.NoError(t, UpsertSearchCapabilityBinding(binding))
	bindings, err := ListSearchCapabilityBindings(stored.Id, true)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, int64(300), bindings[0].UpstreamCostMicros)

	granted, err := IsSearchCapabilityGranted(stored.Id, 11, 7)
	require.NoError(t, err)
	assert.True(t, granted, "capability without explicit grants is public")
	require.NoError(t, ReplaceSearchCapabilityGrants(stored.Id, []SearchCapabilityGrant{{EnterpriseID: 11}}))
	granted, err = IsSearchCapabilityGranted(stored.Id, 11, 7)
	require.NoError(t, err)
	assert.True(t, granted)
	granted, err = IsSearchCapabilityGranted(stored.Id, 12, 8)
	require.NoError(t, err)
	assert.False(t, granted)
	require.NoError(t, ReplaceSearchCapabilityGrants(stored.Id, []SearchCapabilityGrant{{UserID: 7}}))
	granted, err = IsSearchCapabilityGranted(stored.Id, 12, 8)
	require.NoError(t, err)
	assert.True(t, granted, "user-specific rows do not change enterprise capability access")
}

func TestSearchUsageEventsKeepExactMicrosAndTenantIsolation(t *testing.T) {
	openSearchDataTestDB(t)
	events := []*SearchUsageEvent{
		{UserID: 7, EnterpriseID: 11, AgentKeyID: 21, CapabilityID: 31, ServiceID: "vr_svc_one", ServiceName: "Search", Action: SearchUsageActionExecute, Status: SearchUsageStatusSucceeded, HTTPStatus: 200, LatencyMs: 120, UpstreamCostMicros: 200_001, ChargeMicros: 350_003},
		{UserID: 7, EnterpriseID: 11, AgentKeyID: 21, CapabilityID: 31, ServiceID: "vr_svc_one", ServiceName: "Search", Action: SearchUsageActionExecute, Status: SearchUsageStatusFailed, HTTPStatus: 502, LatencyMs: 80, ErrorCode: "UPSTREAM_SERVICE_ERROR", SanitizedErrorMessage: "temporary failure"},
		{UserID: 8, EnterpriseID: 12, AgentKeyID: 22, CapabilityID: 32, ServiceID: "vr_svc_two", ServiceName: "Extract", Action: SearchUsageActionExecute, Status: SearchUsageStatusSucceeded, HTTPStatus: 200, LatencyMs: 50, UpstreamCostMicros: 10, ChargeMicros: 20},
	}
	for _, event := range events {
		require.NoError(t, CreateSearchUsageEvent(event))
		assert.NotEmpty(t, event.RequestID)
	}

	listed, total, err := ListSearchUsageEvents(SearchUsageQuery{UserID: 7, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, listed, 2)

	listed, total, err = ListSearchUsageEvents(SearchUsageQuery{SearchText: "SEARCH", Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "search text must be case-insensitive across supported databases")
	require.Len(t, listed, 2)

	listed, total, err = ListSearchUsageEvents(SearchUsageQuery{SearchText: "%", Limit: 20})
	require.NoError(t, err)
	assert.Zero(t, total, "SQL wildcard characters must be treated as literal search text")
	assert.Empty(t, listed)

	stat, err := GetSearchUsageStat(SearchUsageQuery{EnterpriseID: 11})
	require.NoError(t, err)
	assert.Equal(t, int64(2), stat.RequestCount)
	assert.Equal(t, int64(1), stat.SuccessCount)
	assert.Equal(t, int64(1), stat.ErrorCount)
	assert.Equal(t, int64(200), stat.TotalLatencyMs)
	assert.Equal(t, int64(200_001), stat.UpstreamCostMicros)
	assert.Equal(t, int64(350_003), stat.ChargeMicros)
	assert.Equal(t, int64(150_002), stat.MarginMicros)
}

func TestStalePendingSearchUsageBecomesIndeterminateWithoutInventingMoney(t *testing.T) {
	openSearchDataTestDB(t)
	now := common.GetTimestamp()
	event := &SearchUsageEvent{
		UserID: 7, EnterpriseID: 11, AgentKeyID: 21, CapabilityID: 31,
		ServiceID: "vr_svc_pending", ServiceName: "Search", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusPending, CreatedAt: now - SearchUsagePendingTimeoutSeconds - 1,
		LatencyMs:                 999,
		PlannedUpstreamCostMicros: 17, PlannedChargeMicros: 29,
		ExecutionPhase: SearchUsagePhaseDispatching, BillingState: SearchUsageBillingReserved,
	}
	require.NoError(t, CreateSearchUsageEvent(event))
	require.NoError(t, DB.Model(&SearchUsageEvent{}).Where("id = ?", event.Id).Update("updated_at", event.CreatedAt).Error)

	listed, total, err := ListSearchUsageEvents(SearchUsageQuery{UserID: 7, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, listed, 1)
	assert.Equal(t, SearchUsageStatusPending, listed[0].Status, "read queries must not run global billing recovery")

	stat, err := GetSearchUsageStat(SearchUsageQuery{UserID: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stat.PendingCount)
	assert.Zero(t, stat.IndeterminateCount)

	require.NoError(t, ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	listed, total, err = ListSearchUsageEvents(SearchUsageQuery{UserID: 7, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, listed, 1)
	assert.Equal(t, SearchUsageStatusIndeterminate, listed[0].Status)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", listed[0].ErrorCode)
	assert.Equal(t, int64(17), listed[0].PlannedUpstreamCostMicros)
	assert.Equal(t, int64(29), listed[0].PlannedChargeMicros)
	assert.Zero(t, listed[0].UpstreamCostMicros)
	assert.Zero(t, listed[0].ChargeMicros)

	stat, err = GetSearchUsageStat(SearchUsageQuery{UserID: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stat.IndeterminateCount)
	assert.Zero(t, stat.SuccessCount)
	assert.Zero(t, stat.ErrorCount)
	assert.Zero(t, stat.TotalLatencyMs, "indeterminate latency must not inflate completed-request averages")
}

func TestSearchUsageQueriesDoNotDependOnConsumeLogDatabase(t *testing.T) {
	openSearchDataTestDB(t)
	event := &SearchUsageEvent{
		RequestID: "vsearch-log-db-independent-query", UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		CapabilityID: 31, ServiceID: "vr_svc_log_pending", ServiceName: "Search",
		Action: SearchUsageActionExecute, Status: SearchUsageStatusSucceeded, HTTPStatus: 200,
		ExecutionPhase: SearchUsagePhaseCompleted, BillingState: SearchUsageBillingLogPending,
		ChargeMicros: 25_000,
	}
	require.NoError(t, CreateSearchUsageEvent(event))

	failedLogDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s_log?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := failedLogDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	previousLogDB := LOG_DB
	previousLogConsumeEnabled := common.LogConsumeEnabled
	LOG_DB = failedLogDB
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})

	events, total, err := ListSearchUsageEvents(SearchUsageQuery{UserID: 7, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	assert.Equal(t, SearchUsageBillingLogPending, events[0].BillingState)
	stat, err := GetSearchUsageStat(SearchUsageQuery{UserID: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stat.SuccessCount)

	var stored SearchUsageEvent
	require.NoError(t, DB.First(&stored, event.Id).Error)
	assert.Equal(t, SearchUsageBillingLogPending, stored.BillingState, "read queries must not mutate the billing outbox")
}

func TestPendingSearchUsageUsesLastUpdateForStaleness(t *testing.T) {
	openSearchDataTestDB(t)
	event := &SearchUsageEvent{
		UserID: 7, EnterpriseID: 11, AgentKeyID: 21, CapabilityID: 31,
		ServiceID: "vr_svc_active", ServiceName: "Search", Action: SearchUsageActionExecute,
		Status:         SearchUsageStatusPending,
		CreatedAt:      common.GetTimestamp() - SearchUsagePendingTimeoutSeconds - 1,
		ExecutionPhase: SearchUsagePhaseDispatching, BillingState: SearchUsageBillingReserved,
	}
	require.NoError(t, CreateSearchUsageEvent(event))
	require.NoError(t, ReconcileStaleSearchUsageEvents(common.GetTimestamp()))

	var stored SearchUsageEvent
	require.NoError(t, DB.First(&stored, event.Id).Error)
	assert.Equal(t, SearchUsageStatusPending, stored.Status, "an active request must not be recovered based only on its creation time")
	assert.Equal(t, SearchUsageBillingReserved, stored.BillingState)
}

func TestStaleReservedSearchUsageRefundsWalletOnce(t *testing.T) {
	openSearchDataTestDB(t)
	const (
		userID        = 71
		initialQuota  = 1_000
		reservedQuota = 125
		requestID     = "vsearch-stale-wallet-request"
	)
	require.NoError(t, DB.Create(&User{
		Id: userID, Username: "stale-wallet-user", Password: "test-password",
		Status: common.UserStatusEnabled, Quota: initialQuota,
	}).Error)
	event := &SearchUsageEvent{
		RequestID: requestID, UserID: userID, EnterpriseID: 11, AgentKeyID: 21, CapabilityID: 31,
		ServiceID: "vr_svc_pending_wallet", ServiceName: "Search", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusPending, CreatedAt: common.GetTimestamp() - SearchUsagePendingTimeoutSeconds - 1,
		ExecutionPhase: SearchUsagePhasePrepared, BillingState: SearchUsageBillingReservePending,
		PlannedChargeMicros: 250_000,
	}
	require.NoError(t, CreateSearchUsageEvent(event))
	require.NoError(t, PreConsumeUserWallet(requestID, userID, reservedQuota))
	require.NoError(t, PreConsumeUserWallet(requestID, userID, reservedQuota))
	require.NoError(t, MarkSearchUsageReservation(event, "wallet", reservedQuota))
	event.ExecutionPhase = SearchUsagePhaseDispatching
	require.NoError(t, UpdateSearchUsageEventProgress(event))
	staleAt := common.GetTimestamp() - SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, DB.Model(&SearchUsageEvent{}).Where("id = ?", event.Id).Update("updated_at", staleAt).Error)

	require.NoError(t, ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, ReconcileStaleSearchUsageEvents(common.GetTimestamp()))

	var user User
	require.NoError(t, DB.First(&user, userID).Error)
	assert.Equal(t, initialQuota, user.Quota, "stale recovery and replay must refund exactly once")
	var record WalletPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	assert.Equal(t, WalletPreConsumeStatusRefunded, record.Status)
	var recovered SearchUsageEvent
	require.NoError(t, DB.First(&recovered, event.Id).Error)
	assert.Equal(t, SearchUsageStatusIndeterminate, recovered.Status)
	assert.Equal(t, SearchUsageBillingRefunded, recovered.BillingState)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", recovered.ErrorCode)
	assert.Zero(t, recovered.ChargeMicros)
}

func TestSubscriptionRefundRecordAndQuotaRollbackTogether(t *testing.T) {
	openSearchDataTestDB(t)
	subscription := &UserSubscription{
		UserId: 81, PlanId: 1, AmountTotal: 1_000, AmountUsed: 125,
		Status: "active", StartTime: common.GetTimestamp() - 60, EndTime: common.GetTimestamp() + 60,
	}
	require.NoError(t, DB.Create(subscription).Error)
	record := &SubscriptionPreConsumeRecord{
		RequestId: "vsearch-subscription-refund", UserId: 81,
		UserSubscriptionId: subscription.Id, PreConsumed: 125,
		QuotaResetVersion: subscription.QuotaResetVersion, Status: "consumed",
	}
	require.NoError(t, DB.Create(record).Error)

	callbackName := "test:fail_subscription_refund_record"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updatedRecord, ok := tx.Statement.Dest.(*SubscriptionPreConsumeRecord)
		if tx.Statement.Table == "subscription_pre_consume_records" && ok && updatedRecord.Status == "refunded" {
			tx.AddError(errors.New("forced refund record failure"))
		}
	}))
	require.Error(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, DB.Callback().Update().Remove(callbackName))

	require.NoError(t, DB.First(subscription, subscription.Id).Error)
	assert.Equal(t, int64(125), subscription.AmountUsed, "failed ledger transition must roll back the quota refund")
	require.NoError(t, DB.First(record, record.Id).Error)
	assert.Equal(t, "consumed", record.Status)

	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, DB.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed, "retry and replay must apply one subscription refund")
	require.NoError(t, DB.First(record, record.Id).Error)
	assert.Equal(t, "refunded", record.Status)
}

func TestSubscriptionRefundDoesNotReduceUsageAfterQuotaReset(t *testing.T) {
	openSearchDataTestDB(t)
	now := common.GetTimestamp()
	plan := &SubscriptionPlan{
		Title: "vSearch period-safe refund", PriceAmount: 1, Currency: "CNY",
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
		TotalAmount: 1_000, QuotaResetPeriod: SubscriptionResetCustom, QuotaResetCustomSeconds: 60,
	}
	plan.NormalizeDefaults()
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	subscription := &UserSubscription{
		UserId: 91, PlanId: plan.Id, AmountTotal: 1_000, Status: "active",
		StartTime: now - 60, EndTime: now + 3_600, LastResetTime: now, NextResetTime: now + 60,
	}
	require.NoError(t, DB.Create(subscription).Error)

	oldPeriod, err := PreConsumeUserSubscription("vsearch-old-period", 91, "vsearch:test", 0, 125)
	require.NoError(t, err)
	assert.Equal(t, int64(125), oldPeriod.AmountUsedAfter)

	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]any{
		"last_reset_time": now - 61,
		"next_reset_time": now - 1,
	}).Error)

	newPeriod, err := PreConsumeUserSubscription("vsearch-new-period", 91, "vsearch:test", 0, 40)
	require.NoError(t, err)
	assert.Equal(t, int64(40), newPeriod.AmountUsedAfter)
	require.NoError(t, RefundSubscriptionPreConsume("vsearch-old-period"))

	require.NoError(t, DB.First(subscription, subscription.Id).Error)
	assert.Equal(t, int64(40), subscription.AmountUsed, "an old-period refund must not reduce new-period usage")
	assert.Equal(t, int64(2), subscription.QuotaResetVersion)
	var oldRecord SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "vsearch-old-period").First(&oldRecord).Error)
	assert.Equal(t, int64(1), oldRecord.QuotaResetVersion)
	assert.Equal(t, "refunded", oldRecord.Status)

	require.NoError(t, RefundSubscriptionPreConsume("vsearch-new-period"))
	require.NoError(t, DB.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed, "a current-period refund must still restore current-period quota")
}
