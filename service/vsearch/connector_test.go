package vsearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedMCPRequest struct {
	Body          map[string]any
	Authorization string
	SessionID     string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type fakeMCPServer struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []recordedMCPRequest
	sessions int
}

func startFakeMCPServer(t *testing.T, responseType string) *fakeMCPServer {
	t.Helper()
	fake := &fakeMCPServer{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body map[string]any
		require.NoError(t, common.DecodeJson(request.Body, &body))

		fake.mu.Lock()
		fake.requests = append(fake.requests, recordedMCPRequest{
			Body:          body,
			Authorization: request.Header.Get("Authorization"),
			SessionID:     request.Header.Get("mcp-session-id"),
		})
		method, _ := body["method"].(string)
		if method == "initialize" {
			fake.sessions++
			response.Header().Set("mcp-session-id", "session-"+string(rune('0'+fake.sessions)))
		}
		fake.mu.Unlock()

		if method == "notifications/initialized" {
			response.WriteHeader(http.StatusAccepted)
			return
		}

		id := body["id"]
		var result any
		if method == "initialize" {
			result = map[string]any{
				"protocolVersion": AgentKeyProtocolVersion,
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]any{"name": "fake", "version": "1"},
			}
		} else {
			result = map[string]any{"received": body["params"]}
		}
		payload, err := common.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		require.NoError(t, err)
		if responseType == "sse" {
			response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			_, err = response.Write([]byte("event: message\ndata: " + string(payload) + "\n\n"))
		} else {
			response.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, err = response.Write(payload)
		}
		require.NoError(t, err)
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func newTestConnector(t *testing.T, serverURL string, configPatch func(*AgentKeyConnectorConfig)) *AgentKeyConnector {
	t.Helper()
	config := AgentKeyConnectorConfig{
		BaseURL:           serverURL,
		Secret:            "private-test-secret",
		Timeout:           time.Second,
		MaxResponseBytes:  64 << 10,
		AllowLoopbackHTTP: true,
	}
	if configPatch != nil {
		configPatch(&config)
	}
	connector, err := NewAgentKeyConnector(config)
	require.NoError(t, err)
	return connector
}

func TestAgentKeyConnectorUsesIsolatedMCPSessions(t *testing.T) {
	fake := startFakeMCPServer(t, "json")
	connector := newTestConnector(t, fake.server.URL, nil)

	_, err := connector.Account(context.Background())
	require.NoError(t, err)
	_, err = connector.FindTools(context.Background(), "search recent news", "brave")
	require.NoError(t, err)

	fake.mu.Lock()
	requests := append([]recordedMCPRequest(nil), fake.requests...)
	fake.mu.Unlock()
	require.Len(t, requests, 6)

	assert.Equal(t, "initialize", requests[0].Body["method"])
	initializeParams := requests[0].Body["params"].(map[string]any)
	assert.Equal(t, AgentKeyProtocolVersion, initializeParams["protocolVersion"])
	assert.Equal(t, "Bearer private-test-secret", requests[0].Authorization)
	assert.Empty(t, requests[0].SessionID)
	assert.Equal(t, "notifications/initialized", requests[1].Body["method"])
	assert.Equal(t, "session-1", requests[1].SessionID)
	assert.Equal(t, "tools/call", requests[2].Body["method"])
	assert.Equal(t, "session-1", requests[2].SessionID)

	accountCall := requests[2].Body["params"].(map[string]any)
	assert.Equal(t, "execute_tool", accountCall["name"])
	accountArguments := accountCall["arguments"].(map[string]any)
	assert.Equal(t, "agentkey_account", accountArguments["name"])

	assert.Equal(t, "initialize", requests[3].Body["method"])
	assert.Empty(t, requests[3].SessionID)
	assert.Equal(t, "session-2", requests[4].SessionID)
	findCall := requests[5].Body["params"].(map[string]any)
	assert.Equal(t, "find_tools", findCall["name"])
	findArguments := findCall["arguments"].(map[string]any)
	assert.Equal(t, "search recent news", findArguments["q"])
	assert.Equal(t, "brave", findArguments["prefix"])
}

func TestAgentKeyConnectorToolContractsAndResponseFormats(t *testing.T) {
	for _, responseType := range []string{"json", "sse"} {
		t.Run(responseType, func(t *testing.T) {
			fake := startFakeMCPServer(t, responseType)
			connector := newTestConnector(t, fake.server.URL, nil)

			result, err := connector.DescribeTool(context.Background(), "brave/search")
			require.NoError(t, err)
			assert.NotNil(t, result)
			result, err = connector.ExecuteTool(context.Background(), "brave/search", map[string]any{"query": "agents"})
			require.NoError(t, err)
			assert.NotNil(t, result)

			fake.mu.Lock()
			requests := append([]recordedMCPRequest(nil), fake.requests...)
			fake.mu.Unlock()
			require.Len(t, requests, 6)
			describe := requests[2].Body["params"].(map[string]any)
			assert.Equal(t, "describe_tool", describe["name"])
			assert.Equal(t, "brave/search", describe["arguments"].(map[string]any)["name"])
			execute := requests[5].Body["params"].(map[string]any)
			assert.Equal(t, "execute_tool", execute["name"])
			executeArguments := execute["arguments"].(map[string]any)
			assert.Equal(t, "brave/search", executeArguments["name"])
			assert.Equal(t, "agents", executeArguments["params"].(map[string]any)["query"])
		})
	}
}

func TestAgentKeyConnectorMapsHTTPFailuresWithoutLeakingDetails(t *testing.T) {
	tests := []struct {
		status int
		code   string
	}{
		{status: http.StatusUnauthorized, code: "UPSTREAM_AUTH_FAILED"},
		{status: http.StatusPaymentRequired, code: "UPSTREAM_CREDITS_UNAVAILABLE"},
		{status: http.StatusTooManyRequests, code: "UPSTREAM_RATE_LIMITED"},
		{status: http.StatusInternalServerError, code: "UPSTREAM_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "text/plain")
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte("raw provider failure private-test-secret"))
			}))
			t.Cleanup(server.Close)
			connector := newTestConnector(t, server.URL, nil)

			_, err := connector.Account(context.Background())
			var connectorError *ConnectorError
			require.ErrorAs(t, err, &connectorError)
			assert.Equal(t, test.code, connectorError.Code)
			assert.NotContains(t, err.Error(), "private-test-secret")
			assert.NotContains(t, err.Error(), "raw provider failure")
		})
	}
}

