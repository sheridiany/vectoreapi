package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
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

func setupEnterpriseControllerDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	db, err := gorm.Open(sqlite.Open("file:enterprise_controller_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Enterprise{}, &model.EnterpriseMembership{}, &model.EnterpriseInvitation{}))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func performEnterpriseCreateRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/admin", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	AdminCreateEnterprise(ctx)
	return recorder
}

func TestAdminCreateEnterpriseCreatesOpenEnterprise(t *testing.T) {
	db := setupEnterpriseControllerDB(t)

	recorder := performEnterpriseCreateRequest(t, `{"name":"Acme Operations","code":" acme-ops "}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var enterprise model.Enterprise
	require.NoError(t, db.Where("code = ?", "acme-ops").First(&enterprise).Error)
	assert.Equal(t, "Acme Operations", enterprise.Name)
	assert.True(t, enterprise.RegistrationEnabled)
	assert.Equal(t, model.EnterpriseRegistrationModeOpen, enterprise.RegistrationMode)
}

func TestAdminCreateEnterpriseRejectsInvalidCode(t *testing.T) {
	setupEnterpriseControllerDB(t)

	recorder := performEnterpriseCreateRequest(t, `{"name":"Acme Operations","code":"企业"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestAdminListEnterprisesReturnsPagedItems(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	first, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	second, err := model.NewEnterprise("Beta Operations", "beta-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/enterprise/admin/?p=1&page_size=1", nil)
	AdminListEnterprises(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"total":2`)
	assert.Contains(t, recorder.Body.String(), `"items":[`)
}

func TestAdminUpdateEnterpriseCanDisableRegistration(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/enterprise/admin/1", strings.NewReader(`{"status":2,"registration_enabled":false}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(enterprise.Id)}}
	AdminUpdateEnterprise(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var updated model.Enterprise
	require.NoError(t, db.First(&updated, enterprise.Id).Error)
	assert.Equal(t, model.EnterpriseStatusDisabled, updated.Status)
	assert.False(t, updated.RegistrationEnabled)
	assert.False(t, updated.IsRegistrationAvailable())
}

func TestAdminAssignEnterpriseMemberCreatesScopedMembership(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	user := &model.User{Username: "member-user", Password: "password", DisplayName: "Member User", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/admin/1/members", strings.NewReader(`{"user_id":1,"role":"admin"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(enterprise.Id)}}
	ctx.Set("id", 99)
	AdminAssignEnterpriseMember(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var membership model.EnterpriseMembership
	require.NoError(t, db.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, user.Id).First(&membership).Error)
	assert.Equal(t, model.EnterpriseMembershipRoleAdmin, membership.Role)
}

func TestAdminCreateEnterpriseInvitationReturnsRawCodeOnce(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/admin/1/invitations", strings.NewReader(`{"max_uses":1}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(enterprise.Id)}}
	ctx.Set("id", 99)
	AdminCreateEnterpriseInvitation(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Data.Code)
	var invitation model.EnterpriseInvitation
	require.NoError(t, db.Where("enterprise_id = ?", enterprise.Id).First(&invitation).Error)
	assert.Equal(t, model.HashEnterpriseInvitationCode(response.Data.Code), invitation.CodeHash)
	assert.True(t, invitation.CanUse(common.GetTimestamp()))
}

func TestAdminUpdateEnterpriseMemberAndInvitation(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	user := &model.User{Username: "member-user", Password: "password", DisplayName: "Member User", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, user.Id, model.EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)
	invitation, err := model.NewEnterpriseInvitation(enterprise.Id, "raw-code", 99, 0, 0)
	require.NoError(t, err)
	require.NoError(t, db.Create(invitation).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/enterprise/1/members/1", strings.NewReader(`{"role":" AUDITOR ","status":2}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(enterprise.Id)}, {Key: "user_id", Value: strconv.Itoa(user.Id)}}
	AdminUpdateEnterpriseMember(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/enterprise/1/invitations/1", strings.NewReader(`{"status":2}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(enterprise.Id)}, {Key: "invitation_id", Value: strconv.Itoa(invitation.Id)}}
	AdminUpdateEnterpriseInvitation(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)

	var updatedMembership model.EnterpriseMembership
	require.NoError(t, db.First(&updatedMembership, membership.Id).Error)
	assert.Equal(t, model.EnterpriseMembershipRoleAuditor, updatedMembership.Role)
	assert.Equal(t, model.EnterpriseMembershipStatusDisabled, updatedMembership.Status)
	var updatedInvitation model.EnterpriseInvitation
	require.NoError(t, db.First(&updatedInvitation, invitation.Id).Error)
	assert.Equal(t, model.EnterpriseInvitationStatusDisabled, updatedInvitation.Status)
}
