package model

import (
	"database/sql"
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

type legacySearchCapabilityAvailabilityMigrationFixture struct {
	Id                 int    `gorm:"primaryKey;autoIncrement"`
	PublicID           string `gorm:"type:varchar(32);uniqueIndex"`
	Name               string `gorm:"type:varchar(128);not null"`
	Category           string `gorm:"type:varchar(64);not null"`
	Description        string `gorm:"type:text"`
	InputSchema        string `gorm:"type:text"`
	SchemaStatus       int    `gorm:"type:int;not null"`
	Status             int    `gorm:"type:int;not null;index"`
	UpstreamCostMicros int64  `gorm:"not null"`
	PriceMicros        int64  `gorm:"not null"`
	LastSyncedAt       int64  `gorm:"index"`
	CreatedAt          int64
	UpdatedAt          int64
}

func (legacySearchCapabilityAvailabilityMigrationFixture) TableName() string {
	return "search_capabilities"
}

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
		BaseURL:          "https://api.tikhub.io",
		SecretCiphertext: "ciphertext",
		SecretNonce:      "nonce",
		SecretVersion:    1,
		SecretPrefix:     "ak_live_••••",
		Status:           SearchUpstreamAccountStatusHealthy,
	}
	require.NoError(t, CreateSearchUpstreamAccount(account))
	assert.Equal(t, SearchUpstreamProviderTikHub, account.Provider)
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

func TestSearchCapabilityPriceFloorIgnoresEnabledBindingWithMismatchedSchema(t *testing.T) {
	openSearchDataTestDB(t)
	// NOCASE reproduces MySQL's common _ci TEXT comparison so exact schema matching must happen in Go.
	require.NoError(t, DB.Migrator().DropTable(&SearchCapabilityBinding{}))
	require.NoError(t, DB.Exec(`CREATE TABLE search_capability_bindings (
		id integer PRIMARY KEY AUTOINCREMENT,
		capability_id integer NOT NULL,
		upstream_account_id integer NOT NULL,
		tool_name text NOT NULL,
		input_schema text COLLATE NOCASE,
		status integer NOT NULL,
		weight integer NOT NULL,
		priority integer NOT NULL,
		upstream_cost_micros integer NOT NULL,
		last_synced_at integer,
		created_at integer,
		updated_at integer,
		CONSTRAINT idx_search_capability_binding UNIQUE (capability_id, upstream_account_id, tool_name)
	)`).Error)
	require.NoError(t, DB.AutoMigrate(&SearchCapabilityBinding{}))
	publicID, err := GenerateSearchCapabilityPublicID("private/schema-price-floor")
	require.NoError(t, err)
	capability := &SearchCapability{
		PublicID: publicID, Name: "Schema Price Floor", Category: "web-search",
		InputSchema: `{"type":"object"}`, Status: SearchCapabilityStatusEnabled,
		UpstreamCostMicros: 50, PriceMicros: 50,
	}
	require.NoError(t, CreateSearchCapability(capability))
	require.NoError(t, UpsertSearchCapabilityBinding(&SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: 1, ToolName: "private/schema-price-floor",
		InputSchema: capability.InputSchema, Status: SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 200,
	}))
	require.NoError(t, UpsertSearchCapabilityBinding(&SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: 2, ToolName: "private/schema-price-floor",
		InputSchema: `{"TYPE":"OBJECT"}`,
		Status:      SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 900,
	}))

	priceFloor, err := GetSearchCapabilityPriceFloor(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(200), priceFloor)
	require.NoError(t, RefreshSearchCapabilityPriceFloor(capability.Id))
	stored, err := GetSearchCapabilityByID(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(200), stored.UpstreamCostMicros)
	assert.Equal(t, int64(200), stored.PriceMicros)
}

