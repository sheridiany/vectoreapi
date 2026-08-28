package perfmetrics

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

func flushLoop() {
	for {
		interval := perf_metrics_setting.GetFlushIntervalMinutes()
		time.Sleep(time.Duration(interval) * time.Minute)
		setting := perf_metrics_setting.GetSetting()
		if !setting.Enabled {
			continue
		}
		flushCompletedBuckets()
		cleanupExpiredMetrics(setting.RetentionDays)
	}
}

func flushCompletedBuckets() {
	currentBucket := bucketStart(time.Now().Unix())
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			deleteFlushedBucketIfEmpty(key, bucket)
			return true
		}

		globalMetric := &model.PerfMetric{
			ModelName:      k.model,
			Group:          k.group,
			BucketTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
		}
		var enterpriseMetric *model.EnterprisePerfMetric
		if k.enterpriseID > 0 {
			enterpriseMetric = &model.EnterprisePerfMetric{
				EnterpriseID:   k.enterpriseID,
				ModelName:      k.model,
				Group:          k.group,
				BucketTs:       k.bucketTs,
				RequestCount:   drained.requestCount,
				SuccessCount:   drained.successCount,
				TotalLatencyMs: drained.totalLatencyMs,
				TtftSumMs:      drained.ttftSumMs,
				TtftCount:      drained.ttftCount,
				OutputTokens:   drained.outputTokens,
				GenerationMs:   drained.generationMs,
			}
		}
		err := model.UpsertPerfMetricBuckets(globalMetric, enterpriseMetric)
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush perf metric bucket enterprise=%d model=%s group=%s bucket=%d: %s", k.enterpriseID, k.model, k.group, k.bucketTs, err.Error()))
			return true
		}

		deleteFlushedBucketIfEmpty(key, bucket)
		return true
	})
}

func deleteFlushedBucketIfEmpty(rawKey any, bucket *atomicBucket) {
	if bucket.closeIfEmpty() {
		hotBuckets.CompareAndDelete(rawKey, bucket)
	}
}

func cleanupExpiredMetrics(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	if err := model.DeletePerfMetricsBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired perf metrics: " + err.Error())
	}
}
