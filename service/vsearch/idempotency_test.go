package vsearch

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExecutionIdempotencyCachesSanitizedSuccessAndChargesOnce(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{
		"answer": "cached public result", "apiKey": "must-not-be-cached", "tool": "private/search",
	}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "request-2026-08-28-001",
	}
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	first, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)
	second, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)

	assert.Equal(t, 1, adapter.executeCalls)
	assert.Equal(t, first, second)
	quotaAfter, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)
	assert.Equal(t, quotaBefore-wantQuota, quotaAfter, "cache hits must not charge wallet twice")
	var consumeLogCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ?", principal.UserID, model.LogTypeConsume).Count(&consumeLogCount).Error)
	assert.Equal(t, int64(1), consumeLogCount)
	_, usageCount, err := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), usageCount)

	var stored model.SearchExecutionIdempotency
	require.NoError(t, model.DB.First(&stored).Error)
	assert.Equal(t, sha256Hex(command.IdempotencyKey), stored.KeyHash)
	assert.NotEqual(t, command.IdempotencyKey, stored.KeyHash)
	assert.NotContains(t, string(stored.ResponseCiphertext), command.IdempotencyKey)
	assert.NotContains(t, string(stored.ResponseCiphertext), "cached public result")
	assert.NotContains(t, string(stored.ResponseCiphertext), "must-not-be-cached")

	_, err = runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "different"}, IdempotencyKey: command.IdempotencyKey,
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_KEY_REUSED", publicErr.Code)
	assert.Equal(t, 409, publicErr.HTTPStatus)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionIdempotencyPendingNeverDispatchesDuplicateUpstream(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"answer": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "pending-request",
	}
	requestHash, err := hashIdempotentExecutionRequest(command)
	require.NoError(t, err)
	now := int64(1_800_000_000)
	_, state, err := model.BeginSearchExecutionIdempotency(principal.AgentKeyID, sha256Hex(command.IdempotencyKey), requestHash, now, now+86_400)
	require.NoError(t, err)
	require.Equal(t, model.SearchExecutionIdempotencyAcquired, state)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_IN_PROGRESS", publicErr.Code)
	assert.Equal(t, 409, publicErr.HTTPStatus)
	assert.Zero(t, adapter.executeCalls)
}

func TestExecutionIdempotencyUnknownUpstreamOutcomeKeepsKeyAndReservationPending(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{Dispatched: true},
		executeErr:     newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。"),
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "unknown-upstream-outcome",
	}
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	quotaAfterUnknown, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	wantReservedQuota, quotaErr := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaBefore-wantReservedQuota, quotaAfterUnknown, "unknown upstream outcomes must retain the durable reservation")
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.SearchExecutionIdempotency{}).Count(&recordCount).Error)
	assert.Equal(t, int64(1), recordCount)

	adapter.executeErr = nil
	adapter.executeResult = map[string]any{"answer": "retried"}
	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_IN_PROGRESS", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
	quotaAfterBlockedRetry, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaAfterUnknown, quotaAfterBlockedRetry, "blocked duplicate requests must not reserve quota twice")
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.SearchUsageStatusIndeterminate, events[0].Status)
	assert.Equal(t, model.SearchUsagePhaseDispatching, events[0].ExecutionPhase)
	assert.Equal(t, model.SearchUsageBillingReserved, events[0].BillingState)
	assert.Equal(t, capability.PriceMicros, events[0].ChargeMicros)
	assert.Equal(t, wantReservedQuota, events[0].ReservedQuota)
	assert.Equal(t, "VSEARCH_EXECUTION_INDETERMINATE", events[0].ErrorCode)
	assert.Zero(t, events[0].UpstreamCostMicros)
	assert.Positive(t, events[0].PlannedUpstreamCostMicros)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", events[0].Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	quotaAfterRecovery, quotaErr := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, quotaErr)
	assert.Equal(t, quotaAfterUnknown, quotaAfterRecovery, "stale indeterminate reservations require explicit reconciliation")
	require.NoError(t, model.DB.First(&events[0], events[0].Id).Error)
	assert.Equal(t, model.SearchUsageBillingReserved, events[0].BillingState)
	assert.Equal(t, capability.PriceMicros, events[0].ChargeMicros)
	assert.Equal(t, wantReservedQuota, events[0].ReservedQuota)
}

