package perfmetrics

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var hotBuckets sync.Map
var redisPublisherID = uuid.NewString()
var publishRedisSnapshotScript = redis.NewScript(`
local versionField = "v:" .. ARGV[1]
local snapshotField = "s:" .. ARGV[1]
local incomingVersion = tonumber(ARGV[4])
for i = 1, #KEYS, 3 do
    local currentVersion = tonumber(redis.call("HGET", KEYS[i], versionField) or "-1")
    if incomingVersion >= currentVersion then
        redis.call("HSET", KEYS[i], versionField, ARGV[4], snapshotField, ARGV[3])
    end
    redis.call("SADD", KEYS[i + 1], ARGV[2])
    redis.call("SADD", KEYS[i + 2], ARGV[2])
    redis.call("EXPIRE", KEYS[i], ARGV[5])
    redis.call("EXPIRE", KEYS[i + 1], ARGV[5])
    redis.call("EXPIRE", KEYS[i + 2], ARGV[5])
end
return 1
`)

// seriesSchema is a stable client cache/schema marker. Do not change it when
// hiding fields or making response-only privacy hardening changes.
const seriesSchema = "dbcd0a3c01b55203"

func Init() {
	go flushLoop()
}

func RecordRelaySample(info *relaycommon.RelayInfo, success bool, outputTokens int64) {
	if info == nil {
		return
	}
	now := time.Now()
	hasTtft := info.IsStream && info.HasSendResponse()
	ttftMs := int64(0)
	if hasTtft {
		ttftMs = info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	}
	latencyMs := now.Sub(info.StartTime).Milliseconds()
	generationMs := latencyMs
	if hasTtft {
		generationMs = now.Sub(info.FirstResponseTime).Milliseconds()
	}
	if generationMs <= 0 {
		generationMs = latencyMs
	}
	Record(Sample{
		EnterpriseID: info.EnterpriseID,
		Model:        info.OriginModelName,
		Group:        info.UsingGroup,
		LatencyMs:    latencyMs,
		TtftMs:       ttftMs,
		HasTtft:      hasTtft,
		Success:      success,
		OutputTokens: outputTokens,
		GenerationMs: generationMs,
	})
}

func Record(sample Sample) {
	setting := perf_metrics_setting.GetSetting()
	if !setting.Enabled || sample.Model == "" {
		return
	}
	if sample.Group == "" {
		sample.Group = "default"
	}
	if sample.EnterpriseID < 0 {
		sample.EnterpriseID = 0
	}
	if sample.LatencyMs < 0 {
		sample.LatencyMs = 0
	}

	key := bucketKey{
		enterpriseID: sample.EnterpriseID,
		model:        sample.Model,
		group:        sample.Group,
		bucketTs:     bucketStart(time.Now().Unix()),
	}
	var bucket *atomicBucket
	for {
		actual, _ := hotBuckets.LoadOrStore(key, &atomicBucket{})
		bucket = actual.(*atomicBucket)
		if bucket.add(sample) {
			break
		}
		hotBuckets.CompareAndDelete(key, bucket)
	}
	if common.RedisEnabled && common.RDB != nil {
		if err := publishRedisSnapshot(key, bucket); err != nil {
			common.SysError("failed to publish perf metric Redis snapshot: " + err.Error())
		}
	}
}

