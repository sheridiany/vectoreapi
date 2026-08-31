package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchExecutionIdempotencyLifecycleAndRequestConflict(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.AutoMigrate(&SearchExecutionIdempotency{}))
	const (
		agentKeyID = 17
		keyHash    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		requestOne = "1111111111111111111111111111111111111111111111111111111111111111"
		requestTwo = "2222222222222222222222222222222222222222222222222222222222222222"
		now        = int64(1_800_000_000)
	)

	record, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now, now+86_400)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NotZero(t, record.Id)
	require.Len(t, record.ClaimToken, 64)

	_, state, err = BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now+1, now+86_401)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyPending, state)

	_, state, err = BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestTwo, now+1, now+86_401)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyConflict, state)

	require.Error(t, CompleteSearchExecutionIdempotency(record.Id, requestOne, strings.Repeat("f", 64), "encrypted-response", "nonce", 1))
	require.NoError(t, CompleteSearchExecutionIdempotency(record.Id, requestOne, record.ClaimToken, "encrypted-response", "nonce", 1))
	cached, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now+2, now+86_402)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyCached, state)
	assert.Equal(t, "encrypted-response", string(cached.ResponseCiphertext))

	reused, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestTwo, now+86_401, now+172_801)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	assert.Equal(t, requestTwo, reused.RequestHash)
}

func TestSearchExecutionIdempotencyFailureReleaseAndUnattachedPendingExpiryReclaims(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.AutoMigrate(&SearchExecutionIdempotency{}))
	const (
		agentKeyID = 18
		keyHash    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		requestOne = "3333333333333333333333333333333333333333333333333333333333333333"
		requestTwo = "4444444444444444444444444444444444444444444444444444444444444444"
		now        = int64(1_800_000_000)
	)

	record, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now, now+86_400)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, ReleaseSearchExecutionIdempotency(record.Id, requestOne, record.ClaimToken))

	retry, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now+1, now+86_401)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)

	expired, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now+86_402, now+172_802)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	assert.Equal(t, retry.Id, expired.Id)
	assert.Equal(t, requestOne, expired.RequestHash)

	expired, state, err = BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestTwo, now+86_402, now+172_802)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyConflict, state)
	assert.Equal(t, retry.Id, expired.Id)
	assert.Equal(t, requestOne, expired.RequestHash)
}

func TestSearchExecutionIdempotencyExpiredPendingReclaimsOnlyConfirmedPredispatchUsage(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.AutoMigrate(&SearchExecutionIdempotency{}))
	const (
		agentKeyID  = 19
		requestHash = "5555555555555555555555555555555555555555555555555555555555555555"
		now         = int64(1_800_000_000)
	)

	dispatched := &SearchUsageEvent{
		RequestID: "vsearch-idempotency-dispatched", UserID: 7, EnterpriseID: 11, AgentKeyID: agentKeyID,
		ServiceID: "vr_dispatched", ServiceName: "Dispatched", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusIndeterminate, ExecutionPhase: SearchUsagePhaseDispatching,
		BillingState: SearchUsageBillingReserved,
	}
	require.NoError(t, CreateSearchUsageEvent(dispatched))
	dispatchedRecord, state, err := BeginSearchExecutionIdempotency(agentKeyID, strings.Repeat("c", 64), requestHash, now, now+100)
	require.NoError(t, err)
	require.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, AttachSearchExecutionUsage(dispatchedRecord.Id, requestHash, dispatchedRecord.ClaimToken, dispatched.RequestID))

	_, state, err = BeginSearchExecutionIdempotency(agentKeyID, strings.Repeat("c", 64), requestHash, now+101, now+201)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyPending, state, "an unknown dispatched request must never be replayed automatically")

	predispatch := &SearchUsageEvent{
		RequestID: "vsearch-idempotency-predispatch", UserID: 7, EnterpriseID: 11, AgentKeyID: agentKeyID,
		ServiceID: "vr_predispatch", ServiceName: "Predispatch", Action: SearchUsageActionExecute,
		Status: SearchUsageStatusFailed, ExecutionPhase: SearchUsagePhasePrepared,
		BillingState: SearchUsageBillingRefunded,
	}
	require.NoError(t, CreateSearchUsageEvent(predispatch))
	predispatchRecord, state, err := BeginSearchExecutionIdempotency(agentKeyID, strings.Repeat("d", 64), requestHash, now, now+100)
	require.NoError(t, err)
	require.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, AttachSearchExecutionUsage(predispatchRecord.Id, requestHash, predispatchRecord.ClaimToken, predispatch.RequestID))

	reclaimed, state, err := BeginSearchExecutionIdempotency(agentKeyID, strings.Repeat("d", 64), requestHash, now+101, now+201)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	assert.Empty(t, reclaimed.UsageRequestID)
}

func TestSearchExecutionIdempotencyDoesNotReclaimLegacyUntrackedPendingRows(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.AutoMigrate(&SearchExecutionIdempotency{}))
	const (
		now         = int64(1_800_000_000)
		agentKeyID  = 20
		keyHash     = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		requestHash = "6666666666666666666666666666666666666666666666666666666666666666"
	)
	legacy := &SearchExecutionIdempotency{
		AgentKeyID: agentKeyID, KeyHash: keyHash, RequestHash: requestHash,
		ClaimToken: strings.Repeat("f", 64), Status: SearchExecutionIdempotencyStatusPending,
		ExpiresAt: now - 1, UsageTrackingVersion: 0,
	}
	require.NoError(t, DB.Create(legacy).Error)

	stored, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestHash, now, now+100)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyPending, state)
	assert.Equal(t, legacy.ClaimToken, stored.ClaimToken)
	assert.Zero(t, stored.UsageTrackingVersion)
}

func TestSearchExecutionIdempotencyResolvedKeyExpires(t *testing.T) {
	db := openSearchDataTestDB(t)
	require.NoError(t, db.AutoMigrate(&SearchExecutionIdempotency{}))
	const (
		agentKeyID = 21
		keyHash    = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		requestOne = "7777777777777777777777777777777777777777777777777777777777777777"
		requestTwo = "8888888888888888888888888888888888888888888888888888888888888888"
		now        = int64(1_800_000_000)
	)

	record, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now, now+100)
	require.NoError(t, err)
	require.Equal(t, SearchExecutionIdempotencyAcquired, state)
	require.NoError(t, AttachSearchExecutionUsage(record.Id, requestOne, record.ClaimToken, "vsearch-resolved-expiry"))
	require.NoError(t, ResolveSearchExecutionIdempotencyByUsageRequestID("vsearch-resolved-expiry"))

	_, state, err = BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestOne, now+99, now+199)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyResolved, state)

	reclaimed, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestTwo, now+101, now+201)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	assert.Equal(t, requestTwo, reclaimed.RequestHash)
	assert.Empty(t, reclaimed.UsageRequestID)
}
