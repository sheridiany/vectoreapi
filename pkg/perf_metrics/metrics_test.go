package perfmetrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPerfMetricsTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	previousPublisherID := redisPublisherID
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.RedisEnabled = false
	redisPublisherID = "test-" + strings.ReplaceAll(t.Name(), "/", "-")
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}, &model.EnterprisePerfMetric{}))
	clearHotBuckets()

	t.Cleanup(func() {
		clearHotBuckets()
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
		redisPublisherID = previousPublisherID
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			assert.NoError(t, sqlDB.Close())
		}
	})
	return db
}

func clearHotBuckets() {
	hotBuckets.Range(func(key, _ any) bool {
		hotBuckets.Delete(key)
		return true
	})
}

func usePerfMetricsRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	return server
}

func TestRecordRelaySampleKeepsEnterpriseBucketsIsolatedWithoutRedis(t *testing.T) {
	setupPerfMetricsTest(t)
	now := time.Now()
	RecordRelaySample(&relaycommon.RelayInfo{
		EnterpriseID:    11,
		OriginModelName: "gpt-enterprise",
		UsingGroup:      "default",
		StartTime:       now.Add(-200 * time.Millisecond),
	}, true, 20)

	tenantResult, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-enterprise", Hours: 1})
	require.NoError(t, err)
	require.Len(t, tenantResult.Groups, 1)
	assert.Equal(t, 100.0, tenantResult.Groups[0].SuccessRate)

	otherTenantResult, err := Query(QueryParams{EnterpriseID: 12, Model: "gpt-enterprise", Hours: 1})
	require.NoError(t, err)
	assert.Empty(t, otherTenantResult.Groups)

	globalResult, err := Query(QueryParams{Model: "gpt-enterprise", Hours: 1})
	require.NoError(t, err)
	require.Len(t, globalResult.Groups, 1)
	assert.Equal(t, 100.0, globalResult.Groups[0].SuccessRate)
}

func TestPerformanceQueriesUseGlobalAndEnterpriseDatabaseTables(t *testing.T) {
	db := setupPerfMetricsTest(t)
	bucketTs := bucketStart(time.Now().Add(-time.Hour).Unix())
	require.NoError(t, db.Create(&model.PerfMetric{
		ModelName: "gpt-enterprise", Group: "default", BucketTs: bucketTs,
		RequestCount: 10, SuccessCount: 8, TotalLatencyMs: 2_300,
	}).Error)
	require.NoError(t, db.Create(&model.EnterprisePerfMetric{
		EnterpriseID: 11, ModelName: "gpt-enterprise", Group: "default", BucketTs: bucketTs,
		RequestCount: 3, SuccessCount: 3, TotalLatencyMs: 600,
	}).Error)

	tenantResult, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-enterprise", Hours: 2})
	require.NoError(t, err)
	require.Len(t, tenantResult.Groups, 1)
	assert.Equal(t, int64(200), tenantResult.Groups[0].AvgLatencyMs)
	assert.Equal(t, 100.0, tenantResult.Groups[0].SuccessRate)

	globalResult, err := Query(QueryParams{Model: "gpt-enterprise", Hours: 2})
	require.NoError(t, err)
	require.Len(t, globalResult.Groups, 1)
	assert.Equal(t, int64(230), globalResult.Groups[0].AvgLatencyMs)
	assert.Equal(t, 80.0, globalResult.Groups[0].SuccessRate)

	tenantSummary, err := QuerySummaryAll(2, nil, 11)
	require.NoError(t, err)
	require.Len(t, tenantSummary.Models, 1)
	assert.EqualValues(t, 3, tenantSummary.Models[0].RequestCount)

	globalSummary, err := QuerySummaryAll(2, nil, 0)
	require.NoError(t, err)
	require.Len(t, globalSummary.Models, 1)
	assert.EqualValues(t, 10, globalSummary.Models[0].RequestCount)
}