func Query(params QueryParams) (QueryResult, error) {
	if params.Hours <= 0 {
		params.Hours = 24
	}
	if params.Hours > 24*30 {
		params.Hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(params.Hours)*3600
	activeBucketTs := bucketStart(endTs)
	redisBuckets, redisAuthoritative := readRedisActiveBuckets(activeBucketTs, redisBucketFilter{
		enterpriseID: params.EnterpriseID,
		model:        params.Model,
		group:        params.Group,
	})

	merged := map[bucketKey]counters{}
	if params.EnterpriseID > 0 {
		rows, err := model.GetEnterprisePerfMetrics(params.EnterpriseID, params.Model, params.Group, startTs, endTs)
		if err != nil {
			return QueryResult{}, err
		}
		for _, row := range rows {
			if redisAuthoritative && row.BucketTs == activeBucketTs {
				continue
			}
			mergeCounters(merged, bucketKey{
				enterpriseID: row.EnterpriseID,
				model:        row.ModelName,
				group:        row.Group,
				bucketTs:     row.BucketTs,
			}, counters{
				requestCount:   row.RequestCount,
				successCount:   row.SuccessCount,
				totalLatencyMs: row.TotalLatencyMs,
				ttftSumMs:      row.TtftSumMs,
				ttftCount:      row.TtftCount,
				outputTokens:   row.OutputTokens,
				generationMs:   row.GenerationMs,
			})
		}
	} else {
		rows, err := model.GetPerfMetrics(params.Model, params.Group, startTs, endTs)
		if err != nil {
			return QueryResult{}, err
		}
		for _, row := range rows {
			if redisAuthoritative && row.BucketTs == activeBucketTs {
				continue
			}
			mergeCounters(merged, bucketKey{
				model:    row.ModelName,
				group:    row.Group,
				bucketTs: row.BucketTs,
			}, counters{
				requestCount:   row.RequestCount,
				successCount:   row.SuccessCount,
				totalLatencyMs: row.TotalLatencyMs,
				ttftSumMs:      row.TtftSumMs,
				ttftCount:      row.TtftCount,
				outputTokens:   row.OutputTokens,
				generationMs:   row.GenerationMs,
			})
		}
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != params.Model || k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if params.Group != "" && k.group != params.Group {
			return true
		}
		if params.EnterpriseID > 0 && k.enterpriseID != params.EnterpriseID {
			return true
		}
		if redisAuthoritative && k.bucketTs == activeBucketTs {
			return true
		}
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})
	if redisAuthoritative {
		for key, value := range redisBuckets {
			mergeCounters(merged, key, value)
		}
	}

	return buildQueryResult(params.Model, merged), nil
}

func QuerySummaryAll(hours int, groups []string, enterpriseID int) (SummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	allowedGroups := allowedGroupSet(groups)
	activeBucketTs := bucketStart(endTs)
	redisBuckets, redisAuthoritative := readRedisActiveBuckets(activeBucketTs, redisBucketFilter{
		enterpriseID:  enterpriseID,
		allowedGroups: allowedGroups,
	})

	var rows []model.PerfMetricSummaryBucket
	var err error
	if enterpriseID > 0 {
		rows, err = model.GetEnterprisePerfMetricsSummaryBucketsAll(enterpriseID, startTs, endTs, groups)
	} else {
		rows, err = model.GetPerfMetricsSummaryBucketsAll(startTs, endTs, groups)
	}
	if err != nil {
		return SummaryAllResult{}, err
	}

	totals := map[string]counters{}
	modelBuckets := map[string]map[int64]counters{}
	for _, row := range rows {
		if redisAuthoritative && row.BucketTs == activeBucketTs {
			continue
		}
		value := counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
		}
		mergeModelTotals(totals, row.ModelName, value)
		mergeModelBucket(modelBuckets, row.ModelName, row.BucketTs, value)
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		if enterpriseID > 0 && k.enterpriseID != enterpriseID {
			return true
		}
		if redisAuthoritative && k.bucketTs == activeBucketTs {
			return true
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		mergeModelTotals(totals, k.model, snap)
		mergeModelBucket(modelBuckets, k.model, k.bucketTs, snap)
		return true
	})
	if redisAuthoritative {
		for key, value := range redisBuckets {
			mergeModelTotals(totals, key.model, value)
			mergeModelBucket(modelBuckets, key.model, key.bucketTs, value)
		}
	}

	models := make([]ModelSummary, 0, len(totals))
	for name, total := range totals {
		if total.requestCount == 0 {
			continue
		}
		avgLatency := total.totalLatencyMs / total.requestCount
		successRate := float64(total.successCount) / float64(total.requestCount) * 100
		avgTps := 0.0
		if total.generationMs > 0 {
			avgTps = float64(total.outputTokens) / (float64(total.generationMs) / 1000.0)
		}
		models = append(models, ModelSummary{
			ModelName:          name,
			AvgLatencyMs:       avgLatency,
			SuccessRate:        math.Round(successRate*100) / 100,
			AvgTps:             math.Round(avgTps*100) / 100,
			RecentSuccessRates: recentSuccessRates(modelBuckets[name], 3),
			RequestCount:       total.requestCount,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].RequestCount > models[j].RequestCount
	})

	return SummaryAllResult{Models: models}, nil
}

