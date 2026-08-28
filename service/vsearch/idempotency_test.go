package vsearch

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionIdempotencyCachesSanitizedSuccessAndChargesOnce(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{
		"answer": "cached public result", "apiKey": "must-not-be-cached", "tool": "private/search",
	}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "request-2026-08-28-001",
	}
	quotaBefore, err := model.GetUserQuota(principal.UserID, true)
	require.NoError(t, err)

	first, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)
	second, err := runtime.Execute(context.Background(), principal, command)
	require.NoError(t, err)

	assert.Equal(t, 1, connector.executeCalls)
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
	assert.Equal(t, 1, connector.executeCalls)
}

func TestExecutionIdempotencyPendingNeverDispatchesDuplicateUpstream(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"answer": "must not run"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Zero(t, connector.executeCalls)
}

func TestExecutionIdempotencyDispatchedFailureKeepsKeyPending(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeErr: markExecutionDispatched(newConnectorError("UPSTREAM_TIMEOUT", 504, "上游服务响应超时。"))}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
	command := ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: "retry-after-failure",
	}

	_, err := runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.SearchExecutionIdempotency{}).Count(&recordCount).Error)
	assert.Equal(t, int64(1), recordCount)

	connector.executeErr = nil
	connector.executeResult = map[string]any{"answer": "retried"}
	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_IN_PROGRESS", publicErr.Code)
	assert.Equal(t, 1, connector.executeCalls)
	events, total, listErr := model.ListSearchUsageEvents(model.SearchUsageQuery{UserID: principal.UserID})
	require.NoError(t, listErr)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.SearchUsageStatusIndeterminate, events[0].Status)
	assert.Equal(t, model.SearchUsagePhaseDispatching, events[0].ExecutionPhase)
	assert.Equal(t, model.SearchUsageBillingRefunded, events[0].BillingState)
	assert.Zero(t, events[0].UpstreamCostMicros)
	assert.Positive(t, events[0].PlannedUpstreamCostMicros)
}

func TestExecutionIdempotencyCompensationFailureKeepsKeyPending(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeErr: newConnectorError("UPSTREAM_UNAVAILABLE", 502, "上游服务暂不可用。")}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, 1, connector.executeCalls)
	assert.Equal(t, 1, charge.refundCalls)

	_, err = runtime.Execute(context.Background(), principal, command)
	require.Error(t, err)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_REQUEST_IN_PROGRESS", publicErr.Code)
	assert.Equal(t, 1, connector.executeCalls, "failed compensation must block a second pre-consume and dispatch")
	assert.Equal(t, 1, charge.refundCalls)
}

func TestExecutionIdempotencyCacheRevalidatesCapabilityAuthorization(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"answer": "cached"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, 1, connector.executeCalls)
}

func TestExecutionIdempotencyPredispatchFailureReleasesKey(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"answer": "retried"}}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, 1, connector.executeCalls)
}

func TestExecutionIdempotencyCacheFailureDoesNotTurnChargedSuccessIntoFailure(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{executeResult: map[string]any{"answer": "completed"}}
	connector.executeHook = func() {
		require.NoError(t, os.Unsetenv(UpstreamSecretKeyEnv))
	}
	runtime, principal, capability := seedRuntimeExecution(t, connector)
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
	assert.Equal(t, model.SearchExecutionIdempotencyStatusPending, record.Status, "pending blocks a duplicate charge when caching fails")
}

func TestExecutionIdempotencyRejectsKeysLongerThan128Characters(t *testing.T) {
	openRuntimeTestDB(t)
	connector := &runtimeFakeConnector{}
	runtime, principal, capability := seedRuntimeExecution(t, connector)

	_, err := runtime.Execute(context.Background(), principal, ExecuteCommand{
		ServiceID: capability.PublicID, Params: map[string]any{"query": "news"}, IdempotencyKey: strings.Repeat("密", 129),
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "IDEMPOTENCY_KEY_TOO_LONG", publicErr.Code)
	assert.Equal(t, 400, publicErr.HTTPStatus)
	assert.Zero(t, connector.executeCalls)
}
