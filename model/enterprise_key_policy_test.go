package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEnterpriseKeyPolicyTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	initCol()
	db, err := gorm.Open(sqlite.Open("file:enterprise_key_policy_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&Enterprise{},
		&EnterpriseMembership{},
		&EnterpriseKeyPolicyOperation{},
		&EnterpriseKeyPolicyTokenChange{},
		&Token{},
	))
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		initCol()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestEnterpriseKeyPolicyAppliesOnlyToActiveMembersAndRollsBack(t *testing.T) {
	db := setupEnterpriseKeyPolicyTest(t)
	first, err := NewEnterprise("Acme Operations", "acme-key-policy")
	require.NoError(t, err)
	second, err := NewEnterprise("Other Operations", "other-key-policy")
	require.NoError(t, err)
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, db.Model(&Enterprise{}).Where("id = ?", first.Id).Update("token_group_policy", "").Error)
	owner, err := NewEnterpriseMembership(first.Id, 101, EnterpriseMembershipRoleOwner)
	require.NoError(t, err)
	member, err := NewEnterpriseMembership(first.Id, 102, EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	otherMember, err := NewEnterpriseMembership(second.Id, 201, EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(member).Error)
	require.NoError(t, db.Create(otherMember).Error)

	legacy := &Token{UserId: 101, Key: "policy-legacy", Group: "default", CrossGroupRetry: false, AutoGroups: "[\"vip\"]"}
	auto := &Token{UserId: 102, Key: "policy-auto", Group: EnterpriseTokenGroupAuto, CrossGroupRetry: true}
	unrelated := &Token{UserId: 201, Key: "policy-unrelated", Group: "default"}
	require.NoError(t, db.Create(legacy).Error)
	require.NoError(t, db.Create(auto).Error)
	require.NoError(t, db.Create(unrelated).Error)

	summary, err := GetEnterpriseKeyPolicySummary(first.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.ActiveMemberCount)
	assert.Equal(t, int64(2), summary.TotalKeyCount)
	assert.Equal(t, int64(1), summary.AutoKeyCount)
	assert.Equal(t, int64(1), summary.ConvertibleKeyCount)

	operation, err := ApplyEnterpriseKeyPolicy(first.Id, 101)
	require.NoError(t, err)
	assert.Equal(t, 1, operation.ChangedCount)
	var updatedLegacy Token
	require.NoError(t, db.First(&updatedLegacy, legacy.Id).Error)
	assert.Equal(t, EnterpriseTokenGroupAuto, updatedLegacy.Group)
	assert.True(t, updatedLegacy.CrossGroupRetry)
	assert.Empty(t, updatedLegacy.AutoGroups)
	var unchangedUnrelated Token
	require.NoError(t, db.First(&unchangedUnrelated, unrelated.Id).Error)
	assert.Equal(t, "default", unchangedUnrelated.Group)

	rolledBack, err := RollbackEnterpriseKeyPolicy(first.Id, operation.Id)
	require.NoError(t, err)
	assert.Equal(t, EnterpriseKeyPolicyOperationRolledBack, rolledBack.Status)
	assert.Equal(t, 1, rolledBack.ChangedCount)
	var restoredLegacy Token
	require.NoError(t, db.First(&restoredLegacy, legacy.Id).Error)
	assert.Equal(t, "default", restoredLegacy.Group)
	assert.False(t, restoredLegacy.CrossGroupRetry)
	assert.JSONEq(t, "[\"vip\"]", restoredLegacy.AutoGroups)
}

func TestEnterpriseKeyPolicyRollbackPreservesLaterManualChange(t *testing.T) {
	db := setupEnterpriseKeyPolicyTest(t)
	enterprise, err := NewEnterprise("Acme Operations", "acme-key-policy-manual")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	membership, err := NewEnterpriseMembership(enterprise.Id, 301, EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)
	token := &Token{UserId: 301, Key: "policy-manual", Group: "default"}
	require.NoError(t, db.Create(token).Error)

	operation, err := ApplyEnterpriseKeyPolicy(enterprise.Id, 301)
	require.NoError(t, err)
	require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{
		"group":             "vip",
		"cross_group_retry": false,
	}).Error)

	rolledBack, err := RollbackEnterpriseKeyPolicy(enterprise.Id, operation.Id)
	require.NoError(t, err)
	assert.Equal(t, 1, rolledBack.RollbackSkippedCount)
	var unchanged Token
	require.NoError(t, db.First(&unchanged, token.Id).Error)
	assert.Equal(t, "vip", unchanged.Group)
	assert.False(t, unchanged.CrossGroupRetry)
}
