package vsearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type runtimeFakeAdapter struct {
	probeState     AccountState
	probeErr       error
	snapshot       CatalogSnapshot
	snapshotErr    error
	executeResult  any
	executeAttempt AttemptResult
	executeErr     error
	executeCalls   int
	executeStarted chan struct{}
	executeRelease <-chan struct{}
	executeHook    func()
}

type runtimeFakeCharge struct {
	commitErr    error
	refundErr    error
	chargeMicros int64
	commitCalls  int
	refundCalls  int
}

var runtimeTestDatabaseID atomic.Uint64

func (charge *runtimeFakeCharge) commit() error {
	charge.commitCalls++
	return charge.commitErr
}

func (charge *runtimeFakeCharge) refund(context.Context) error {
	charge.refundCalls++
	return charge.refundErr
}

func (charge *runtimeFakeCharge) potentialChargeMicros() int64 {
	return charge.chargeMicros
}

func (*runtimeFakeCharge) billingSource() string { return "test" }
func (*runtimeFakeCharge) reservedQuota() int    { return 0 }

func (adapter *runtimeFakeAdapter) Probe(context.Context) (AccountState, error) {
	if adapter.probeState == (AccountState{}) {
		adapter.probeState = AccountState{Plan: "test", BalanceAmountMicros: 10_000_000, BalanceCurrency: "CNY"}
	}
	return adapter.probeState, adapter.probeErr
}

func (adapter *runtimeFakeAdapter) SnapshotCatalog(context.Context) (CatalogSnapshot, error) {
	if adapter.snapshot.Provider == "" {
		adapter.snapshot = standardProviderCatalog(ProviderTikHub)
	}
	return adapter.snapshot, adapter.snapshotErr
}

func (adapter *runtimeFakeAdapter) Execute(context.Context, ProviderOperation, CanonicalRequest) (AttemptResult, error) {
	adapter.executeCalls++
	if adapter.executeStarted != nil {
		select {
		case adapter.executeStarted <- struct{}{}:
		default:
		}
	}
	if adapter.executeRelease != nil {
		<-adapter.executeRelease
	}
	if adapter.executeHook != nil {
		adapter.executeHook()
	}
	attempt := adapter.executeAttempt
	if attempt.Data == nil {
		attempt.Data = adapter.executeResult
	}
	if adapter.executeErr == nil {
		billable := true
		attempt.Dispatched = true
		attempt.Billable = &billable
		if attempt.HTTPStatus == 0 {
			attempt.HTTPStatus = http.StatusOK
		}
	}
	return attempt, adapter.executeErr
}

func openRuntimeTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	databaseName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), runtimeTestDatabaseID.Add(1))
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	model.LOG_DB = database
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		if common.RedisEnabled != previousRedisEnabled {
			common.RedisEnabled = previousRedisEnabled
		}
		common.BatchUpdateEnabled = previousBatchUpdateEnabled
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})
	require.NoError(t, database.AutoMigrate(
		&model.User{}, &model.Log{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{},
		&model.WalletPreConsumeRecord{}, &model.Enterprise{},
		&model.SearchUpstreamPool{}, &model.SearchUpstreamAccount{},
		&model.SearchCapability{}, &model.SearchCapabilityBinding{},
		&model.SearchCapabilityGrant{}, &model.SearchUsageEvent{}, &model.SearchExecutionIdempotency{},
	))
	configuredKey := bytes.Repeat([]byte{0x31}, upstreamSecretKeySize)
	t.Setenv(UpstreamSecretKeyEnv, base64.RawStdEncoding.EncodeToString(configuredKey))
	t.Setenv(LocalSecretFileEnv, "")
}

func seedRuntimeExecution(t *testing.T, adapter *runtimeFakeAdapter) (*ExecutionRuntime, Principal, *model.SearchCapability) {
	t.Helper()
	user := &model.User{
		Id: 7, Username: "vsearch-user", Password: "test-password", Status: common.UserStatusEnabled,
		Quota: 1_000_000, Setting: `{"billing_preference":"wallet_only"}`,
	}
	require.NoError(t, model.DB.Create(user).Error)
	pool := &model.SearchUpstreamPool{Name: "default"}
	require.NoError(t, model.CreateSearchUpstreamPool(pool))
	encrypted, err := EncryptUpstreamSecret("ak_live_runtime")
	require.NoError(t, err)
	account := &model.SearchUpstreamAccount{
		PoolID: pool.Id, Name: "primary", BaseURL: DefaultTikHubBaseURL,
		SecretCiphertext: encrypted.Ciphertext, SecretNonce: encrypted.Nonce,
		SecretVersion: encrypted.Version, SecretPrefix: UpstreamSecretPrefix("ak_live_runtime"),
		Status: model.SearchUpstreamAccountStatusHealthy,
	}
	require.NoError(t, model.CreateSearchUpstreamAccount(account))
	publicID, err := model.GenerateSearchCapabilityPublicID("private/search")
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID: publicID, Name: "Public Search", Category: "搜索", Description: "Search public data",
		InputSchema: `{"type":"object","required":["query"],"properties":{"query":{"type":"string"}},"additionalProperties":false}`,
		Status:      model.SearchCapabilityStatusEnabled, UpstreamCostMicros: 100_000, PriceMicros: 250_000,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.Id, ToolName: "private/search",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))
	runtime := NewExecutionRuntime(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) { return adapter, nil })
	return runtime, Principal{UserID: 7, EnterpriseID: 11, AgentKeyID: 21, Scopes: []string{"web-search"}}, capability
}