func TestSearchCapabilityAutoMigrationPreservesLegacyAvailabilityState(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&SearchCapabilityGrant{}, &SearchCapabilityBinding{}, &SearchCapability{}))
	require.NoError(t, db.AutoMigrate(&legacySearchCapabilityAvailabilityMigrationFixture{}))
	require.NoError(t, db.Create(&legacySearchCapabilityAvailabilityMigrationFixture{
		PublicID: "vr_svc_legacy0000001", Name: "Legacy", Category: "搜索", Description: "legacy",
		InputSchema: `{"type":"object"}`, SchemaStatus: SearchCapabilitySchemaAvailable,
		Status: SearchCapabilityStatusDisabled, UpstreamCostMicros: 100, PriceMicros: 250,
		LastSyncedAt: 1, CreatedAt: 1, UpdatedAt: 1,
	}).Error)

	require.NoError(t, db.AutoMigrate(&SearchCapability{}))
	var stored SearchCapability
	require.NoError(t, db.Where("public_id = ?", "vr_svc_legacy0000001").First(&stored).Error)
	assert.Equal(t, SearchCapabilityAvailabilityLegacyPreserved, stored.AvailabilitySource)
	assert.Equal(t, SearchCapabilityStatusDisabled, stored.Status)
	assert.Equal(t, int64(250), stored.PriceMicros)
}

func TestRefreshSearchCapabilityPriceFloorReadsAndUpdatesInOneTransaction(t *testing.T) {
	openSearchDataTestDB(t)
	publicID, err := GenerateSearchCapabilityPublicID("private/transactional-price-floor")
	require.NoError(t, err)
	capability := &SearchCapability{
		PublicID: publicID, Name: "Transactional Price Floor", Category: "web-search",
		InputSchema: `{"type":"object"}`, Status: SearchCapabilityStatusEnabled,
		UpstreamCostMicros: 50, PriceMicros: 50,
	}
	require.NoError(t, CreateSearchCapability(capability))
	require.NoError(t, UpsertSearchCapabilityBinding(&SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: 1, ToolName: "private/transactional-price-floor",
		InputSchema: capability.InputSchema, Status: SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 200,
	}))

	operations := make([]string, 0, 3)
	var transaction *sql.Tx
	recordOperation := func(operation string, tx *gorm.DB) {
		sqlTx, inTransaction := tx.Statement.ConnPool.(*sql.Tx)
		if !inTransaction {
			operations = append(operations, operation+":outside")
			return
		}
		if transaction == nil {
			transaction = sqlTx
		}
		if transaction != sqlTx {
			operations = append(operations, operation+":different")
			return
		}
		operations = append(operations, operation+":same")
	}
	const queryCallback = "test:capture_price_floor_transaction_queries"
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "search_capabilities":
			recordOperation("capability-read", tx)
		case "search_capability_bindings":
			recordOperation("bindings-read", tx)
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(queryCallback) })
	const updateCallback = "test:capture_price_floor_transaction_update"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(updateCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "search_capabilities" {
			recordOperation("capability-update", tx)
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Update().Remove(updateCallback) })

	require.NoError(t, RefreshSearchCapabilityPriceFloor(capability.Id))
	assert.Equal(t, []string{
		"capability-read:same",
		"bindings-read:same",
		"capability-update:same",
	}, operations)
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

