package vsearch

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type justOneRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn justOneRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newJustOneTestAdapter(t *testing.T, serverURL string, patch func(*AdapterConfig)) ProviderAdapter {
	t.Helper()
	config := AdapterConfig{
		Provider:          ProviderJustOneAPI,
		BaseURL:           serverURL,
		Secret:            "private-test-secret",
		Timeout:           time.Second,
		MaxResponseBytes:  64 << 10,
		AllowLoopbackHTTP: true,
	}
	if patch != nil {
		patch(&config)
	}
	adapter, err := NewJustOneAPIAdapter(config)
	require.NoError(t, err)
	return adapter
}

func justOneTestOperation(operation ProviderOperation) ProviderOperation {
	operation.MappingKey = justOneAPIDirectMappingKey
	operation.MappingVersion = "v1"
	return operation
}

func writeJustOneJSON(t *testing.T, response http.ResponseWriter, status int, payload any) {
	t.Helper()
	encoded, err := common.Marshal(payload)
	if !assert.NoError(t, err) {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_, err = response.Write(encoded)
	assert.NoError(t, err)
}

func TestJustOneAPIAdapterDoesNotInheritInsecureDefaultTLS(t *testing.T) {
	originalTransport := http.DefaultTransport
	unsafeTransport := originalTransport.(*http.Transport).Clone()
	unsafeTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS10, InsecureSkipVerify: true}
	http.DefaultTransport = unsafeTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	adapter, err := NewJustOneAPIAdapter(AdapterConfig{
		Provider: ProviderJustOneAPI, BaseURL: DefaultJustOneAPIBaseURL, Secret: "private-test-secret",
	})
	require.NoError(t, err)
	transport := adapter.(*JustOneAPIAdapter).client.Transport.(*http.Transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.GreaterOrEqual(t, transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
}

func TestJustOneAPIAdapterUsesSecureTransportWhenDefaultIsWrapped(t *testing.T) {
	originalTransport := http.DefaultTransport
	var wrappedTransportCalled atomic.Bool
	http.DefaultTransport = justOneRoundTripFunc(func(*http.Request) (*http.Response, error) {
		wrappedTransportCalled.Store(true)
		return nil, http.ErrAbortHandler
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	adapter, err := NewJustOneAPIAdapter(AdapterConfig{
		Provider: ProviderJustOneAPI, BaseURL: DefaultJustOneAPIBaseURL, Secret: "private-test-secret",
	})
	require.NoError(t, err)
	transport, ok := adapter.(*JustOneAPIAdapter).client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.GreaterOrEqual(t, transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
	assert.False(t, wrappedTransportCalled.Load())
}

func TestJustOneAPIAdapterRejectsInjectedInsecureTransportForHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		writeJustOneJSON(t, response, http.StatusOK, map[string]any{"code": 0, "data": map[string]any{"ok": true}})
	}))
	t.Cleanup(server.Close)
	adapter := newJustOneTestAdapter(t, server.URL, func(config *AdapterConfig) {
		config.HTTPClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	})

	_, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
		OperationKey:  "social.content.search",
		Method:        http.MethodGet,
		Path:          "/api/search/v1",
		AuthPlacement: AuthPlacementQuery,
	}), CanonicalRequest{OperationKey: "social.content.search"})

	var connectorErr *ConnectorError
	require.ErrorAs(t, err, &connectorErr)
	assert.Equal(t, "UPSTREAM_UNAVAILABLE", connectorErr.Code)
}

