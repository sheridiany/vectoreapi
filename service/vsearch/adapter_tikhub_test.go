package vsearch

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTikHubTestAdapter(t *testing.T, serverURL string, client *http.Client) ProviderAdapter {
	t.Helper()
	adapter, err := NewTikHubAdapter(AdapterConfig{
		Provider:          ProviderTikHub,
		BaseURL:           serverURL,
		Secret:            "private-tikhub-token",
		Timeout:           time.Second,
		MaxResponseBytes:  64 << 10,
		HTTPClient:        client,
		AllowLoopbackHTTP: true,
	})
	require.NoError(t, err)
	return adapter
}

func tikHubTestOperation(method string) ProviderOperation {
	return ProviderOperation{
		OperationKey:    "social.content.search",
		ContractVersion: "v1",
		Platform:        "tiktok",
		OperationID:     "fetch_search_video",
		Method:          method,
		Path:            "/api/v1/tiktok/web/fetch_search_video",
		AuthPlacement:   AuthPlacementBearer,
		MappingKey:      tikHubDirectMappingKey,
		MappingVersion:  "v1",
		ParameterMap: map[string]string{
			"query":       "keyword",
			"limit":       "count",
			"include_ads": "include_ads",
		},
		CostAmountMicros: 2_000,
		CostCurrency:     "USD",
	}
}

func TestTikHubAdapterDoesNotInheritInsecureDefaultTLS(t *testing.T) {
	originalTransport := http.DefaultTransport
	unsafeTransport := originalTransport.(*http.Transport).Clone()
	unsafeTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS10, InsecureSkipVerify: true}
	http.DefaultTransport = unsafeTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	adapter, err := NewTikHubAdapter(AdapterConfig{
		Provider: ProviderTikHub, BaseURL: DefaultTikHubBaseURL, Secret: "private-tikhub-token",
	})
	require.NoError(t, err)
	transport := adapter.(*TikHubAdapter).client.Transport.(*http.Transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.GreaterOrEqual(t, transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
}

func TestTikHubAdapterUsesSecureTransportWhenDefaultIsWrapped(t *testing.T) {
	originalTransport := http.DefaultTransport
	var wrappedTransportCalled atomic.Bool
	http.DefaultTransport = justOneRoundTripFunc(func(*http.Request) (*http.Response, error) {
		wrappedTransportCalled.Store(true)
		return nil, http.ErrAbortHandler
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	adapter, err := NewTikHubAdapter(AdapterConfig{
		Provider: ProviderTikHub, BaseURL: DefaultTikHubBaseURL, Secret: "private-tikhub-token",
	})
	require.NoError(t, err)
	transport, ok := adapter.(*TikHubAdapter).client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.GreaterOrEqual(t, transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
	assert.False(t, wrappedTransportCalled.Load())
}

func TestTikHubAdapterRejectsInjectedWrappedInsecureTransportForHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = response.Write([]byte(`{"code":200,"request_id":"provider-request-1","data":{"items":[]}}`))
	}))
	t.Cleanup(server.Close)
	var roundTrips atomic.Int32
	insecureTransport := server.Client().Transport
	adapter := newTikHubTestAdapter(t, server.URL, &http.Client{Transport: justOneRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		roundTrips.Add(1)
		return insecureTransport.RoundTrip(request)
	})})

	_, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodGet), CanonicalRequest{
		OperationKey: "social.content.search",
		Platform:     "tiktok",
	})

	var connectorErr *ConnectorError
	require.ErrorAs(t, err, &connectorErr)
	assert.Equal(t, "UPSTREAM_UNAVAILABLE", connectorErr.Code)
	assert.Zero(t, roundTrips.Load())
}

