package middleware

import (
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

func openSearchMiddlewareDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, database.AutoMigrate(
		&model.User{},
		&model.Enterprise{},
		&model.EnterpriseMembership{},
		&model.SearchAgentKey{},
	))
}

func TestSearchAgentKeyAuthSetsIdentityAndTouchesLastUsed(t *testing.T) {
	openSearchMiddlewareDB(t)
	user := &model.User{Username: "vsearch-user", Status: common.UserStatusEnabled, Role: common.RoleCommonUser}
	require.NoError(t, model.DB.Create(user).Error)
	key, secret, err := model.CreateSearchAgentKey(user.Id, 0, "agent", nil)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SearchAgentKeyAuth())
	engine.POST("/v1/mcp", func(c *gin.Context) {
		assert.Equal(t, user.Id, c.GetInt("id"))
		assert.Equal(t, key.Id, c.GetInt("search_agent_key_id"))
		authenticatedKey, ok := c.MustGet(SearchAgentKeyContextKey).(*model.SearchAgentKey)
		require.True(t, ok)
		assert.Equal(t, key.Id, authenticatedKey.Id)
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/mcp", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	stored, err := model.GetSearchAgentKeyByID(key.Id)
	require.NoError(t, err)
	assert.Positive(t, stored.LastUsedAt)
}

func TestSearchAgentKeyAuthRejectsInvalidSecretWithoutEchoingIt(t *testing.T) {
	openSearchMiddlewareDB(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SearchAgentKeyAuth())
	engine.POST("/v1/mcp", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/v1/mcp", nil)
	request.Header.Set("Authorization", "Bearer vr_live_sensitive_invalid")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "vr_live_sensitive_invalid")
	assert.Contains(t, recorder.Body.String(), "-32001")
}

func TestSearchAdminAuthAllowsRootAndActiveEnterpriseManagers(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		platformRole   int
		enterpriseRole string
		wantEnterprise int
	}{
		{name: "root", platformRole: common.RoleRootUser},
		{name: "enterprise owner", platformRole: common.RoleCommonUser, enterpriseRole: model.EnterpriseMembershipRoleOwner, wantEnterprise: 11},
		{name: "enterprise admin", platformRole: common.RoleCommonUser, enterpriseRole: model.EnterpriseMembershipRoleAdmin, wantEnterprise: 11},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			openSearchMiddlewareDB(t)
			user := &model.User{Username: "search-manager", Status: common.UserStatusEnabled, Role: testCase.platformRole}
			require.NoError(t, model.DB.Create(user).Error)
			if testCase.enterpriseRole != "" {
				require.NoError(t, model.DB.Create(&model.Enterprise{
					Id: 11, Name: "Northstar", Code: "northstar", Status: model.EnterpriseStatusEnabled,
				}).Error)
				require.NoError(t, model.DB.Create(&model.EnterpriseMembership{
					EnterpriseID: 11, UserID: user.Id, Role: testCase.enterpriseRole, Status: model.EnterpriseMembershipStatusActive,
				}).Error)
			}

			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				c.Set("id", user.Id)
				c.Set("role", testCase.platformRole)
				c.Next()
			})
			engine.Use(SearchAdminAuth())
			engine.GET("/api/search/admin/agent-keys", func(c *gin.Context) {
				assert.Equal(t, testCase.wantEnterprise, c.GetInt("enterprise_id"))
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/search/admin/agent-keys", nil)
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusNoContent, recorder.Code)
		})
	}
}

func TestSearchAdminAuthRejectsEnterpriseMembersAndAuditors(t *testing.T) {
	for _, enterpriseRole := range []string{
		model.EnterpriseMembershipRoleMember,
		model.EnterpriseMembershipRoleAuditor,
	} {
		t.Run(enterpriseRole, func(t *testing.T) {
			openSearchMiddlewareDB(t)
			user := &model.User{Username: "search-viewer", Status: common.UserStatusEnabled, Role: common.RoleCommonUser}
			require.NoError(t, model.DB.Create(user).Error)
			require.NoError(t, model.DB.Create(&model.Enterprise{
				Id: 11, Name: "Northstar", Code: "northstar", Status: model.EnterpriseStatusEnabled,
			}).Error)
			require.NoError(t, model.DB.Create(&model.EnterpriseMembership{
				EnterpriseID: 11, UserID: user.Id, Role: enterpriseRole, Status: model.EnterpriseMembershipStatusActive,
			}).Error)

			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				c.Set("id", user.Id)
				c.Set("role", common.RoleCommonUser)
				c.Next()
			})
			engine.Use(SearchAdminAuth())
			engine.GET("/api/search/admin/agent-keys", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/search/admin/agent-keys", nil)
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}