func TestJustOneAPIAdapterInjectsQueryAuthenticationAndPreservesZeroValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "/api/search/v1", request.URL.Path)
		assert.Equal(t, "private-test-secret", request.URL.Query().Get("token"))
		assert.Equal(t, "agents", request.URL.Query().Get("keyword"))
		assert.Equal(t, "0", request.URL.Query().Get("page"))
		assert.Equal(t, "false", request.URL.Query().Get("include_ad"))
		assert.Equal(t, "zh-CN", request.URL.Query().Get("locale"))
		assert.Empty(t, request.URL.Query().Get("platform"))
		response.Header().Set("X-Request-ID", "joa-request-1")
		writeJustOneJSON(t, response, http.StatusOK, map[string]any{
			"code":    0,
			"message": "success",
			"data": map[string]any{
				"count":   0,
				"enabled": false,
			},
		})
	}))
	t.Cleanup(server.Close)
	adapter := newJustOneTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
		OperationKey:  "social.content.search",
		Method:        http.MethodGet,
		Path:          "/api/search/v1",
		AuthPlacement: AuthPlacementQuery,
		ParameterMap: map[string]string{
			"query":      "keyword",
			"page":       "page",
			"include_ad": "include_ad",
		},
		FixedParams:      map[string]any{"locale": "zh-CN"},
		CostAmountMicros: 200_000,
		CostCurrency:     "CNY",
	}), CanonicalRequest{
		OperationKey: "social.content.search",
		Platform:     "douyin",
		Params: map[string]any{
			"query":      "agents",
			"page":       0,
			"include_ad": false,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Dispatched)
	assert.Equal(t, http.StatusOK, result.HTTPStatus)
	assert.Equal(t, "0", result.BusinessCode)
	assert.Equal(t, "joa-request-1", result.ProviderRequestID)
	assert.Equal(t, int64(200_000), result.CostAmountMicros)
	assert.Equal(t, "CNY", result.CostCurrency)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), data["count"])
	assert.Equal(t, false, data["enabled"])
}

func TestJustOneAPIAdapterInjectsFormAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Empty(t, request.URL.Query().Get("token"))
		if assert.NoError(t, request.ParseForm()) {
			assert.Equal(t, "private-test-secret", request.PostForm.Get("token"))
			assert.Equal(t, "zero", request.PostForm.Get("keyword"))
			assert.Equal(t, "0", request.PostForm.Get("offset"))
			assert.Equal(t, "false", request.PostForm.Get("exact"))
		}
		writeJustOneJSON(t, response, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"items": []any{}}})
	}))
	t.Cleanup(server.Close)
	adapter := newJustOneTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
		OperationKey:  "social.account.search",
		Method:        http.MethodPost,
		Path:          "/api/accounts/search/v1",
		AuthPlacement: AuthPlacementForm,
		ParameterMap: map[string]string{
			"query":  "keyword",
			"offset": "offset",
			"exact":  "exact",
		},
	}), CanonicalRequest{
		OperationKey: "social.account.search",
		Params: map[string]any{
			"query":  "zero",
			"offset": 0,
			"exact":  false,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
}

func TestJustOneAPIAdapterKeepsPostParamsInFormWhenTokenUsesQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "private-test-secret", request.URL.Query().Get("token"))
		assert.Empty(t, request.URL.Query().Get("keyword"))
		if assert.NoError(t, request.ParseForm()) {
			assert.Empty(t, request.PostForm.Get("token"))
			assert.Equal(t, "agents", request.PostForm.Get("keyword"))
		}
		writeJustOneJSON(t, response, http.StatusOK, map[string]any{"code": 0, "data": map[string]any{}})
	}))
	t.Cleanup(server.Close)
	adapter := newJustOneTestAdapter(t, server.URL, nil)

	_, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
		OperationKey:  "social.content.search",
		Method:        http.MethodPost,
		Path:          "/api/search/v1",
		AuthPlacement: AuthPlacementQuery,
		ParameterMap:  map[string]string{"query": "keyword"},
	}), CanonicalRequest{OperationKey: "social.content.search", Params: map[string]any{"query": "agents"}})
	require.NoError(t, err)
}

