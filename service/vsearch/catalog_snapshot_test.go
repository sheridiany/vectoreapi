package vsearch

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminCatalogDoesNotCountHealthyRouteWithoutSchemaAsCallable(t *testing.T) {
	openRuntimeTestDB(t)
	runtime, _, capability := seedRuntimeExecution(t, &runtimeFakeAdapter{})
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).Updates(map[string]any{
		"input_schema":  "",
		"schema_status": model.SearchCapabilitySchemaUnavailable,
	}).Error)

	catalog, err := runtime.ListCatalog(context.Background(), Principal{}, true)

	require.NoError(t, err)
	require.Len(t, catalog, 1)
	assert.Equal(t, "unavailable", catalog[0].Status)
	assert.False(t, catalog[0].Enabled)
	assert.Zero(t, catalog[0].InterfaceCount)
	assert.Zero(t, catalog[0].HealthyRouteCount)
	assert.Equal(t, "unavailable", catalog[0].SchemaStatus)
}

func TestAdminCatalogDoesNotCountHealthyRouteWithStaleBindingSchema(t *testing.T) {
	openRuntimeTestDB(t)
	runtime, _, capability := seedRuntimeExecution(t, &runtimeFakeAdapter{})
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").
		Update("status", model.SearchUpstreamAccountStatusStandby).Error)
	require.NoError(t, model.DB.Model(&model.SearchCapabilityBinding{}).
		Where("capability_id = ?", capability.Id).
		Update("input_schema", `{"type":"object","properties":{"stale":{"type":"string"}}}`).Error)
	require.NoError(t, model.DB.Model(&model.SearchUpstreamAccount{}).Where("id > 0").
		Update("status", model.SearchUpstreamAccountStatusHealthy).Error)

	catalog, err := runtime.ListCatalog(context.Background(), Principal{}, true)

	require.NoError(t, err)
	require.Len(t, catalog, 1)
	assert.Equal(t, "unavailable", catalog[0].Status)
	assert.False(t, catalog[0].Enabled)
	assert.Zero(t, catalog[0].InterfaceCount)
	assert.Zero(t, catalog[0].HealthyRouteCount)
}

func TestStandardBindingSchemaMatchIncludesOutputContract(t *testing.T) {
	capability := &model.SearchCapability{
		ContractVersion: "v1", InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"string"}`,
	}
	binding := &model.SearchCapabilityBinding{
		InputSchema: capability.InputSchema, OutputSchema: `{"type":"number"}`,
	}
	assert.False(t, searchBindingMatchesCapabilitySchema(binding, capability))

	binding.OutputSchema = capability.OutputSchema
	assert.True(t, searchBindingMatchesCapabilitySchema(binding, capability))

	capability.ContractVersion = "legacy-v1"
	binding.OutputSchema = ""
	assert.True(t, searchBindingMatchesCapabilitySchema(binding, capability))
}

func TestCatalogOmitsArchivedRetiredCapabilitiesForAdminAndUser(t *testing.T) {
	openRuntimeTestDB(t)
	runtime, _, capability := seedRuntimeExecution(t, &runtimeFakeAdapter{})
	require.NoError(t, model.DB.Model(&model.SearchCapability{}).Where("id = ?", capability.Id).
		Update("status", model.SearchCapabilityStatusDisabled).Error)
	require.NoError(t, model.DB.Delete(&model.SearchCapability{}, capability.Id).Error)

	adminCatalog, err := runtime.ListCatalog(context.Background(), Principal{}, true)
	require.NoError(t, err)
	assert.Empty(t, adminCatalog)

	userCatalog, err := runtime.ListCatalog(context.Background(), Principal{}, false)
	require.NoError(t, err)
	assert.Empty(t, userCatalog)

	var archived model.SearchCapability
	require.NoError(t, model.DB.Unscoped().First(&archived, capability.Id).Error)
	assert.True(t, archived.DeletedAt.Valid)
}
