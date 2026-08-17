package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetEnterpriseLogsScopesAndSanitizesResults(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_logs_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&Log{}, &EnterpriseMembership{}))
	require.NoError(t, db.Create(&EnterpriseMembership{EnterpriseID: 10, UserID: 701, Role: EnterpriseMembershipRoleAdmin, Status: EnterpriseMembershipStatusActive}).Error)
	require.NoError(t, db.Create(&EnterpriseMembership{EnterpriseID: 20, UserID: 702, Role: EnterpriseMembershipRoleAdmin, Status: EnterpriseMembershipStatusActive}).Error)
	require.NoError(t, db.Create(&Log{
		UserId: 701, EnterpriseID: 10, Username: "alice", CreatedAt: 200, Type: LogTypeManage,
		Ip: "10.0.0.1", TokenName: "secret-key-name", Content: "internal request body",
		UpstreamRequestId: "upstream-secret", Other: `{"admin_info":{"admin_id":1},"audit_info":{"path":"/api"},"op":{"action":"member.update"}}`,
	}).Error)
	require.NoError(t, db.Create(&Log{UserId: 702, EnterpriseID: 20, Username: "bob", CreatedAt: 300, Type: LogTypeManage}).Error)

	logs, total, err := GetEnterpriseLogs(10, LogTypeUnknown, 0, 0, "", "", 0, 20, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	assert.Equal(t, 701, logs[0].UserId)
	assert.Empty(t, logs[0].Ip)
	assert.Empty(t, logs[0].TokenName)
	assert.Empty(t, logs[0].Content)
	assert.Empty(t, logs[0].UpstreamRequestId)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, other, "admin_info")
	assert.NotContains(t, other, "audit_info")
	assert.Contains(t, other, "op")

	RecordOperationAuditLog(701, "PUT /api/enterprise", "10.0.0.2", "enterprise.update", nil, nil, nil)
	var audit Log
	require.NoError(t, db.Order("id DESC").First(&audit).Error)
	assert.Equal(t, 10, audit.EnterpriseID)

	RecordEnterpriseOperationAuditLog(20, 999, "PUT /api/enterprise/20/budget", "10.0.0.3", "enterprise.budget.update", nil, nil, nil)
	audit = Log{}
	require.NoError(t, db.Order("id DESC").First(&audit).Error)
	assert.Equal(t, 20, audit.EnterpriseID)

}
