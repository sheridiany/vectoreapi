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

func TestTikHubAdapterNormalizesVerifiedYouTubeContracts(t *testing.T) {
	tests := []struct {
		name      string
		operation ProviderOperation
		request   CanonicalRequest
		response  string
		assertion func(*testing.T, map[string]any)
	}{
		{
			name: "account",
			operation: ProviderOperation{OperationKey: "social.account.get", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/account", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"account_ref": "channel_id"}, OutputSchema: entityOutputSchema("account")},
			request: CanonicalRequest{OperationKey: "social.account.get", Platform: "youtube", Params: map[string]any{"platform": "youtube", "account_ref": "UC1"}},
			response: `{"code":200,"data":{"channel_id":"UC1","title":"Channel One","description":"Public channel","verified":true,
				"subscriber_count":"1.2M","video_count":"345","avatar":[{"url":"https://img.example/avatar.jpg"}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "UC1", data["id"])
				assert.Equal(t, "Channel One", data["display_name"])
				assert.Equal(t, int64(1_200_000), data["follower_count"])
			},
		},
		{
			name: "account contents",
			operation: ProviderOperation{OperationKey: "social.account.contents.list", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/account-contents", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"account_ref": "channel_id"}, FixedParams: map[string]any{"need_format": true}, OutputSchema: listOutputSchema("content")},
			request: CanonicalRequest{OperationKey: "social.account.contents.list", Platform: "youtube", Params: map[string]any{"platform": "youtube", "account_ref": "UC1"}},
			response: `{"code":200,"data":{"channel":{"id":"UC1","name":"Channel One","url":"https://youtube.example/c/one"},
				"continuation_token":"next-videos","videos":[{"video_id":"video-1","title":"First video","description":"Summary",
				"url":"https://youtube.example/watch?v=video-1","published_time":"1 day ago","view_count":"12,345","thumbnail":"https://img.example/video.jpg"}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				items := data["items"].([]any)
				assert.Equal(t, "video-1", items[0].(map[string]any)["id"])
				assert.Equal(t, map[string]any{"cursor": "next-videos", "has_more": true}, data["page"])
			},
		},
		{
			name: "content",
			operation: ProviderOperation{OperationKey: "social.content.get", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/content", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"content_ref": "video_id"}, FixedParams: map[string]any{"need_format": true}, OutputSchema: entityOutputSchema("content")},
			request: CanonicalRequest{OperationKey: "social.content.get", Platform: "youtube", Params: map[string]any{"platform": "youtube", "content_ref": "video-1"}},
			response: `{"code":200,"data":{"video_id":"video-1","video_url":"https://youtube.example/watch?v=video-1","title":"First video",
				"description":"Summary","channel_id":"UC1","author":"Channel One","upload_date":"2026-08-31","view_count":"12345","like_count":"678",
				"comment_count":"90","thumbnails":[{"url":"https://img.example/video.jpg"}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "video-1", data["id"])
				assert.Equal(t, map[string]any{"view_count": int64(12345), "like_count": int64(678), "comment_count": int64(90)}, data["metrics"])
			},
		},
		{
			name: "search",
			operation: ProviderOperation{OperationKey: "social.content.search", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/search", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"query": "keyword"}, FixedParams: map[string]any{"type": "video"}, OutputSchema: listOutputSchema("content")},
			request: CanonicalRequest{OperationKey: "social.content.search", Platform: "youtube", Params: map[string]any{"platform": "youtube", "query": "OpenAI"}},
			response: `{"code":200,"data":{"continuation_token":"next-search","videos":[{"video_id":"video-2","title":"Search result",
				"author":"Channel Two","channel_id":"UC2","url":"https://youtube.example/watch?v=video-2","view_count":"999"}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				items := data["items"].([]any)
				assert.Equal(t, "video-2", items[0].(map[string]any)["id"])
			},
		},
		{
			name: "comments",
			operation: ProviderOperation{OperationKey: "social.comment.list", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/comments", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"content_ref": "video_id"}, FixedParams: map[string]any{"need_format": true}, OutputSchema: listOutputSchema("comment")},
			request: CanonicalRequest{OperationKey: "social.comment.list", Platform: "youtube", Params: map[string]any{"platform": "youtube", "content_ref": "video-1"}},
			response: `{"code":200,"data":{"continuation_token":"next-comments","comments":[{"comment_id":"comment-1","content":"Useful",
				"published_time":"2 hours ago","like_count":"42","reply_count":"3","reply_continuation_token":"reply-cursor-1",
				"author":{"channel_id":"UC3","display_name":"Viewer","channel_url":"https://youtube.example/c/viewer","avatar_url":"https://img.example/viewer.jpg"}}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				items := data["items"].([]any)
				comment := items[0].(map[string]any)
				assert.Equal(t, "comment-1", comment["id"])
				assert.Equal(t, "reply-cursor-1", comment["reply_cursor"])
			},
		},
		{
			name: "replies ignores content context",
			operation: ProviderOperation{OperationKey: "social.comment.replies.list", ContractVersion: "v1", Platform: "youtube", Method: http.MethodGet,
				Path: "/replies", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1",
				ParameterMap: map[string]string{"comment_ref": "continuation_token"}, FixedParams: map[string]any{"need_format": true}, OutputSchema: listOutputSchema("comment")},
			request: CanonicalRequest{OperationKey: "social.comment.replies.list", Platform: "youtube", Params: map[string]any{
				"platform": "youtube", "content_ref": "video-1", "comment_ref": "reply-cursor-1"}},
			response: `{"code":200,"data":{"continuation_token":null,"comments":[{"comment_id":"reply-1","content":"Thanks",
				"published_time":"1 hour ago","like_count":"2","reply_count":"0","reply_level":1,
				"author":{"channel_id":"UC4","display_name":"Author"}}]}}`,
			assertion: func(t *testing.T, data map[string]any) {
				items := data["items"].([]any)
				assert.Equal(t, "reply-1", items[0].(map[string]any)["id"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if test.name == "replies ignores content context" {
					assert.Empty(t, request.URL.Query().Get("content_ref"))
					assert.Equal(t, "reply-cursor-1", request.URL.Query().Get("continuation_token"))
				}
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(test.response))
			}))
			t.Cleanup(server.Close)
			adapter := newTikHubTestAdapter(t, server.URL, nil)

			result, err := adapter.Execute(context.Background(), test.operation, test.request)

			require.NoError(t, err)
			require.NoError(t, validateSchemaValue(result.Data, test.operation.OutputSchema, "$"))
			data, ok := result.Data.(map[string]any)
			require.True(t, ok)
			test.assertion(t, data)
		})
	}
}

func TestTikHubAdapterNormalizesNestedTikTokShopContracts(t *testing.T) {
	tests := []struct {
		name      string
		operation ProviderOperation
		response  string
		expected  string
	}{
		{
			name: "search",
			operation: ProviderOperation{OperationKey: "commerce.product.search", ContractVersion: "v1", Platform: "tiktok_shop", Method: http.MethodGet,
				Path: "/shop-search", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1", OutputSchema: listOutputSchema("product")},
			response: `{"code":200,"data":{"code":0,"message":"success","data":{"has_more":true,"load_more_params":{"page_token":"next-products"},
				"products":[{"product_id":"product-1","title":"Product One","seo_url":"https://shop.example/product-1",
				"image":{"url":"https://img.example/product.jpg"},"seller_info":{"seller_id":"seller-1","shop_name":"Shop One"},
				"product_price_info":{"sale_price":"12.34"},"rate_info":{"rating":"4.8"},"sold_info":{"sold_count":"123"}}]}}}`,
			expected: "product-1",
		},
		{
			name: "detail",
			operation: ProviderOperation{OperationKey: "commerce.product.get", ContractVersion: "v1", Platform: "tiktok_shop", Method: http.MethodGet,
				Path: "/shop-detail", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1", OutputSchema: entityOutputSchema("product")},
			response: `{"code":200,"data":{"code":0,"message":"success","data":{"global_data":{"product_info":{"product_info":{
				"product_id":"product-1","title":"Product One","seo_url":"https://shop.example/product-1","description":"Public description",
				"images":[{"url":"https://img.example/product.jpg"}],"seller_info":{"seller_id":"seller-1","shop_name":"Shop One"}}}}}}}`,
			expected: "product-1",
		},
		{
			name: "detail v3",
			operation: ProviderOperation{OperationKey: "commerce.product.get", ContractVersion: "v1", Platform: "tiktok_shop", Method: http.MethodGet,
				Path: "/shop-detail-v3", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1", OutputSchema: entityOutputSchema("product")},
			response: `{"code":200,"data":{"product_data":{"page_config":{"components_map":[{"component_name":"product_info","component_data":{"product_info":{
				"product_model":{"product_id":"product-2","name":"Product Two","description":"[{\"type\":\"text\",\"text\":\"Public description\"}]",
				"sold_count":123,"images":[{"uri":"private-uri","url_list":["https://img.example/product-2.jpg"]}]},
				"seller_model":{"shop_name":"Shop Two"},"review_model":{"product_overall_score":4.8,"product_review_count":"42"},
				"promotion_model":{"promotion_product_price":{"min_price":{"currency_name":"USD","sale_price_decimal":"12.34"}}}}}}]}}}}`,
			expected: "product-2",
		},
		{
			name: "reviews",
			operation: ProviderOperation{OperationKey: "commerce.product.reviews.list", ContractVersion: "v1", Platform: "tiktok_shop", Method: http.MethodGet,
				Path: "/shop-reviews", AuthPlacement: AuthPlacementBearer, MappingKey: tikHubDirectMappingKey, MappingVersion: "v1", OutputSchema: listOutputSchema("review")},
			response: `{"code":200,"data":{"code":0,"message":"success","data":{"has_more":false,"product_reviews":[{
				"review_id":"review-1","review_text":"Works well","review_time":"1788210337082","like_count":5,
				"reviewer_id":"buyer-1","reviewer_name":"Buyer","reviewer_avatar_url":"https://img.example/buyer.jpg"}]}}}`,
			expected: "review-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(test.response))
			}))
			t.Cleanup(server.Close)
			adapter := newTikHubTestAdapter(t, server.URL, nil)

			result, err := adapter.Execute(context.Background(), test.operation, CanonicalRequest{OperationKey: test.operation.OperationKey, Platform: "tiktok_shop"})

			require.NoError(t, err)
			require.NoError(t, validateSchemaValue(result.Data, test.operation.OutputSchema, "$"))
			data, ok := result.Data.(map[string]any)
			require.True(t, ok)
			if items, ok := data["items"].([]any); ok {
				require.NotEmpty(t, items)
				item := items[0].(map[string]any)
				assert.Equal(t, test.expected, item["id"])
				if test.name == "reviews" {
					assert.Equal(t, "Works well", item["text"])
					assert.Equal(t, "2026-08-31T21:05:37Z", item["published_at"])
					assert.Equal(t, map[string]any{
						"id": "buyer-1", "display_name": "Buyer", "avatar_url": "https://img.example/buyer.jpg",
					}, item["author"])
				}
			} else {
				assert.Equal(t, test.expected, data["id"])
			}
		})
	}
}

func TestTikHubAdapterNormalizesVerifiedWeChatContracts(t *testing.T) {
	tests := []struct {
		name      string
		operation ProviderOperation
		params    map[string]any
		response  string
		expected  map[string]any
	}{
		{
			name: "article search",
			operation: verifiedProviderPOST("social.content.search", "wechat_mp", "wechat-search", "/api/v1/wechat_search/v2/fetch_search",
				map[string]string{"query": "keyword"}, map[string]any{"business_type": "article", "raw": false}, listOutputSchema("content"), 1_000),
			params: map[string]any{"query": "天工机器人"},
			response: `{"code":200,"data":{"continue_flag":1,"cursor":"next-search","items":[{
				"docID":"article-1","title":"<em class=\"highlight\">天工</em>机器人","desc":"智慧<em class=\"highlight\">分拣</em>",
				"doc_url":"https://mp.weixin.qq.com/s/article-1","timestamp":1787909400,"thumbUrl":"https://img.example/article.jpg",
				"source":{"title":"Tegene Robot"}}]}}`,
			expected: map[string]any{"id": "article-1", "title": "天工机器人", "text": "智慧分拣", "cursor": "next-search", "has_more": true},
		},
		{
			name: "account articles",
			operation: verifiedProviderPOST("social.account.contents.list", "wechat_mp", "wechat-account-articles", "/api/v1/wechat_mp/v2/fetch_account_articles",
				map[string]string{"account_ref": "username"}, map[string]any{"raw": false}, listOutputSchema("content"), 1_000),
			params: map[string]any{"account_ref": "gh_1a2f6fc19398"},
			response: `{"code":200,"data":{"is_end":0,"next_offset":"next-account","articles":[{
				"app_msg_id":2247524552,"idx":1,"title":"复杂件型如何一次分拣到位","digest":"木马速运一体机方案",
				"url":"https://mp.weixin.qq.com/s/article-2","cover":"https://img.example/cover.jpg","create_time":1787904549}]}}`,
			expected: map[string]any{"id": "2247524552-1", "title": "复杂件型如何一次分拣到位", "text": "木马速运一体机方案", "cursor": "next-account", "has_more": true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(test.response))
			}))
			t.Cleanup(server.Close)
			adapter := newTikHubTestAdapter(t, server.URL, nil)

			result, err := adapter.Execute(context.Background(), test.operation, CanonicalRequest{
				OperationKey: test.operation.OperationKey, Platform: "wechat_mp", Params: test.params,
			})

			require.NoError(t, err)
			require.NoError(t, validateSchemaValue(result.Data, test.operation.OutputSchema, "$"))
			data := result.Data.(map[string]any)
			items := data["items"].([]any)
			require.Len(t, items, 1)
			item := items[0].(map[string]any)
			assert.Equal(t, test.expected["id"], item["id"])
			assert.Equal(t, test.expected["title"], item["title"])
			assert.Equal(t, test.expected["text"], item["text"])
			assert.Equal(t, test.expected["cursor"], data["page"].(map[string]any)["cursor"])
			assert.Equal(t, test.expected["has_more"], data["page"].(map[string]any)["has_more"])
			assert.NotContains(t, item["title"], "<em")
		})
	}
}

func TestTikHubCatalogUsesCurrentTikTokShopReviewsV2Binding(t *testing.T) {
	snapshot := standardProviderCatalog(ProviderTikHub)
	detailFound := false
	reviewsFound := false
	for _, operation := range snapshot.Operations {
		if operation.OperationKey == "commerce.product.get" && operation.Platform == "tiktok_shop" {
			assert.Equal(t, "/api/v1/tiktok/shop/web/fetch_product_detail_v3", operation.Path)
			detailFound = true
		}
		if operation.OperationKey == "commerce.product.reviews.list" && operation.Platform == "tiktok_shop" {
			assert.Equal(t, "/api/v1/tiktok/shop/web/fetch_product_reviews_v2", operation.Path)
			assert.NotContains(t, operation.FixedParams, "page_size")
			reviewsFound = true
		}
	}
	assert.True(t, detailFound, "TikTok Shop detail binding not found")
	assert.True(t, reviewsFound, "TikTok Shop review binding not found")
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
			assert.Equal(t, false, body["raw"])
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":200,"request_id":"post-1","data":{"ok":true}}`))
	}))
	t.Cleanup(server.Close)
	adapter := newTikHubTestAdapter(t, server.URL, nil)

	operation := tikHubTestOperation(http.MethodPost)
	operation.FixedParams = map[string]any{"raw": false}
	result, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: "social.content.search",
		Platform:     "tiktok",
		Params:       map[string]any{"query": "cats", "limit": 0, "include_ads": false},
	})
	require.NoError(t, err)
	assert.Equal(t, "post-1", result.ProviderRequestID)
}

func TestTikHubAdapterRejectsFixedRawResponseEnablement(t *testing.T) {
	adapter := newTikHubTestAdapter(t, "http://127.0.0.1:8080", nil)
	operation := tikHubTestOperation(http.MethodPost)
	operation.FixedParams = map[string]any{"raw": true}

	_, err := adapter.Execute(context.Background(), operation, CanonicalRequest{
		OperationKey: operation.OperationKey, Platform: operation.Platform, Params: map[string]any{"query": "cats"},
	})

	var connectorError *ConnectorError
	require.ErrorAs(t, err, &connectorError)
	assert.Equal(t, "UPSTREAM_BINDING_INVALID", connectorError.Code)
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
	verifiedCosts := map[string]int64{
		"social.account.get/youtube":                1_000,
		"social.account.contents.list/wechat_mp":    1_000,
		"social.account.contents.list/youtube":      1_000,
		"social.content.get/youtube":                1_000,
		"social.content.search/youtube":             2_000,
		"social.comment.list/youtube":               1_000,
		"social.comment.replies.list/youtube":       1_000,
		"social.trend.list/douyin":                  1_000,
		"commerce.product.search/tiktok_shop":       1_000,
		"commerce.product.get/tiktok_shop":          1_000,
		"commerce.product.reviews.list/tiktok_shop": 1_000,
	}
	verified := 0
	for _, operation := range snapshot.Operations {
		assert.Equal(t, tikHubDirectMappingKey, operation.MappingKey)
		assert.Equal(t, "USD", operation.CostCurrency)
		cost, expectedVerified := verifiedCosts[operation.OperationKey+"/"+operation.Platform]
		if expectedVerified {
			assert.True(t, operation.ContractEquivalent)
			assert.True(t, operation.BillingReady)
			assert.Equal(t, cost, operation.CostAmountMicros)
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
	assert.Equal(t, len(verifiedCosts), verified)
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
