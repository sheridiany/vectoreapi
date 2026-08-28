package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openPerfMetricTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		initCol()
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			assert.NoError(t, sqlDB.Close())
		}
	})
	return db
}

func verifyPerfMetricRollingCompatibility(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&PerfMetric{}))
	require.NoError(t, UpsertPerfMetric(&PerfMetric{
		ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000,
		RequestCount: 2, SuccessCount: 2, TotalLatencyMs: 200,
	}))

	require.NoError(t, db.AutoMigrate(&EnterprisePerfMetric{}))
	assert.True(t, db.Migrator().HasIndex(&PerfMetric{}, "idx_perf_model_group_bucket"))
	assert.True(t, db.Migrator().HasIndex(&EnterprisePerfMetric{}, "idx_enterprise_perf_model_group_bucket"))

	require.NoError(t, UpsertPerfMetricBuckets(
		&PerfMetric{ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000, RequestCount: 3, SuccessCount: 2},
		&EnterprisePerfMetric{EnterpriseID: 11, ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000, RequestCount: 3, SuccessCount: 2},
	))
	require.NoError(t, UpsertPerfMetricBuckets(
		&PerfMetric{ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000, RequestCount: 5, SuccessCount: 4},
		&EnterprisePerfMetric{EnterpriseID: 12, ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000, RequestCount: 5, SuccessCount: 4},
	))

	// Simulate an old replica continuing to flush after the new table exists.
	require.NoError(t, UpsertPerfMetric(&PerfMetric{
		ModelName: "gpt-enterprise", Group: "default", BucketTs: 1_700_000_000,
		RequestCount: 1, SuccessCount: 1,
	}))

	globalRows, err := GetPerfMetrics("gpt-enterprise", "default", 1_699_999_999, 1_700_000_001)
	require.NoError(t, err)
	require.Len(t, globalRows, 1)
	assert.EqualValues(t, 11, globalRows[0].RequestCount)
	assert.EqualValues(t, 9, globalRows[0].SuccessCount)

	firstTenantRows, err := GetEnterprisePerfMetrics(11, "gpt-enterprise", "default", 1_699_999_999, 1_700_000_001)
	require.NoError(t, err)
	require.Len(t, firstTenantRows, 1)
	assert.EqualValues(t, 3, firstTenantRows[0].RequestCount)
	secondTenantRows, err := GetEnterprisePerfMetrics(12, "gpt-enterprise", "default", 1_699_999_999, 1_700_000_001)
	require.NoError(t, err)
	require.Len(t, secondTenantRows, 1)
	assert.EqualValues(t, 5, secondTenantRows[0].RequestCount)
}

func TestPerfMetricDualTablesKeepLegacyReplicaCompatible(t *testing.T) {
	verifyPerfMetricRollingCompatibility(t, openPerfMetricTestDB(t))
}

func TestPerfMetricDualTableMigrationConfiguredDatabases(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		databaseType common.DatabaseType
		dialector    func(string) gorm.Dialector
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", databaseType: common.DatabaseTypeMySQL, dialector: func(dsn string) gorm.Dialector {
			return mysql.Open(dsn)
		}},
		{name: "postgres", env: "TEST_POSTGRES_DSN", databaseType: common.DatabaseTypePostgreSQL, dialector: func(dsn string) gorm.Dialector {
			return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.env))
			if dsn == "" {
				t.Skip(test.env + " is not configured")
			}

			previousDB, previousLogDB := DB, LOG_DB
			previousMainDatabaseType := common.MainDatabaseType()
			previousLogDatabaseType := common.LogDatabaseType()
			db, err := gorm.Open(test.dialector(dsn), &gorm.Config{})
			require.NoError(t, err)
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(test.databaseType, test.databaseType)
			initCol()
			managedTables := false
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() {
				if managedTables {
					assert.NoError(t, db.Migrator().DropTable(&EnterprisePerfMetric{}, &PerfMetric{}))
				}
				assert.NoError(t, sqlDB.Close())
				DB, LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
				initCol()
			})

			if db.Migrator().HasTable(&PerfMetric{}) || db.Migrator().HasTable(&EnterprisePerfMetric{}) {
				t.Skipf("refusing to run against %s because a performance metrics table already exists", test.env)
			}
			managedTables = true
			verifyPerfMetricRollingCompatibility(t, db)
		})
	}
}

func TestPerfMetricDualTableMigrationRunsThroughDatabaseEntrypoints(t *testing.T) {
	tests := []struct {
		name    string
		migrate func() error
	}{
		{name: "standard", migrate: migrateDB},
		{name: "fast", migrate: migrateDBFast},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openPerfMetricTestDB(t)
			require.NoError(t, db.AutoMigrate(&PerfMetric{}))
			require.NoError(t, UpsertPerfMetric(&PerfMetric{
				ModelName: "gpt-entrypoint", Group: "default", BucketTs: 1_700_000_001, RequestCount: 2,
			}))
			if test.name == "fast" {
				sqlDB, err := db.DB()
				require.NoError(t, err)
				sqlDB.SetMaxOpenConns(1)
			}

			require.NoError(t, test.migrate())
			assert.True(t, db.Migrator().HasIndex(&PerfMetric{}, "idx_perf_model_group_bucket"))
			assert.True(t, db.Migrator().HasTable(&EnterprisePerfMetric{}))
			require.NoError(t, UpsertPerfMetric(&PerfMetric{
				ModelName: "gpt-entrypoint", Group: "default", BucketTs: 1_700_000_001, RequestCount: 1,
			}))
			var historical PerfMetric
			require.NoError(t, db.Where("model_name = ?", "gpt-entrypoint").First(&historical).Error)
			assert.EqualValues(t, 3, historical.RequestCount)
		})
	}
}