func mergeModelTotals(totals map[string]counters, modelName string, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := totals[modelName]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	totals[modelName] = current
}

func mergeModelBucket(modelBuckets map[string]map[int64]counters, modelName string, bucketTs int64, value counters) {
	if value.requestCount == 0 {
		return
	}
	if _, ok := modelBuckets[modelName]; !ok {
		modelBuckets[modelName] = map[int64]counters{}
	}
	current := modelBuckets[modelName][bucketTs]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	modelBuckets[modelName][bucketTs] = current
}

func recentSuccessRates(buckets map[int64]counters, limit int) []float64 {
	if len(buckets) == 0 || limit <= 0 {
		return nil
	}
	timestamps := make([]int64, 0, len(buckets))
	for ts := range buckets {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	if len(timestamps) > limit {
		timestamps = timestamps[len(timestamps)-limit:]
	}
	rates := make([]float64, 0, len(timestamps))
	for _, ts := range timestamps {
		rates = append(rates, math.Round(successRate(buckets[ts])*100)/100)
	}
	return rates
}

func allowedGroupSet(groups []string) map[string]struct{} {
	if groups == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		allowed[group] = struct{}{}
	}
	return allowed
}

func bucketStart(ts int64) int64 {
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	return ts - (ts % bucketSeconds)
}

func mergeCounters(merged map[bucketKey]counters, key bucketKey, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := merged[key]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	merged[key] = current
}

