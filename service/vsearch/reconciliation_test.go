package vsearch

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileUsageValidatesContractAndMapsStateConflicts(t *testing.T) {
	openRuntimeTestDB(t)
	_, principal, capability := seedRuntimeExecution(t, &runtimeFakeAdapter{})

	_, err := ReconcileUsage(context.Background(), UsageReconciliationCommand{
		EventID: 1, Action: model.SearchUsageReconciliationRefund, AdminID: 99, Note: strings.Repeat("x", 256),
	})
	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_RECONCILIATION_INVALID", publicErr.Code)
	assert.Equal(t, 400, publicErr.HTTPStatus)

	event := &model.SearchUsageEvent{
		RequestID: "service-reconciliation-conflict", UserID: principal.UserID, EnterpriseID: principal.EnterpriseID,
		AgentKeyID: principal.AgentKeyID, CapabilityID: capability.Id,
		ServiceID: capability.PublicID, ServiceName: capability.Name, Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusIndeterminate, ExecutionPhase: model.SearchUsagePhaseDispatching,
		BillingState:         model.SearchUsageBillingRefunded,
		ReconciliationAction: model.SearchUsageReconciliationRefund,
		ReconciledBy:         98, ReconciledAt: 1_800_000_000, ReconciliationNote: "already refunded",
	}
	require.NoError(t, model.CreateSearchUsageEvent(event))

	_, err = ReconcileUsage(context.Background(), UsageReconciliationCommand{
		EventID: event.Id, Action: model.SearchUsageReconciliationSettle, AdminID: 99, Note: "conflicting decision",
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "VSEARCH_RECONCILIATION_CONFLICT", publicErr.Code)
	assert.Equal(t, 409, publicErr.HTTPStatus)
}
