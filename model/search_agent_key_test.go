package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
