package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type billingSessionRefundTestFunding struct {
	settleErr   error
	refundErr   error
	refundCalls atomic.Int32
	refunded    chan struct{}
}

func (*billingSessionRefundTestFunding) Source() string           { return BillingSourceWallet }
func (*billingSessionRefundTestFunding) PreConsume(int) error     { return nil }
func (funding *billingSessionRefundTestFunding) Settle(int) error { return funding.settleErr }
func (funding *billingSessionRefundTestFunding) Refund() error {
	if funding.refundCalls.Add(1) == 1 && funding.refunded != nil {
		close(funding.refunded)
	}
	return funding.refundErr
}

func TestBillingSessionRefundNowCompensatesFailedSettlement(t *testing.T) {
	funding := &billingSessionRefundTestFunding{settleErr: errors.New("settlement failed")}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 7, IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 100,
		tokenConsumed:    100,
	}

	require.Error(t, session.Settle(120))
	assert.True(t, session.NeedsRefund())
	require.NoError(t, session.RefundNow(context.Background()))
	assert.Equal(t, int32(1), funding.refundCalls.Load())
	assert.False(t, session.NeedsRefund())
	require.NoError(t, session.RefundNow(context.Background()))
	assert.Equal(t, int32(1), funding.refundCalls.Load(), "synchronous refund must remain idempotent")
}

func TestBillingSessionRefundNowCanRetryFailedPlaygroundCompensation(t *testing.T) {
	funding := &billingSessionRefundTestFunding{refundErr: errors.New("temporary refund failure")}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 7, IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 100,
		tokenConsumed:    100,
	}

	require.Error(t, session.RefundNow(context.Background()))
	assert.True(t, session.NeedsRefund(), "failed synchronous compensation must remain retryable")
	funding.refundErr = nil
	require.NoError(t, session.RefundNow(context.Background()))
	assert.Equal(t, int32(2), funding.refundCalls.Load())
	assert.False(t, session.NeedsRefund())
}

func TestBillingSessionRefundKeepsAsyncContractAndRunsOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	funding := &billingSessionRefundTestFunding{refunded: make(chan struct{})}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 7, IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 100,
		tokenConsumed:    100,
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	session.Refund(ctx)
	select {
	case <-funding.refunded:
	case <-time.After(time.Second):
		t.Fatal("asynchronous refund did not run")
	}
	session.Refund(ctx)
	assert.Equal(t, int32(1), funding.refundCalls.Load())
}