func TestPerfMetricQueriesUseGlobalAndEnterpriseTables(t *testing.T) {
	db := openPerfMetricTestDB(t)
	require.NoError(t, db.AutoMigrate(&PerfMetric{}, &EnterprisePerfMetric{}))
	bucketTs := time.Now().Add(-time.Hour).Unix()
	require.NoError(t, db.Create(&PerfMetric{
		ModelName: "gpt-enterprise", Group: "default", BucketTs: bucketTs,
		RequestCount: 10, SuccessCount: 8, TotalLatencyMs: 2_300,
	}).Error)
	for _, metric := range []EnterprisePerfMetric{
		{EnterpriseID: 11, ModelName: "gpt-enterprise", Group: "default", BucketTs: bucketTs, RequestCount: 3, SuccessCount: 3, TotalLatencyMs: 450},
		{EnterpriseID: 12, ModelName: "gpt-enterprise", Group: "default", BucketTs: bucketTs, RequestCount: 5, SuccessCount: 4, TotalLatencyMs: 1_000},
	} {
		require.NoError(t, db.Create(&metric).Error)
	}

	globalRows, err := GetPerfMetrics("gpt-enterprise", "", bucketTs-1, bucketTs+1)
	require.NoError(t, err)
	require.Len(t, globalRows, 1)
	assert.EqualValues(t, 10, globalRows[0].RequestCount)

	tenantRows, err := GetEnterprisePerfMetrics(11, "gpt-enterprise", "", bucketTs-1, bucketTs+1)
	require.NoError(t, err)
	require.Len(t, tenantRows, 1)
	assert.EqualValues(t, 3, tenantRows[0].RequestCount)

	globalSummary, err := GetPerfMetricsSummaryBucketsAll(bucketTs-1, bucketTs+1, nil)
	require.NoError(t, err)
	require.Len(t, globalSummary, 1)
	assert.EqualValues(t, 10, globalSummary[0].RequestCount)

	tenantSummary, err := GetEnterprisePerfMetricsSummaryBucketsAll(11, bucketTs-1, bucketTs+1, nil)
	require.NoError(t, err)
	require.Len(t, tenantSummary, 1)
	assert.EqualValues(t, 3, tenantSummary[0].RequestCount)
}

func TestPerfMetricDualWriteRollsBackGlobalWhenEnterpriseWriteFails(t *testing.T) {
	db := openPerfMetricTestDB(t)
	require.NoError(t, db.AutoMigrate(&PerfMetric{}))
	global := &PerfMetric{
		ModelName: "gpt-atomic", Group: "default", BucketTs: 1_700_000_002,
		RequestCount: 3, SuccessCount: 2,
	}
	enterprise := &EnterprisePerfMetric{
		EnterpriseID: 11, ModelName: "gpt-atomic", Group: "default", BucketTs: 1_700_000_002,
		RequestCount: 3, SuccessCount: 2,
	}

	require.Error(t, UpsertPerfMetricBuckets(global, enterprise))
	var globalCount int64
	require.NoError(t, db.Model(&PerfMetric{}).Where("model_name = ?", "gpt-atomic").Count(&globalCount).Error)
	assert.Zero(t, globalCount, "the legacy aggregate must roll back when the tenant write fails")

	require.NoError(t, db.AutoMigrate(&EnterprisePerfMetric{}))
	require.NoError(t, UpsertPerfMetricBuckets(global, enterprise))
	globalRows, err := GetPerfMetrics("gpt-atomic", "default", 1_700_000_001, 1_700_000_003)
	require.NoError(t, err)
	require.Len(t, globalRows, 1)
	assert.EqualValues(t, 3, globalRows[0].RequestCount, "retry must not double the rolled-back legacy write")
	tenantRows, err := GetEnterprisePerfMetrics(11, "gpt-atomic", "default", 1_700_000_001, 1_700_000_003)
	require.NoError(t, err)
	require.Len(t, tenantRows, 1)
	assert.EqualValues(t, 3, tenantRows[0].RequestCount)
}

func TestDeletePerfMetricsBeforeCleansBothTables(t *testing.T) {
	db := openPerfMetricTestDB(t)
	require.NoError(t, db.AutoMigrate(&PerfMetric{}, &EnterprisePerfMetric{}))
	for _, metric := range []PerfMetric{
		{ModelName: "legacy-old", Group: "default", BucketTs: 100, RequestCount: 1},
		{ModelName: "legacy-current", Group: "default", BucketTs: 300, RequestCount: 1},
	} {
		require.NoError(t, db.Create(&metric).Error)
	}
	for _, metric := range []EnterprisePerfMetric{
		{EnterpriseID: 11, ModelName: "tenant-old", Group: "default", BucketTs: 100, RequestCount: 1},
		{EnterpriseID: 11, ModelName: "tenant-current", Group: "default", BucketTs: 300, RequestCount: 1},
	} {
		require.NoError(t, db.Create(&metric).Error)
	}

	require.NoError(t, DeletePerfMetricsBefore(200))
	var globalRows []PerfMetric
	require.NoError(t, db.Order("bucket_ts").Find(&globalRows).Error)
	require.Len(t, globalRows, 1)
	assert.Equal(t, "legacy-current", globalRows[0].ModelName)
	var tenantRows []EnterprisePerfMetric
	require.NoError(t, db.Order("bucket_ts").Find(&tenantRows).Error)
	require.Len(t, tenantRows, 1)
	assert.Equal(t, "tenant-current", tenantRows[0].ModelName)
}
