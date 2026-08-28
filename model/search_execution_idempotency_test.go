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
}

func TestSearchExecutionIdempotencyFailureReleaseAndExpiryAllowRetry(t *testing.T) {
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

	expired, state, err := BeginSearchExecutionIdempotency(agentKeyID, keyHash, requestTwo, now+86_402, now+172_802)
	require.NoError(t, err)
	assert.Equal(t, SearchExecutionIdempotencyAcquired, state)
	assert.Equal(t, retry.Id, expired.Id)
	assert.Equal(t, requestTwo, expired.RequestHash)
}