func TestJustOneAPIAdapterMapsExplicitNonBillableBusinessErrors(t *testing.T) {
	tests := []struct {
		code      int
		httpCode  int
		errorCode string
	}{
		{code: 301, httpCode: http.StatusOK, errorCode: "UPSTREAM_COLLECTION_FAILED"},
		{code: 302, httpCode: http.StatusTooManyRequests, errorCode: "UPSTREAM_RATE_LIMITED"},
		{code: 303, httpCode: http.StatusTooManyRequests, errorCode: "UPSTREAM_DAILY_QUOTA_EXHAUSTED"},
		{code: 400, httpCode: http.StatusOK, errorCode: "UPSTREAM_REQUEST_FAILED"},
		{code: 500, httpCode: http.StatusOK, errorCode: "UPSTREAM_UNAVAILABLE"},
		{code: 600, httpCode: http.StatusOK, errorCode: "UPSTREAM_FORBIDDEN"},
		{code: 601, httpCode: http.StatusOK, errorCode: "UPSTREAM_CREDITS_UNAVAILABLE"},
		{code: 602, httpCode: http.StatusOK, errorCode: "UPSTREAM_BUDGET_EXHAUSTED"},
	}
	for _, test := range tests {
		t.Run(test.errorCode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Retry-After", "3")
				writeJustOneJSON(t, response, test.httpCode, map[string]any{
					"code":    test.code,
					"message": "raw provider detail private-test-secret",
				})
			}))
			t.Cleanup(server.Close)
			adapter := newJustOneTestAdapter(t, server.URL, nil)

			result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
				OperationKey:  "resource.resolve",
				Method:        http.MethodGet,
				Path:          "/api/resolve/v1",
				AuthPlacement: AuthPlacementQuery,
				ParameterMap:  map[string]string{"url": "url"},
			}), CanonicalRequest{OperationKey: "resource.resolve", Params: map[string]any{"url": "https://example.com"}})
			var connectorErr *ConnectorError
			require.ErrorAs(t, err, &connectorErr)
			assert.Equal(t, test.errorCode, connectorErr.Code)
			assert.NotContains(t, err.Error(), "private-test-secret")
			assert.True(t, result.Dispatched)
			assert.Equal(t, strconv.Itoa(test.code), result.BusinessCode)
			require.NotNil(t, result.Billable)
			assert.False(t, *result.Billable)
			assert.Equal(t, 3*time.Second, result.RetryAfter)
		})
	}
}

func TestJustOneAPIAdapterDoesNotLeakTokenInTransportOrResponseErrors(t *testing.T) {
	t.Run("rate limited non json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "text/plain")
			response.Header().Set("Retry-After", "4")
			response.WriteHeader(http.StatusTooManyRequests)
			_, _ = response.Write([]byte("private-test-secret"))
		}))
		t.Cleanup(server.Close)
		adapter := newJustOneTestAdapter(t, server.URL, nil)

		result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
			OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
		}), CanonicalRequest{OperationKey: "content.get"})
		var connectorErr *ConnectorError
		require.ErrorAs(t, err, &connectorErr)
		assert.Equal(t, "UPSTREAM_RATE_LIMITED", connectorErr.Code)
		assert.NotContains(t, err.Error(), "private-test-secret")
		assert.Equal(t, "429", result.BusinessCode)
		assert.Equal(t, 4*time.Second, result.RetryAfter)
		require.NotNil(t, result.Billable)
		assert.False(t, *result.Billable)
	})

	t.Run("raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "text/plain")
			response.WriteHeader(http.StatusInternalServerError)
			_, _ = response.Write([]byte("private-test-secret"))
		}))
		t.Cleanup(server.Close)
		adapter := newJustOneTestAdapter(t, server.URL, nil)

		_, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
			OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
		}), CanonicalRequest{OperationKey: "content.get"})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "private-test-secret")
	})

	t.Run("post dispatch transport error", func(t *testing.T) {
		adapter := newJustOneTestAdapter(t, "http://127.0.0.1:8080", func(config *AdapterConfig) {
			config.HTTPClient = &http.Client{Transport: justOneRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				trace := httptrace.ContextClientTrace(request.Context())
				require.NotNil(t, trace)
				trace.WroteRequest(httptrace.WroteRequestInfo{})
				return nil, errors.New("transport rejected private-test-secret")
			})}
		})

		result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
			OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
		}), CanonicalRequest{OperationKey: "content.get"})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "private-test-secret")
		assert.True(t, result.Dispatched)
		assert.Nil(t, result.Billable)
	})
}

