package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSearchUpstreamProviderMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		initCol()
		assert.NoError(t, sqlDB.Close())
	})
	return db
}

func TestUnsupportedSearchUpstreamProvidersAreRetiredThroughDatabaseEntrypoints(t *testing.T) {
	tests := []struct {
		name    string
		migrate func() error
	}{
		{name: "standard", migrate: migrateDB},
		{name: "fast", migrate: migrateDBFast},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openSearchUpstreamProviderMigrationTestDB(t)
			require.NoError(t, db.AutoMigrate(&SearchUpstreamAccount{}, &SearchCapability{}, &SearchCapabilityBinding{}, &SearchUsageEvent{}))

			legacy := createProviderMigrationAccount(t, db, "agentkey_mcp", SearchUpstreamAccountStatusHealthy)
			unknown := createProviderMigrationAccount(t, db, "future_provider", SearchUpstreamAccountStatusWarning)
			empty := createProviderMigrationAccount(t, db, "", SearchUpstreamAccountStatusStandby)
			justOneAPI := createProviderMigrationAccount(t, db, SearchUpstreamProviderJustOneAPI, SearchUpstreamAccountStatusHealthy)
			tikHub := createProviderMigrationAccount(t, db, SearchUpstreamProviderTikHub, SearchUpstreamAccountStatusStandby)

			for index, account := range []*SearchUpstreamAccount{legacy, unknown, empty, justOneAPI, tikHub} {
				capability := &SearchCapability{
					Id: index + 1, PublicID: "vr_svc_provider_migration_" + string(rune('a'+index)),
					Name: "Provider migration", Category: "test", Status: SearchCapabilityStatusEnabled,
				}
				require.NoError(t, db.Create(capability).Error)
				require.NoError(t, db.Create(&SearchCapabilityBinding{
					CapabilityID:       index + 1,
					UpstreamAccountID:  account.Id,
					ToolName:           "provider-operation-" + account.Name,
					Status:             SearchCapabilityBindingStatusEnabled,
					Weight:             1,
					UpstreamCostMicros: 1,
				}).Error)
			}
			sharedCapability := &SearchCapability{
				Id: 6, PublicID: "vr_svc_provider_migration_shared", OperationKey: "provider.migration.shared", ContractVersion: "v1",
				Name: "Shared provider migration", Category: "test", Status: SearchCapabilityStatusEnabled,
			}
			require.NoError(t, db.Create(sharedCapability).Error)
			for _, account := range []*SearchUpstreamAccount{legacy, justOneAPI} {
				require.NoError(t, db.Create(&SearchCapabilityBinding{
					CapabilityID:       sharedCapability.Id,
					UpstreamAccountID:  account.Id,
					ToolName:           "shared-provider-operation-" + account.Name,
					Status:             SearchCapabilityBindingStatusEnabled,
					Weight:             1,
					UpstreamCostMicros: 1,
				}).Error)
			}
			usage := &SearchUsageEvent{
				RequestID: "provider-migration-history", UserID: 1, AgentKeyID: 1,
				UpstreamAccountID: legacy.Id, CapabilityID: 1,
				ServiceID: "vr_svc_provider_migration_a", ServiceName: "Retired provider history",
				Action: SearchUsageActionExecute, Status: SearchUsageStatusSucceeded, HTTPStatus: 200,
			}
			require.NoError(t, CreateSearchUsageEvent(usage))

			require.NoError(t, test.migrate())
			assertProviderMigrationState(t, db, legacy.Id, SearchUpstreamAccountStatusPaused, SearchCapabilityBindingStatusDisabled)
			assertProviderMigrationState(t, db, unknown.Id, SearchUpstreamAccountStatusPaused, SearchCapabilityBindingStatusDisabled)
			assertProviderMigrationState(t, db, empty.Id, SearchUpstreamAccountStatusPaused, SearchCapabilityBindingStatusDisabled)
			assertProviderMigrationState(t, db, justOneAPI.Id, SearchUpstreamAccountStatusHealthy, SearchCapabilityBindingStatusEnabled)
			assertProviderMigrationState(t, db, tikHub.Id, SearchUpstreamAccountStatusStandby, SearchCapabilityBindingStatusEnabled)
			assertCapabilityMigrationState(t, db, 1, SearchCapabilityStatusDisabled, true)
			assertCapabilityMigrationState(t, db, 2, SearchCapabilityStatusDisabled, true)
			assertCapabilityMigrationState(t, db, 3, SearchCapabilityStatusDisabled, true)
			assertCapabilityMigrationState(t, db, 4, SearchCapabilityStatusEnabled, false)
			assertCapabilityMigrationState(t, db, 5, SearchCapabilityStatusEnabled, false)
			assertCapabilityMigrationState(t, db, 6, SearchCapabilityStatusEnabled, false)
			assertCapabilityBindingMigrationStatus(t, db, sharedCapability.Id, legacy.Id, SearchCapabilityBindingStatusDisabled)
			assertCapabilityBindingMigrationStatus(t, db, sharedCapability.Id, justOneAPI.Id, SearchCapabilityBindingStatusEnabled)

			visibleCapabilities, err := ListSearchCapabilities(true)
			require.NoError(t, err)
			assert.ElementsMatch(t, []int{4, 5, 6}, capabilityIDs(visibleCapabilities))
			events, total, err := ListSearchUsageEvents(SearchUsageQuery{CapabilityID: 1})
			require.NoError(t, err)
			assert.EqualValues(t, 1, total)
			require.Len(t, events, 1)
			assert.Equal(t, usage.RequestID, events[0].RequestID)

			require.NoError(t, migrateUnsupportedSearchUpstreamProviders(), "migration must be idempotent")
			assertProviderMigrationState(t, db, legacy.Id, SearchUpstreamAccountStatusPaused, SearchCapabilityBindingStatusDisabled)
			assertProviderMigrationState(t, db, justOneAPI.Id, SearchUpstreamAccountStatusHealthy, SearchCapabilityBindingStatusEnabled)
			assertCapabilityMigrationState(t, db, 1, SearchCapabilityStatusDisabled, true)
			assertCapabilityMigrationState(t, db, 6, SearchCapabilityStatusEnabled, false)
		})
	}
}