func TestExecutionRuntimeRejectsInvalidParamsBeforeDispatchOrUsage(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": float64(42)}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INVALID_TOOL_PARAMS", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
	_, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	events, _, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, "INVALID_TOOL_PARAMS", events[0].ErrorCode)
}

func TestValidateCapabilityParamsEnforcesParameterConstraints(t *testing.T) {
	schema := `{
		"type":"object",
		"additionalProperties":false,
		"properties":{
			"query":{"type":"string","enum":["news","web"],"minLength":3,"maxLength":4},
			"page":{"type":"integer","minimum":1,"maximum":10},
			"scores":{"type":"array","minItems":1,"maxItems":2,"items":{"type":"number"}}
		}
	}`

	tests := []struct {
		name   string
		params map[string]any
	}{
		{name: "unknown parameter", params: map[string]any{"query": "news", "extra": true}},
		{name: "enum", params: map[string]any{"query": "other"}},
		{name: "minimum", params: map[string]any{"page": float64(0)}},
		{name: "maximum", params: map[string]any{"page": float64(11)}},
		{name: "numeric string", params: map[string]any{"page": "2"}},
		{name: "string minimum length", params: map[string]any{"query": "we"}},
		{name: "string maximum length", params: map[string]any{"query": "newss"}},
		{name: "array minimum items", params: map[string]any{"scores": []any{}}},
		{name: "array maximum items", params: map[string]any{"scores": []any{float64(1), float64(2), float64(3)}}},
		{name: "array items", params: map[string]any{"scores": []any{"invalid"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, validateCapabilityParams(test.params, schema))
		})
	}
	assert.NoError(t, validateCapabilityParams(map[string]any{
		"query": "web", "page": float64(2), "scores": []any{float64(0.5), float64(1)},
	}, schema))
}

func TestValidateSchemaValueEnforcesOutputContractConstAndNullableUnion(t *testing.T) {
	schema := listOutputSchema("trend")
	valid := map[string]any{
		"items": []any{map[string]any{"id": "trend-1", "type": "trend", "platform": "douyin"}},
		"page":  map[string]any{"cursor": nil, "has_more": false},
	}
	assert.NoError(t, validateSchemaValue(valid, schema, "$"))

	invalidType := map[string]any{
		"items": []any{map[string]any{"id": "trend-1", "type": "content", "platform": "douyin"}},
	}
	assert.Error(t, validateSchemaValue(invalidType, schema, "$"))

	invalidCursor := map[string]any{
		"items": []any{},
		"page":  map[string]any{"cursor": false, "has_more": nil},
	}
	assert.Error(t, validateSchemaValue(invalidCursor, schema, "$"))
}

func TestExecutionRuntimeRefundsAndClosesIdempotencyOnOutputContractMismatch(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{ProviderRequestID: "provider-contract-1", CostAmountMicros: 100_000},
		executeResult: map[string]any{
			"items": []any{map[string]any{"id": "trend-1", "type": "content", "platform": "douyin"}},
		},
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	outputSchemaBytes, err := common.Marshal(listOutputSchema("trend"))
	require.NoError(t, err)
	outputSchema := string(outputSchemaBytes)
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).Updates(map[string]any{
		"operation_key": "social.trend.list", "contract_version": "v1", "output_schema": outputSchema,
	}).Error)
	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).Where("capability_id = ?", capability.Id).Updates(map[string]any{
		"output_schema": outputSchema, "contract_equivalent": true, "billing_ready": true, "cost_currency": "CNY",
	}).Error)
	charge := &runtimeFakeCharge{chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "contract-failure-1",
	}

	_, err = runtime.Execute(context.Background(), principal, command)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_CONTRACT_MISMATCH", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
	assert.Zero(t, charge.commitCalls)
	assert.Equal(t, 1, charge.refundCalls)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_RESOLVED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls, "known paid upstream failures must not be replayed")

	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, model.SearchUsageBillingRefunded, events[0].BillingState)
	assert.Equal(t, int64(100_000), events[0].UpstreamCostMicros)
	assert.Zero(t, events[0].ChargeMicros)
	assert.Equal(t, "provider-contract-1", events[0].UpstreamRequestID)
	var consumeLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ?", principal.UserID, model.LogTypeConsume).Count(&consumeLogCount).Error)
	assert.Zero(t, consumeLogCount)
}

func TestExecutionRuntimeAuditsKnownBillableAdapterFailureWithoutReplay(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{
			Dispatched: true, Billable: tikHubBool(true), ProviderRequestID: "provider-shape-1", CostAmountMicros: 100_000,
		},
		executeErr: newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。"),
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	charge := &runtimeFakeCharge{chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "adapter-contract-failure-1",
	}

	_, err := runtime.Execute(context.Background(), principal, command)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_CONTRACT_MISMATCH", publicErr.Code)
	assert.Equal(t, 1, charge.refundCalls)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_RESOLVED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)

	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, int64(100_000), events[0].UpstreamCostMicros)
	assert.Zero(t, events[0].ChargeMicros)
}