func TestJustOneAPIAdapterRejectsUnmappedInputsAndUnsafeBindingsBeforeDispatch(t *testing.T) {
	var dispatched atomic.Bool
	adapter := newJustOneTestAdapter(t, "http://127.0.0.1:8080", func(config *AdapterConfig) {
		config.HTTPClient = &http.Client{Transport: justOneRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			dispatched.Store(true)
			return nil, http.ErrAbortHandler
		})}
	})

	tests := []struct {
		name      string
		operation ProviderOperation
		request   CanonicalRequest
	}{
		{
			name: "unmapped input",
			operation: justOneTestOperation(ProviderOperation{
				OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
			}),
			request: CanonicalRequest{OperationKey: "content.get", Params: map[string]any{"url": "https://example.com"}},
		},
		{
			name: "page token",
			operation: justOneTestOperation(ProviderOperation{
				OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
			}),
			request: CanonicalRequest{OperationKey: "content.get", PageToken: "raw-upstream-cursor"},
		},
		{
			name: "absolute path",
			operation: justOneTestOperation(ProviderOperation{
				OperationKey: "content.get", Method: http.MethodGet, Path: "https://attacker.example/api", AuthPlacement: AuthPlacementQuery,
			}),
			request: CanonicalRequest{OperationKey: "content.get"},
		},
		{
			name: "unsupported method",
			operation: justOneTestOperation(ProviderOperation{
				OperationKey: "content.get", Method: http.MethodDelete, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
			}),
			request: CanonicalRequest{OperationKey: "content.get"},
		},
		{
			name: "query token override",
			operation: justOneTestOperation(ProviderOperation{
				OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
				ParameterMap: map[string]string{"credential": "token"},
			}),
			request: CanonicalRequest{OperationKey: "content.get", Params: map[string]any{"credential": "attacker-token"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := adapter.Execute(context.Background(), test.operation, test.request)
			require.Error(t, err)
			assert.False(t, result.Dispatched)
			assert.Nil(t, result.Billable)
			assert.False(t, dispatched.Load())
		})
	}
}

func TestJustOneAPIAdapterRejectsRedirectsWithoutFollowingThem(t *testing.T) {
	var redirected atomic.Bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirected" {
			redirected.Store(true)
			writeJustOneJSON(t, response, http.StatusOK, map[string]any{"code": 0})
			return
		}
		http.Redirect(response, request, server.URL+"/redirected?token=private-test-secret", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	adapter := newJustOneTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), justOneTestOperation(ProviderOperation{
		OperationKey: "content.get", Method: http.MethodGet, Path: "/api/content/v1", AuthPlacement: AuthPlacementQuery,
	}), CanonicalRequest{OperationKey: "content.get"})
	var connectorErr *ConnectorError
	require.ErrorAs(t, err, &connectorErr)
	assert.Equal(t, "UPSTREAM_REDIRECT_REJECTED", connectorErr.Code)
	assert.False(t, redirected.Load())
	assert.True(t, result.Dispatched)
	assert.Nil(t, result.Billable)
	assert.NotContains(t, err.Error(), "private-test-secret")
}

func TestJustOneAPIAdapterProbeIsExplicitlyUnsupported(t *testing.T) {
	adapter := newJustOneTestAdapter(t, "http://127.0.0.1:8080", nil)
	_, err := adapter.Probe(context.Background())
	var connectorErr *ConnectorError
	require.ErrorAs(t, err, &connectorErr)
	assert.Equal(t, "UPSTREAM_PROBE_UNSUPPORTED", connectorErr.Code)
}

