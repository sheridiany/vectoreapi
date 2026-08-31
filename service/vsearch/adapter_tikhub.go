package vsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

const (
	defaultTikHubTimeout          = 60 * time.Second
	defaultTikHubMaxResponseBytes = int64(4 << 20)
	tikHubDirectMappingKey        = "tikhub.direct.v1"
	tikHubUserInfoPath            = "/api/v1/tikhub/user/get_user_info"
)

type TikHubAdapter struct {
	baseURL          *url.URL
	secret           string
	timeout          time.Duration
	maxResponseBytes int64
	client           *http.Client
}

type tikHubEnvelope struct {
	Code      int             `json:"code"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type tikHubUserInfoResponse struct {
	Code     int `json:"code"`
	UserData struct {
		Balance         float64 `json:"balance"`
		FreeCredit      float64 `json:"free_credit"`
		AccountDisabled bool    `json:"account_disabled"`
		Active          bool    `json:"is_active"`
	} `json:"user_data"`
}

func NewTikHubAdapter(config AdapterConfig) (ProviderAdapter, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = DefaultTikHubBaseURL
	}
	endpoint, err := model.ValidateSearchUpstreamProviderBaseURL(ProviderTikHub, baseURL, config.AllowLoopbackHTTP)
	if errors.Is(err, model.ErrSearchUpstreamURLHTTPSRequired) {
		return nil, newConnectorError("UPSTREAM_URL_HTTPS_REQUIRED", http.StatusBadRequest, "上游服务地址必须使用 HTTPS。")
	}
	if err != nil {
		return nil, newConnectorError("UPSTREAM_URL_INVALID", http.StatusBadRequest, "上游服务地址无效。")
	}
	if provider := strings.TrimSpace(config.Provider); provider != "" && provider != ProviderTikHub {
		return nil, newConnectorError("UPSTREAM_PROVIDER_INVALID", http.StatusBadRequest, "上游服务类型无效。")
	}
	secret := strings.TrimSpace(config.Secret)
	if secret == "" {
		return nil, newConnectorError("UPSTREAM_SECRET_REQUIRED", http.StatusInternalServerError, "上游服务密钥未配置。")
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTikHubTimeout
	}
	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultTikHubMaxResponseBytes
	}
	client := &http.Client{Timeout: timeout}
	if config.HTTPClient != nil {
		clientCopy := *config.HTTPClient
		client = &clientCopy
	}
	if config.HTTPClient == nil || strings.EqualFold(endpoint.Scheme, "https") {
		client.Transport = secureSearchProviderTransport()
	}
	if client.Timeout <= 0 || client.Timeout > timeout {
		client.Timeout = timeout
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &TikHubAdapter{
		baseURL: endpoint, secret: secret, timeout: timeout,
		maxResponseBytes: maxResponseBytes, client: client,
	}, nil
}

func (a *TikHubAdapter) Probe(ctx context.Context) (AccountState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	operationContext, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	request, err := a.newRequest(operationContext, http.MethodGet, tikHubUserInfoPath, nil)
	if err != nil {
		return AccountState{}, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		if errors.Is(operationContext.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return AccountState{}, newConnectorContextError(context.Canceled)
		}
		if errors.Is(operationContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return AccountState{}, newConnectorContextError(context.DeadlineExceeded)
		}
		return AccountState{}, newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "无法连接上游服务。")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return AccountState{}, tikHubHTTPError(response.StatusCode)
	}
	body, err := readTikHubBody(response, a.maxResponseBytes)
	if err != nil {
		return AccountState{}, err
	}
	var payload tikHubUserInfoResponse
	if err := common.Unmarshal(body, &payload); err != nil || payload.Code != http.StatusOK {
		return AccountState{}, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	if payload.UserData.AccountDisabled || !payload.UserData.Active {
		return AccountState{}, newConnectorError("UPSTREAM_ACCOUNT_DISABLED", http.StatusBadGateway, "上游服务账号不可用。")
	}

	balance := decimal.NewFromFloat(payload.UserData.Balance).
		Add(decimal.NewFromFloat(payload.UserData.FreeCredit)).
		Shift(6)
	if balance.IsNegative() || balance.GreaterThan(decimal.NewFromInt(1<<63-1)) {
		return AccountState{}, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	return AccountState{
		BalanceAmountMicros: balance.IntPart(), BalanceCurrency: "USD",
	}, nil
}

func (a *TikHubAdapter) SnapshotCatalog(ctx context.Context) (CatalogSnapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return CatalogSnapshot{}, newConnectorContextError(err)
		}
	}
	return standardProviderCatalog(ProviderTikHub), nil
}

func (a *TikHubAdapter) Execute(ctx context.Context, operation ProviderOperation, canonicalRequest CanonicalRequest) (AttemptResult, error) {
	result := AttemptResult{
		CostAmountMicros: operation.CostAmountMicros,
		CostCurrency:     operation.CostCurrency,
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateTikHubOperation(operation, canonicalRequest); err != nil {
		return result, err
	}
	providerParams, err := mapTikHubParams(operation, canonicalRequest)
	if err != nil {
		return result, err
	}

	operationContext, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	request, err := a.newRequest(operationContext, operation.Method, operation.Path, providerParams)
	if err != nil {
		return result, err
	}
	var wroteRequest atomic.Bool
	trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
		if info.Err == nil {
			wroteRequest.Store(true)
		}
	}}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))

	response, err := a.client.Do(request)
	if err != nil {
		result.Dispatched = wroteRequest.Load()
		if errors.Is(operationContext.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return result, newConnectorContextError(context.Canceled)
		}
		if errors.Is(operationContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return result, newConnectorContextError(context.DeadlineExceeded)
		}
		return result, newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "无法连接上游服务。")
	}
	defer response.Body.Close()
	result.Dispatched = true
	result.HTTPStatus = response.StatusCode
	result.ProviderRequestID = sanitizeProviderRequestID(response.Header.Get("X-Request-ID"), a.secret)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result.BusinessCode = strconv.Itoa(response.StatusCode)
		result.Billable = tikHubBool(false)
		if response.StatusCode == http.StatusTooManyRequests {
			result.RetryAfter = tikHubRetryAfter(response.Header, time.Now())
		}
		return result, tikHubHTTPError(response.StatusCode)
	}

	body, err := readTikHubBody(response, a.maxResponseBytes)
	if err != nil {
		return result, err
	}
	var payload tikHubEnvelope
	if err := common.Unmarshal(body, &payload); err != nil {
		return result, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	if requestID := strings.TrimSpace(payload.RequestID); requestID != "" {
		result.ProviderRequestID = sanitizeProviderRequestID(requestID, a.secret)
	}
	result.BusinessCode = strconv.Itoa(payload.Code)
	if payload.Code != http.StatusOK {
		if payload.Code == http.StatusTooManyRequests {
			result.RetryAfter = tikHubRetryAfter(response.Header, time.Now())
		}
		return result, tikHubHTTPError(payload.Code)
	}
	result.Billable = tikHubBool(true)
	if operation.OperationKey == "social.trend.list" {
		result.Data, err = normalizeTikHubTrendList(payload.Data, operation.Platform)
		if err != nil {
			return result, err
		}
		return result, nil
	}
	if err := common.Unmarshal(payload.Data, &result.Data); err != nil {
		return result, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	return result, nil
}

func normalizeTikHubTrendList(data json.RawMessage, platform string) (map[string]any, error) {
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
	}
	wordList, ok := payload["word_list"]
	if !ok || common.GetJsonType(wordList) != "array" {
		return nil, newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
	}
	var entries []map[string]json.RawMessage
	if err := common.Unmarshal(wordList, &entries); err != nil {
		return nil, newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
	}
	items := make([]any, 0, len(entries))
	for index, entry := range entries {
		id, validID := tikHubTrendID(entry["sentence_id"])
		var title string
		if !validID || common.Unmarshal(entry["word"], &title) != nil || strings.TrimSpace(title) == "" {
			return nil, newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
		}
		item := map[string]any{
			"id": id, "type": "trend", "platform": platform,
			"title": title, "rank": index + 1,
		}
		if scoreRaw, exists := entry["hot_value"]; exists {
			var score float64
			if common.Unmarshal(scoreRaw, &score) != nil {
				return nil, newConnectorError("UPSTREAM_CONTRACT_MISMATCH", http.StatusBadGateway, "上游服务返回的数据不符合能力约定。")
			}
			item["score"] = score
		}
		items = append(items, item)
	}
	return map[string]any{
		"items": items,
		"page":  map[string]any{"cursor": nil, "has_more": false},
	}, nil
}

func tikHubTrendID(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	var id string
	switch common.GetJsonType(trimmed) {
	case "string":
		if common.Unmarshal(trimmed, &id) != nil {
			return "", false
		}
	case "number":
		id = string(trimmed)
	default:
		return "", false
	}
	if id == "" {
		return "", false
	}
	for _, character := range id {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	return id, true
}

func validateTikHubOperation(operation ProviderOperation, request CanonicalRequest) error {
	method := strings.ToUpper(strings.TrimSpace(operation.Method))
	if method != http.MethodGet && method != http.MethodPost {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	if operation.Method != method {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	operationPath := strings.TrimSpace(operation.Path)
	decodedPath, err := url.PathUnescape(operationPath)
	if err != nil || operationPath == "" || !strings.HasPrefix(operationPath, "/") || strings.HasPrefix(operationPath, "//") ||
		strings.ContainsAny(operationPath, "?#{}") || path.Clean(decodedPath) != decodedPath {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	if strings.TrimSpace(operation.MappingKey) != tikHubDirectMappingKey || strings.TrimSpace(operation.MappingVersion) == "" {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	if operation.OperationKey == "social.trend.list" &&
		(operation.ContractVersion != "v1" || !strings.EqualFold(operation.Platform, "douyin")) {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	if operation.AuthPlacement != "" && operation.AuthPlacement != AuthPlacementBearer {
		return newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
	}
	if request.OperationKey != "" && request.OperationKey != operation.OperationKey {
		return newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求的能力标识无效。")
	}
	if request.Platform != "" && operation.Platform != "" && !strings.EqualFold(request.Platform, operation.Platform) {
		return newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求的平台与能力不匹配。")
	}
	if strings.TrimSpace(request.PageToken) != "" {
		return newConnectorError("UPSTREAM_PAGE_TOKEN_UNSUPPORTED", http.StatusBadRequest, "当前能力暂不支持翻页令牌。")
	}
	return nil
}

func mapTikHubParams(operation ProviderOperation, request CanonicalRequest) (map[string]any, error) {
	providerParams := make(map[string]any, len(request.Params)+len(operation.FixedParams))
	for canonicalName, value := range request.Params {
		canonicalName = strings.TrimSpace(canonicalName)
		if canonicalName == "platform" {
			platform, ok := value.(string)
			if !ok || (request.Platform != "" && !strings.EqualFold(strings.TrimSpace(platform), request.Platform)) ||
				(operation.Platform != "" && !strings.EqualFold(strings.TrimSpace(platform), operation.Platform)) {
				return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求的平台与能力不匹配。")
			}
			continue
		}
		if canonicalName == "" || searchProviderReservedParam(canonicalName) {
			return nil, newConnectorError("UPSTREAM_PARAMETER_FORBIDDEN", http.StatusBadRequest, "请求包含不允许的参数。")
		}
		providerName, ok := operation.ParameterMap[canonicalName]
		providerName = strings.TrimSpace(providerName)
		if !ok || providerName == "" {
			return nil, newConnectorError("UPSTREAM_PARAMETER_UNSUPPORTED", http.StatusBadRequest, "请求包含当前能力不支持的参数。")
		}
		if searchProviderReservedParam(providerName) {
			return nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
		}
		providerParams[providerName] = value
	}
	for name, value := range operation.FixedParams {
		name = strings.TrimSpace(name)
		if name == "" || searchProviderReservedParam(name) {
			return nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游能力绑定无效。")
		}
		providerParams[name] = value
	}
	return providerParams, nil
}

func (a *TikHubAdapter) newRequest(ctx context.Context, method string, operationPath string, params map[string]any) (*http.Request, error) {
	endpoint := *a.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + operationPath
	endpoint.RawPath = ""

	var body io.Reader
	if method == http.MethodGet {
		query := endpoint.Query()
		for name, value := range params {
			encoded, err := tikHubQueryValue(value)
			if err != nil {
				return nil, err
			}
			query.Set(name, encoded)
		}
		endpoint.RawQuery = query.Encode()
	} else {
		payload, err := common.Marshal(params)
		if err != nil {
			return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "无法构造上游请求。")
		}
		body = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "无法构造上游请求。")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+a.secret)
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func tikHubQueryValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	default:
		return "", newConnectorError("UPSTREAM_PARAMETER_INVALID", http.StatusBadRequest, "请求参数格式无效。")
	}
}

func readTikHubBody(response *http.Response, maxBytes int64) ([]byte, error) {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "无法读取上游服务响应。")
	}
	if int64(len(body)) > maxBytes {
		return nil, newConnectorError("UPSTREAM_RESPONSE_TOO_LARGE", http.StatusBadGateway, "上游服务响应超过大小限制。")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	return body, nil
}

func tikHubRetryAfter(headers http.Header, now time.Time) time.Duration {
	retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds > 0 {
		if seconds > int64((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(retryAfter); err == nil && retryAt.After(now) {
		duration := retryAt.Sub(now)
		if duration > 24*time.Hour {
			return 24 * time.Hour
		}
		return duration
	}
	reset := strings.TrimSpace(headers.Get("X-RateLimit-Reset"))
	if unixSeconds, err := strconv.ParseInt(reset, 10, 64); err == nil {
		retryAt := time.Unix(unixSeconds, 0)
		if retryAt.After(now) {
			duration := retryAt.Sub(now)
			if duration > 24*time.Hour {
				return 24 * time.Hour
			}
			return duration
		}
	}
	return 0
}

func tikHubHTTPError(status int) error {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return newConnectorError("UPSTREAM_REQUEST_FAILED", http.StatusBadGateway, "上游服务拒绝了请求参数。")
	case http.StatusUnauthorized:
		return newConnectorError("UPSTREAM_AUTH_FAILED", http.StatusBadGateway, "上游服务认证失败。")
	case http.StatusPaymentRequired:
		return newConnectorError("UPSTREAM_CREDITS_UNAVAILABLE", http.StatusBadGateway, "上游服务额度不可用。")
	case http.StatusForbidden:
		return newConnectorError("UPSTREAM_FORBIDDEN", http.StatusBadGateway, "上游服务拒绝了请求。")
	case http.StatusTooManyRequests:
		return newConnectorError("UPSTREAM_RATE_LIMITED", http.StatusBadGateway, "上游服务当前限流。")
	default:
		if status >= http.StatusInternalServerError {
			return newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "上游服务暂不可用。")
		}
		return newConnectorError("UPSTREAM_REQUEST_FAILED", http.StatusBadGateway, "上游服务请求失败。")
	}
}

func tikHubBool(value bool) *bool {
	return &value
}

var _ ProviderAdapter = (*TikHubAdapter)(nil)