func TestSearchUsageStatExcludesUnrealizedReservations(t *testing.T) {
	openSearchDataTestDB(t)
	realized := &SearchUsageEvent{
		RequestID: "vsearch-realized-stat", UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		ServiceID: "vr_svc_realized", ServiceName: "Realized", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusSucceeded, HTTPStatus: 200,
		BillingState: SearchUsageBillingCommitted, UpstreamCostMicros: 100_000, ChargeMicros: 250_000,
	}
	unresolved := &SearchUsageEvent{
		RequestID: "vsearch-unresolved-stat", UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		ServiceID: "vr_svc_unresolved", ServiceName: "Unresolved", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusIndeterminate, ExecutionPhase: SearchUsagePhaseDispatching,
		BillingState: SearchUsageBillingReserved, ReservedQuota: 125,
		UpstreamCostMicros: 900_000, ChargeMicros: 1_500_000,
	}
	require.NoError(t, CreateSearchUsageEvent(realized))
	require.NoError(t, CreateSearchUsageEvent(unresolved))

	stat, err := GetSearchUsageStat(SearchUsageQuery{UserID: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(2), stat.RequestCount)
	assert.Equal(t, int64(1), stat.IndeterminateCount)
	assert.Equal(t, int64(100_000), stat.UpstreamCostMicros)
	assert.Equal(t, int64(250_000), stat.ChargeMicros)
	assert.Equal(t, int64(150_000), stat.MarginMicros)
}

func TestSearchUsageStatIncludesKnownVendorCostOnRefundedFailure(t *testing.T) {
	openSearchDataTestDB(t)
	event := &SearchUsageEvent{
		RequestID: "vsearch-known-vendor-loss", UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		ServiceID: "vr_svc_contract_failure", ServiceName: "Trend", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusFailed, HTTPStatus: 502, ExecutionPhase: SearchUsagePhaseDispatching,
		BillingState: SearchUsageBillingRefunded, UpstreamCostMicros: 100_000, ChargeMicros: 0,
	}
	require.NoError(t, CreateSearchUsageEvent(event))

	stat, err := GetSearchUsageStat(SearchUsageQuery{UserID: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stat.ErrorCount)
	assert.Equal(t, int64(100_000), stat.UpstreamCostMicros)
	assert.Zero(t, stat.ChargeMicros)
	assert.Equal(t, int64(-100_000), stat.MarginMicros)
}

func TestPreConsumeCleanupProtectsUnresolvedSearchReservations(t *testing.T) {
	openSearchDataTestDB(t)
	const protectedRequestID = "vsearch-protected-reservation"
	require.NoError(t, CreateSearchUsageEvent(&SearchUsageEvent{
		RequestID: protectedRequestID, UserID: 7, EnterpriseID: 11, AgentKeyID: 21,
		ServiceID: "vr_svc_unresolved", ServiceName: "Unresolved", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusIndeterminate, ExecutionPhase: SearchUsagePhaseDispatching,
		BillingState: SearchUsageBillingReserved, ReservedQuota: 125,
	}))
	require.NoError(t, DB.Create(&WalletPreConsumeRecord{
		RequestID: protectedRequestID, UserID: 7, PreConsumed: 125, Status: WalletPreConsumeStatusConsumed,
	}).Error)
	require.NoError(t, DB.Create(&WalletPreConsumeRecord{
		RequestID: "vsearch-deletable-wallet", UserID: 7, PreConsumed: 10, Status: WalletPreConsumeStatusRefunded,
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionPreConsumeRecord{
		RequestId: protectedRequestID, UserId: 7, UserSubscriptionId: 1, PreConsumed: 125, Status: "consumed",
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionPreConsumeRecord{
		RequestId: "vsearch-deletable-subscription", UserId: 7, UserSubscriptionId: 1, PreConsumed: 10, Status: "refunded",
	}).Error)
	staleAt := GetDBTimestamp() - 100
	require.NoError(t, DB.Model(&WalletPreConsumeRecord{}).Where("request_id IN ?", []string{protectedRequestID, "vsearch-deletable-wallet"}).Update("updated_at", staleAt).Error)
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id IN ?", []string{protectedRequestID, "vsearch-deletable-subscription"}).Update("updated_at", staleAt).Error)

	walletDeleted, err := CleanupWalletPreConsumeRecords(10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), walletDeleted)
	subscriptionDeleted, err := CleanupSubscriptionPreConsumeRecords(10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), subscriptionDeleted)

	var walletCount int64
	require.NoError(t, DB.Model(&WalletPreConsumeRecord{}).Where("request_id = ?", protectedRequestID).Count(&walletCount).Error)
	assert.Equal(t, int64(1), walletCount)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", protectedRequestID).Count(&subscriptionCount).Error)
	assert.Equal(t, int64(1), subscriptionCount)
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

func TestStaleDispatchedSearchUsageRetainsWalletReservation(t *testing.T) {
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
	assert.Equal(t, initialQuota-reservedQuota, user.Quota, "a potentially dispatched request must keep its reservation")
	var record WalletPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	assert.Equal(t, WalletPreConsumeStatusConsumed, record.Status)
	var recovered SearchUsageEvent
	require.NoError(t, DB.First(&recovered, event.Id).Error)
	assert.Equal(t, SearchUsageStatusIndeterminate, recovered.Status)
	assert.Equal(t, SearchUsageBillingReserved, recovered.BillingState)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", recovered.ErrorCode)
	assert.Zero(t, recovered.ChargeMicros)
	assert.Equal(t, int64(250_000), recovered.PlannedChargeMicros)
	assert.Equal(t, reservedQuota, recovered.ReservedQuota)
}

func seedIndeterminateWalletUsage(t *testing.T, userID, agentKeyID int, requestID, keyHash string) (*SearchUsageEvent, *SearchExecutionIdempotency) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: userID, Username: requestID + "-user", Password: "test-password",
		Status: common.UserStatusEnabled, Quota: 1_000,
	}).Error)
	require.NoError(t, PreConsumeUserWallet(requestID, userID, 125))
	event := &SearchUsageEvent{
		RequestID: requestID, UserID: userID, EnterpriseID: 11, AgentKeyID: agentKeyID,
		CapabilityID: 31, UpstreamAccountID: 41, ServiceID: "vr_manual_reconcile", ServiceName: "Manual Reconcile",
		Action: SearchUsageActionExecute, Status: SearchUsageStatusIndeterminate, HTTPStatus: 504,
		ExecutionPhase: SearchUsagePhaseDispatching, BillingState: SearchUsageBillingReserved,
		BillingSource: "wallet", ReservedQuota: 125,
		PlannedUpstreamCostMicros: 100_000, PlannedChargeMicros: 250_000,
		ChargeMicros: 250_000, ErrorCode: "VSEARCH_EXECUTION_INDETERMINATE",
	}
	require.NoError(t, CreateSearchUsageEvent(event))
	requestHash := strings.Repeat("e", 64)
	now := common.GetTimestamp()
	idempotency, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestHash, now, now+86_400)
	require.NoError(t, err)
	require.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, AttachSearchExecutionUsage(idempotency.Id, requestHash, idempotency.ClaimToken, requestID))
	return event, idempotency
}

