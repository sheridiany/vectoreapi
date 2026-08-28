package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEnterpriseRankingsTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_rankings_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Enterprise{}, &model.Log{}, &model.User{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestEnterpriseRankingsUseNetQuotaAndSeparateMemberRanking(t *testing.T) {
	db := setupEnterpriseRankingsTest(t)
	first, err := model.NewEnterprise("Acme Operations", "acme-ops")
	require.NoError(t, err)
	second, err := model.NewEnterprise("Beta Operations", "beta-ops")
	require.NoError(t, err)
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, db.Create(&model.User{
		Id: 101, Username: "alice", Password: "password123", DisplayName: "Alice Chen",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}).Error)

	now := time.Now().Unix()
	start := now - 3600
	end := now
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, EnterpriseID: first.Id, Username: "alice", CreatedAt: now - 100,
		Type: model.LogTypeConsume, Quota: 100, PromptTokens: 10, CompletionTokens: 20,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, EnterpriseID: first.Id, Username: "alice", CreatedAt: now - 90,
		Type: model.LogTypeRefund, Quota: 20,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 202, EnterpriseID: second.Id, Username: "bob", CreatedAt: now - 80,
		Type: model.LogTypeConsume, Quota: 60, PromptTokens: 5, CompletionTokens: 5,
	}).Error)

	rankings, err := GetEnterpriseRankings("custom", formatInt64(start), formatInt64(end))
	require.NoError(t, err)
	require.Len(t, rankings.Enterprises, 2)
	assert.Equal(t, first.Id, rankings.Enterprises[0].EnterpriseID)
	assert.Equal(t, int64(80), rankings.Enterprises[0].NetQuota)
	assert.Equal(t, int64(30), rankings.Enterprises[0].TotalTokens)
	assert.Equal(t, int64(1), rankings.Enterprises[0].RequestCount)
	assert.Equal(t, int64(1), rankings.Enterprises[0].ActiveUsers)

	members, err := GetEnterpriseMemberRankings(first.Id, "custom", formatInt64(start), formatInt64(end))
	require.NoError(t, err)
	require.NotNil(t, members.Enterprise)
	assert.Equal(t, first.Id, members.Enterprise.EnterpriseID)
	assert.Equal(t, int64(80), members.Enterprise.NetQuota)
	require.Len(t, members.Members, 1)
	assert.Equal(t, 101, members.Members[0].UserID)
	assert.Equal(t, "Alice Chen", members.Members[0].DisplayName)
	assert.Equal(t, int64(80), members.Members[0].NetQuota)
}

func TestEnterpriseRankingsRejectInvalidCustomRange(t *testing.T) {
	setupEnterpriseRankingsTest(t)

	_, err := GetEnterpriseRankings("custom", "200", "199")
	require.Error(t, err)

	_, err = GetEnterpriseRankings("custom", "-1", "200")
	require.Error(t, err)
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
