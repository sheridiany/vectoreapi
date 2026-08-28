package vsearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

type runtimeFakeConnector struct {
	findResult     any
	findErrByQuery map[string]error
	describeResult any
	executeResult  any
	accountResult  any
	accountErr     error
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

func (connector *runtimeFakeConnector) Account(context.Context) (any, error) {
	if connector.accountErr != nil {
		return nil, connector.accountErr
	}
	if connector.accountResult != nil {
		return connector.accountResult, nil
	}
	return map[string]any{"plan": "test", "balance": float64(10)}, nil
}

func (connector *runtimeFakeConnector) FindTools(_ context.Context, query string, _ string) (any, error) {
	if err := connector.findErrByQuery[query]; err != nil {
		return nil, err
	}
	return connector.findResult, nil
}

func (connector *runtimeFakeConnector) DescribeTool(context.Context, string) (any, error) {
	return connector.describeResult, nil
}

func (connector *runtimeFakeConnector) ExecuteTool(context.Context, string, map[string]any) (any, error) {
	connector.executeCalls++
	if connector.executeStarted != nil {
		select {
		case connector.executeStarted <- struct{}{}:
		default:
		}
	}
	if connector.executeRelease != nil {
		<-connector.executeRelease
	}
	if connector.executeHook != nil {
		connector.executeHook()
	}
	return connector.executeResult, connector.executeErr
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
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
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

func seedRuntimeExecution(t *testing.T, connector *runtimeFakeConnector) (*ExecutionRuntime, Principal, *model.SearchCapability) {
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
		PoolID: pool.Id, Name: "primary", BaseURL: DefaultAgentKeyMCPURL,
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
	runtime := NewExecutionRuntime(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })
	return runtime, Principal{UserID: 7, EnterpriseID: 11, AgentKeyID: 21, Scopes: []string{"web-search"}}, capability
}

func TestExecutionRuntimeRejectsInvalidParamsBeforeDispatchOrUsage(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{}
	runtime, principal, capability := seedRuntimeExecution(t, connector)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": float64(42)}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INVALID_TOOL_PARAMS", publicErr.Code)
	assert.Zero(t, connector.executeCalls)
	_, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	events, _, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	assert.Equal(t, model.SearchUsageStatusFailed, events[0].Status)
	assert.Equal(t, "INVALID_TOOL_PARAMS", events[0].ErrorCode)
}

func TestExecutionRuntimeDispatchesOnceSanitizesAndRecordsExactMicros(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{
		"data":   []any{map[string]any{"title": "Public result", "toolName": "private/search"}},
		"apiKey": "must-not-leak", "note": "AgentKey internal",
	}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)

	result, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, connector.executeCalls)
	payload, err := common.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "must-not-leak")
	assert.NotContains(t, string(payload), "private/search")
	assert.NotContains(t, strings.ToLower(string(payload)), "agentkey")

	events, total, err := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	assert.Equal(t, int64(100_000), events[0].UpstreamCostMicros)
	assert.Equal(t, int64(250_000), events[0].ChargeMicros)
	assert.Equal(t, model.SearchUsageStatusSucceeded, events[0].Status)

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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	connector.executeHook = func() {
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
	connector := &runtimeFakeConnector{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	assert.Equal(t, 1, connector.executeCalls)
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
	assert.Zero(t, events[0].ChargeMicros, "a fully refunded failure must not be reported as charged")
}

func TestExecutionRuntimeChargesWalletOnlyAfterSuccessfulCall(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, connector.executeCalls)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", principal.UserID).Update("quota", wantQuota-1).Error)

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "INSUFFICIENT_QUOTA", publicErr.Code)
	assert.Zero(t, connector.executeCalls)
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
	connector := &runtimeFakeConnector{}
	_, _, capability := seedRuntimeExecution(t, connector)
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").Update("weight", 10).Error)

	binding, account, err := selectExecutionTarget(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, capability.Id, binding.CapabilityID)
	assert.Equal(t, 10, account.Weight)
}

