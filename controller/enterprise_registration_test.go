package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func performRegisterRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)
	return recorder
}

func TestRegisterRequiresEnterpriseSelection(t *testing.T) {
	setupEnterpriseControllerDB(t)
	oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled := common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = true, true, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled
	})

	recorder := performRegisterRequest(t, `{"username":"no-enterprise","password":"password123"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestRegisterOpenEnterpriseWithoutInvitationCreatesMember(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled := common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = true, true, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled
	})

	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)

	recorder := performRegisterRequest(t, `{"username":"open-enterprise-user","password":"password123","enterprise_code":"acme-ops"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, db.Where("username = ?", "open-enterprise-user").First(&user).Error)
	var membership model.EnterpriseMembership
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&membership).Error)
	assert.Equal(t, enterprise.Id, membership.EnterpriseID)
}

func TestRegisterInviteOnlyEnterpriseRequiresInvitation(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled := common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = true, true, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled
	})

	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	enterprise.RegistrationMode = model.EnterpriseRegistrationModeInvite
	require.NoError(t, db.Create(enterprise).Error)

	recorder := performRegisterRequest(t, `{"username":"invite-enterprise-user","password":"password123","enterprise_code":"acme-ops"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var user model.User
	assert.Error(t, db.Where("username = ?", "invite-enterprise-user").First(&user).Error)
}

func TestRegisterConsumesEnterpriseInvitationAndCreatesMember(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled := common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = true, true, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.RedisEnabled = oldRegisterEnabled, oldPasswordRegisterEnabled, oldRedisEnabled
	})

	enterprise, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	invitation, err := model.NewEnterpriseInvitation(enterprise.Id, "invite-code", 99, 0, 1)
	require.NoError(t, err)
	require.NoError(t, db.Create(invitation).Error)

	recorder := performRegisterRequest(t, `{"username":"enterprise-user","password":"password123","enterprise_code":"acme-ops","enterprise_invitation_code":"invite-code"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, db.Where("username = ?", "enterprise-user").First(&user).Error)
	var membership model.EnterpriseMembership
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&membership).Error)
	assert.Equal(t, enterprise.Id, membership.EnterpriseID)
	assert.Equal(t, model.EnterpriseMembershipRoleMember, membership.Role)
	var reloadedInvitation model.EnterpriseInvitation
	require.NoError(t, db.First(&reloadedInvitation, invitation.Id).Error)
	assert.Equal(t, 1, reloadedInvitation.UsedCount)
}