func TestExecutionIdempotencyCompensationFailureKeepsKeyPending(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeErr: newConnectorError("UPSTREAM_UNAVAILABLE", 502, "上游服务暂不可用。")}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	charge := &runtimeFakeCharge{refundErr: errors.New("refund failed"), chargeMicros: capability.PriceMicros}
	runtime.chargeFactory = func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error) {
		return charge, nil
	}
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "compensation-failed",
	}

	_, err := runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_BILLING_COMPENSATION_FAILED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
	assert.Equal(t, 1, charge.refundCalls)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_IN_PROGRESS", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls, "failed compensation must block a second pre-consume and dispatch")
	assert.Equal(t, 1, charge.refundCalls)
}

func TestExecutionIdempotencyKnownBillableFailureResolvesAfterRefundRecovery(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{
		executeAttempt: AttemptResult{
			Dispatched: true, Billable: tikHubBool(true), ProviderRequestID: "provider-refund-recovery-1", CostAmountMicros: 100_000,
		},
		executeErr: newConnectorError("UPSTREAM_CONTRACT_MISMATCH", 502, "上游服务返回的数据不符合能力约定。"),
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "known-refund-recovery",
	}
	var failed atomic.Bool
	callbackName := "test:fail_known_vsearch_wallet_refund"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "wallet_pre_consume_records" && ok && updates["status"] == model.WalletPreConsumeStatusRefunded && failed.CompareAndSwap(false, true) {
			tx.AddError(errors.New("forced wallet refund failure"))
		}
	}))

	_, err := runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_BILLING_COMPENSATION_FAILED", publicErr.Code)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var event model.SearchUsageEvent
	require.NoError(t, model.DB.Where("user_id = ?", principal.UserID).First(&event).Error)
	assert.Equal(t, model.SearchUsageBillingRefundFailed, event.BillingState)
	assert.Equal(t, "provider-refund-recovery-1", event.UpstreamRequestID)
	assert.Equal(t, int64(100_000), event.UpstreamCostMicros)
	var record model.SearchExecutionIdempotency
	require.NoError(t, model.DB.Where("usage_request_id = ?", event.RequestID).First(&record).Error)
	assert.Equal(t, model.SearchExecutionIdempotencyStatusPending, record.Status)

	staleAt := common.GetTimestamp() - model.SearchUsagePendingTimeoutSeconds - 1
	require.NoError(t, model.DB.Model(&model.SearchUsageEvent{}).Where("id = ?", event.Id).Update("updated_at", staleAt).Error)
	require.NoError(t, model.ReconcileStaleSearchUsageEvents(common.GetTimestamp()))
	require.NoError(t, model.DB.First(&event, event.Id).Error)
	assert.Equal(t, model.SearchUsageBillingRefunded, event.BillingState)
	require.NoError(t, model.DB.First(&record, record.Id).Error)
	assert.Equal(t, model.SearchExecutionIdempotencyStatusResolved, record.Status)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_RESOLVED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionIdempotencyCacheRevalidatesCapabilityAuthorization(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"answer": "cached"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "authorization-changed",
	}

	_, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
		Update("status", model.SearchCapabilityStatusDisabled).Error)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_NOT_ALLOWED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionIdempotencyPredispatchFailureReleasesKey(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"answer": "retried"}}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	key := "retry-after-validation-failure"

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": float64(42)}, IdempotencyKey: key,
	})
	require.Error(t, err)
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.SearchExecutionIdempotency{}).Count(&recordCount).Error)
	assert.Zero(t, recordCount)

	result, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: key,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.RequestID)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionIdempotencyCacheFailureDoesNotTurnChargedSuccessIntoFailure(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{executeResult: map[string]any{"answer": "completed"}}
	adapter.executeHook = func() {
		require.NoError(t, os.Unsetenv(UpstreamSecretKeyEnv))
	}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "cache-write-failure",
	}
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	wantQuota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	require.NoError(t, err)

	result, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)
	assert.NotEmpty(t, result.RequestID)
	quotaAfter, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)
	assert.Equal(t, quotaBefore-wantQuota, quotaAfter)
	var record model.SearchExecutionIdempotency
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.SearchExecutionIdempotencyStatusResolved, record.Status, "a charged success without a cache entry must close the key without allowing replay")
	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_RESOLVED", publicErr.Code)
	assert.Equal(t, 1, adapter.executeCalls)
}

func TestExecutionIdempotencyRejectsKeysLongerThan128Characters(t *testing.T) {
	openRuntimeTestDB(t)
	adapter := &runtimeFakeAdapter{}
	runtime, principal, capability := seedRuntimeExecution(t, adapter)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: strings.Repeat("密", 129),
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_KEY_TOO_LONG", publicErr.Code)
	assert.Equal(t, 400, publicErr.HTTPStatus)
	assert.Zero(t, adapter.executeCalls)
}