func TestExecutionRuntimeCountsFreeSuccessWithoutConsumingQuota(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).Update("price_micros", 0).Error)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, 1, connector.executeCalls)
	require.NoError(t, model.DB.First(subscription, subscription.Id).Error)
	assert.Equal(t, int64(wantQuota), subscription.AmountUsed)
	walletQuota, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, walletQuota, "subscription billing must not consume wallet quota")
}

func TestExecutionRuntimeRefundsSubscriptionOnUpstreamFailure(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, 1, connector.executeCalls)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Zero(t, connector.executeCalls)
	require.NoError(t, model.DB.First(subscription, subscription.Id).Error)
	assert.Zero(t, subscription.AmountUsed)
}

func TestExecutionRuntimeRefundsWhenSettlementFailsAfterUpstreamSuccess(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	charge := &runtimeFakeCharge{commitErr: errors.New("settlement failed"), chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}})
	require.Error(t, err)
	assert.Equal(t, 1, connector.executeCalls)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "must not dispatch"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Zero(t, connector.executeCalls)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	connector := &runtimeFakeConnector{executeResult: map[string]any{"result": "ok"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)

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
	connector := &runtimeFakeConnector{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	connector := &runtimeFakeConnector{executeErr: newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。")}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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

func TestControlPlaneSyncCreatesPublicCapabilityWithoutExposingToolName(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{
		findResult:     map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"tools":[{"name":"private/search_news","title":"News Search","description":"Find current news"}]}`}}},
		describeResult: map[string]any{"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []any{"query"}}, "cost": float64(0.2)},
	}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })
	account, err := control.SaveAccount(context.Background(), AccountCommand{Name: "primary", Secret: "ak_live_sync", Status: "healthy"})
	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v", account), "ak_live_sync")

	result, err := control.SyncCatalog(context.Background(), SyncCommand{Queries: []string{"news"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Synced)
	capabilities, err := model.ListSearchCapabilities(true)
	require.NoError(t, err)
	require.Len(t, capabilities, 1)
	assert.Equal(t, model.SearchCapabilityStatusDisabled, capabilities[0].Status, "newly synchronized capabilities require explicit admin enablement")
	assert.Regexp(t, `^vr_svc_[a-f0-9]{16}$`, capabilities[0].PublicID)
	assert.NotEqual(t, "private/search_news", capabilities[0].Name)
	bindings, err := model.ListSearchCapabilityBindings(capabilities[0].Id, true)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, "private/search_news", bindings[0].ToolName)
}

func TestControlPlaneSyncDoesNotPersistUpstreamSecretsFromResponses(t *testing.T) {
	openRuntimeTestDB(t)
	const upstreamSecret = "ak_live_sync_sensitive_value"
	connector := &runtimeFakeConnector{
		findResult: map[string]any{"tools": []any{map[string]any{
			"name": "private/search_secret", "title": "Search " + upstreamSecret,
			"description": "uses " + upstreamSecret,
		}}},
		describeResult: map[string]any{
			"description": "credential " + upstreamSecret,
			"inputSchema": map[string]any{
				"type": "object", "properties": map[string]any{
					"query": map[string]any{"type": "string", "default": upstreamSecret},
				},
			},
		},
		accountResult: map[string]any{"plan": "plan-" + upstreamSecret, "balance": float64(3)},
	}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", BaseURL: "https://upstream.example/v1/mcp", Secret: upstreamSecret, Status: "healthy",
	})
	require.NoError(t, err)

	account, err = control.ProbeAccount(context.Background(), account.ID)
	require.NoError(t, err)
	assert.NotContains(t, account.Plan, upstreamSecret)

	_, err = control.SyncCatalog(context.Background(), SyncCommand{Queries: []string{"secret"}})
	require.NoError(t, err)
	var capability model.SearchCapability
	require.NoError(t, model.DB.First(&capability).Error)
	capabilityData, err := common.Marshal(capability)
	require.NoError(t, err)
	assert.NotContains(t, string(capabilityData), upstreamSecret)
	assert.NotContains(t, capability.InputSchema, upstreamSecret)

	catalog, err := control.runtime.ListCatalog(context.Background(), Principal{}, true)
	require.NoError(t, err)
	catalogData, err := common.Marshal(catalog)
	require.NoError(t, err)
	assert.NotContains(t, string(catalogData), upstreamSecret)
}

func TestControlPlanePartialSyncKeepsPreviouslyHealthyBindings(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{
		findResult:     map[string]any{"tools": []any{map[string]any{"name": "private/new_search", "title": "New Search"}}},
		findErrByQuery: map[string]error{"broken": errors.New("temporary discovery failure")},
	}
	_, _, existingCapability := seedRuntimeExecution(t, connector)
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })

	result, err := control.SyncCatalog(context.Background(), SyncCommand{Queries: []string{"news", "broken"}})
	require.NoError(t, err)
	assert.Positive(t, result.Synced)
	assert.NotEmpty(t, result.Failures)
	bindings, err := model.ListSearchCapabilityBindings(existingCapability.Id, true)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, model.SearchCapabilityBindingStatusEnabled, bindings[0].Status)
}

func TestControlPlaneTargetedSyncNeverDisablesUnreturnedBindings(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{
		findResult: map[string]any{"tools": []any{map[string]any{"name": "private/new_search", "title": "New Search"}}},
	}
	_, _, existingCapability := seedRuntimeExecution(t, connector)
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })

	result, err := control.SyncCatalog(context.Background(), SyncCommand{Queries: []string{"news"}, Prefix: "private/new"})
	require.NoError(t, err)
	assert.Positive(t, result.Synced)
	bindings, err := model.ListSearchCapabilityBindings(existingCapability.Id, true)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, model.SearchCapabilityBindingStatusEnabled, bindings[0].Status)
}

func TestControlPlaneDiscoverySyncNeverTreatsSearchResultsAsAuthoritativeDeletion(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{
		findResult: map[string]any{"tools": []any{map[string]any{"name": "private/new_search", "title": "New Search"}}},
	}
	_, _, existingCapability := seedRuntimeExecution(t, connector)
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })

	result, err := control.SyncCatalog(context.Background(), SyncCommand{})
	require.NoError(t, err)
	assert.Positive(t, result.Synced)
	bindings, err := model.ListSearchCapabilityBindings(existingCapability.Id, true)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, model.SearchCapabilityBindingStatusEnabled, bindings[0].Status)
}

func TestControlPlaneUpdateAccountEditsRoutingAndKeepsBlankSecret(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	created, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", BaseURL: "https://old.example.com/v1/mcp", Secret: "ak_live_original",
		Weight: 2, Priority: 1, Status: "healthy",
	})
	require.NoError(t, err)
	before, err := model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, err)

	secondaryPool := &model.SearchUpstreamPool{Name: "secondary"}
	require.NoError(t, model.CreateSearchUpstreamPool(secondaryPool))
	updated, err := control.SaveAccount(context.Background(), AccountCommand{
		ID: created.ID, Name: "renamed", BaseURL: "https://new.example.com/v1/mcp",
		PoolID: secondaryPool.Id, Weight: 7, Priority: 3, Status: "paused",
	})
	require.NoError(t, err)
	after, err := model.GetSearchUpstreamAccountByID(created.ID)
	require.NoError(t, err)

	assert.Equal(t, "renamed", updated.Name)
	assert.Equal(t, "https://new.example.com/v1/mcp", updated.BaseURL)
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

func TestPublicToolNameRejectsPrivateIdentifierDisguisedAsTitle(t *testing.T) {
	name := publicToolName(discoveredTool{Name: "private/search_news", Title: "private_search_news"})
	assert.NotEqual(t, "private_search_news", name)
	assert.NotContains(t, strings.ToLower(name), "search news")
}

func TestControlPlaneProbeFailureReturnsSafeError(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{accountErr: errors.New("secret upstream response")}
	control := NewControlPlane(func(*model.SearchUpstreamAccount, string) (UpstreamConnector, error) { return connector, nil })
	account, err := control.SaveAccount(context.Background(), AccountCommand{Name: "primary", Secret: "ak_live_probe"})
	require.NoError(t, err)
	_, err = control.ProbeAccount(context.Background(), account.ID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret upstream response")
	stored, getErr := model.GetSearchUpstreamAccountByID(account.ID)
	require.NoError(t, getErr)
	assert.Equal(t, model.SearchUpstreamAccountStatusWarning, stored.Status)
}
