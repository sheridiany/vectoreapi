package vsearch

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlanePublishCatalogPublishesEligibleCapabilitiesAtCostFloorForAllEnterprises(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Secret: "ak_live_publish", Status: "healthy",
	})
	require.NoError(t, err)

	eligible := createPublishCapability(t, "private/eligible", `{"type":"object","properties":{"query":{"type":"string"}}}`, 300_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: eligible.Id, UpstreamAccountID: account.ID, ToolName: "private/eligible",
		InputSchema: eligible.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 400_000,
	}))
	require.NoError(t, model.ReplaceSearchCapabilityEnterpriseGrants(eligible.Id, []int{11}))

	missingSchema := createPublishCapability(t, "private/missing-schema", "", 200_000, 200_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: missingSchema.Id, UpstreamAccountID: account.ID, ToolName: "private/missing-schema",
		Status: model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 200_000,
	}))

	result, err := control.PublishCatalog(context.Background(), PublishCommand{
		ServiceIDs: []string{eligible.PublicID, missingSchema.PublicID},
		AccessMode: PublishAccessAllEnterprises,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Published)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, []string{eligible.PublicID}, result.PublishedServiceIDs)
	require.Len(t, result.SkippedServices, 1)
	assert.Equal(t, missingSchema.PublicID, result.SkippedServices[0].ServiceID)
	assert.Equal(t, PublishSkipSchemaUnavailable, result.SkippedServices[0].Reason)

	storedEligible, err := model.GetSearchCapabilityByID(eligible.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusEnabled, storedEligible.Status)
	assert.Equal(t, int64(400_000), storedEligible.PriceMicros)
	grants, err := model.ListSearchCapabilityGrants(eligible.Id)
	require.NoError(t, err)
	assert.Empty(t, grants)

	storedMissingSchema, err := model.GetSearchCapabilityByID(missingSchema.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusDisabled, storedMissingSchema.Status)
}

func TestControlPlanePublishCatalogSkipsCapabilityWithoutHealthyRoute(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "standby", Secret: "ak_live_standby", Status: "standby",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/no-route", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/no-route",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))

	result, err := control.PublishCatalog(context.Background(), PublishCommand{
		ServiceIDs: []string{capability.PublicID}, AccessMode: PublishAccessAllEnterprises,
	})

	require.NoError(t, err)
	assert.Zero(t, result.Published)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.SkippedServices, 1)
	assert.Equal(t, PublishSkipHealthyRouteUnavailable, result.SkippedServices[0].Reason)
	stored, err := model.GetSearchCapabilityByID(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusDisabled, stored.Status)
}

func TestControlPlanePublishCatalogSkipsHealthyRouteWithStaleBindingSchema(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "recovered", Secret: "ak_live_recovered", Status: "standby",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/stale-route", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/stale-route",
		InputSchema: `{"type":"object","properties":{"stale":{"type":"string"}}}`,
		Status:      model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id = ?", account.ID).
		Update("status", model.SearchUpstreamAccountStatusHealthy).Error)

	result, err := control.PublishCatalog(context.Background(), PublishCommand{
		ServiceIDs: []string{capability.PublicID}, AccessMode: PublishAccessAllEnterprises,
	})

	require.NoError(t, err)
	assert.Zero(t, result.Published)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.SkippedServices, 1)
	assert.Equal(t, PublishSkipHealthyRouteUnavailable, result.SkippedServices[0].Reason)
}

func TestPublishSearchCapabilitiesPreservesHigherCurrentPriceAndIsIdempotent(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Secret: "ak_live_concurrent", Status: "healthy",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/concurrent-price", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/concurrent-price",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
		Update("price_micros", int64(400_000)).Error)
	config := []model.SearchCapabilityPublishConfig{{
		ID: capability.Id, PriceMicros: 200_000,
		ExpectedInputSchema: capability.InputSchema, ExpectedSchemaStatus: model.SearchCapabilitySchemaAvailable,
	}}

	require.NoError(t, model.PublishSearchCapabilities(config, true))
	require.NoError(t, model.PublishSearchCapabilities(config, true))

	stored, err := model.GetSearchCapabilityByID(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusEnabled, stored.Status)
	assert.Equal(t, int64(400_000), stored.PriceMicros)
}

func TestPublishSearchCapabilitiesRollsBackWhenEligibilityChanges(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Secret: "ak_live_changed", Status: "healthy",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/changed", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/changed",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.ReplaceSearchCapabilityEnterpriseGrants(capability.Id, []int{11}))
	config := []model.SearchCapabilityPublishConfig{{
		ID: capability.Id, PriceMicros: 100_000,
		ExpectedInputSchema: capability.InputSchema, ExpectedSchemaStatus: model.SearchCapabilitySchemaAvailable,
	}}
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).Updates(map[string]any{
		"input_schema": "", "schema_status": model.SearchCapabilitySchemaUnavailable,
	}).Error)

	err = model.PublishSearchCapabilities(config, true)

	assert.ErrorIs(t, err, model.ErrSearchCapabilityPublishStateChanged)
	stored, getErr := model.GetSearchCapabilityByID(capability.Id)
	require.NoError(t, getErr)
	assert.Equal(t, model.SearchCapabilityStatusDisabled, stored.Status)
	grants, grantErr := model.ListSearchCapabilityGrants(capability.Id)
	require.NoError(t, grantErr)
	assert.Len(t, grants, 1)
}

