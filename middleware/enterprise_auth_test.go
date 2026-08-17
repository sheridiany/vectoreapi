package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSetEnterpriseAuthContextOmitsDisabledEnterprise(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_auth_context_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	enterprise, err := model.NewEnterprise("Disabled Enterprise", "disabled-enterprise")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	enterprise.Status = model.EnterpriseStatusDisabled
	require.NoError(t, db.Save(enterprise).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, 901, model.EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)

	ctx, _ := gin.CreateTestContext(nil)
	assert.False(t, setEnterpriseAuthContext(ctx, 901))

	_, hasEnterpriseID := common.GetContextKey(ctx, constant.ContextKeyEnterpriseId)
	_, hasMembershipID := common.GetContextKey(ctx, constant.ContextKeyEnterpriseMembershipId)
	assert.False(t, hasEnterpriseID)
	assert.False(t, hasMembershipID)
}

func TestSetEnterpriseAuthContextIncludesMembershipIdentity(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_auth_context_active_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	enterprise, err := model.NewEnterprise("Active Enterprise", "active-enterprise")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, 903, model.EnterpriseMembershipRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setEnterpriseAuthContext(ctx, 903)

	assert.Equal(t, enterprise.Id, common.GetContextKeyInt(ctx, constant.ContextKeyEnterpriseId))
	assert.Equal(t, membership.Id, common.GetContextKeyInt(ctx, constant.ContextKeyEnterpriseMembershipId))
	assert.Equal(t, model.EnterpriseMembershipRoleAdmin, common.GetContextKeyString(ctx, constant.ContextKeyEnterpriseRole))
}

func TestEnterpriseAdminAuthRejectsDisabledEnterprise(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_admin_auth_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	enterprise, err := model.NewEnterprise("Disabled Admin Test", "disabled-admin-test")
	require.NoError(t, err)
	enterprise.Status = model.EnterpriseStatusDisabled
	require.NoError(t, db.Create(enterprise).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, 902, model.EnterpriseMembershipRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("id", 902)
	ctx.Set("role", common.RoleCommonUser)
	EnterpriseAdminAuth()(ctx)

	assert.True(t, ctx.IsAborted())
	assert.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}

func TestEnterpriseMemberAuthAllowsActiveMember(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_member_auth_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	enterprise, err := model.NewEnterprise("Active Member Test", "active-member-test")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	membership, err := model.NewEnterpriseMembership(enterprise.Id, 904, model.EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("id", 904)
	ctx.Set("role", common.RoleCommonUser)
	EnterpriseMemberAuth()(ctx)

	assert.False(t, ctx.IsAborted())
}

func TestEnterpriseViewerAuthAllowsAuditorButRejectsMember(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_viewer_auth_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	enterprise, err := model.NewEnterprise("Viewer Test", "viewer-test")
	require.NoError(t, err)
	require.NoError(t, db.Create(enterprise).Error)
	auditor, err := model.NewEnterpriseMembership(enterprise.Id, 905, model.EnterpriseMembershipRoleAuditor)
	require.NoError(t, err)
	require.NoError(t, db.Create(auditor).Error)
	member, err := model.NewEnterpriseMembership(enterprise.Id, 906, model.EnterpriseMembershipRoleMember)
	require.NoError(t, err)
	require.NoError(t, db.Create(member).Error)

	auditorContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	auditorContext.Params = gin.Params{{Key: "id", Value: "1"}}
	auditorContext.Set("id", 905)
	auditorContext.Set("role", common.RoleCommonUser)
	EnterpriseViewerAuth()(auditorContext)
	assert.False(t, auditorContext.IsAborted())

	memberContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	memberContext.Params = gin.Params{{Key: "id", Value: "1"}}
	memberContext.Set("id", 906)
	memberContext.Set("role", common.RoleCommonUser)
	EnterpriseViewerAuth()(memberContext)
	assert.True(t, memberContext.IsAborted())
	assert.Equal(t, http.StatusForbidden, memberContext.Writer.Status())
}

func TestEnterpriseViewerAuthRejectsMembershipFromAnotherEnterprise(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_cross_tenant_auth_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.EnterpriseMembership{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	first, err := model.NewEnterprise("First Tenant", "first-tenant")
	require.NoError(t, err)
	second, err := model.NewEnterprise("Second Tenant", "second-tenant")
	require.NoError(t, err)
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)
	membership, err := model.NewEnterpriseMembership(second.Id, 907, model.EnterpriseMembershipRoleAuditor)
	require.NoError(t, err)
	require.NoError(t, db.Create(membership).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("id", 907)
	ctx.Set("role", common.RoleCommonUser)
	EnterpriseViewerAuth()(ctx)

	assert.True(t, ctx.IsAborted())
	assert.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}
