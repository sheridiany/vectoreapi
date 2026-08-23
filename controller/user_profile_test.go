package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpdateSelfTrimsDisplayName(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	user := &model.User{
		Id: 1, Username: "profile-user", Password: "password123", DisplayName: "Old Name",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}
	require.NoError(t, db.Create(user).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", user.Id)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/user/self",
		strings.NewReader(`{"display_name":"  Alice Chen  "}`),
	)

	UpdateSelf(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var stored model.User
	require.NoError(t, db.Select("display_name").First(&stored, user.Id).Error)
	require.Equal(t, "Alice Chen", stored.DisplayName)
}