func TestAgentKeyConnectorMarksOnlyDispatchedToolFailures(t *testing.T) {
	t.Run("initialize failure is predispatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(server.Close)
		connector := newTestConnector(t, server.URL, nil)

		_, err := connector.ExecuteTool(context.Background(), "brave/search", map[string]any{"query": "agents"})
		require.Error(t, err)
		var dispatchedErr *executionDispatchedError
		assert.False(t, errors.As(err, &dispatchedErr))
	})

	t.Run("tools call failure is dispatched", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			var body map[string]any
			require.NoError(t, common.DecodeJson(request.Body, &body))
			method, _ := body["method"].(string)
			if method == "initialize" {
				response.Header().Set("Content-Type", "application/json")
				payload, err := common.Marshal(map[string]any{
					"jsonrpc": "2.0", "id": body["id"], "result": map[string]any{"protocolVersion": AgentKeyProtocolVersion},
				})
				require.NoError(t, err)
				_, _ = response.Write(payload)
				return
			}
			if method == "notifications/initialized" {
				response.WriteHeader(http.StatusAccepted)
				return
			}
			response.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		connector := newTestConnector(t, server.URL, nil)

		_, err := connector.ExecuteTool(context.Background(), "brave/search", map[string]any{"query": "agents"})
		require.Error(t, err)
		var dispatchedErr *executionDispatchedError
		assert.True(t, errors.As(err, &dispatchedErr))
		var connectorErr *ConnectorError
		require.ErrorAs(t, err, &connectorErr)
		assert.Equal(t, "UPSTREAM_UNAVAILABLE", connectorErr.Code)
	})
}

func TestAgentKeyConnectorMapsRPCAndMalformedResponses(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		code        string
	}{
		{
			name:        "rpc error",
			contentType: "application/json",
			body:        `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"raw private failure"}}`,
			code:        "UPSTREAM_TOOL_ERROR",
		},
		{
			name:        "malformed json",
			contentType: "application/json",
			body:        `{"jsonrpc":`,
			code:        "UPSTREAM_INVALID_RESPONSE",
		},
		{
			name:        "unsupported content type",
			contentType: "text/plain",
			body:        `private raw response`,
			code:        "UPSTREAM_INVALID_RESPONSE",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", test.contentType)
				_, _ = response.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			connector := newTestConnector(t, server.URL, nil)

			_, err := connector.Account(context.Background())
			var connectorError *ConnectorError
			require.ErrorAs(t, err, &connectorError)
			assert.Equal(t, test.code, connectorError.Code)
			assert.NotContains(t, err.Error(), "private")
		})
	}
}