func TestTikHubAdapterUsesBearerAndReturnsOnlyNormalizedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/v1/tiktok/web/fetch_search_video", request.URL.Path)
		assert.Equal(t, "Bearer private-tikhub-token", request.Header.Get("Authorization"))
		assert.Equal(t, "cats", request.URL.Query().Get("keyword"))
		assert.Equal(t, "0", request.URL.Query().Get("count"))
		assert.Equal(t, "false", request.URL.Query().Get("include_ads"))
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = response.Write([]byte(`{
			"code":200,
			"request_id":"provider-request-1",
			"message":"Request successful",
			"support":"private support detail",
			"docs":"https://private.example/docs",
			"router":"/internal/provider/route",
			"params":{"token":"private-tikhub-token"},
			"data":{"items":[{"id":"video-1"}]}
		}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodGet), CanonicalRequest{
		OperationKey: "social.content.search",
		Platform:     "tiktok",
		Params: map[string]any{
			"query": "cats", "limit": 0, "include_ads": false,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Dispatched)
	assert.Equal(t, http.StatusOK, result.HTTPStatus)
	assert.Equal(t, "200", result.BusinessCode)
	assert.Equal(t, "provider-request-1", result.ProviderRequestID)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
	assert.Equal(t, map[string]any{"items": []any{map[string]any{"id": "video-1"}}}, result.Data)

	serialized, err := common.Marshal(result.Data)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), "private support detail")
	assert.NotContains(t, string(serialized), "private.example")
	assert.NotContains(t, string(serialized), "internal/provider")
	assert.NotContains(t, string(serialized), "private-tikhub-token")
}

func TestTikHubAdapterNormalizesDouyinTrendList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/v1/douyin/web/fetch_hot_search_result", request.URL.Path)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"code":200,
			"request_id":"trend-request-1",
			"data":{
				"status_code":0,
				"data":{
					"trending_list":[{"sentence_id":"ignored","word":"not the ranked list"}],
					"word_list":[
						{"sentence_id":7300000000000000001,"word":"AI Agent","position":3,"hot_value":987654},
						{"sentence_id":"7300000000000000002","word":"具身智能","position":8,"hot_value":876543}
					]
				}
			}
		}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)
	operation := ProviderOperation{
		OperationKey: "social.trend.list", ContractVersion: "v1", Platform: "douyin",
		OperationID: "fetch_hot_search_result", Method: http.MethodGet,
		Path: "/api/v1/douyin/web/fetch_hot_search_result", AuthPlacement: AuthPlacementBearer,
		MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
	}

	result, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: "social.trend.list", Platform: "douyin", Params: map[string]any{"platform": "douyin"},
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"items": []any{
			map[string]any{"id": "7300000000000000001", "type": "trend", "platform": "douyin", "title": "AI Agent", "rank": 3, "score": float64(987654)},
			map[string]any{"id": "7300000000000000002", "type": "trend", "platform": "douyin", "title": "具身智能", "rank": 8, "score": float64(876543)},
		},
		"page": map[string]any{"cursor": nil, "has_more": false},
	}, result.Data)
}

func TestTikHubAdapterRejectsMalformedDouyinTrendListAfterDispatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"request_id":"trend-request-2","data":{"trending_list":[]}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)
	operation := ProviderOperation{
		OperationKey: "social.trend.list", ContractVersion: "v1", Platform: "douyin",
		OperationID: "fetch_hot_search_result", Method: http.MethodGet,
		Path: "/api/v1/douyin/web/fetch_hot_search_result", AuthPlacement: AuthPlacementBearer,
		MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
	}

	result, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: "social.trend.list", Platform: "douyin", Params: map[string]any{"platform": "douyin"},
	})

	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_CONTRACT_MISMATCH", connectorError.Code)
	assert.NotContains(t, err.Error(), "trending_list")
	assert.True(t, result.Dispatched)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
	assert.Nil(t, result.Data)
}

func TestTikHubAdapterRejectsInvalidDouyinTrendID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"data":{"word_list":[{"sentence_id":{},"word":"invalid"}]}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)
	operation := ProviderOperation{
		OperationKey: "social.trend.list", ContractVersion: "v1", Platform: "douyin",
		OperationID: "fetch_hot_search_result", Method: http.MethodGet,
		Path: "/api/v1/douyin/web/fetch_hot_search_result", AuthPlacement: AuthPlacementBearer,
		MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
	}

	result, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: "social.trend.list", Platform: "douyin", Params: map[string]any{"platform": "douyin"},
	})

	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_CONTRACT_MISMATCH", connectorError.Code)
	assert.True(t, result.Dispatched)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
}

func TestTikHubAdapterRejectsNullDouyinWordList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"data":{"word_list":null}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)
	operation := ProviderOperation{
		OperationKey: "social.trend.list", ContractVersion: "v1", Platform: "douyin",
		OperationID: "fetch_hot_search_result", Method: http.MethodGet,
		Path: "/api/v1/douyin/web/fetch_hot_search_result", AuthPlacement: AuthPlacementBearer,
		MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
	}

	result, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: "social.trend.list", Platform: "douyin", Params: map[string]any{"platform": "douyin"},
	})

	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_CONTRACT_MISMATCH", connectorError.Code)
	assert.True(t, result.Dispatched)
	require.NotNil(t, result.Billable)
	assert.True(t, *result.Billable)
}

func TestTikHubAdapterPreservesExplicitZeroAndFalseInPostBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
		var body map[string]any
		if assert.NoError(t, common.DecodeJson(request.Body, &body)) {
			assert.Contains(t, body, "count")
			assert.Equal(t, float64(0), body["count"])
			assert.Contains(t, body, "include_ads")
			assert.Equal(t, false, body["include_ads"])
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"request_id":"post-1","data":{"ok":true}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodPost), CanonicalRequest{
		OperationKey: "social.content.search",
		Platform:     "tiktok",
		Params:       map[string]any{"query": "cats", "limit": 0, "include_ads": false},
	})
	require.NoError(t, err)
	assert.Equal(t, "post-1", result.ProviderRequestID)
}

func TestTikHubAdapterMapsRateLimitAndDoesNotLeakProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Retry-After", "7")
		response.WriteHeader(http.StatusTooManyRequests)
		_, _ = response.Write([]byte("raw provider failure private-tikhub-token"))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	result, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodGet), CanonicalRequest{
		OperationKey: "social.content.search", Platform: "tiktok", Params: map[string]any{"query": "cats"},
	})
	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_RATE_LIMITED", connectorError.Code)
	assert.NotContains(t, err.Error(), "private-tikhub-token")
	assert.NotContains(t, err.Error(), "raw provider failure")
	assert.True(t, result.Dispatched)
	assert.Equal(t, http.StatusTooManyRequests, result.HTTPStatus)
	assert.Equal(t, "429", result.BusinessCode)
	assert.Equal(t, 7*time.Second, result.RetryAfter)
	require.NotNil(t, result.Billable)
	assert.False(t, *result.Billable)
}

func TestTikHubAdapterRejectsUnmappedAndProviderControlParamsBeforeDispatch(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"data":{}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)
	operation := tikHubTestOperation(http.MethodGet)

	tests := []struct {
		name    string
		request CanonicalRequest
		code    string
	}{
		{
			name: "provider token", code: "UPSTREAM_PARAMETER_FORBIDDEN",
			request: CanonicalRequest{OperationKey: operation.OperationKey, Platform: operation.Platform, Params: map[string]any{"token": "do-not-forward"}},
		},
		{
			name: "raw response toggle", code: "UPSTREAM_PARAMETER_FORBIDDEN",
			request: CanonicalRequest{OperationKey: operation.OperationKey, Platform: operation.Platform, Params: map[string]any{"raw": true}},
		},
		{
			name: "unmapped provider field", code: "UPSTREAM_PARAMETER_UNSUPPORTED",
			request: CanonicalRequest{OperationKey: operation.OperationKey, Platform: operation.Platform, Params: map[string]any{"device_id": "provider-only"}},
		},
		{
			name: "opaque page token not implemented", code: "UPSTREAM_PAGE_TOKEN_UNSUPPORTED",
			request: CanonicalRequest{OperationKey: operation.OperationKey, Platform: operation.Platform, PageToken: "raw-provider-cursor"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := adapter.Execute(context.Background(), operation, test.request)
			var connectorError *ConnectorError
			require.ErrorAs(t, err, &connectorError)
			assert.Equal(t, test.code, connectorError.Code)
		})
	}
	assert.Zero(t, requests.Load())
}

func TestTikHubAdapterProbesUsableBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, tikHubUserInfoPath, request.URL.Path)
		assert.Equal(t, "Bearer private-tikhub-token", request.Header.Get("Authorization"))
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"code":200,
			"api_key_data":{"api_key_name":"production"},
			"user_data":{"balance":1.25,"free_credit":0.05,"account_disabled":false,"is_active":true}
		}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	state, err := adapter.Probe(context.Background())
	require.NoError(t, err)
	assert.Empty(t, state.Plan)
	assert.Equal(t, int64(1_300_000), state.BalanceAmountMicros)
	assert.Equal(t, "USD", state.BalanceCurrency)
}

func TestTikHubAdapterReturnsOnlyCuratedCatalogBindings(t *testing.T) {
	adapter := newTikHubTestAdapter(t, "http://127.0.0.1:8080", nil)

	snapshot, err := adapter.SnapshotCatalog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ProviderTikHub, snapshot.Provider)
	assert.NotEmpty(t, snapshot.Version)
	assert.NotEmpty(t, snapshot.SchemaHash)
	require.NotEmpty(t, snapshot.Operations)
	verified := 0
	for _, operation := range snapshot.Operations {
		assert.Equal(t, tikHubDirectMappingKey, operation.MappingKey)
		assert.Equal(t, "USD", operation.CostCurrency)
		if operation.OperationKey == "social.trend.list" && operation.Platform == "douyin" {
			assert.True(t, operation.ContractEquivalent)
			assert.True(t, operation.BillingReady)
			assert.Equal(t, int64(2_000), operation.CostAmountMicros)
			verified++
		} else {
			assert.False(t, operation.ContractEquivalent)
			assert.False(t, operation.BillingReady)
			assert.Zero(t, operation.CostAmountMicros)
		}
		assert.Equal(t, AuthPlacementBearer, operation.AuthPlacement)
		assert.Contains(t, []string{http.MethodGet, http.MethodPost}, operation.Method)
		assert.NotEmpty(t, operation.OperationID)
		assert.True(t, strings.HasPrefix(operation.Path, "/api/"), operation.OperationID)
	}
	assert.Equal(t, 1, verified)
}

func TestTikHubAdapterDisablesRedirectsEvenWithCustomClient(t *testing.T) {
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"data":{}}`))
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Location", target.URL)
		response.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, &http.Client{})

	_, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodGet), CanonicalRequest{
		OperationKey: "social.content.search", Platform: "tiktok", Params: map[string]any{"query": "cats"},
	})
	require.Error(t, err)
	assert.False(t, followed.Load())
	assert.NotContains(t, err.Error(), target.URL)
}

