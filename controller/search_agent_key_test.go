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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SearchAgentKey{}, &model.EnterpriseMembership{}, &model.AuthFlow{}))
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "alice", Password: "password"}).Error)
	return db
}

func TestCreateSearchAgentKeyReturnsSecretOnce(t *testing.T) {
	openSearchAgentKeyControllerDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 7)
	c.Request = httptest.NewRequest("POST", "/api/search/keys", bytes.NewBufferString(`{"name":"browser","scopes":["web-search"]}`))
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

func TestSearchAgentInstallKeepsOldSecretUntilIdempotentActivation(t *testing.T) {
	t.Setenv("VSEARCH_PUBLIC_MCP_URL", "https://search.example.com/v1/mcp")
	openSearchAgentKeyControllerDB(t)
	key, oldSecret, err := model.CreateSearchAgentKey(7, 0, "browser", nil)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	tokenRecorder := httptest.NewRecorder()
	tokenContext, _ := gin.CreateTestContext(tokenRecorder)
	tokenContext.Set("id", 7)
	tokenContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(key.Id)}}
	CreateSearchAgentKeyInstallToken(tokenContext)
	require.Equal(t, http.StatusOK, tokenRecorder.Code)
	var tokenPayload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(tokenRecorder.Body.Bytes(), &tokenPayload))
	require.NotEmpty(t, tokenPayload.Data.Token)

	installRecorder := httptest.NewRecorder()
	installContext, _ := gin.CreateTestContext(installRecorder)
	installContext.Request = httptest.NewRequest("POST", "/api/agent/install", bytes.NewBufferString(fmt.Sprintf(`{"token":%q,"label":"browser"}`, tokenPayload.Data.Token)))
	installContext.Request.Header.Set("Content-Type", "application/json")
	InstallSearchAgent(installContext)
	require.Equal(t, http.StatusOK, installRecorder.Code, installRecorder.Body.String())
	var installPayload struct {
		Data struct {
			Secret          string `json:"secret"`
			ActivationToken string `json:"activation_token"`
			Installed       bool   `json:"installed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(installRecorder.Body.Bytes(), &installPayload))
	require.NotEmpty(t, installPayload.Data.Secret)
	require.NotEmpty(t, installPayload.Data.ActivationToken)
	assert.False(t, installPayload.Data.Installed)
	_, err = model.FindSearchAgentKeyBySecret(oldSecret)
	require.NoError(t, err, "old secret must stay valid until local configuration is durable")
	_, err = model.FindSearchAgentKeyBySecret(installPayload.Data.Secret)
	require.Error(t, err, "prepared secret must not be active before acknowledgement")

	activate := func() bool {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/agent/install/activate", bytes.NewBufferString(fmt.Sprintf(`{"token":%q}`, installPayload.Data.ActivationToken)))
		context.Request.Header.Set("Content-Type", "application/json")
		ActivateSearchAgentInstall(context)
		var payload struct {
			Success bool `json:"success"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload.Success
	}
	require.True(t, activate())
	require.True(t, activate(), "lost activation responses must be safe to retry")
	_, err = model.FindSearchAgentKeyBySecret(oldSecret)
	require.Error(t, err, "old secret should be invalid after activation")
	_, err = model.FindSearchAgentKeyBySecret(installPayload.Data.Secret)
	require.NoError(t, err)

	replayRecorder := httptest.NewRecorder()
	replayContext, _ := gin.CreateTestContext(replayRecorder)
	replayContext.Request = httptest.NewRequest("POST", "/api/agent/install", bytes.NewBufferString(fmt.Sprintf(`{"token":%q}`, tokenPayload.Data.Token)))
	replayContext.Request.Header.Set("Content-Type", "application/json")
	InstallSearchAgent(replayContext)
	var replayPayload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload))
	assert.False(t, replayPayload.Success, "install token replay should fail")
}