func assertCapabilityMigrationState(t *testing.T, db *gorm.DB, capabilityID, status int, archived bool) {
	t.Helper()
	var capability SearchCapability
	require.NoError(t, db.Unscoped().First(&capability, capabilityID).Error)
	assert.Equal(t, status, capability.Status)
	assert.Equal(t, archived, capability.DeletedAt.Valid)
}

func assertCapabilityBindingMigrationStatus(t *testing.T, db *gorm.DB, capabilityID, accountID, status int) {
	t.Helper()
	var binding SearchCapabilityBinding
	require.NoError(t, db.Where("capability_id = ? AND upstream_account_id = ?", capabilityID, accountID).First(&binding).Error)
	assert.Equal(t, status, binding.Status)
}

func capabilityIDs(capabilities []*SearchCapability) []int {
	ids := make([]int, 0, len(capabilities))
	for _, capability := range capabilities {
		ids = append(ids, capability.Id)
	}
	return ids
}

func createProviderMigrationAccount(t *testing.T, db *gorm.DB, provider string, status int) *SearchUpstreamAccount {
	t.Helper()
	account := &SearchUpstreamAccount{
		PoolID:           1,
		Provider:         provider,
		Name:             "account-" + provider,
		BaseURL:          "https://provider.example",
		SecretCiphertext: "ciphertext",
		SecretNonce:      "nonce",
		SecretVersion:    1,
		SecretPrefix:     "secret-prefix",
		Weight:           1,
		Status:           status,
		LastErrorCode:    "preserved-code",
		LastErrorMessage: "preserved-message",
	}
	if provider == "" {
		account.Name = "account-empty-provider"
	}
	require.NoError(t, db.Create(account).Error)
	return account
}

func assertProviderMigrationState(t *testing.T, db *gorm.DB, accountID int, accountStatus int, bindingStatus int) {
	t.Helper()
	var account SearchUpstreamAccount
	require.NoError(t, db.First(&account, accountID).Error)
	assert.Equal(t, accountStatus, account.Status)
	if accountStatus == SearchUpstreamAccountStatusPaused {
		assert.Equal(t, retiredSearchUpstreamProviderCode, account.LastErrorCode)
		assert.Equal(t, "旧上游类型已停用，请改用 JustOneAPI 或 TikHub。", account.LastErrorMessage)
	} else {
		assert.Equal(t, "preserved-code", account.LastErrorCode)
		assert.Equal(t, "preserved-message", account.LastErrorMessage)
	}

	var binding SearchCapabilityBinding
	require.NoError(t, db.Where("upstream_account_id = ?", accountID).First(&binding).Error)
	assert.Equal(t, bindingStatus, binding.Status)
}