func TestPublishSearchCapabilitiesIgnoresStaleSchemaBindingWhenMatchingRouteExists(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	matchingAccount, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "matching", Secret: "ak_live_matching", Status: "healthy",
	})
	require.NoError(t, err)
	staleAccount, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "stale", Secret: "ak_live_stale", Status: "healthy",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/mixed-schema", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: matchingAccount.ID, ToolName: "private/mixed-schema",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: staleAccount.ID, ToolName: "private/mixed-schema",
		InputSchema: `{"type":"object","properties":{"stale":{"type":"string"}}}`,
		Status:      model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 500_000,
	}))
	config := []model.SearchCapabilityPublishConfig{{
		ID: capability.Id, PriceMicros: 100_000,
		ExpectedInputSchema: capability.InputSchema, ExpectedSchemaStatus: model.SearchCapabilitySchemaAvailable,
	}}

	require.NoError(t, model.PublishSearchCapabilities(config, true))
	stored, err := model.GetSearchCapabilityByID(capability.Id)
	require.NoError(t, err)
	assert.Equal(t, model.SearchCapabilityStatusEnabled, stored.Status)
	assert.Equal(t, int64(100_000), stored.PriceMicros)
}

func TestControlPlaneConfigureCapabilityEnforcesSchemaRouteAndCostFloor(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Secret: "ak_live_configure", Status: "healthy",
	})
	require.NoError(t, err)

	missingSchema := createPublishCapability(t, "private/configure-no-schema", "", 100_000, 100_000)
	_, err = control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: missingSchema.Id, Enabled: true, PriceMicros: 100_000,
	})
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_SCHEMA_UNAVAILABLE", publicErr.Code)

	capability := createPublishCapability(t, "private/configure-cost", `{"type":"object"}`, 400_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/configure-cost",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 400_000,
	}))
	_, err = control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: capability.Id, Enabled: true, PriceMicros: 100_000,
	})
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_PRICE_BELOW_COST", publicErr.Code)

	configured, err := control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: capability.Id, Enabled: true, PriceMicros: 400_000,
	})
	require.NoError(t, err)
	assert.True(t, configured.Enabled)
}

func TestControlPlaneConfigureCapabilityIgnoresMismatchedBindingCostFloor(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	matchingAccount, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "matching", Secret: "ak_live_configure_matching", Status: "healthy",
	})
	require.NoError(t, err)
	staleAccount, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "stale", Secret: "ak_live_configure_stale", Status: "healthy",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/configure-mixed-schema", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: matchingAccount.ID, ToolName: "private/configure-mixed-schema",
		InputSchema: capability.InputSchema, Status: model.SearchCapabilityBindingStatusEnabled,
		UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: staleAccount.ID, ToolName: "private/configure-mixed-schema",
		InputSchema: `{"type":"object","properties":{"stale":{"type":"string"}}}`,
		Status:      model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 500_000,
	}))

	configured, err := control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: capability.Id, Enabled: true, PriceMicros: 100_000,
	})

	require.NoError(t, err)
	assert.True(t, configured.Enabled)
	assert.Equal(t, int64(100_000), configured.PriceMicros)
}

func TestControlPlaneConfigureCapabilityRejectsRecoveredRouteWithStaleBindingSchema(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)
	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "recovered", Secret: "ak_live_configure_recovered", Status: "standby",
	})
	require.NoError(t, err)
	capability := createPublishCapability(t, "private/configure-stale", `{"type":"object"}`, 100_000, 100_000)
	require.NoError(t, model.UpsertSearchCapabilityBinding(&model.SearchCapabilityBinding{
		CapabilityID: capability.Id, UpstreamAccountID: account.ID, ToolName: "private/configure-stale",
		InputSchema: `{"type":"object","properties":{"stale":{"type":"string"}}}`,
		Status:      model.SearchCapabilityBindingStatusEnabled, UpstreamCostMicros: 100_000,
	}))
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id = ?", account.ID).
		Update("status", model.SearchUpstreamAccountStatusHealthy).Error)

	_, err = control.ConfigureCapability(context.Background(), CapabilityCommand{
		ID: capability.Id, Enabled: true, PriceMicros: 100_000,
	})

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "CAPABILITY_UNAVAILABLE", publicErr.Code)
}

func createPublishCapability(t *testing.T, toolName, schema string, upstreamCostMicros, priceMicros int64) *model.SearchCapability {
	t.Helper()
	publicID, err := model.GenerateSearchCapabilityPublicID(toolName)
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID: publicID, Name: toolName, Category: "搜索", InputSchema: schema,
		Status: model.SearchCapabilityStatusDisabled, UpstreamCostMicros: upstreamCostMicros,
		PriceMicros: priceMicros,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	return capability
}
