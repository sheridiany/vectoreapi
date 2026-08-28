package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/vsearch"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSearchMCPContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	key := &model.SearchAgentKey{Id: 9, UserId: 7, EnterpriseID: 11}
	require.NoError(t, key.SetScopes(nil))
	context.Set("search_agent_key", key)
	return context, recorder
}

func TestSearchMCPToolsListExposesOnlyFourStablePublicTools(t *testing.T) {
	context, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)

	HandleSearchMCP(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Result.Tools, 4)
	assert.Equal(t, []string{
		"vector_relay_capabilities", "vector_relay_find_tools",
		"vector_relay_describe_tool", "vector_relay_call",
	}, []string{payload.Result.Tools[0].Name, payload.Result.Tools[1].Name, payload.Result.Tools[2].Name, payload.Result.Tools[3].Name})
	assert.NotContains(t, recorder.Body.String(), "private/")
}

func TestSearchMCPRejectsRawUpstreamToolName(t *testing.T) {
	context, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vector_relay_call","arguments":{"serviceId":"private/search","params":{}}}}`)

	HandleSearchMCP(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, -32602, payload.Error.Code)
}

func TestSearchMCPInitializeReturnsProtocolAndSession(t *testing.T) {
	context, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":"init","method":"initialize","params":{}}`)

	HandleSearchMCP(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("mcp-session-id"))
	assert.Contains(t, recorder.Body.String(), searchMCPProtocolVersion)
}

func TestSearchMCPNotificationReturnsAcceptedWithoutBody(t *testing.T) {
	context, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`)

	HandleSearchMCP(context)

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}

func TestVSearchPublicErrorPreservesWrappedIndeterminateError(t *testing.T) {
	expected := &vsearch.PublicError{
		Code:       "VSEARCH_EXECUTION_INDETERMINATE",
		Message:    "执行结果不确定，请勿自动重试。",
		HTTPStatus: http.StatusInternalServerError,
	}

	actual := vsearchPublicError(fmt.Errorf("execution dispatched: %w", expected))

	assert.Same(t, expected, actual)
}