func TestExecutionRuntimeKeepsIdempotencyPendingWhenKnownFailureAuditCannotFinalize(t *testing.T) {
	openRuntimeTestDB(t)
	callbackName := "test:fail_known_vsearch_failure_finalize"
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{
			Dispatched: true, Billable: tikHubBool(true), ProviderRequestID: "provider-audit-failure-1", CostAmountMicros: 100_000,
		},
		executeErr: newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。"),
		executeHook: func() {
			require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
				updates, ok := tx.Statement.Dest.(map[string]any)
				if tx.Statement.Table == "search_usage_events" && ok && updates["status"] == model.SearchUsageStatusFailed {
					tx.AddError(errors.New("forced search usage finalize failure"))
				}
			}))
		},
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	charge := &runtimeFakeCharge{chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "known-audit-failure-1",
	}

	_, err := runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))
	var record model.SearchExecutionIdempotency
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.SearchExecutionIdempotencyStatusPending, record.Status)
	var pending model.SearchUsageEvent
	require.NoError(t, model.DB.Where("request_id = ?", record.UsageRequestID).First(&pending).Error)
	assert.Equal(t, model.SearchUsageStatusPending, pending.Status)
	assert.Equal(t, model.SearchUsageBillingRefundPending, pending.BillingState)
	assert.Equal(t, "provider-audit-failure-1", pending.UpstreamRequestID)
	assert.Equal(t, int64(100_000), pending.UpstreamCostMicros)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", pending.Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.DB.First(&pending, pending.Id).Error)
	assert.Equal(t, model.SearchUsageStatusFailed, pending.Status)
	assert.Equal(t, model.SearchUsageBillingRefunded, pending.BillingState)
	assert.Equal(t, "provider-audit-failure-1", pending.UpstreamRequestID)
	assert.Equal(t, int64(100_000), pending.UpstreamCostMicros)
	require.NoError(t, model.DB.First(&record, record.Id).Error)
	assert.Equal(t, model.SearchExecutionIdempotencyStatusResolved, record.Status)

	_, err = runtime.Execute(context.Background(), principal, command)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_RESOLVED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionRuntimeRejectsPriceBelowSelectedUpstreamCostBeforeDispatch(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).
		Where("capability_id = ?", capability.Id).
		Update("upstream_cost_micros", int64(300_000)).Error)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID,
		Params:    map[string]any{"query": "pricing safety"},
	})

	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_PRICING_STALE", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
}

func TestDescribeAndExecuteRejectPriceBelowHealthyFallbackCost(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	accounts, err := model.ListSearchUpstreamAccounts()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	encrypted, err := EncryptUpstreamSecret("ak_live_fallback")
	require.NoError(t, err)
	fallback := &model.SearchUpstreamAccount{
		PoolID: accounts[0].PoolID, Name: "fallback", BaseURL: DefaultTikHubBaseURL,
		SecretCiphertext: encrypted.Ciphertext, SecretNonce: encrypted.Nonce,
		SecretVersion: encrypted.Version, SecretPrefix: UpstreamSecretPrefix("ak_live_fallback"),
		Status: model.SearchUpstreamAccountStatusHealthy, Priority: 1,
	}
	require.NoError(t, model.CreateSearchUpstreamAccount(fallback))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: fallback.Id, ToolName: "private/search-fallback",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		Priority: 1, UpstreamCostMicros: 300_000,
	}))

	_, err = runtime.Describe(context.Background(), principal, capability.PublicID)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_PRICING_STALE", publicErr.Code)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID,
		Params:    map[string]any{"query": "pricing safety"},
	})
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_PRICING_STALE", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
}

func TestDescribeAndExecuteRejectStaleBindingSchema(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").
		Update("status", model.SearchUpstreamAccountStatusStandby).Error)
	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).
		Where("capability_id = ?", capability.Id).
		Update("input_schema", `{"type":"object","properties":{"stale":{"type":"string"}}}`).Error)
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").
		Update("status", model.SearchUpstreamAccountStatusHealthy).Error)

	_, err := runtime.Describe(context.Background(), principal, capability.PublicID)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_UNAVAILABLE", publicErr.Code)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID,
		Params:    map[string]any{"query": "schema safety"},
	})
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_UNAVAILABLE", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
}

func TestExecutionRuntimeRejectsBindingFromNewerSchemaSnapshot(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	newSchema := `{"type":"object","required":["url"],"properties":{"url":{"type":"string"}},"additionalProperties":false}`

	var schemaSynced atomic.Bool
	var syncErr error
	callbackName := "test:vsearch_sync_schema_after_execution_snapshot"
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "search_capabilities" || !schemaSynced.CompareAndSwap(false, true) {
			return
		}
		syncErr = model.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
				Update("input_schema", newSchema).Error; err != nil {
				return err
			}
			return tx.Model(&model.SearchCapabilityBinding{}).Where("capability_id = ?", capability.Id).
				Update("input_schema", newSchema).Error
		})
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID,
		Params:    map[string]any{"query": "validated against the first snapshot"},
	})

	require.NoError(t, syncErr)
	assert.True(t, schemaSynced.Load())
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_UNAVAILABLE", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
}

func TestExecutionRuntimeDispatchesOnceSanitizesAndRecordsExactMicros(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{ProviderRequestID: "provider-success-1", CostAmountMicros: 999, CostCurrency: "USD"},
		executeResult: map[string]any{
			"data":   []any{map[string]any{"title": "Public result", "toolName": "private/search"}},
			"apiKey": "must-not-leak", "note": "AgentKey internal",
		},
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)

	result, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	payload, err := common.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "must-not-leak")
	assert.NotContains(t, string(payload), "private/search")
	assert.Contains(t, string(payload), "AgentKey internal", "public result text must not be rewritten by legacy provider cleanup")

	events, total, err := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	assert.Equal(t, int64(100_000), events[0].UpstreamCostMicros)
	assert.Equal(t, int64(250_000), events[0].ChargeMicros)
	assert.Equal(t, model.SearchUsageStatusSucceeded, events[0].Status)
	assert.Equal(t, "provider-success-1", events[0].UpstreamRequestID)

	userCatalog, err := runtime.ListCatalog(context.Background(), principal, false)
	require.NoError(t, err)
	require.Len(t, userCatalog, 1)
	assert.Zero(t, userCatalog[0].UpstreamCostMicros, "user catalog must hide procurement cost")
	adminCatalog, err := runtime.ListCatalog(context.Background(), Principal{}, true)
	require.NoError(t, err)
	assert.Equal(t, int64(100_000), adminCatalog[0].UpstreamCostMicros)
}

