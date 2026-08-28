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

type legacyUserSubscriptionWithoutQuotaVersion struct {
	Id            int
	UserId        int
	PlanId        int
	AmountTotal   int64
	AmountUsed    int64
	StartTime     int64
	EndTime       int64
	Status        string
	LastResetTime int64
	NextResetTime int64
	CreatedAt     int64
	UpdatedAt     int64
}

func (legacyUserSubscriptionWithoutQuotaVersion) TableName() string {
	return "user_subscriptions"
}

type legacySubscriptionPreConsumeWithoutQuotaVersion struct {
	Id                 int
	RequestId          string `gorm:"type:varchar(64);uniqueIndex"`
	UserId             int
	UserSubscriptionId int
	PreConsumed        int64
	Status             string
	CreatedAt          int64
	UpdatedAt          int64
}

func (legacySubscriptionPreConsumeWithoutQuotaVersion) TableName() string {
	return "subscription_pre_consume_records"
}

func openSubscriptionQuotaVersionMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		assert.NoError(t, sqlDB.Close())
	})
	return db
}

func migrateSubscriptionQuotaVersionForTest(t *testing.T, db *gorm.DB) subscriptionQuotaVersionMigrationState {
	t.Helper()
	state := inspectSubscriptionQuotaVersionMigration()
	require.NoError(t, db.AutoMigrate(&UserSubscription{}, &SubscriptionPreConsumeRecord{}))
	require.NoError(t, finalizeSubscriptionQuotaVersionMigration(state))
	return state
}

func TestSubscriptionQuotaVersionFirstMigrationEstablishesBaseline(t *testing.T) {
	db := openSubscriptionQuotaVersionMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&legacyUserSubscriptionWithoutQuotaVersion{}))
	legacy := &legacyUserSubscriptionWithoutQuotaVersion{
		UserId: 201, PlanId: 1, AmountTotal: 1_000, AmountUsed: 165,
		StartTime: 100, EndTime: 10_000, Status: "active", CreatedAt: 100, UpdatedAt: 200,
	}
	require.NoError(t, db.Create(legacy).Error)

	state := migrateSubscriptionQuotaVersionForTest(t, db)
	assert.True(t, state.userSubscriptionTableExisted)
	assert.False(t, state.userSubscriptionColumnExisted)
	assert.False(t, state.preConsumeTableExisted)

	var subscription UserSubscription
	require.NoError(t, db.First(&subscription, legacy.Id).Error)
	assert.Equal(t, subscriptionQuotaInitialVersion, subscription.QuotaResetVersion)
	freshSubscription := &UserSubscription{
		UserId: 204, PlanId: 1, AmountTotal: 1_000, StartTime: 100, EndTime: 10_000, Status: "active",
	}
	require.NoError(t, db.Create(freshSubscription).Error)
	assert.Equal(t, subscriptionQuotaInitialVersion, freshSubscription.QuotaResetVersion)
	record := &SubscriptionPreConsumeRecord{
		RequestId: "post-migration-current-period", UserId: subscription.UserId,
		UserSubscriptionId: subscription.Id, PreConsumed: 125,
		QuotaResetVersion: subscription.QuotaResetVersion, Status: "consumed",
	}
	require.NoError(t, db.Create(record).Error)
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, db.First(&subscription, subscription.Id).Error)
	assert.Equal(t, int64(40), subscription.AmountUsed, "post-migration same-period records must remain refundable")
}

func TestSubscriptionQuotaVersionMigrationFailsSafeForHistoricalConsumedRecords(t *testing.T) {
	db := openSubscriptionQuotaVersionMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&legacyUserSubscriptionWithoutQuotaVersion{},
		&legacySubscriptionPreConsumeWithoutQuotaVersion{},
	))
	legacy := &legacyUserSubscriptionWithoutQuotaVersion{
		UserId: 202, PlanId: 1, AmountTotal: 1_000, AmountUsed: 40,
		StartTime: 100, EndTime: 10_000, Status: "active", CreatedAt: 100, UpdatedAt: 200,
	}
	require.NoError(t, db.Create(legacy).Error)
	legacyRecord := &legacySubscriptionPreConsumeWithoutQuotaVersion{
		RequestId: "pre-migration-ambiguous", UserId: legacy.UserId,
		UserSubscriptionId: legacy.Id, PreConsumed: 125, Status: "consumed",
		CreatedAt: 150, UpdatedAt: 150,
	}
	require.NoError(t, db.Create(legacyRecord).Error)

	state := migrateSubscriptionQuotaVersionForTest(t, db)
	assert.True(t, state.userSubscriptionTableExisted)
	assert.False(t, state.userSubscriptionColumnExisted)
	assert.True(t, state.preConsumeTableExisted)
	assert.False(t, state.preConsumeColumnExisted)

	var subscription UserSubscription
	require.NoError(t, db.First(&subscription, legacy.Id).Error)
	assert.Equal(t, subscriptionQuotaInitialVersion, subscription.QuotaResetVersion)
	var record SubscriptionPreConsumeRecord
	require.NoError(t, db.First(&record, legacyRecord.Id).Error)
	assert.Zero(t, record.QuotaResetVersion)
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, db.First(&subscription, subscription.Id).Error)
	assert.Equal(t, int64(40), subscription.AmountUsed, "ambiguous historical refunds must not reduce current-period usage")
	require.NoError(t, db.First(&record, record.Id).Error)
	assert.Equal(t, "refunded", record.Status)
}

func TestSubscriptionQuotaVersionMigrationFailsSafeForIntermediateSchema(t *testing.T) {
	db := openSubscriptionQuotaVersionMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&UserSubscription{}, &SubscriptionPreConsumeRecord{}))
	subscription := &UserSubscription{
		UserId: 203, PlanId: 1, AmountTotal: 1_000, AmountUsed: 40,
		StartTime: 100, EndTime: 10_000, Status: "active",
	}
	require.NoError(t, db.Create(subscription).Error)
	require.NoError(t, db.Model(&UserSubscription{}).Where("id = ?", subscription.Id).
		UpdateColumn("quota_reset_version", 0).Error)
	record := &SubscriptionPreConsumeRecord{
		RequestId: "intermediate-zero-version", UserId: subscription.UserId,
		UserSubscriptionId: subscription.Id, PreConsumed: 125, QuotaResetVersion: 0, Status: "consumed",
	}
	require.NoError(t, db.Create(record).Error)

	state := migrateSubscriptionQuotaVersionForTest(t, db)
	assert.True(t, state.userSubscriptionColumnExisted)
	assert.True(t, state.preConsumeColumnExisted)
	require.NoError(t, db.First(subscription, subscription.Id).Error)
	assert.Equal(t, subscriptionQuotaInitialVersion, subscription.QuotaResetVersion)
	require.NoError(t, RefundSubscriptionPreConsume(record.RequestId))
	require.NoError(t, db.First(subscription, subscription.Id).Error)
	assert.Equal(t, int64(40), subscription.AmountUsed)
}