func TestManualSearchUsageRefundIsAuditedIdempotentAndClosesIdempotencyKey(t *testing.T) {
	openSearchDataTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SearchExecutionIdempotency{}))
	event, idempotency := seedIndeterminateWalletUsage(t, 72, 22, "vsearch-manual-refund", strings.Repeat("f", 64))

	resolved, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationRefund, 9001, "upstream confirmed no charge")
	require.NoError(t, err)
	assert.True(t, started)
	assert.Equal(t, SearchUsageStatusIndeterminate, resolved.Status)
	assert.Equal(t, SearchUsageBillingRefunded, resolved.BillingState)
	assert.Equal(t, SearchUsageReconciliationRefund, resolved.ReconciliationAction)
	assert.Equal(t, 9001, resolved.ReconciledBy)
	assert.Positive(t, resolved.ReconciledAt)
	assert.Equal(t, "upstream confirmed no charge", resolved.ReconciliationNote)
	assert.Zero(t, resolved.ChargeMicros)

	replayed, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationRefund, 9002, "retry must preserve the first audit decision")
	require.NoError(t, err)
	assert.False(t, started)
	assert.Equal(t, 9001, replayed.ReconciledBy)
	assert.Equal(t, "upstream confirmed no charge", replayed.ReconciliationNote)
	require.ErrorIs(t, func() error {
		_, _, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationSettle, 9001, "opposite decision")
		return err
	}(), ErrSearchUsageReconciliationConflict)

	var user User
	require.NoError(t, DB.First(&user, 72).Error)
	assert.Equal(t, 1_000, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	var wallet WalletPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", event.RequestID).First(&wallet).Error)
	assert.Equal(t, WalletPreConsumeStatusRefunded, wallet.Status)
	require.NoError(t, DB.First(idempotency, idempotency.Id).Error)
	assert.Equal(t, SearchExecutionIdempotencyStatusResolved, idempotency.Status)
}