func TestJustOneAPIAdapterUsesReviewedCatalogSnapshot(t *testing.T) {
	adapter := newJustOneTestAdapter(t, "http://127.0.0.1:8080", nil)
	snapshot, err := adapter.SnapshotCatalog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ProviderJustOneAPI, snapshot.Provider)
	assert.NotEmpty(t, snapshot.Version)
	assert.NotEmpty(t, snapshot.SchemaHash)
	require.NotEmpty(t, snapshot.Operations)
	for _, operation := range snapshot.Operations {
		assert.Equal(t, justOneAPIDirectMappingKey, operation.MappingKey)
		assert.Empty(t, operation.CostCurrency)
		assert.False(t, operation.ContractEquivalent)
		assert.False(t, operation.BillingReady)
		assert.NotEmpty(t, operation.Method)
		assert.NotEmpty(t, operation.Path)
	}
}

func TestNewJustOneAPIAdapterValidatesURLAndSecret(t *testing.T) {
	tests := []struct {
		name   string
		config AdapterConfig
		code   string
	}{
		{name: "non tls remote", config: AdapterConfig{BaseURL: "http://example.com", Secret: "key"}, code: "UPSTREAM_URL_HTTPS_REQUIRED"},
		{name: "loopback requires opt in", config: AdapterConfig{BaseURL: "http://127.0.0.1:8080", Secret: "key"}, code: "UPSTREAM_URL_HTTPS_REQUIRED"},
		{name: "userinfo rejected", config: AdapterConfig{BaseURL: "https://user@example.com", Secret: "key"}, code: "UPSTREAM_URL_INVALID"},
		{name: "untrusted https host", config: AdapterConfig{BaseURL: "https://example.com", Secret: "key"}, code: "UPSTREAM_URL_INVALID"},
		{name: "missing secret", config: AdapterConfig{BaseURL: DefaultJustOneAPIBaseURL}, code: "UPSTREAM_SECRET_REQUIRED"},
		{name: "provider mismatch", config: AdapterConfig{Provider: ProviderTikHub, BaseURL: "https://example.com", Secret: "key"}, code: "UPSTREAM_PROVIDER_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewJustOneAPIAdapter(test.config)
			var connectorErr *ConnectorError
			require.ErrorAs(t, err, &connectorErr)
			assert.Equal(t, test.code, connectorErr.Code)
		})
	}

	adapter, err := NewJustOneAPIAdapter(AdapterConfig{Secret: "key"})
	require.NoError(t, err)
	concrete, ok := adapter.(*JustOneAPIAdapter)
	require.True(t, ok)
	assert.Equal(t, DefaultJustOneAPIBaseURL, concrete.endpoint.String())
}

func TestDecodeJustOneAPIEnvelopeRejectsMissingCode(t *testing.T) {
	payload, err := common.Marshal(map[string]any{"data": map[string]any{"ok": true}})
	require.NoError(t, err)
	_, err = decodeJustOneAPIEnvelope("application/json", payload)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "data")
}

func TestParseJustOneAPIRetryAfterRejectsInvalidValues(t *testing.T) {
	now := time.Now()
	assert.Zero(t, parseJustOneAPIRetryAfter("", now))
	assert.Zero(t, parseJustOneAPIRetryAfter("-1", now))
	assert.Zero(t, parseJustOneAPIRetryAfter("not-a-date", now))
	assert.Equal(t, 2*time.Second, parseJustOneAPIRetryAfter("2", now))
	duration := parseJustOneAPIRetryAfter(now.Add(time.Minute).UTC().Format(http.TimeFormat), now)
	assert.GreaterOrEqual(t, duration, 59*time.Second)
	assert.LessOrEqual(t, duration, time.Minute)
}