func TestExecutionRuntimePersistsPendingAuditBeforeDispatch(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	adapter.executeHook = func() {
		events, total, err := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, events, 1)
		assert.Equal(t, model.SearchUsageStatusPending, events[0].Status)
		assert.Zero(t, events[0].ChargeMicros)
	}

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})
	require.NoError(t, err)
	events, total, err := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	assert.Equal(t, model.SearchUsageStatusSucceeded, events[0].Status)
	assert.Equal(t, capability.PriceMicros, events[0].ChargeMicros)
}

func TestExecutionRuntimeDoesNotRetryFailedExecute(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{ProviderRequestID: "provider-failure-1"},
		executeErr:     newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。"),
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	quotaAfter, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore, quotaAfter, "failed upstream execution must synchronously refund wallet quota")
	user, userErr := model.GetUserById(principal.UserID, true)
	require.NoError(t, userErr)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	var consumeLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ?", principal.UserID, model.LogTypeConsume).Count(&consumeLogCount).Error)
	assert.Zero(t, consumeLogCount)
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, "UPSTREAM_TIMEOUT", events[0].ErrorCode)
	assert.Equal(t, "provider-failure-1", events[0].UpstreamRequestID)
	assert.Zero(t, events[0].ChargeMicros, "a fully refunded failure must not be reported as charged")
}

func TestExecutionRuntimeReturnsRetryAfterWithoutReplaying(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{RetryAfter: 1500 * time.Millisecond},
		executeErr:     newConnectorError("UPSTREAM_RATE_LIMITED", http.StatusTooManyRequests, "上游服务请求过于频繁。"),
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, http.StatusTooManyRequests, publicErr.HTTPStatus)
	assert.Equal(t, 2, publicErr.RetryAfterSeconds)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionRuntimeChargesWalletOnlyAfterSuccessfulCall(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	quotaAfter, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, quotaBefore-wantQuota, quotaAfter)
	user, err := model.GetUserById(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, wantQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	var coreLog model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", principal.UserID, model.LogTypeConsume).First(&coreLog).Error)
	assert.Equal(t, wantQuota, coreLog.Quota)
	assert.Equal(t, principal.EnterpriseID, coreLog.EnterpriseID)
	assert.Equal(t, "vsearch:"+capability.PublicID, coreLog.ModelName)
}

func TestExecutionRuntimeRejectsInsufficientWalletBeforeUpstream(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", principal.UserID).Update("quota", wantQuota-1).Error)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INSUFFICIENT_QUOTA", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
	quotaAfter, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, wantQuota-1, quotaAfter)
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Positive(t, events[0].UpstreamAccountID, "admin failure audit should retain the selected account")
	assert.Zero(t, events[0].ChargeMicros)
}

func TestSelectExecutionTargetIncludesPositiveWeightAccount(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{}
	_, _, capability := seedRuntimeExecution(t, adapter)
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").Update("weight", 10).Error)

	binding, account, _, err := selectExecutionTarget(capability, nil)
	require.NoError(t, err)
	assert.Equal(t, capability.Id, binding.CapabilityID)
	assert.Equal(t, 10, account.Weight)
}

func TestExecutionRuntimeCountsFreeSuccessWithoutConsumingQuota(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
		Updates(map[string]any{"price_micros": 0, "upstream_cost_micros": 0}).Error)
	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).Where("capability_id = ?", capability.Id).
		Update("upstream_cost_micros", 0).Error)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	quotaAfter, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, quotaBefore, quotaAfter)
	user, err := model.GetUserById(principal.UserID, true)
	require.NoError(t, err)
	assert.Zero(t, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
}

func TestExecutionRuntimeChargesSubscriptionOnSuccess(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", principal.UserID).
		Updates(map[string]any{"setting": `{"billing_preference":"subscription_only"}`, "quota": 1_000_000}).Error)
	plan := &model.SubscriptionPlan{Title: "vSearch", Enabled: true, TotalAmount: int64(wantQuota * 10)}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	subscription := &model.UserSubscription{
		UserId: principal.UserID, PlanId: plan.Id, AmountTotal: int64(wantQuota * 10),
		Status: "active", StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(subscription).Error)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	require.NoError(t, model.DB.First(subscription, subscription.Id).Error)
	assert.Equal(t, int64(wantQuota), subscription.AmountUsed)
	walletQuota, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, walletQuota, "subscription billing must not consume wallet quota")
}

func TestExecutionRuntimeRefundsSubscriptionOnUpstreamFailure(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", principal.UserID).
		Update("setting", `{"billing_preference":"subscription_only"}`).Error)
	plan := &model.SubscriptionPlan{Title: "vSearch", Enabled: true, TotalAmount: int64(wantQuota * 10)}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	subscription := &model.UserSubscription{
		UserId: principal.UserID, PlanId: plan.Id, AmountTotal: int64(wantQuota * 10),
		Status: "active", StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(subscription).Error)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	require.NoError(t, model.DB.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed, "failed upstream execution must synchronously refund subscription quota")
	var record model.SubscriptionPreConsumeRecord
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&record).Error)
	assert.Equal(t, "refunded", record.Status)
	user, userErr := model.GetUserById(principal.UserID, true)
	require.NoError(t, userErr)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
}

