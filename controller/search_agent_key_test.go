package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSearchAgentKeyControllerDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SearchAgentKey{}, &model.EnterpriseMembership{}))
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "alice", Password: "password"}).Error)
	return db
}

func TestCreateSearchAgentKeyReturnsSecretOnce(t *testing.T) {
	openSearchAgentKeyControllerDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 7)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/search/keys", bytes.NewBufferString(`{"name":"browser","scopes":["web-search"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateSearchAgentKey(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Secret string `json:"secret"`
			Prefix string `json:"prefix"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.NotEmpty(t, payload.Data.Secret)
	require.True(t, strings.HasPrefix(payload.Data.Secret, "vr_live_"))
	assert.NotEqual(t, payload.Data.Secret, payload.Data.Prefix, "prefix must not contain the complete secret")
	var stored model.SearchAgentKey
	require.NoError(t, model.DB.First(&stored).Error)
	assert.NotContains(t, recorder.Body.String(), stored.KeyHash, "raw secret/hash leaked in response")
	assert.NotEqual(t, payload.Data.Secret, stored.KeyHash, "raw secret/hash leaked in response")
}

func TestRevokeSearchAgentKeyEnforcesOwnership(t *testing.T) {
	db := openSearchAgentKeyControllerDB(t)
	key, _, err := model.CreateSearchAgentKey(7, 0, "browser", nil)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 8)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(key.Id)}}
	RevokeSearchAgentKey(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var stored model.SearchAgentKey
	require.NoError(t, db.First(&stored, key.Id).Error)
	assert.NotEqual(t, model.SearchAgentKeyStatusRevoked, stored.Status, "non-owner revoked the key")
}