func TestAgentKeyConnectorHonorsCancellationTimeoutAndResponseLimit(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		connector := newTestConnector(t, "http://127.0.0.1:8080/v1/mcp", func(config *AgentKeyConnectorConfig) {
			config.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			})}
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := connector.Account(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		var connectorError *ConnectorError
		require.ErrorAs(t, err, &connectorError)
		assert.Equal(t, "UPSTREAM_REQUEST_CANCELED", connectorError.Code)
	})

	t.Run("operation timeout", func(t *testing.T) {
		connector := newTestConnector(t, "http://127.0.0.1:8080/v1/mcp", func(config *AgentKeyConnectorConfig) {
			config.Timeout = 20 * time.Millisecond
			config.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			})}
		})

		_, err := connector.Account(context.Background())
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		var connectorError *ConnectorError
		require.ErrorAs(t, err, &connectorError)
		assert.Equal(t, "UPSTREAM_TIMEOUT", connectorError.Code)
	})

	t.Run("response limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(strings.Repeat("x", 128)))
		}))
		t.Cleanup(server.Close)
		connector := newTestConnector(t, server.URL, func(config *AgentKeyConnectorConfig) {
			config.MaxResponseBytes = 64
		})

		_, err := connector.Account(context.Background())
		var connectorError *ConnectorError
		require.ErrorAs(t, err, &connectorError)
		assert.Equal(t, "UPSTREAM_RESPONSE_TOO_LARGE", connectorError.Code)
	})
}

func TestNewAgentKeyConnectorValidatesURLAndSecret(t *testing.T) {
	tests := []struct {
		name   string
		config AgentKeyConnectorConfig
		code   string
	}{
		{name: "non tls remote", config: AgentKeyConnectorConfig{BaseURL: "http://example.com/v1/mcp", Secret: "key"}, code: "UPSTREAM_URL_HTTPS_REQUIRED"},
		{name: "loopback requires opt in", config: AgentKeyConnectorConfig{BaseURL: "http://127.0.0.1:8080/v1/mcp", Secret: "key"}, code: "UPSTREAM_URL_HTTPS_REQUIRED"},
		{name: "userinfo rejected", config: AgentKeyConnectorConfig{BaseURL: "https://user@example.com/v1/mcp", Secret: "key"}, code: "UPSTREAM_URL_INVALID"},
		{name: "missing secret", config: AgentKeyConnectorConfig{BaseURL: "https://example.com/v1/mcp"}, code: "UPSTREAM_SECRET_REQUIRED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAgentKeyConnector(test.config)
			var connectorError *ConnectorError
			require.ErrorAs(t, err, &connectorError)
			assert.Equal(t, test.code, connectorError.Code)
		})
	}

	connector, err := NewAgentKeyConnector(AgentKeyConnectorConfig{
		BaseURL:           "http://localhost:8080/v1/mcp",
		Secret:            "key",
		AllowLoopbackHTTP: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/v1/mcp", connector.endpoint.String())
	assert.Equal(t, DefaultAgentKeyMCPURL, func() string {
		defaultConnector, defaultErr := NewAgentKeyConnector(AgentKeyConnectorConfig{Secret: "key"})
		require.NoError(t, defaultErr)
		return defaultConnector.endpoint.String()
	}())
}

func TestAgentKeyConnectorRejectsMissingToolNameBeforeNetwork(t *testing.T) {
	connector, err := NewAgentKeyConnector(AgentKeyConnectorConfig{Secret: "private-test-secret"})
	require.NoError(t, err)

	_, err = connector.DescribeTool(context.Background(), "  ")
	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_TOOL_NAME_REQUIRED", connectorError.Code)
	_, err = connector.ExecuteTool(context.Background(), "", nil)
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_TOOL_NAME_REQUIRED", connectorError.Code)
}

func TestConnectorContextErrorUnwrapsOnlySafeContextErrors(t *testing.T) {
	assert.True(t, errors.Is(newConnectorContextError(context.Canceled), context.Canceled))
	assert.True(t, errors.Is(newConnectorContextError(context.DeadlineExceeded), context.DeadlineExceeded))
}