func TestManualSearchUsageRefundAcceptsNullReconciliationAction(t *testing.T) {
	openSearchDataTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SearchExecutionIdempotency{}))
	event, _ := seedIndeterminateWalletUsage(t, 76, 26, "vsearch-manual-refund-null-action", strings.Repeat("d", 64))
	require.NoError(t, DB.Model(&SearchUsageEvent{}).Where("id = ?", event.Id).
		UpdateColumn("reconciliation_action", nil).Error)

	resolved, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationRefund, 9007, "refund legacy null action")
	require.NoError(t, err)
	assert.True(t, started)
	assert.Equal(t, SearchUsageBillingRefunded, resolved.BillingState)
	assert.Equal(t, SearchUsageReconciliationRefund, resolved.ReconciliationAction)
}

func TestSearchUsageRefundFailureIsVisibleForAutomaticAndManualRecovery(t *testing.T) {
	openSearchDataTestDB(t)
	tests := []struct {
		name                 string
		status               int
		reconciliationAction string
	}{
		{name: "automatic pending recovery", status: SearchUsageStatusPending},
		{name: "manual indeterminate refund", status: SearchUsageStatusIndeterminate, reconciliationAction: SearchUsageReconciliationRefund},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := &SearchUsageEvent{
				RequestID: fmt.Sprintf("vsearch-missing-reservation-%d", index), UserID: 700 + index,
				EnterpriseID: 11, AgentKeyID: 21, ServiceID: "vr_refund_failure", ServiceName: "Refund Failure",
				Action: SearchUsageActionExecute, Status: test.status, ExecutionPhase: SearchUsagePhaseDispatching,
				BillingState: SearchUsageBillingRefundPending, BillingSource: "wallet", ReservedQuota: 125,
				ReconciliationAction: test.reconciliationAction,
			}
			require.NoError(t, CreateSearchUsageEvent(event))
			require.Error(t, reconcileSearchUsageRefund(event))
			require.NoError(t, DB.First(event, event.Id).Error)
			assert.Equal(t, SearchUsageBillingRefundFailed, event.BillingState)
		})
	}
}

func TestManualSearchUsageRefundRetriesTheSameDecisionAfterCompensationFailure(t *testing.T) {
	openSearchDataTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SearchExecutionIdempotency{}))
	event, _ := seedIndeterminateWalletUsage(t, 74, 24, "vsearch-manual-refund-retry", strings.Repeat("b", 64))
	callbackName := "test:fail_manual_vsearch_wallet_refund"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "wallet_pre_consume_records" {
			tx.AddError(errors.New("forced wallet refund failure"))
		}
	}))

	_, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationRefund, 9005, "refund after provider review")
	require.Error(t, err)
	assert.True(t, started)
	require.NoError(t, DB.First(event, event.Id).Error)
	assert.Equal(t, SearchUsageBillingRefundFailed, event.BillingState)
	require.NoError(t, DB.Callback().Update().Remove(callbackName))

	resolved, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationRefund, 9005, "refund after provider review")
	require.NoError(t, err)
	assert.False(t, started)
	assert.Equal(t, SearchUsageBillingRefunded, resolved.BillingState)
	var user User
	require.NoError(t, DB.First(&user, 74).Error)
	assert.Equal(t, 1_000, user.Quota)
}