func buildQueryResult(modelName string, merged map[bucketKey]counters) QueryResult {
	groupBuckets := map[string]map[int64]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if _, ok := groupBuckets[key.group]; !ok {
			groupBuckets[key.group] = map[int64]counters{}
		}
		current := groupBuckets[key.group][key.bucketTs]
		current.requestCount += value.requestCount
		current.successCount += value.successCount
		current.totalLatencyMs += value.totalLatencyMs
		current.ttftSumMs += value.ttftSumMs
		current.ttftCount += value.ttftCount
		current.outputTokens += value.outputTokens
		current.generationMs += value.generationMs
		groupBuckets[key.group][key.bucketTs] = current
	}

	groups := make([]string, 0, len(groupBuckets))
	for group := range groupBuckets {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	results := make([]GroupResult, 0, len(groups))
	for _, group := range groups {
		buckets := groupBuckets[group]
		timestamps := make([]int64, 0, len(buckets))
		for ts := range buckets {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		total := counters{}
		series := make([]BucketPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := buckets[ts]
			total.requestCount += value.requestCount
			total.successCount += value.successCount
			total.totalLatencyMs += value.totalLatencyMs
			total.ttftSumMs += value.ttftSumMs
			total.ttftCount += value.ttftCount
			total.outputTokens += value.outputTokens
			total.generationMs += value.generationMs
			series = append(series, bucketPoint(ts, value))
		}

		results = append(results, GroupResult{
			Group:        group,
			AvgTtftMs:    avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs: avg(total.totalLatencyMs, total.requestCount),
			SuccessRate:  successRate(total),
			AvgTps:       avgTps(total),
			Series:       series,
		})
	}

	return QueryResult{
		ModelName:    modelName,
		SeriesSchema: seriesSchema,
		Groups:       results,
	}
}

func bucketPoint(ts int64, value counters) BucketPoint {
	return BucketPoint{
		Ts:           ts,
		AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
		AvgLatencyMs: avg(value.totalLatencyMs, value.requestCount),
		SuccessRate:  successRate(value),
		AvgTps:       avgTps(value),
	}
}

func avg(sum int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return sum / count
}

func successRate(value counters) float64 {
	if value.requestCount <= 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func avgTps(value counters) float64 {
	if value.outputTokens <= 0 || value.generationMs <= 0 {
		return 0
	}
	return float64(value.outputTokens) / (float64(value.generationMs) / 1000)
}

type redisBucketMember struct {
	Model string `json:"model"`
	Group string `json:"group"`
}

type redisPublisherMember struct {
	PublisherID  string `json:"publisher_id"`
	EnterpriseID int    `json:"enterprise_id"`
}

type redisCountersSnapshot struct {
	RequestCount   int64 `json:"req"`
	SuccessCount   int64 `json:"ok"`
	TotalLatencyMs int64 `json:"lat"`
	TtftSumMs      int64 `json:"ttft"`
	TtftCount      int64 `json:"ttft_n"`
	OutputTokens   int64 `json:"out"`
	GenerationMs   int64 `json:"gen_ms"`
}

type redisBucketFilter struct {
	enterpriseID  int
	model         string
	group         string
	allowedGroups map[string]struct{}
}

func (filter redisBucketFilter) matches(key bucketKey) bool {
	if filter.enterpriseID > 0 && key.enterpriseID != filter.enterpriseID {
		return false
	}
	if filter.model != "" && key.model != filter.model {
		return false
	}
	if filter.group != "" && key.group != filter.group {
		return false
	}
	if filter.allowedGroups != nil {
		if _, ok := filter.allowedGroups[key.group]; !ok {
			return false
		}
	}
	return true
}

func readRedisActiveBuckets(bucketTs int64, filter redisBucketFilter) (map[bucketKey]counters, bool) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, false
	}
	if !syncRedisActiveSnapshots(bucketTs, filter) {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	type pendingBucket struct {
		key     bucketKey
		command *redis.StringStringMapCmd
	}
	pending := make([]pendingBucket, 0)
	pipe := common.RDB.Pipeline()
	queueMember := func(encodedMember string) {
		var member redisBucketMember
		if err := common.Unmarshal([]byte(encodedMember), &member); err != nil {
			common.SysError("failed to decode perf metric Redis bucket member: " + err.Error())
			return
		}
		key := bucketKey{
			enterpriseID: filter.enterpriseID,
			model:        member.Model,
			group:        member.Group,
			bucketTs:     bucketTs,
		}
		if key.model == "" || key.group == "" || !filter.matches(key) {
			return
		}
		pending = append(pending, pendingBucket{
			key:     key,
			command: pipe.HGetAll(ctx, redisBucketDataKey(bucketTs, filter.enterpriseID, encodedMember)),
		})
	}

	if filter.model != "" && filter.group != "" {
		memberBytes, err := common.Marshal(redisBucketMember{Model: filter.model, Group: filter.group})
		if err != nil {
			common.SysError("failed to encode perf metric Redis bucket member: " + err.Error())
			return nil, false
		}
		queueMember(string(memberBytes))
	} else {
		indexKey := redisBucketIndexKey(bucketTs, filter.enterpriseID)
		if filter.model != "" {
			indexKey = redisBucketModelIndexKey(bucketTs, filter.enterpriseID, filter.model)
		}
		members, err := common.RDB.SMembers(ctx, indexKey).Result()
		if err != nil {
			common.SysError("failed to read perf metric Redis bucket index, using local fallback: " + err.Error())
			return nil, false
		}
		for _, encodedMember := range members {
			queueMember(encodedMember)
		}
	}
	if len(pending) == 0 {
		return nil, false
	}
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("failed to read perf metric Redis buckets, using local fallback: " + err.Error())
		return nil, false
	}

	merged := make(map[bucketKey]counters, len(pending))
	for _, item := range pending {
		values := item.command.Val()
		if len(values) == 0 {
			continue
		}
		value, err := decodeRedisCounters(values)
		if err != nil {
			common.SysError("failed to decode perf metric Redis snapshot, using local fallback: " + err.Error())
			return nil, false
		}
		mergeCounters(merged, item.key, value)
	}
	if len(merged) == 0 {
		return nil, false
	}
	return merged, true
}

func publishRedisSnapshot(key bucketKey, bucket *atomicBucket) error {
	snapshot := bucket.snapshot()
	if snapshot.requestCount <= bucket.lastPublishedRequestCount.Load() {
		return nil
	}
	if err := writeRedisBucket(key, snapshot, redisPublisherID); err != nil {
		return err
	}
	bucket.markPublished(snapshot.requestCount)
	return nil
}

func syncRedisActiveSnapshots(bucketTs int64, filter redisBucketFilter) bool {
	return syncRedisActiveSnapshotsWithPublisher(bucketTs, filter, publishRedisSnapshot)
}