func TestExecutionRuntimeRejectsInsufficientSubscriptionBeforeUpstream(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", principal.UserID).
		Update("setting", `{"billing_preference":"subscription_only"}`).Error)
	plan := &model.SubscriptionPlan{Title: "vSearch", Enabled: true, TotalAmount: int64(wantQuota - 1)}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	subscription := &model.UserSubscription{
		UserId: principal.UserID, PlanId: plan.Id, AmountTotal: int64(wantQuota - 1),
		Status: "active", StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(subscription).Error)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INSUFFICIENT_QUOTA", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
	require.NoError(t, model.DB.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed)
}

func TestExecutionRuntimeRefundsWhenSettlementFailsAfterUpstreamSuccess(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	charge := &runtimeFakeCharge{commitErr: errors.New("settlement failed"), chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	assert.Equal(t, 1, adapter.executeCalls)
	assert.Equal(t, 1, charge.commitCalls)
	assert.Equal(t, 1, charge.refundCalls, "settlement failure must synchronously compensate before returning")
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", events[0].ErrorCode)
	assert.Zero(t, events[0].ChargeMicros, "successful compensation leaves no residual charge")
	assert.Equal(t, capability.UpstreamCostMicros, events[0].UpstreamCostMicros, "a completed upstream call must retain its actual cost")
}

func TestExecutionRuntimeRefundsDurablyWhenReservationProgressCannotPersist(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "must not dispatch"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	var failed atomic.Bool
	callbackName := "test:fail_vsearch_reserved_progress"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "search_usage_events" && ok && updates["billing_state"] == model.SearchUsageBillingReserved && failed.CompareAndSwap(false, true) {
			tx.AddError(errors.New("forced reservation progress failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})
	require.Error(t, err)
	assert.Zero(t, adapter.executeCalls)
	quotaAfter, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore, quotaAfter, "failed durable progress must refund the reservation exactly once")

	var event model.SearchUsageEvent
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&event).Error)
	assert.Equal(t, model.SearchUsageStatusFailed, event.Status)
	assert.Equal(t, model.SearchUsageBillingRefunded, event.BillingState)
	var walletRecord model.WalletPreConsumeRecord
	require.NoError(t, model.DB.Where("request_id = ?", event.RequestID).First(&walletRecord).Error)
	assert.Equal(t, model.WalletPreConsumeStatusRefunded, walletRecord.Status)
	require.NoError(t, model.RefundUserWalletPreConsume(event.RequestID))
	quotaAfterReplay, replayErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, replayErr)
	assert.Equal(t, quotaBefore, quotaAfterReplay, "replayed compensation must not double-credit the wallet")
}

func TestExecutionRuntimeRecoversCommitAfterTerminalWriteFailure(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	var failed atomic.Bool
	callbackName := "test:fail_vsearch_terminal_commit"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "search_usage_events" && ok && updates["status"] == model.SearchUsageStatusSucceeded && failed.CompareAndSwap(false, true) {
			tx.AddError(errors.New("forced terminal commit failure"))
		}
	}))

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", publicErr.Code)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var pending model.SearchUsageEvent
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&pending).Error)
	assert.Equal(t, model.SearchUsageStatusPending, pending.Status)
	assert.Equal(t, model.SearchUsageBillingCommitPending, pending.BillingState)
	quotaAfterFailure, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore-wantQuota, quotaAfterFailure)
	userBeforeRecovery, userErr := model.GetUserById(principal.UserID, true)
	require.NoError(t, userErr)
	assert.Zero(t, userBeforeRecovery.UsedQuota)
	assert.Zero(t, userBeforeRecovery.RequestCount)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", pending.Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))

	var recovered model.SearchUsageEvent
	require.NoError(t, model.DB.First(&recovered, pending.Id).Error)
	assert.Equal(t, model.SearchUsageStatusSucceeded, recovered.Status)
	assert.Equal(t, model.SearchUsageBillingCommitted, recovered.BillingState)
	userAfterRecovery, userErr := model.GetUserById(principal.UserID, true)
	require.NoError(t, userErr)
	assert.Equal(t, wantQuota, userAfterRecovery.UsedQuota)
	assert.Equal(t, 1, userAfterRecovery.RequestCount)
	var consumeLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where(&model.Log{RequestId: recovered.RequestID, Type: model.LogTypeConsume, Group: "vsearch"}).
		Count(&consumeLogCount).Error)
	assert.Equal(t, int64(1), consumeLogCount, "recovery must materialize one consume log")
}

func TestExecutionRuntimeRecoversConsumeLogAfterOutboxFinalizeFailure(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)

	var failed atomic.Bool
	callbackName := "test:fail_vsearch_log_outbox_finalize"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "search_usage_events" && ok && updates["billing_state"] == model.SearchUsageBillingCommitted && failed.CompareAndSwap(false, true) {
			tx.AddError(errors.New("forced outbox finalize failure"))
		}
	}))

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var event model.SearchUsageEvent
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&event).Error)
	assert.Equal(t, model.SearchUsageBillingLogWriting, event.BillingState)
	var consumeLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where(&model.Log{RequestId: event.RequestID, Type: model.LogTypeConsume, Group: "vsearch"}).
		Count(&consumeLogCount).Error)
	assert.Equal(t, int64(1), consumeLogCount)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", event.Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.DB.First(&event, event.Id).Error)
	assert.Equal(t, model.SearchUsageBillingCommitted, event.BillingState)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where(&model.Log{RequestId: event.RequestID, Type: model.LogTypeConsume, Group: "vsearch"}).
		Count(&consumeLogCount).Error)
	assert.Equal(t, int64(1), consumeLogCount, "outbox replay must not duplicate the consume log")
}

