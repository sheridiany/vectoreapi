package vsearch

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"
)

type usageQueryCounter struct {
	logger.Interface
	traces atomic.Int64
}

func (counter *usageQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	counter.traces.Add(1)
}

func TestListUsageLogsBulkLoadsDisplayNames(t *testing.T) {
	openRuntimeTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Enterprise{}, &model.SearchAgentKey{}))
	for index := 1; index <= 25; index++ {
		require.NoError(t, model.DB.Create(&model.User{
			Id: index, Username: fmt.Sprintf("user-%d", index), Password: "test-password", Status: common.UserStatusEnabled,
			AffCode: fmt.Sprintf("aff-%d", index),
		}).Error)
		require.NoError(t, model.DB.Create(&model.Enterprise{
			Id: index, Name: fmt.Sprintf("enterprise-%d", index), Code: fmt.Sprintf("enterprise-%d", index),
			Status: model.EnterpriseStatusEnabled, RegistrationMode: model.EnterpriseRegistrationModeOpen,
		}).Error)
		key := &model.SearchAgentKey{
			Id: index, UserId: index, EnterpriseID: index, Name: fmt.Sprintf("key-%d", index),
			KeyHash: fmt.Sprintf("%064x", index), KeyPrefix: fmt.Sprintf("vr_live_%d", index), Status: model.SearchAgentKeyStatusActive,
		}
		require.NoError(t, key.SetScopes(nil))
		require.NoError(t, model.DB.Create(key).Error)
		require.NoError(t, model.DB.Create(&model.SearchUpstreamAccount{
			Id: index, PoolID: 1, Provider: model.SearchUpstreamProviderAgentKeyMCP,
			Name: fmt.Sprintf("account-%d", index), BaseURL: DefaultAgentKeyMCPURL,
			SecretCiphertext: "ciphertext", SecretNonce: "nonce", SecretVersion: 1,
			SecretPrefix: "ak_live_test", Status: model.SearchUpstreamAccountStatusHealthy,
		}).Error)
		require.NoError(t, model.CreateSearchUsageEvent(&model.SearchUsageEvent{
			RequestID: fmt.Sprintf("request-%d", index), UserID: index, EnterpriseID: index,
			AgentKeyID: index, UpstreamAccountID: index, ServiceID: "vr_search", ServiceName: "Search",
			Action: model.SearchUsageActionExecute, Status: model.SearchUsageStatusSucceeded, HTTPStatus: 200,
		}))
	}

	previousLogger := model.DB.Config.Logger
	counter := &usageQueryCounter{Interface: logger.Discard}
	model.DB.Config.Logger = counter
	t.Cleanup(func() { model.DB.Config.Logger = previousLogger })

	logs, total, err := ListUsageLogs(context.Background(), model.SearchUsageQuery{Limit: 100}, true)

	require.NoError(t, err)
	require.Equal(t, int64(25), total)
	require.Len(t, logs, 25)
	for _, log := range logs {
		require.NotNil(t, log.Event)
		assert.Equal(t, fmt.Sprintf("key-%d", log.Event.AgentKeyID), log.AgentKeyName)
		assert.Equal(t, fmt.Sprintf("user-%d", log.Event.UserID), log.UserName)
		assert.Equal(t, fmt.Sprintf("enterprise-%d", log.Event.EnterpriseID), log.EnterpriseName)
		assert.Equal(t, fmt.Sprintf("account-%d", log.Event.UpstreamAccountID), log.AccountName)
	}
	assert.LessOrEqual(t, counter.traces.Load(), int64(10), "relation enrichment must not issue one query per usage row")
}