func TestRedisCurrentBucketIsAuthoritativeAcrossProcesses(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)

	// Process A only contributes through shared Redis from Process B's point of view.
	redisPublisherID = "process-a"
	Record(Sample{EnterpriseID: 11, Model: "gpt-enterprise", Group: "default", LatencyMs: 100, Success: false})
	clearHotBuckets()
	// Process B retains local samples as well, which must not be added on top of Redis.
	redisPublisherID = "process-b"
	Record(Sample{EnterpriseID: 11, Model: "gpt-enterprise", Group: "default", LatencyMs: 300, Success: true})
	Record(Sample{EnterpriseID: 12, Model: "gpt-enterprise", Group: "default", LatencyMs: 500, Success: true})
	Record(Sample{EnterpriseID: 11, Model: "gpt-enterprise", Group: "vip", LatencyMs: 700, Success: false})

	firstTenant, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-enterprise", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, firstTenant.Groups, 1)
	assert.Equal(t, int64(200), firstTenant.Groups[0].AvgLatencyMs)
	assert.Equal(t, 50.0, firstTenant.Groups[0].SuccessRate)

	secondTenant, err := Query(QueryParams{EnterpriseID: 12, Model: "gpt-enterprise", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, secondTenant.Groups, 1)
	assert.Equal(t, int64(500), secondTenant.Groups[0].AvgLatencyMs)
	assert.Equal(t, 100.0, secondTenant.Groups[0].SuccessRate)

	global, err := Query(QueryParams{Model: "gpt-enterprise", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, global.Groups, 1)
	assert.Equal(t, int64(300), global.Groups[0].AvgLatencyMs)
	assert.InDelta(t, 66.67, global.Groups[0].SuccessRate, 0.01)

	tenantSummary, err := QuerySummaryAll(1, []string{"default"}, 11)
	require.NoError(t, err)
	require.Len(t, tenantSummary.Models, 1)
	assert.EqualValues(t, 2, tenantSummary.Models[0].RequestCount)
	assert.Equal(t, 50.0, tenantSummary.Models[0].SuccessRate)

	globalSummary, err := QuerySummaryAll(1, []string{"default"}, 0)
	require.NoError(t, err)
	require.Len(t, globalSummary.Models, 1)
	assert.EqualValues(t, 3, globalSummary.Models[0].RequestCount)
	assert.InDelta(t, 66.67, globalSummary.Models[0].SuccessRate, 0.01)

	bucketTs := bucketStart(time.Now().Unix())
	globalMembers, err := common.RDB.SMembers(context.Background(), redisBucketIndexKey(bucketTs, 0)).Result()
	require.NoError(t, err)
	assert.Len(t, globalMembers, 2, "global index cardinality must depend on model/group, not enterprise count")
	firstTenantMembers, err := common.RDB.SMembers(context.Background(), redisBucketIndexKey(bucketTs, 11)).Result()
	require.NoError(t, err)
	assert.Len(t, firstTenantMembers, 2)
	secondTenantMembers, err := common.RDB.SMembers(context.Background(), redisBucketIndexKey(bucketTs, 12)).Result()
	require.NoError(t, err)
	assert.Len(t, secondTenantMembers, 1)
}

func TestRedisBucketIndexUsesUnambiguousModelAndGroupEncoding(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	Record(Sample{EnterpriseID: 11, Model: "model:variant", Group: "default", LatencyMs: 100, Success: true})
	Record(Sample{EnterpriseID: 11, Model: "model", Group: "variant:default", LatencyMs: 300, Success: false})
	clearHotBuckets()

	first, err := Query(QueryParams{EnterpriseID: 11, Model: "model:variant", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, first.Groups, 1)
	assert.Equal(t, 100.0, first.Groups[0].SuccessRate)

	second, err := Query(QueryParams{EnterpriseID: 11, Model: "model", Group: "variant:default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, second.Groups, 1)
	assert.Equal(t, 0.0, second.Groups[0].SuccessRate)
}

func TestRedisFailureFallsBackToLocalCurrentBucket(t *testing.T) {
	setupPerfMetricsTest(t)
	server := usePerfMetricsRedis(t)
	Record(Sample{EnterpriseID: 11, Model: "gpt-fallback", Group: "default", LatencyMs: 120, Success: true})
	server.Close()

	result, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-fallback", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(120), result.Groups[0].AvgLatencyMs)
	assert.Equal(t, 100.0, result.Groups[0].SuccessRate)
}

func TestRedisQuerySkipsRepublishingCleanLocalBucket(t *testing.T) {
	setupPerfMetricsTest(t)
	server := usePerfMetricsRedis(t)
	Record(Sample{EnterpriseID: 11, Model: "gpt-clean", Group: "default", LatencyMs: 100, Success: true})
	key := bucketKey{
		enterpriseID: 11,
		model:        "gpt-clean",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	server.FastForward(5 * time.Minute)
	indexKey := redisBucketIndexKey(key.bucketTs, key.enterpriseID)
	beforeQueryTTL := server.TTL(indexKey)
	require.Positive(t, beforeQueryTTL)

	result, err := Query(QueryParams{EnterpriseID: 11, Model: key.model, Group: key.group, Hours: 1})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, beforeQueryTTL, server.TTL(indexKey), "a clean local bucket must not refresh Redis publication TTL")
}

func TestRedisQueryRetriesDirtySnapshotAfterPublishFailure(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	healthyClient := common.RDB

	unavailableServer := miniredis.RunT(t)
	unavailableAddress := unavailableServer.Addr()
	unavailableServer.Close()
	unavailableClient := redis.NewClient(&redis.Options{
		Addr:        unavailableAddress,
		DialTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { assert.NoError(t, unavailableClient.Close()) })
	common.RDB = unavailableClient
	Record(Sample{EnterpriseID: 11, Model: "gpt-dirty", Group: "default", LatencyMs: 180, Success: true})
	key := bucketKey{
		enterpriseID: 11,
		model:        "gpt-dirty",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	stored, ok := hotBuckets.Load(key)
	require.True(t, ok)
	assert.Zero(t, stored.(*atomicBucket).lastPublishedRequestCount.Load())

	common.RDB = healthyClient
	result, err := Query(QueryParams{EnterpriseID: 11, Model: key.model, Group: key.group, Hours: 1})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(180), result.Groups[0].AvgLatencyMs)
	assert.EqualValues(t, 1, stored.(*atomicBucket).lastPublishedRequestCount.Load())

	clearHotBuckets()
	sharedResult, err := Query(QueryParams{EnterpriseID: 11, Model: key.model, Group: key.group, Hours: 1})
	require.NoError(t, err)
	require.Len(t, sharedResult.Groups, 1)
	assert.Equal(t, int64(180), sharedResult.Groups[0].AvgLatencyMs)
}

func TestRedisTenantQueryDoesNotPublishAnotherTenantDirtyBucket(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	healthyClient := common.RDB
	Record(Sample{EnterpriseID: 11, Model: "gpt-scoped-sync", Group: "default", LatencyMs: 100, Success: true})

	unavailableServer := miniredis.RunT(t)
	unavailableAddress := unavailableServer.Addr()
	unavailableServer.Close()
	unavailableClient := redis.NewClient(&redis.Options{
		Addr:        unavailableAddress,
		DialTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { assert.NoError(t, unavailableClient.Close()) })
	common.RDB = unavailableClient
	Record(Sample{EnterpriseID: 12, Model: "gpt-scoped-sync", Group: "default", LatencyMs: 300, Success: false})
	dirtyKey := bucketKey{
		enterpriseID: 12,
		model:        "gpt-scoped-sync",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	dirtyValue, ok := hotBuckets.Load(dirtyKey)
	require.True(t, ok)
	assert.Zero(t, dirtyValue.(*atomicBucket).lastPublishedRequestCount.Load())

	common.RDB = healthyClient
	result, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-scoped-sync", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(100), result.Groups[0].AvgLatencyMs)
	assert.Zero(t, dirtyValue.(*atomicBucket).lastPublishedRequestCount.Load(), "tenant 11 query must not publish tenant 12 dirty state")
}

func TestRedisDirtySyncStopsAfterFirstInfrastructureError(t *testing.T) {
	setupPerfMetricsTest(t)
	bucketTs := bucketStart(time.Now().Unix())
	for enterpriseID := 11; enterpriseID <= 15; enterpriseID++ {
		key := bucketKey{
			enterpriseID: enterpriseID,
			model:        "gpt-bounded-sync",
			group:        "default",
			bucketTs:     bucketTs,
		}
		bucket := &atomicBucket{}
		bucket.add(Sample{LatencyMs: 100, Success: true})
		hotBuckets.Store(key, bucket)
	}
	attempts := 0
	assert.False(t, syncRedisActiveSnapshotsWithPublisher(bucketTs, redisBucketFilter{
		model: "gpt-bounded-sync",
		group: "default",
	}, func(bucketKey, *atomicBucket) error {
		attempts++
		return errors.New("redis unavailable")
	}))
	assert.Equal(t, 1, attempts, "the first Redis infrastructure error must stop dirty bucket synchronization")
}

func TestRedisSampleMissedDuringTransientFailureIsReplayedAfterRecovery(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	healthyClient := common.RDB

	unavailableServer := miniredis.RunT(t)
	unavailableAddress := unavailableServer.Addr()
	unavailableServer.Close()
	unavailableClient := redis.NewClient(&redis.Options{
		Addr:        unavailableAddress,
		DialTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { assert.NoError(t, unavailableClient.Close()) })
	common.RDB = unavailableClient
	Record(Sample{EnterpriseID: 11, Model: "gpt-recovery", Group: "default", LatencyMs: 100, Success: false})

	common.RDB = healthyClient
	Record(Sample{EnterpriseID: 11, Model: "gpt-recovery", Group: "default", LatencyMs: 300, Success: true})
	clearHotBuckets()

	result, err := Query(QueryParams{EnterpriseID: 11, Model: "gpt-recovery", Group: "default", Hours: 1})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(200), result.Groups[0].AvgLatencyMs)
	assert.Equal(t, 50.0, result.Groups[0].SuccessRate)

	global, err := QuerySummaryAll(1, nil, 0)
	require.NoError(t, err)
	require.Len(t, global.Models, 1)
	assert.EqualValues(t, 2, global.Models[0].RequestCount)
}

func TestRedisPublisherConcurrentSnapshotsPreserveCompleteSamples(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	redisPublisherID = "concurrent-publisher"
	key := bucketKey{
		enterpriseID: 11,
		model:        "gpt-concurrent-recovery",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	const sampleCount = 100
	var wg sync.WaitGroup
	for i := 1; i <= sampleCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			Record(Sample{
				EnterpriseID: key.enterpriseID,
				Model:        key.model,
				Group:        key.group,
				LatencyMs:    int64(i),
				TtftMs:       int64(i),
				HasTtft:      true,
				Success:      i%2 == 0,
				OutputTokens: 2,
				GenerationMs: 4,
			})
		}()
	}
	wg.Wait()

	buckets, authoritative := readRedisActiveBuckets(key.bucketTs, redisBucketFilter{
		enterpriseID: 11,
		model:        key.model,
		group:        key.group,
	})
	require.True(t, authoritative)
	value, ok := buckets[key]
	require.True(t, ok)
	assert.EqualValues(t, sampleCount, value.requestCount)
	assert.EqualValues(t, sampleCount/2, value.successCount)
	assert.EqualValues(t, 5_050, value.totalLatencyMs)
	assert.EqualValues(t, 5_050, value.ttftSumMs)
	assert.EqualValues(t, sampleCount, value.ttftCount)
	assert.EqualValues(t, sampleCount*2, value.outputTokens)
	assert.EqualValues(t, sampleCount*4, value.generationMs)
}

func TestRedisPublisherSnapshotRetryIsIdempotent(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	key := bucketKey{
		enterpriseID: 11,
		model:        "gpt-idempotent-retry",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	firstSnapshot := counters{
		requestCount: 2, successCount: 1, totalLatencyMs: 400,
		ttftSumMs: 100, ttftCount: 2, outputTokens: 20, generationMs: 200,
	}
	require.NoError(t, writeRedisBucket(key, firstSnapshot, "publisher-a"))
	require.NoError(t, writeRedisBucket(key, firstSnapshot, "publisher-a"), "an ambiguous retry must overwrite the publisher snapshot")
	latestSnapshot := counters{
		requestCount: 4, successCount: 3, totalLatencyMs: 800,
		ttftSumMs: 200, ttftCount: 4, outputTokens: 40, generationMs: 400,
	}
	require.NoError(t, writeRedisBucket(key, latestSnapshot, "publisher-a"))
	require.NoError(t, writeRedisBucket(key, firstSnapshot, "publisher-a"), "an out-of-order older snapshot must not replace the latest publisher state")
	require.NoError(t, writeRedisBucket(key, counters{
		requestCount: 3, successCount: 3, totalLatencyMs: 300,
		ttftSumMs: 60, ttftCount: 3, outputTokens: 30, generationMs: 300,
	}, "publisher-b"))

	buckets, authoritative := readRedisActiveBuckets(key.bucketTs, redisBucketFilter{
		enterpriseID: 11,
		model:        key.model,
		group:        key.group,
	})
	require.True(t, authoritative)
	value, ok := buckets[key]
	require.True(t, ok)
	assert.EqualValues(t, 7, value.requestCount)
	assert.EqualValues(t, 6, value.successCount)
	assert.EqualValues(t, 1_100, value.totalLatencyMs)
	assert.EqualValues(t, 260, value.ttftSumMs)
	assert.EqualValues(t, 7, value.ttftCount)
	assert.EqualValues(t, 70, value.outputTokens)
	assert.EqualValues(t, 700, value.generationMs)
}

func TestCompletedRedisBucketIsNotAddedAfterDatabaseFlush(t *testing.T) {
	setupPerfMetricsTest(t)
	usePerfMetricsRedis(t)
	previousBucket := bucketStart(time.Now().Unix()) - perf_metrics_setting.GetBucketSeconds()
	key := bucketKey{enterpriseID: 11, model: "gpt-flushed", group: "default", bucketTs: previousBucket}
	sample := Sample{EnterpriseID: 11, Model: "gpt-flushed", Group: "default", LatencyMs: 100, Success: true}
	bucket := &atomicBucket{}
	bucket.add(sample)
	hotBuckets.Store(key, bucket)
	require.NoError(t, publishRedisSnapshot(key, bucket))

	flushCompletedBuckets()

	global, err := QuerySummaryAll(2, nil, 0)
	require.NoError(t, err)
	require.Len(t, global.Models, 1)
	assert.EqualValues(t, 1, global.Models[0].RequestCount)
	tenant, err := QuerySummaryAll(2, nil, 11)
	require.NoError(t, err)
	require.Len(t, tenant.Models, 1)
	assert.EqualValues(t, 1, tenant.Models[0].RequestCount)
	_, exists := hotBuckets.Load(key)
	assert.False(t, exists, "a successfully flushed enterprise bucket must be released immediately")
}

func TestFlushReleasesCompletedEnterpriseBucketCardinality(t *testing.T) {
	db := setupPerfMetricsTest(t)
	previousBucket := bucketStart(time.Now().Unix()) - perf_metrics_setting.GetBucketSeconds()
	keys := make([]bucketKey, 0, 6)
	for enterpriseID := 11; enterpriseID <= 15; enterpriseID++ {
		key := bucketKey{
			enterpriseID: enterpriseID,
			model:        fmt.Sprintf("gpt-enterprise-%d", enterpriseID),
			group:        "default",
			bucketTs:     previousBucket,
		}
		bucket := &atomicBucket{}
		assert.True(t, bucket.add(Sample{LatencyMs: 100, Success: true}))
		hotBuckets.Store(key, bucket)
		keys = append(keys, key)
	}
	emptyKey := bucketKey{enterpriseID: 99, model: "gpt-empty", group: "default", bucketTs: previousBucket}
	hotBuckets.Store(emptyKey, &atomicBucket{})
	keys = append(keys, emptyKey)

	flushCompletedBuckets()

	for _, key := range keys {
		_, exists := hotBuckets.Load(key)
		assert.False(t, exists)
	}
	var tenantRows int64
	require.NoError(t, db.Model(&model.EnterprisePerfMetric{}).Count(&tenantRows).Error)
	assert.EqualValues(t, 5, tenantRows)
}

func TestRecordReplacesClosedBucketWithoutLosingSample(t *testing.T) {
	setupPerfMetricsTest(t)
	key := bucketKey{
		enterpriseID: 11,
		model:        "gpt-closed",
		group:        "default",
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	closed := &atomicBucket{}
	require.True(t, closed.closeIfEmpty())
	hotBuckets.Store(key, closed)

	Record(Sample{EnterpriseID: 11, Model: key.model, Group: key.group, LatencyMs: 120, Success: true})

	stored, ok := hotBuckets.Load(key)
	require.True(t, ok)
	assert.NotSame(t, closed, stored)
	assert.EqualValues(t, 1, stored.(*atomicBucket).snapshot().requestCount)
}