func TestExecutionRuntimeRetriesDurableWalletCompensation(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	var failed atomic.Bool
	callbackName := "test:fail_vsearch_wallet_refund"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "wallet_pre_consume_records" && ok && updates["status"] == model.WalletPreConsumeStatusRefunded && failed.CompareAndSwap(false, true) {
			tx.AddError(errors.New("forced wallet refund failure"))
		}
	}))

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"},
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_BILLING_COMPENSATION_FAILED", publicErr.Code)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var event model.SearchUsageEvent
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&event).Error)
	assert.Equal(t, model.SearchUsageBillingRefundFailed, event.BillingState)
	assert.Equal(t, capability.PriceMicros, event.ChargeMicros)
	quotaAfterFailure, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore-wantQuota, quotaAfterFailure)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", event.Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))

	quotaAfterRecovery, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore, quotaAfterRecovery)
	require.NoError(t, model.DB.First(&event, event.Id).Error)
	assert.Equal(t, model.SearchUsageBillingRefunded, event.BillingState)
	assert.Zero(t, event.ChargeMicros)
	assert.Equal(t, "VSEARCH_BILLING_COMPENSATION_RECOVERED", event.ErrorCode)
	var walletRecord model.WalletPreConsumeRecord
	require.NoError(t, model.DB.Where("request_id = ?", event.RequestID).First(&walletRecord).Error)
	assert.Equal(t, model.WalletPreConsumeStatusRefunded, walletRecord.Status)
}

func TestExecutionRuntimeAuditsResidualChargeWhenCompensationFails(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	charge := &runtimeFakeCharge{refundErr: errors.New("refund failed"), chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_BILLING_COMPENSATION_FAILED", publicErr.Code)
	assert.Equal(t, 1, charge.refundCalls)
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, capability.PriceMicros, events[0].ChargeMicros, "admin audit must expose a possibly unrecovered charge")
}

func TestControlPlaneSyncCreatesDraftCapabilitiesFromStandardCatalog(t *testing.T) {
	openRuntimeTestDB(t)
	snapshot := standardProviderCatalog(ProviderTikHub)
	adapter := &runtimeFakeAdapter{snapshot: snapshot}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) {
		return adapter, nil
	})
	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "tikhub-primary", Provider: ProviderTikHub, Secret: "tk_live_sync", Status: "healthy",
	})
	require.NoError(t, err)

	result, err := control.SyncCatalog(context.Background())

	require.NoError(t, err)
	assert.Equal(t, len(snapshot.Operations), result.Synced)
	assert.Zero(t, result.Published)
	capabilities, err := model.ListSearchCapabilities(true)
	require.NoError(t, err)
	require.NotEmpty(t, capabilities)
	verifiedBindings := map[string]struct{}{
		"social.account.get/youtube":                {},
		"social.account.contents.list/wechat_mp":    {},
		"social.account.contents.list/youtube":      {},
		"social.content.get/youtube":                {},
		"social.content.search/youtube":             {},
		"social.comment.list/youtube":               {},
		"social.comment.replies.list/youtube":       {},
		"social.trend.list/douyin":                  {},
		"commerce.product.search/tiktok_shop":       {},
		"commerce.product.get/tiktok_shop":          {},
		"commerce.product.reviews.list/tiktok_shop": {},
	}
	for _, capability := range capabilities {
		assert.Equal(t, model.SearchCapabilityStatusDisabled, capability.Status)
		assert.Equal(t, model.SearchCapabilitySchemaAvailable, capability.SchemaStatus)
		assert.Equal(t, "v1", capability.ContractVersion)
		assert.NotEmpty(t, capability.OperationKey)
		bindings, bindingErr := model.ListSearchCapabilityBindings(capability.Id, true)
		require.NoError(t, bindingErr)
		require.NotEmpty(t, bindings)
		for _, binding := range bindings {
			_, verified := verifiedBindings[capability.OperationKey+"/"+binding.Platform]
			assert.Equal(t, verified, binding.ContractEquivalent)
			assert.Equal(t, verified, binding.BillingReady)
			assert.Equal(t, "tikhub.direct.v1", binding.MappingKey)
			assert.Equal(t, "CNY", binding.CostCurrency)
			if verified {
				assert.Positive(t, binding.UpstreamCostMicros)
				assert.GreaterOrEqual(t, capability.UpstreamCostMicros, binding.UpstreamCostMicros, "capability cost floor must cover every verified route")
				assert.Equal(t, capability.UpstreamCostMicros, capability.PriceMicros, "verified ability must use the normalized upstream cost without markup")
			}
		}
	}
	catalog, err := control.runtime.ListCatalog(context.Background(), Principal{}, true)
	require.NoError(t, err)
	require.NotEmpty(t, catalog)
	for _, item := range catalog {
		assert.Equal(t, "verified", item.ContractStatus)
		assert.Positive(t, item.HealthyRouteCount)
		assert.False(t, item.Enabled)
	}
}

func TestControlPlaneSyncDisablesBindingsRemovedFromCuratedCatalog(t *testing.T) {
	openRuntimeTestDB(t)
	snapshot := standardProviderCatalog(ProviderTikHub)
	require.GreaterOrEqual(t, len(snapshot.Operations), 2)
	adapter := &runtimeFakeAdapter{snapshot: snapshot}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) {
		return adapter, nil
	})
	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "tikhub-primary", Provider: ProviderTikHub, Secret: "tk_live_sync", Status: "healthy",
	})
	require.NoError(t, err)
	_, err = control.SyncCatalog(context.Background())
	require.NoError(t, err)

	removed := snapshot.Operations[0]
	adapter.snapshot.Operations = snapshot.Operations[1:]
	_, err = control.SyncCatalog(context.Background())
	require.NoError(t, err)

	var binding model.SearchCapabilityBinding
	require.NoError(t, model.DB.Where("provider_operation_id = ?", removed.OperationID).First(&binding).Error)
	assert.Equal(t, model.SearchCapabilityBindingStatusDisabled, binding.Status)
	var retained model.SearchCapabilityBinding
	require.NoError(t, model.DB.Where("provider_operation_id = ?", snapshot.Operations[1].OperationID).First(&retained).Error)
	assert.Equal(t, model.SearchCapabilityBindingStatusEnabled, retained.Status)
}

