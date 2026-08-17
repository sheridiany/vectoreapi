package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingLogsSnapshotEnterpriseContext(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousLogEnabled, previousDataExportEnabled := common.LogConsumeEnabled, common.DataExportEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open("file:enterprise_attribution_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	require.NoError(t, db.AutoMigrate(&Log{}))
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.LogConsumeEnabled, common.DataExportEnabled = previousLogEnabled, previousDataExportEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyEnterpriseId, 42)
	ctx.Set("username", "alice")
	RecordConsumeLog(ctx, 7, RecordConsumeLogParams{Quota: 100})
	RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: 7, EnterpriseID: 42, LogType: LogTypeRefund, Quota: 25})

	var logs []Log
	require.NoError(t, db.Order("id ASC").Find(&logs).Error)
	require.Len(t, logs, 2)
	assert.Equal(t, 42, logs[0].EnterpriseID)
	assert.Equal(t, 42, logs[1].EnterpriseID)
}