func TestManualSearchUsageSettlementRealizesMoneyWithoutInventingSuccess(t *testing.T) {
	openSearchDataTestDB(t)
	previousLogDB := LOG_DB
	previousLogConsumeEnabled := common.LogConsumeEnabled
	LOG_DB = DB
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})
	require.NoError(t, DB.AutoMigrate(&Log{}, &SearchExecutionIdempotency{}))
	event, idempotency := seedIndeterminateWalletUsage(t, 73, 23, "vsearch-manual-settle", strings.Repeat("a", 64))

	resolved, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationSettle, 9003, "upstream confirmed a billable request")
	require.NoError(t, err)
	assert.True(t, started)
	assert.Equal(t, SearchUsageStatusIndeterminate, resolved.Status)
	assert.Equal(t, SearchUsagePhaseDispatching, resolved.ExecutionPhase)
	assert.Equal(t, SearchUsageBillingCommitted, resolved.BillingState)
	assert.Equal(t, SearchUsageReconciliationSettle, resolved.ReconciliationAction)
	assert.Equal(t, int64(100_000), resolved.UpstreamCostMicros)
	assert.Equal(t, int64(250_000), resolved.ChargeMicros)
	assert.True(t, IsSearchUsageFinanciallyRealized(resolved))

	replayed, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationSettle, 9004, "replay")
	require.NoError(t, err)
	assert.False(t, started)
	assert.Equal(t, 9003, replayed.ReconciledBy)
	var user User
	require.NoError(t, DB.First(&user, 73).Error)
	assert.Equal(t, 875, user.Quota, "manual settlement keeps the existing reservation")
	assert.Equal(t, 125, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	var consumeLogCount int64
	require.NoError(t, DB.Model(&Log{}).Where("request_id = ? AND type = ?", event.RequestID, LogTypeConsume).Count(&consumeLogCount).Error)
	assert.Equal(t, int64(1), consumeLogCount)
	stat, err := GetSearchUsageStat(SearchUsageQuery{UserID: 73})
	require.NoError(t, err)
	assert.Zero(t, stat.SuccessCount)
	assert.Equal(t, int64(1), stat.IndeterminateCount)
	assert.Equal(t, int64(250_000), stat.ChargeMicros)
	assert.Equal(t, int64(100_000), stat.UpstreamCostMicros)
	require.NoError(t, DB.First(idempotency, idempotency.Id).Error)
	assert.Equal(t, SearchExecutionIdempotencyStatusResolved, idempotency.Status)
}

func TestManualSearchUsageSettlementRetriesTheSameDecisionAfterLogFailure(t *testing.T) {
	openSearchDataTestDB(t)
	previousLogDB := LOG_DB
	previousLogConsumeEnabled := common.LogConsumeEnabled
	LOG_DB = DB
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})
	require.NoError(t, DB.AutoMigrate(&Log{}, &SearchExecutionIdempotency{}))
	event, _ := seedIndeterminateWalletUsage(t, 75, 25, "vsearch-manual-settle-retry", strings.Repeat("c", 64))
	callbackName := "test:fail_manual_vsearch_consume_log"
	require.NoError(t, LOG_DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			tx.AddError(errors.New("forced consume log failure"))
		}
	}))

	_, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationSettle, 9006, "settle after provider invoice")
	require.Error(t, err)
	assert.True(t, started)
	require.NoError(t, DB.First(event, event.Id).Error)
	assert.Equal(t, SearchUsageBillingLogWriting, event.BillingState)
	require.NoError(t, LOG_DB.Callback().Create().Remove(callbackName))
	require.NoError(t, DB.Model(&SearchUsageEvent{}).Where("id = ?", event.Id).
		Update("updated_at", common.GetTimestamp()-SearchUsagePendingTimeoutSeconds-1).Error)

	resolved, started, err := ReconcileIndeterminateSearchUsage(event.Id, SearchUsageReconciliationSettle, 9006, "settle after provider invoice")
	require.NoError(t, err)
	assert.False(t, started)
	assert.Equal(t, SearchUsageStatusIndeterminate, resolved.Status)
	assert.Equal(t, SearchUsageBillingCommitted, resolved.BillingState)
	var user User
	require.NoError(t, DB.First(&user, 75).Error)
	assert.Equal(t, 125, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	var logCount int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("request_id = ? AND type = ?", event.RequestID, LogTypeConsume).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
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