func TestControlPlaneSyncDisablesRemovedJustOneAPIOperations(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{snapshot: standardProviderCatalog(ProviderJustOneAPI)}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) {
		return adapter, nil
	})
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "justone-primary", Provider: ProviderJustOneAPI, Secret: "jo_live_sync", Status: "healthy",
	})
	require.NoError(t, err)
	oldPublicID, err := model.GenerateSearchCapabilityPublicID("social.trend.list@v1")
	require.NoError(t, err)
	oldCapability := &model.SearchCapability{
		PublicID: oldPublicID, OperationKey: "social.trend.list", ContractVersion: "v1",
		Name: "平台趋势榜", Category: "社交媒体", InputSchema: `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`, Status: model.SearchCapabilityStatusDisabled,
	}
	require.NoError(t, model.CreateSearchCapability(oldCapability))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: oldCapability.Id, UpstreamAccountID: account.ID,
		ToolName: "getApiDouyinHotSearchV1", ProviderOperationID: "getApiDouyinHotSearchV1",
		Platform: "douyin", MappingKey: justOneAPIDirectMappingKey,
		Status: model.SearchCapabilityBindingStatusEnabled,
	}))

	_, err = control.SyncCatalog(context.Background())
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CATALOG_SYNC_FAILED", publicErr.Code)

	var oldBinding model.SearchCapabilityBinding
	require.NoError(t, model.DB.Where("capability_id = ? AND provider_operation_id = ?", oldCapability.Id, "getApiDouyinHotSearchV1").First(&oldBinding).Error)
	assert.Equal(t, model.SearchCapabilityBindingStatusDisabled, oldBinding.Status)
	assert.Empty(t, adapter.snapshot.Operations)
}

func TestControlPlaneRejectsPublishingUnverifiedStandardContract(t *testing.T) {
	openRuntimeTestDB(t)
	snapshot := standardProviderCatalog(ProviderTikHub)
	var unverifiedOperationKey string
	for index := range snapshot.Operations {
		if snapshot.Operations[index].ContractEquivalent {
			unverifiedOperationKey = snapshot.Operations[index].OperationKey
			snapshot.Operations[index].ContractEquivalent = false
			snapshot.Operations[index].BillingReady = false
			snapshot.Operations[index].CostAmountMicros = 0
			break
		}
	}
	require.NotEmpty(t, unverifiedOperationKey)
	adapter := &runtimeFakeAdapter{snapshot: snapshot}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) {
		return adapter, nil
	})
	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "tikhub-primary", Provider: ProviderTikHub, Secret: "tk_live_unverified", Status: "healthy",
	})
	require.NoError(t, err)
	_, err = control.SyncCatalog(context.Background())
	require.NoError(t, err)
	capabilities, err := model.ListSearchCapabilities(true)
	require.NoError(t, err)
	require.NotEmpty(t, capabilities)
	serviceID, err := model.GenerateSearchCapabilityPublicID(unverifiedOperationKey + "@v1")
	require.NoError(t, err)
	capability, err := model.GetSearchCapabilityByPublicID(serviceID)
	require.NoError(t, err)

	published, err := control.PublishCatalog(context.Background(), PublishCommand{
		ServiceIDs: []string{capability.PublicID}, AccessMode: PublishAccessAllEnterprises,
	})

	require.NoError(t, err)
	assert.Zero(t, published.Published)
	require.Len(t, published.SkippedServices, 1)
	assert.Equal(t, PublishSkipHealthyRouteUnavailable, published.SkippedServices[0].Reason)
	_, err = control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: capability.Id, Enabled: true, PriceMicros: capability.PriceMicros,
	})
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_CONTRACT_UNVERIFIED", publicErr.Code)
}

func TestExecutionRuntimeRejectsUnadvertisedPlatformBeforeDispatch(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{snapshot: standardProviderCatalog(ProviderTikHub)}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) {
		return adapter, nil
	})
	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "tikhub-primary", Provider: ProviderTikHub, Secret: "tk_live_execute", Status: "healthy",
	})
	require.NoError(t, err)
	_, err = control.SyncCatalog(context.Background())
	require.NoError(t, err)
	serviceID, err := model.GenerateSearchCapabilityPublicID("social.content.search@v1")
	require.NoError(t, err)
	capability, err := model.GetSearchCapabilityByPublicID(serviceID)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
		Update("status", model.SearchCapabilityStatusEnabled).Error)

	_, err = control.runtime.Execute(context.Background(), Principal{UserID: 7, EnterpriseID: 11, AgentKeyID: 21}, ExecuteCommand{
		ServiceID: serviceID, Params: map[string]any{"platform": "tiktok", "query": "AI Agent"},
	})

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INVALID_TOOL_PARAMS", publicErr.Code)
	assert.Zero(t, adapter.executeCalls)
}

