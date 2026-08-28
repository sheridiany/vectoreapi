package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacySearchAgentKey struct {
	Id           int `gorm:"primaryKey"`
	UserId       int
	EnterpriseID int
	Name         string
	KeyHash      string
	KeyPrefix    string
	Status       int
	Scopes       string
	CreatedAt    int64
	LastUsedAt   int64
	ExpiresAt    int64
	UpdatedAt    int64
	DeletedAt    gorm.DeletedAt
}

func (legacySearchAgentKey) TableName() string { return "search_agent_keys" }

func TestSearchAgentKeyLifecycle(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err, "open sqlite")
	modelDB := DB
	DB = db
	t.Cleanup(func() { DB = modelDB })
	require.NoError(t, DB.AutoMigrate(&SearchAgentKey{}), "migrate")

	key, secret, err := CreateSearchAgentKey(7, 11, "web-agent", []string{"web-search", "web-search", "extract"})
	require.NoError(t, err, "create")
	require.NotEmpty(t, secret)
	assert.NotEqual(t, secret, key.KeyHash)
	assert.NotContains(t, key.Scopes, "web-search\",\"web-search")
	assert.Equal(t, secret[:15], key.KeyPrefix)
	found, err := FindSearchAgentKeyBySecret(secret)
	require.NoError(t, err, "lookup")
	require.Equal(t, key.Id, found.Id)
	require.NoError(t, RevokeSearchAgentKey(key.Id, 7), "revoke")
	_, err = FindSearchAgentKeyBySecret(secret)
	require.Error(t, err, "revoked key should not validate")
	require.Error(t, RevokeSearchAgentKey(key.Id, 8), "another user must not revoke the key")
}

func TestNormalizeSearchAgentKeyScopesRejectsUnknown(t *testing.T) {
	_, err := NormalizeSearchAgentKeyScopes([]string{"unknown"})
	require.Error(t, err, "unknown scope should be rejected")
	all, err := NormalizeSearchAgentKeyScopes(nil)
	require.NoError(t, err)
	assert.Len(t, all, 8)
}

func TestSearchAgentKeyCredentialVersionLegacyMigration(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	modelDB := DB
	DB = db
	t.Cleanup(func() { DB = modelDB })
	require.NoError(t, DB.AutoMigrate(&legacySearchAgentKey{}))
	require.NoError(t, DB.Create(&legacySearchAgentKey{
		UserId: 7, EnterpriseID: 11, Name: "legacy", KeyHash: strings.Repeat("a", 64),
		KeyPrefix: "vr_live_legacy", Status: SearchAgentKeyStatusActive, Scopes: `["web-search"]`,
	}).Error)

	require.NoError(t, DB.AutoMigrate(&SearchAgentKey{}), "adding credential_version must not block an existing gateway database")
	require.NoError(t, InitializeSearchAgentKeyCredentialVersions())
	var key SearchAgentKey
	require.NoError(t, DB.First(&key).Error)
	assert.Equal(t, 1, key.CredentialVersion)
}

func TestPreparedSearchAgentKeyRotationRejectsExpiredKey(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, DB.AutoMigrate(&SearchAgentKey{}, &AuthFlow{}))

	key, _, err := CreateSearchAgentKey(7, 11, "expiring-agent", nil)
	require.NoError(t, err)
	require.NoError(t, DB.Model(key).Update("expires_at", common.GetTimestamp()-1).Error)
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, _, prepareErr := PrepareSearchAgentKeyRotationWithTx(tx, key.Id, key.CredentialVersion, time.Now().Add(time.Minute))
		return prepareErr
	})
	require.Error(t, err, "an expired key must not prepare a replacement")

	require.NoError(t, DB.Model(key).Update("expires_at", common.GetTimestamp()+60).Error)
	var candidateSecret string
	var activationToken string
	err = DB.Transaction(func(tx *gorm.DB) error {
		var prepareErr error
		candidateSecret, activationToken, prepareErr = PrepareSearchAgentKeyRotationWithTx(tx, key.Id, key.CredentialVersion, time.Now().Add(time.Minute))
		return prepareErr
	})
	require.NoError(t, err)
	require.NoError(t, DB.Model(key).Update("expires_at", common.GetTimestamp()-1).Error)
	_, err = ActivatePreparedSearchAgentKeyRotation(activationToken)
	require.Error(t, err, "a key that expires after prepare must not activate")
	_, err = FindSearchAgentKeyBySecret(candidateSecret)
	require.Error(t, err)
}
