package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type flowQuotaResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Data    []model.FlowQuotaData `json:"data"`
}

type userQuotaResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    []model.QuotaData `json:"data"`
}

func setupFlowControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.QuotaData{}, &model.Enterprise{}))
	for _, enterprise := range []*model.Enterprise{
		{Id: 11, Name: "Alpha", Code: "alpha", Status: model.EnterpriseStatusEnabled},
		{Id: 12, Name: "Beta", Code: "beta", Status: model.EnterpriseStatusEnabled},
		{Id: 42, Name: "Gamma", Code: "gamma", Status: model.EnterpriseStatusDisabled},
	} {
		require.NoError(t, model.DB.Create(enterprise).Error)
	}
	require.NoError(t, model.DB.Create(&model.Channel{Id: 1, Name: "east"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 11, UserId: 1, Key: "sk-primary", Name: "primary"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 22, UserId: 2, Key: "sk-backup", Name: "backup"}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:       1,
		EnterpriseID: 11,
		Username:     "alice",
		NodeName:     "node-a",
		TokenID:      11,
		UseGroup:     "default",
		ChannelID:    1,
		ModelName:    "gpt-a",
		CreatedAt:    1100,
		Count:        2,
		Quota:        100,
		TokenUsed:    40,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:       2,
		EnterpriseID: 12,
		Username:     "bob",
		NodeName:     "node-b",
		TokenID:      22,
		UseGroup:     "vip",
		ChannelID:    1,
		ModelName:    "gpt-b",
		CreatedAt:    1200,
		Count:        1,
		Quota:        70,
		TokenUsed:    30,
	}).Error)
}

func TestDashboardEnterpriseScopeRequiresRootAndExistingPositiveEnterprise(t *testing.T) {
	setupFlowControllerTestDB(t)

	tests := []struct {
		name       string
		role       int
		query      string
		wantStatus int
	}{
		{name: "admin cannot select an enterprise", role: common.RoleAdminUser, query: "enterprise_id=11", wantStatus: http.StatusForbidden},
		{name: "enterprise id must be numeric", role: common.RoleRootUser, query: "enterprise_id=invalid", wantStatus: http.StatusBadRequest},
		{name: "enterprise id must be positive", role: common.RoleRootUser, query: "enterprise_id=0", wantStatus: http.StatusBadRequest},
		{name: "enterprise must exist", role: common.RoleRootUser, query: "enterprise_id=999", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("role", test.role)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data?start_timestamp=1000&end_timestamp=2000&"+test.query, nil)

			GetAllQuotaDates(ctx)

			require.Equal(t, test.wantStatus, recorder.Code)
			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			require.False(t, payload.Success)
			require.NotEmpty(t, payload.Message)
		})
	}
}

func TestEveryAdminDashboardEndpointRejectsEnterpriseScopeFromAdmin(t *testing.T) {
	setupFlowControllerTestDB(t)

	tests := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "model data", path: "/api/data?start_timestamp=1000&end_timestamp=2000&enterprise_id=11", handler: GetAllQuotaDates},
		{name: "user data", path: "/api/data/users?start_timestamp=1000&end_timestamp=2000&enterprise_id=11", handler: GetQuotaDatesByUser},
		{name: "flow data", path: "/api/data/flow?start_timestamp=1000&end_timestamp=2000&enterprise_id=11", handler: GetAllFlowQuotaDates},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("role", common.RoleAdminUser)
			ctx.Request = httptest.NewRequest(http.MethodGet, test.path, nil)

			test.handler(ctx)

			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}

func TestGetAllQuotaDatesFiltersByEnterpriseAndUsername(t *testing.T) {
	setupFlowControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: 4, EnterpriseID: 12, Username: "alice", ModelName: "gpt-other-enterprise",
		CreatedAt: 1150, Count: 7, Quota: 700, TokenUsed: 200,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleRootUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data?start_timestamp=1000&end_timestamp=2000&enterprise_id=11&username=alice", nil)

	GetAllQuotaDates(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.Equal(t, []model.QuotaData{{
		UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 1100, Count: 2, Quota: 100, TokenUsed: 40,
	}}, payload.Data)
}

func decodeFlowQuotaResponse(t *testing.T, recorder *httptest.ResponseRecorder) flowQuotaResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload flowQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	return payload
}

