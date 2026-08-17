package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnterpriseNormalizesCodeAndUsesOpenRegistration(t *testing.T) {
	enterprise, err := NewEnterprise("Vector Epoch", " VE-OPS ")

	require.NoError(t, err)
	assert.Equal(t, "Vector Epoch", enterprise.Name)
	assert.Equal(t, "ve-ops", enterprise.Code)
	assert.Equal(t, EnterpriseStatusEnabled, enterprise.Status)
	assert.True(t, enterprise.RegistrationEnabled)
	assert.Equal(t, EnterpriseRegistrationModeOpen, enterprise.RegistrationMode)
}

func TestNewEnterpriseRejectsInvalidCode(t *testing.T) {
	_, err := NewEnterprise("Vector Epoch", "企业客户")

	require.Error(t, err)
}

func TestNewEnterpriseMembershipRequiresSupportedRole(t *testing.T) {
	_, err := NewEnterpriseMembership(7, 11, "platform_admin")

	require.Error(t, err)

	membership, err := NewEnterpriseMembership(7, 11, EnterpriseMembershipRoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, EnterpriseMembershipStatusActive, membership.Status)
}

func TestNewEnterpriseInvitationRejectsNegativeExpiry(t *testing.T) {
	_, err := NewEnterpriseInvitation(7, "invite-code", 11, -1, 1)

	require.Error(t, err)
}

func TestEnterpriseModelsPersistRegistrationOwnershipAndInvitation(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Enterprise{}, &EnterpriseMembership{}, &EnterpriseInvitation{}))
	t.Cleanup(func() {
		DB.Where("code = ?", "enterprise-model-test").Delete(&Enterprise{})
		DB.Where("enterprise_id NOT IN (SELECT id FROM enterprises)").Delete(&EnterpriseMembership{})
		DB.Where("enterprise_id NOT IN (SELECT id FROM enterprises)").Delete(&EnterpriseInvitation{})
	})

	enterprise, err := NewEnterprise("Enterprise Model Test", "enterprise-model-test")
	require.NoError(t, err)
	require.NoError(t, enterprise.Insert())

	membership, err := NewEnterpriseMembership(enterprise.Id, 77, EnterpriseMembershipRoleOwner)
	require.NoError(t, err)
	require.NoError(t, membership.Insert())

	invitation, err := NewEnterpriseInvitation(enterprise.Id, "invite-code", 77, 0, 2)
	require.NoError(t, err)
	require.NoError(t, DB.Create(invitation).Error)

	loadedEnterprise, err := GetEnterpriseByCode(" ENTERPRISE-MODEL-TEST ")
	require.NoError(t, err)
	assert.Equal(t, enterprise.Id, loadedEnterprise.Id)

	loadedMembership, err := GetEnterpriseMembershipByUserID(77)
	require.NoError(t, err)
	assert.Equal(t, EnterpriseMembershipRoleOwner, loadedMembership.Role)

	var loadedInvitation EnterpriseInvitation
	require.NoError(t, DB.First(&loadedInvitation, invitation.Id).Error)
	assert.True(t, loadedInvitation.CanUse(100))
	assert.Equal(t, HashEnterpriseInvitationCode("invite-code"), loadedInvitation.CodeHash)
}

func TestEnterpriseMembershipRejectsSecondActiveOwner(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Enterprise{}, &EnterpriseMembership{}))

	enterprise, err := NewEnterprise("Enterprise Owner Test", "enterprise-owner-test")
	require.NoError(t, err)
	require.NoError(t, enterprise.Insert())
	t.Cleanup(func() {
		DB.Where("enterprise_id = ?", enterprise.Id).Delete(&EnterpriseMembership{})
		DB.Delete(&Enterprise{}, enterprise.Id)
	})

	firstOwner, err := NewEnterpriseMembership(enterprise.Id, 901, EnterpriseMembershipRoleOwner)
	require.NoError(t, err)
	require.NoError(t, firstOwner.Insert())

	secondOwner, err := NewEnterpriseMembership(enterprise.Id, 902, EnterpriseMembershipRoleOwner)
	require.NoError(t, err)
	require.Error(t, secondOwner.Insert())
}