func TestSearchAgentInstallTokenInvalidatesOtherOutstandingTokens(t *testing.T) {
	t.Setenv("VSEARCH_PUBLIC_MCP_URL", "https://search.example.com/v1/mcp")
	openSearchAgentKeyControllerDB(t)
	key, _, err := model.CreateSearchAgentKey(7, 0, "browser", nil)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	issueToken := func() string {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Set("id", 7)
		context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(key.Id)}}
		CreateSearchAgentKeyInstallToken(context)
		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.NotEmpty(t, payload.Data.Token)
		return payload.Data.Token
	}
	prepareToken := func(token string) (bool, string, string) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/agent/install", bytes.NewBufferString(fmt.Sprintf(`{"token":%q}`, token)))
		context.Request.Header.Set("Content-Type", "application/json")
		InstallSearchAgent(context)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Secret          string `json:"secret"`
				ActivationToken string `json:"activation_token"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload.Success, payload.Data.Secret, payload.Data.ActivationToken
	}
	activateToken := func(token string) bool {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/agent/install/activate", bytes.NewBufferString(fmt.Sprintf(`{"token":%q}`, token)))
		context.Request.Header.Set("Content-Type", "application/json")
		ActivateSearchAgentInstall(context)
		var payload struct {
			Success bool `json:"success"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload.Success
	}

	firstToken := issueToken()
	secondToken := issueToken()
	success, installedSecret, activationToken := prepareToken(secondToken)
	require.True(t, success)
	require.NotEmpty(t, installedSecret)
	require.True(t, activateToken(activationToken))

	success, _, _ = prepareToken(firstToken)
	assert.False(t, success, "one successful exchange must invalidate every token issued for the prior credential version")
	_, err = model.FindSearchAgentKeyBySecret(installedSecret)
	require.NoError(t, err, "a stale outstanding token must not rotate the newly installed secret")
}

func TestSearchAgentInstallTokenUserRouteRequiresKeyOwner(t *testing.T) {
	openSearchAgentKeyControllerDB(t)
	key, oldSecret, err := model.CreateSearchAgentKey(7, 11, "browser", nil)
	require.NoError(t, err)

	for _, role := range []string{model.EnterpriseMembershipRoleMember, model.EnterpriseMembershipRoleAuditor} {
		t.Run(role, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", 8)
			c.Set("enterprise_id", 11)
			c.Set("enterprise_role", role)
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(key.Id)}}

			CreateSearchAgentKeyInstallToken(c)

			require.Equal(t, http.StatusForbidden, recorder.Code)
			var payload struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.False(t, payload.Success)
			_, err := model.FindSearchAgentKeyBySecret(oldSecret)
			require.NoError(t, err)
		})
	}
}

func TestSearchAgentInstallTokenAdminRouteAllowsManagedKey(t *testing.T) {
	openSearchAgentKeyControllerDB(t)
	key, _, err := model.CreateSearchAgentKey(7, 11, "browser", nil)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 8)
	c.Set("enterprise_id", 11)
	c.Set("enterprise_role", model.EnterpriseMembershipRoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(key.Id)}}

	AdminCreateSearchAgentKeyInstallToken(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.NotEmpty(t, payload.Data.Token)
}

func TestSearchPublicMCPURLOnlyFallsBackForLoopbackHosts(t *testing.T) {
	t.Setenv("VSEARCH_PUBLIC_MCP_URL", "")
	gin.SetMode(gin.TestMode)

	localhostRecorder := httptest.NewRecorder()
	localhostContext, _ := gin.CreateTestContext(localhostRecorder)
	localhostContext.Request = httptest.NewRequest(http.MethodPost, "http://localhost/api/agent/install", nil)
	localhostContext.Request.Host = "127.0.0.1:3000"
	localhostContext.Request.Header.Set("X-Forwarded-Proto", "https")
	assert.Equal(t, "https://127.0.0.1:3000/v1/mcp", searchPublicMCPURL(localhostContext))

	publicRecorder := httptest.NewRecorder()
	publicContext, _ := gin.CreateTestContext(publicRecorder)
	publicContext.Request = httptest.NewRequest(http.MethodPost, "https://relay.example/api/agent/install", nil)
	publicContext.Request.Host = "attacker.example"
	publicContext.Request.Header.Set("X-Forwarded-Proto", "https")
	assert.Empty(t, searchPublicMCPURL(publicContext))
}

func TestSearchPublicMCPURLPrefersConfiguredURL(t *testing.T) {
	t.Setenv("VSEARCH_PUBLIC_MCP_URL", "https://search.example.com/v1/mcp/")
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://relay.example/api/agent/install", nil)
	c.Request.Host = "attacker.example"
	assert.Equal(t, "https://search.example.com/v1/mcp", searchPublicMCPURL(c))
}

func TestSearchPublicMCPURLRejectsInsecureOrAmbiguousConfiguredURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "https://relay.example/api/agent/install", nil)
	c.Request.Host = "relay.example"

	invalid := []string{
		"http://search.example.com/v1/mcp",
		"https://user@search.example.com/v1/mcp",
		"https://search.example.com/v1/mcp?token=x",
		"https://search.example.com/v1/mcp#fragment",
	}
	for _, configured := range invalid {
		t.Run(configured, func(t *testing.T) {
			t.Setenv("VSEARCH_PUBLIC_MCP_URL", configured)
			assert.Empty(t, searchPublicMCPURL(c))
		})
	}

	t.Setenv("VSEARCH_PUBLIC_MCP_URL", "http://127.0.0.1:3000/v1/mcp")
	assert.Equal(t, "http://127.0.0.1:3000/v1/mcp", searchPublicMCPURL(c))
}
