package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseAutoPolicyAppliesToNewAndUpdatedKeys(t *testing.T) {
	db := setupEnterpriseControllerDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
	})
	enterprise, err := model.NewEnterprise("Acme Operations", "acme-token-policy")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	user := &model.User{Id: 801, Username: "enterprise-key-user", Password: "password123"}
	require.NoError(t, db.Create(user).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, user.Id, model.EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)
	require.NoError(t, db.AutoMigrate(&model.Token{}))

	addCtx, addRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{
		"name":            "enterprise-key",
		"expired_time":    -1,
		"remain_quota":    0,
		"unlimited_quota": true,
		"group":           "default",
	}, user.Id)
	AddToken(addCtx)
	assert.True(t, decodeAPIResponse(t, addRecorder).Success)
	var token model.Token
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&token).Error)
	assert.Equal(t, model.EnterpriseTokenGroupAuto, token.Group)
	assert.True(t, token.CrossGroupRetry)
	assert.Empty(t, token.AutoGroups)

	updateCtx, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", map[string]any{
		"id":              token.Id,
		"name":            "enterprise-key-updated",
		"status":          common.TokenStatusEnabled,
		"expired_time":    -1,
		"remain_quota":    0,
		"unlimited_quota": true,
		"group":           "default",
	}, user.Id)
	UpdateToken(updateCtx)
	assert.True(t, decodeAPIResponse(t, updateRecorder).Success)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, model.EnterpriseTokenGroupAuto, token.Group)
	assert.True(t, token.CrossGroupRetry)
}