func syncRedisActiveSnapshotsWithPublisher(
	bucketTs int64,
	filter redisBucketFilter,
	publish func(bucketKey, *atomicBucket) error,
) bool {
	allPublished := true
	hotBuckets.Range(func(rawKey, value any) bool {
		key := rawKey.(bucketKey)
		if key.bucketTs != bucketTs || !filter.matches(key) {
			return true
		}
		bucket := value.(*atomicBucket)
		if bucket.requestCount.Load() <= bucket.lastPublishedRequestCount.Load() {
			return true
		}
		if err := publish(key, bucket); err != nil {
			common.SysError("failed to synchronize perf metric Redis snapshot, using local fallback: " + err.Error())
			allPublished = false
			return false
		}
		return true
	})
	return allPublished
}

func writeRedisBucket(key bucketKey, value counters, publisherID string) error {
	if !common.RedisEnabled || common.RDB == nil || value.requestCount == 0 || publisherID == "" {
		return fmt.Errorf("Redis is not available")
	}
	memberBytes, err := common.Marshal(redisBucketMember{Model: key.model, Group: key.group})
	if err != nil {
		return fmt.Errorf("encode bucket member: %w", err)
	}
	member := string(memberBytes)
	publisherBytes, err := common.Marshal(redisPublisherMember{
		PublisherID:  publisherID,
		EnterpriseID: key.enterpriseID,
	})
	if err != nil {
		return fmt.Errorf("encode publisher member: %w", err)
	}
	snapshotBytes, err := common.Marshal(redisCountersSnapshot{
		RequestCount:   value.requestCount,
		SuccessCount:   value.successCount,
		TotalLatencyMs: value.totalLatencyMs,
		TtftSumMs:      value.ttftSumMs,
		TtftCount:      value.ttftCount,
		OutputTokens:   value.outputTokens,
		GenerationMs:   value.generationMs,
	})
	if err != nil {
		return fmt.Errorf("encode counters snapshot: %w", err)
	}
	publisherField := base64.RawURLEncoding.EncodeToString(publisherBytes)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	scopes := []int{0}
	if key.enterpriseID > 0 {
		scopes = append(scopes, key.enterpriseID)
	}
	keys := make([]string, 0, len(scopes)*3)
	for _, enterpriseID := range scopes {
		keys = append(keys,
			redisBucketDataKey(key.bucketTs, enterpriseID, member),
			redisBucketIndexKey(key.bucketTs, enterpriseID),
			redisBucketModelIndexKey(key.bucketTs, enterpriseID, key.model),
		)
	}
	_, err = publishRedisSnapshotScript.Run(ctx, common.RDB, keys,
		publisherField,
		member,
		string(snapshotBytes),
		value.requestCount,
		int64(time.Hour/time.Second),
	).Result()
	return err
}

func decodeRedisCounters(values map[string]string) (counters, error) {
	merged := counters{}
	for field, encoded := range values {
		if !strings.HasPrefix(field, "s:") {
			continue
		}
		var snapshot redisCountersSnapshot
		if err := common.Unmarshal([]byte(encoded), &snapshot); err != nil {
			return counters{}, err
		}
		merged.requestCount += snapshot.RequestCount
		merged.successCount += snapshot.SuccessCount
		merged.totalLatencyMs += snapshot.TotalLatencyMs
		merged.ttftSumMs += snapshot.TtftSumMs
		merged.ttftCount += snapshot.TtftCount
		merged.outputTokens += snapshot.OutputTokens
		merged.generationMs += snapshot.GenerationMs
	}
	return merged, nil
}

func redisBucketScope(enterpriseID int) string {
	if enterpriseID > 0 {
		return fmt.Sprintf("enterprise:%d", enterpriseID)
	}
	return "global"
}

func redisBucketIndexKey(bucketTs int64, enterpriseID int) string {
	return fmt.Sprintf("perf:v4:index:%d:%s", bucketTs, redisBucketScope(enterpriseID))
}

func redisBucketModelIndexKey(bucketTs int64, enterpriseID int, model string) string {
	encodedModel := base64.RawURLEncoding.EncodeToString([]byte(model))
	return fmt.Sprintf("perf:v4:model-index:%d:%s:%s", bucketTs, redisBucketScope(enterpriseID), encodedModel)
}

func redisBucketDataKey(bucketTs int64, enterpriseID int, member string) string {
	encodedMember := base64.RawURLEncoding.EncodeToString([]byte(member))
	return fmt.Sprintf("perf:v4:data:%d:%s:%s", bucketTs, redisBucketScope(enterpriseID), encodedMember)
}