func TestControlPlaneUpdateAccountEditsRoutingAndKeepsBlankSecret(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	created, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", BaseURL: "https://api.tikhub.io/old", Secret: "ak_live_original",
		Weight: 2, Priority: 1, Status: "healthy",
	})
	require.NoError(t, err)
	before, err := model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, err)

	secondaryPool := &model.SearchUpstreamPool{Name: "secondary"}
	require.NoError(t, model.CreateSearchUpstreamPool(secondaryPool))
	updated, err := control.SaveAccount(context.Background(), AccountCommand{
		ID: created.ID, Name: "renamed", BaseURL: "https://api.tikhub.io/new",
		PoolID: secondaryPool.Id, Weight: 7, Priority: 3, Status: "paused",
	})
	require.NoError(t, err)
	after, err := model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, err)

	assert.Equal(t, "renamed", updated.Name)
	assert.Equal(t, "https://api.tikhub.io/new", updated.BaseURL)
	assert.Equal(t, secondaryPool.Id, updated.PoolID)
	assert.Equal(t, 7, updated.Weight)
	assert.Equal(t, 3, updated.Priority)
	assert.Equal(t, "paused", updated.Status)
	assert.Equal(t, before.SecretCiphertext, after.SecretCiphertext)
	assert.Equal(t, before.SecretNonce, after.SecretNonce)
	assert.Equal(t, before.SecretVersion, after.SecretVersion)
	assert.Equal(t, before.SecretPrefix, after.SecretPrefix)

	rotated, err := control.SaveAccount(context.Background(), AccountCommand{
		ID: created.ID, Name: updated.Name, BaseURL: updated.BaseURL, Secret: "ak_live_replacement",
		PoolID: updated.PoolID, Weight: updated.Weight, Priority: updated.Priority, Status: updated.Status,
	})
	require.NoError(t, err)
	assert.Equal(t, UpstreamSecretPrefix("ak_live_replacement"), rotated.KeyPrefix)
	assert.NotContains(t, fmt.Sprintf("%+v", rotated), "ak_live_replacement")
}

func TestControlPlaneRequiresNewSecretWhenSwitchingProvider(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	created, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Provider: ProviderTikHub, Secret: "tk_live_original",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/provider-switch", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: created.ID, ToolName: "private/provider-switch",
		MappingKey: "tikhub.direct.v1", InputSchema: capability.InputSchema,
		Status: model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id = ?", created.ID).Updates(map[string]any{
		"plan": "old-plan", "balance_micros": 9_000_000, "balance_currency": "USD",
		"failure_count": 3, "concurrent_requests": 2, "last_checked_at": int64(1234),
		"last_error_code": "OLD_ERROR", "last_error_message": "old provider state",
	}).Error)

	_, err = control.SaveAccount(context.Background(), AccountCommand{
		ID: created.ID, Name: "primary", Provider: ProviderJustOneAPI,
		Weight: 1, Status: "standby",
	})

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_SECRET_REQUIRED", publicErr.Code)
	stored, getErr := model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, getErr)
	assert.Equal(t, ProviderTikHub, stored.Provider)
	bindings, listErr := model.ListSearchCapabilityBindings(capability.Id, true)
	require.NoError(t, listErr)
	require.Len(t, bindings, 1, "rejected provider switch must preserve current routes")

	updated, err := control.SaveAccount(context.Background(), AccountCommand{
		ID: created.ID, Name: "primary", Provider: ProviderJustOneAPI,
		Secret: "jo_live_replacement", Weight: 1, Status: "standby",
	})
	require.NoError(t, err)
	assert.Equal(t, ProviderJustOneAPI, updated.Provider)
	assert.Equal(t, DefaultJustOneAPIBaseURL, updated.BaseURL)
	stored, getErr = model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, getErr)
	assert.Empty(t, stored.Plan)
	assert.Zero(t, stored.BalanceMicros)
	assert.Empty(t, stored.BalanceCurrency)
	assert.Zero(t, stored.FailureCount)
	assert.Zero(t, stored.ConcurrentRequests)
	assert.Zero(t, stored.LastCheckedAt)
	assert.Empty(t, stored.LastErrorCode)
	assert.Empty(t, stored.LastErrorMessage)
	bindings, listErr = model.ListSearchCapabilityBindings(capability.Id, true)
	require.NoError(t, listErr)
	assert.Empty(t, bindings, "successful provider switch must disable routes discovered for the old provider")
}

func TestControlPlaneProbeFailureReturnsSafeError(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{probeErr: errors.New("secret upstream response")}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) { return adapter, nil })
	account, err := control.SaveAccount(context.Background(), AccountCommand{Name: "primary", Secret: "ak_live_probe"})
	require.NoError(t, err)
	_, err = control.ProbeAccount(context.Background(), account.ID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret upstream response")
	stored, getErr := model.GetSearchUpstreamAccountByID(account.ID)
	require.NoError(t, getErr)
	assert.Equal(t, model.SearchUpstreamAccountStatusWarning, stored.Status)
}

func TestControlPlaneUnsupportedProbePreservesHealthyAccountWithoutFailureIncrement(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{probeErr: newConnectorError(
		"UPSTREAM_PROBE_UNSUPPORTED", http.StatusNotImplemented, "该上游不提供无计费健康检查。",
	)}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (ProviderAdapter, error) { return adapter, nil })
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "justone-primary", Provider: ProviderJustOneAPI, Secret: "jo_live_probe", Status: "healthy",
	})
	require.NoError(t, err)

	_, err = control.ProbeAccount(context.Background(), account.ID)

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_PROBE_UNSUPPORTED", publicErr.Code)
	stored, getErr := model.GetSearchUpstreamAccountByID(account.ID)
	require.NoError(t, getErr)
	assert.Equal(t, model.SearchUpstreamAccountStatusHealthy, stored.Status)
	assert.Zero(t, stored.FailureCount)
	assert.Equal(t, "UPSTREAM_PROBE_UNSUPPORTED", stored.LastErrorCode)
	assert.NotEmpty(t, stored.LastErrorMessage)
}
