package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSearchCapabilityGrantControllerDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.SearchCapability{}, &model.SearchCapabilityGrant{}))
	return db
}

func createSearchCapabilityGrantFixture(t *testing.T) (*model.SearchCapability, *model.Enterprise, *model.Enterprise) {
	t.Helper()
	publicID, err := model.GenerateSearchCapabilityPublicID(t.Name())
	require.NoError(t, err)
	capability := &model.SearchCapability{
		PublicID:     publicID,
		Name:         "Web search",
		Category:     "search",
		InputSchema:  `{"type":"object"}`,
		SchemaStatus: model.SearchCapabilitySchemaAvailable,
		Status:       model.SearchCapabilityStatusEnabled,
	}
	require.NoError(t, model.CreateSearchCapability(capability))
	first, err := model.NewEnterprise("First enterprise", "first-enterprise")
	require.NoError(t, err)
	require.NoError(t, first.Insert())
	second, err := model.NewEnterprise("Second enterprise", "second-enterprise")
	require.NoError(t, err)
	require.NoError(t, second.Insert())
	return capability, first, second
}

func TestAdminSearchCapabilityEnterpriseGrantsRestrictAndRestoreGlobalAccess(t *testing.T) {
	openSearchCapabilityGrantControllerDB(t)
	capability, first, second := createSearchCapabilityGrantFixture(t)
	gin.SetMode(gin.TestMode)

	restrictedRecorder := httptest.NewRecorder()
	restrictedContext, _ := gin.CreateTestContext(restrictedRecorder)
	restrictedContext.Params = gin.Params{{Key: "id", Value: capability.PublicID}}
	restrictedContext.Request = httptest.NewRequest(http.MethodPut, "/api/search/admin/catalog/"+capability.PublicID+"/grants", bytes.NewBufferString(fmt.Sprintf(`{"enterprise_ids":[%d,%d,%d]}`, second.Id, first.Id, second.Id)))
	restrictedContext.Request.Header.Set("Content-Type", "application/json")

	AdminSetSearchCapabilityEnterpriseGrants(restrictedContext)

	require.Equal(t, http.StatusOK, restrictedRecorder.Code)
	var restrictedPayload struct {
		Success bool                                     `json:"success"`
		Data    searchCapabilityEnterpriseGrantsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(restrictedRecorder.Body.Bytes(), &restrictedPayload))
	require.True(t, restrictedPayload.Success)
	assert.Equal(t, "selected_enterprises", restrictedPayload.Data.AccessMode)
	assert.ElementsMatch(t, []int{first.Id, second.Id}, restrictedPayload.Data.EnterpriseIDs)
	granted, err := model.IsSearchCapabilityGranted(capability.Id, first.Id, 100)
	require.NoError(t, err)
	assert.True(t, granted)
	granted, err = model.IsSearchCapabilityGranted(capability.Id, second.Id+1000, 200)
	require.NoError(t, err)
	assert.False(t, granted)

	globalRecorder := httptest.NewRecorder()
	globalContext, _ := gin.CreateTestContext(globalRecorder)
	globalContext.Params = gin.Params{{Key: "id", Value: capability.PublicID}}
	globalContext.Request = httptest.NewRequest(http.MethodPut, "/api/search/admin/catalog/"+capability.PublicID+"/grants", bytes.NewBufferString(`{"enterprise_ids":[]}`))
	globalContext.Request.Header.Set("Content-Type", "application/json")

	AdminSetSearchCapabilityEnterpriseGrants(globalContext)

	var globalPayload struct {
		Success bool                                     `json:"success"`
		Data    searchCapabilityEnterpriseGrantsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(globalRecorder.Body.Bytes(), &globalPayload))
	require.True(t, globalPayload.Success)
	assert.Equal(t, "all_enterprises", globalPayload.Data.AccessMode)
	assert.Empty(t, globalPayload.Data.EnterpriseIDs)
	granted, err = model.IsSearchCapabilityGranted(capability.Id, second.Id+1000, 200)
	require.NoError(t, err)
	assert.True(t, granted)
}

func TestAdminSearchCapabilityEnterpriseGrantsRejectUnknownEnterpriseWithoutChangingAccess(t *testing.T) {
	openSearchCapabilityGrantControllerDB(t)
	capability, first, _ := createSearchCapabilityGrantFixture(t)
	require.NoError(t, model.ReplaceSearchCapabilityEnterpriseGrants(capability.Id, []int{first.Id}))
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: capability.PublicID}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/search/admin/catalog/"+capability.PublicID+"/grants", bytes.NewBufferString(`{"enterprise_ids":[999999]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	AdminSetSearchCapabilityEnterpriseGrants(c)

	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	grants, err := model.ListSearchCapabilityEnterpriseGrants(capability.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, first.Id, grants[0].EnterpriseID)
}

func TestAdminGetSearchCapabilityEnterpriseGrantsReturnsStableContract(t *testing.T) {
	openSearchCapabilityGrantControllerDB(t)
	capability, _, second := createSearchCapabilityGrantFixture(t)
	require.NoError(t, model.ReplaceSearchCapabilityEnterpriseGrants(capability.Id, []int{second.Id}))
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: capability.PublicID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search/admin/catalog/"+capability.PublicID+"/grants", nil)

	AdminGetSearchCapabilityEnterpriseGrants(c)

	var payload struct {
		Success bool                                     `json:"success"`
		Data    searchCapabilityEnterpriseGrantsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, capability.PublicID, payload.Data.CapabilityID)
	assert.Equal(t, "selected_enterprises", payload.Data.AccessMode)
	assert.Equal(t, []int{second.Id}, payload.Data.EnterpriseIDs)
}