func TestNewTikHubAdapterValidatesProviderURLAndSecret(t *testing.T) {
	tests := []struct {
		name   string
		config AdapterConfig
		code   string
	}{
		{name: "wrong provider", config: AdapterConfig{Provider: ProviderJustOneAPI, BaseURL: "https://api.tikhub.io", Secret: "key"}, code: "UPSTREAM_PROVIDER_INVALID"},
		{name: "non tls remote", config: AdapterConfig{BaseURL: "http://example.com", Secret: "key"}, code: "UPSTREAM_URL_HTTPS_REQUIRED"},
		{name: "untrusted https host", config: AdapterConfig{BaseURL: "https://example.com", Secret: "key"}, code: "UPSTREAM_URL_INVALID"},
		{name: "missing secret", config: AdapterConfig{BaseURL: "https://api.tikhub.io"}, code: "UPSTREAM_SECRET_REQUIRED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewTikHubAdapter(test.config)
			var connectorError *ConnectorError
			require.ErrorAs(t, err, &connectorError)
			assert.Equal(t, test.code, connectorError.Code)
		})
	}
}

func TestTikHubAdapterErrorsDoNotContainCredentialsOrRawResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"detail":"private-tikhub-token has expired"}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	_, err := adapter.Execute(context.Background(), tikHubTestOperation(http.MethodGet), CanonicalRequest{
		OperationKey: "social.content.search", Platform: "tiktok", Params: map[string]any{"query": "cats"},
	})
	require.Error(t, err)
	assert.NotContains(t, strings.ToLower(err.Error()), "token")
	assert.NotContains(t, err.Error(), "expired")
}

func TestTikHubRetryAfterClampsUntrustedProviderValues(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	resetHeaders := make(http.Header)
	resetHeaders.Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(7*24*time.Hour).Unix(), 10))
	tests := []struct {
		name    string
		headers http.Header
	}{
		{name: "large delta seconds", headers: http.Header{"Retry-After": []string{"9223372036854775807"}}},
		{name: "distant HTTP date", headers: http.Header{"Retry-After": []string{now.Add(7 * 24 * time.Hour).Format(http.TimeFormat)}}},
		{name: "distant reset timestamp", headers: resetHeaders},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, 24*time.Hour, tikHubRetryAfter(test.headers, now))
		})
	}
}