func TestGetAllFlowQuotaDatesUsesAdminDimensions(t *testing.T) {
	setupFlowControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/flow?start_timestamp=1000&end_timestamp=2000&username=bob", nil)

	GetAllFlowQuotaDates(ctx)

	payload := decodeFlowQuotaResponse(t, recorder)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "bob", payload.Data[0].Username)
	require.Equal(t, "vip", payload.Data[0].UseGroup)
	require.Equal(t, "east", payload.Data[0].ChannelName)
	require.Empty(t, payload.Data[0].TokenName)
	require.Empty(t, payload.Data[0].NodeName)
}

func TestGetAllFlowQuotaDatesUsesRootDimensions(t *testing.T) {
	setupFlowControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID: 3, EnterpriseID: 12, Username: "alice", NodeName: "node-other",
		TokenID: 22, UseGroup: "vip", ChannelID: 1, ModelName: "gpt-other-enterprise",
		CreatedAt: 1150, Count: 5, Quota: 500, TokenUsed: 200,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleRootUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/flow?start_timestamp=1000&end_timestamp=2000&username=alice&enterprise_id=11", nil)

	GetAllFlowQuotaDates(ctx)

	payload := decodeFlowQuotaResponse(t, recorder)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "alice", payload.Data[0].Username)
	require.Equal(t, "node-a", payload.Data[0].NodeName)
	require.Equal(t, "primary", payload.Data[0].TokenName)
	require.Equal(t, "default", payload.Data[0].UseGroup)
	require.Equal(t, "east", payload.Data[0].ChannelName)
}

func TestGetUserFlowQuotaDatesRestrictsToAuthenticatedUser(t *testing.T) {
	setupFlowControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/flow/self?start_timestamp=1000&end_timestamp=2000&enterprise_id=12", nil)

	GetUserFlowQuotaDates(ctx)

	payload := decodeFlowQuotaResponse(t, recorder)
	require.Len(t, payload.Data, 1)
	require.Empty(t, payload.Data[0].Username)
	require.Equal(t, "primary", payload.Data[0].TokenName)
	require.Equal(t, "default", payload.Data[0].UseGroup)
	require.Empty(t, payload.Data[0].ChannelName)
}

func TestGetUserQuotaDatesIgnoresEnterpriseScopeAndUsesAuthenticatedUser(t *testing.T) {
	setupFlowControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/self?start_timestamp=1000&end_timestamp=2000&enterprise_id=12", nil)

	GetUserQuotaDates(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data, 1)
	require.Equal(t, 1, payload.Data[0].UserID)
	require.Equal(t, "alice", payload.Data[0].Username)
}

func TestGetUserFlowQuotaDatesRejectsInvalidTimeRange(t *testing.T) {
	setupFlowControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/flow/self?start_timestamp=bad&end_timestamp=2000", nil)

	GetUserFlowQuotaDates(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload flowQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Equal(t, "invalid start_timestamp", payload.Message)
}

func TestGetQuotaDatesByUserFiltersByEnterprise(t *testing.T) {
	setupFlowControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 3, Username: "carol", Password: "password123", DisplayName: "Carol Chen",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:       3,
		EnterpriseID: 42,
		Username:     "carol",
		ModelName:    "gpt-c",
		CreatedAt:    1300,
		Count:        1,
		Quota:        90,
		TokenUsed:    30,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleRootUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/users?start_timestamp=1000&end_timestamp=2000&enterprise_id=42", nil)

	GetQuotaDatesByUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data, 1)
	require.Equal(t, 42, payload.Data[0].EnterpriseID)
	require.Equal(t, "carol", payload.Data[0].Username)
	require.Equal(t, "Carol Chen", payload.Data[0].DisplayName)
}

func TestGetQuotaDatesByUserRejectsInvalidEnterpriseFilter(t *testing.T) {
	setupFlowControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleRootUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/users?start_timestamp=1000&end_timestamp=2000&enterprise_id=invalid", nil)

	GetQuotaDatesByUser(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var payload userQuotaResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Equal(t, "invalid enterprise_id", payload.Message)
}
