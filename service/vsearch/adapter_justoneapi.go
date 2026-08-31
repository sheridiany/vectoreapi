package vsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
)

const (
	defaultJustOneAPITimeout          = 60 * time.Second
	defaultJustOneAPIMaxResponseBytes = int64(4 << 20)
	justOneAPIDirectMappingKey        = "justoneapi.direct.v1"
)

type JustOneAPIAdapter struct {
	endpoint         *url.URL
	secret           string
	timeout          time.Duration
	maxResponseBytes int64
	client           *http.Client
}

type justOneAPIEnvelope struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func NewJustOneAPIAdapter(config AdapterConfig) (ProviderAdapter, error) {
	if provider := strings.TrimSpace(config.Provider); provider != "" && provider != ProviderJustOneAPI {
		return nil, newConnectorError("UPSTREAM_PROVIDER_INVALID", http.StatusBadRequest, "上游服务类型无效。")
	}
	endpoint, err := validateJustOneAPIURL(config.BaseURL, config.AllowLoopbackHTTP)
	if err != nil {
		return nil, err
	}
	secret := strings.TrimSpace(config.Secret)
	if secret == "" {
		return nil, newConnectorError("UPSTREAM_SECRET_REQUIRED", http.StatusInternalServerError, "上游服务密钥未配置。")
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultJustOneAPITimeout
	}
	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultJustOneAPIMaxResponseBytes
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

	return &JustOneAPIAdapter{
		endpoint:         endpoint,
		secret:           secret,
		timeout:          timeout,
		maxResponseBytes: maxResponseBytes,
		client:           client,
	}, nil
}

func secureSearchProviderTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

func validateJustOneAPIURL(rawURL string, allowLoopbackHTTP bool) (*url.URL, error) {
	if strings.TrimSpace(rawURL) == "" {
		rawURL = DefaultJustOneAPIBaseURL
	}
	endpoint, err := model.ValidateSearchUpstreamProviderBaseURL(ProviderJustOneAPI, rawURL, allowLoopbackHTTP)
	if errors.Is(err, model.ErrSearchUpstreamURLHTTPSRequired) {
		return nil, newConnectorError("UPSTREAM_URL_HTTPS_REQUIRED", http.StatusBadRequest, "上游服务地址必须使用 HTTPS。")
	}
	if err != nil {
		return nil, newConnectorError("UPSTREAM_URL_INVALID", http.StatusBadRequest, "上游服务地址无效。")
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")
	return endpoint, nil
}

func (a *JustOneAPIAdapter) Probe(ctx context.Context) (AccountState, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return AccountState{}, newConnectorContextError(err)
		}
	}
	return AccountState{}, newConnectorError(
		"UPSTREAM_PROBE_UNSUPPORTED",
		http.StatusNotImplemented,
		"JustOneAPI 未提供可用于无计费凭据校验的公开账户接口。",
	)
}

func (a *JustOneAPIAdapter) SnapshotCatalog(ctx context.Context) (CatalogSnapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return CatalogSnapshot{}, newConnectorContextError(err)
		}
	}
	return standardProviderCatalog(ProviderJustOneAPI), nil
}

func (a *JustOneAPIAdapter) Execute(ctx context.Context, operation ProviderOperation, canonical CanonicalRequest) (AttemptResult, error) {
	result := AttemptResult{
		CostAmountMicros: operation.CostAmountMicros,
		CostCurrency:     operation.CostCurrency,
	}
	method, endpoint, err := a.resolveOperation(operation)
	if err != nil {
		return result, err
	}

	mappedParams, err := mapJustOneAPIParams(operation, canonical)
	if err != nil {
		return result, err
	}
	values, err := encodeJustOneAPIParams(mappedParams)
	if err != nil {
		return result, err
	}
	query := endpoint.Query()
	if method == http.MethodGet {
		for key, entries := range values {
			for _, entry := range entries {
				query.Add(key, entry)
			}
		}
	}
	switch operation.AuthPlacement {
	case AuthPlacementQuery:
		query.Set("token", a.secret)
	case AuthPlacementForm:
		if method != http.MethodPost {
			return result, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口认证位置配置无效。")
		}
		values.Set("token", a.secret)
	default:
		return result, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口认证位置配置无效。")
	}
	endpoint.RawQuery = query.Encode()
	body := bytes.NewReader(nil)
	if method == http.MethodPost {
		body = bytes.NewReader([]byte(values.Encode()))
	}

	if ctx == nil {
		ctx = context.Background()
	}
	requestContext, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, method, endpoint.String(), body)
	if err != nil {
		return result, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusInternalServerError, "无法构造上游请求。")
	}
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	var wroteRequest atomic.Bool
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wroteRequest.Store(true)
			}
		},
	}))
	response, err := a.client.Do(request)
	if err != nil {
		result.Dispatched = wroteRequest.Load()
		if errors.Is(requestContext.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return result, newConnectorContextError(context.Canceled)
		}
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return result, newConnectorContextError(context.DeadlineExceeded)
		}
		return result, newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "无法连接上游服务。")
	}
	defer response.Body.Close()

	result.Dispatched = true
	result.HTTPStatus = response.StatusCode
	result.ProviderRequestID = sanitizeProviderRequestID(
		firstJustOneAPIHeader(response.Header, "x-request-id", "request-id", "x-trace-id", "trace-id"), a.secret,
	)
	result.RetryAfter = parseJustOneAPIRetryAfter(response.Header.Get("Retry-After"), time.Now())
	if response.StatusCode == http.StatusTooManyRequests {
		result.BusinessCode = strconv.Itoa(response.StatusCode)
		billable := false
		result.Billable = &billable
	}

	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return result, newConnectorError("UPSTREAM_REDIRECT_REJECTED", http.StatusBadGateway, "上游服务返回了不允许的重定向。")
	}
	payload, readErr := readJustOneAPIBody(response.Body, a.maxResponseBytes)
	if readErr != nil {
		return result, readErr
	}

	envelope, decodeErr := decodeJustOneAPIEnvelope(response.Header.Get("Content-Type"), payload)
	if decodeErr == nil {
		businessCode, codeErr := justOneAPIBusinessCode(envelope.Code)
		if codeErr == nil {
			result.BusinessCode = strconv.Itoa(businessCode)
			if businessCode == 0 && response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
				billable := true
				result.Billable = &billable
				result.Data = envelope.Data
				return result, nil
			}
			if justOneAPIExplicitNonBillableCode(businessCode) {
				billable := false
				result.Billable = &billable
				return result, justOneAPIBusinessError(businessCode)
			}
			if businessCode != 0 {
				return result, newConnectorError("UPSTREAM_BUSINESS_ERROR", http.StatusBadGateway, "上游服务返回了业务错误。")
			}
		}
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return result, justOneAPIHTTPError(response.StatusCode)
	}
	if decodeErr != nil {
		return result, decodeErr
	}
	return result, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
}

func mapJustOneAPIParams(operation ProviderOperation, canonical CanonicalRequest) (map[string]any, error) {
	if canonical.OperationKey != "" && canonical.OperationKey != operation.OperationKey {
		return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求能力与上游绑定不匹配。")
	}
	if strings.TrimSpace(canonical.PageToken) != "" {
		return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "当前能力尚不支持分页令牌。")
	}

	canonicalParams := make(map[string]any, len(canonical.Params)+1)
	for key, value := range canonical.Params {
		canonicalParams[key] = value
	}
	if platform := strings.TrimSpace(canonical.Platform); platform != "" {
		if existing, ok := canonicalParams["platform"]; ok {
			existingPlatform, isString := existing.(string)
			if !isString || !strings.EqualFold(strings.TrimSpace(existingPlatform), platform) {
				return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求平台参数不一致。")
			}
		}
		canonicalParams["platform"] = platform
	}

	mapped := make(map[string]any, len(canonicalParams)+len(operation.FixedParams))
	mappedKeys := make(map[string]struct{}, len(operation.ParameterMap))
	for canonicalKey, value := range canonicalParams {
		if canonicalKey == "platform" {
			platform, ok := value.(string)
			if !ok || (canonical.Platform != "" && !strings.EqualFold(strings.TrimSpace(platform), strings.TrimSpace(canonical.Platform))) ||
				(operation.Platform != "" && !strings.EqualFold(strings.TrimSpace(platform), strings.TrimSpace(operation.Platform))) {
				return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求平台与上游绑定不匹配。")
			}
		}
		if searchProviderReservedParam(canonicalKey) {
			return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求参数包含不允许的字段。")
		}
		upstreamKey, ok := operation.ParameterMap[canonicalKey]
		if !ok {
			if canonicalKey == "platform" {
				continue
			}
			return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求包含当前能力不支持的参数。")
		}
		upstreamKey = strings.TrimSpace(upstreamKey)
		if upstreamKey == "" || searchProviderReservedParam(upstreamKey) {
			return nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口参数映射配置无效。")
		}
		if _, exists := mappedKeys[upstreamKey]; exists {
			return nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口参数映射配置无效。")
		}
		mappedKeys[upstreamKey] = struct{}{}
		mapped[upstreamKey] = value
	}
	for key, value := range operation.FixedParams {
		key = strings.TrimSpace(key)
		if key == "" || searchProviderReservedParam(key) {
			return nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口固定参数配置无效。")
		}
		mapped[key] = value
	}
	return mapped, nil
}

func (a *JustOneAPIAdapter) resolveOperation(operation ProviderOperation) (string, *url.URL, error) {
	method := strings.ToUpper(strings.TrimSpace(operation.Method))
	if method != http.MethodGet && method != http.MethodPost {
		return "", nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口请求方法配置无效。")
	}
	if operation.Method != method || strings.TrimSpace(operation.MappingKey) != justOneAPIDirectMappingKey || strings.TrimSpace(operation.MappingVersion) != "v1" {
		return "", nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口绑定配置无效。")
	}
	rawPath := strings.TrimSpace(operation.Path)
	parsedPath, err := url.Parse(rawPath)
	if err != nil || rawPath == "" || !strings.HasPrefix(rawPath, "/") || parsedPath.IsAbs() || parsedPath.Host != "" || parsedPath.User != nil || parsedPath.RawQuery != "" || parsedPath.Fragment != "" {
		return "", nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口路径配置无效。")
	}
	unescapedPath, err := url.PathUnescape(parsedPath.EscapedPath())
	if err != nil || path.Clean(unescapedPath) != unescapedPath || strings.Contains(unescapedPath, "//") {
		return "", nil, newConnectorError("UPSTREAM_BINDING_INVALID", http.StatusInternalServerError, "上游接口路径配置无效。")
	}
	endpoint := *a.endpoint
	endpoint.Path = strings.TrimSuffix(a.endpoint.Path, "/") + parsedPath.Path
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return method, &endpoint, nil
}

func encodeJustOneAPIParams(params map[string]any) (url.Values, error) {
	values := make(url.Values, len(params))
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" || strings.EqualFold(key, "token") {
			return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求参数包含不允许的字段。")
		}
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			values.Set(key, typed)
		case bool:
			values.Set(key, strconv.FormatBool(typed))
		case int:
			values.Set(key, strconv.Itoa(typed))
		case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			values.Set(key, fmt.Sprint(typed))
		default:
			encoded, err := common.Marshal(typed)
			if err != nil {
				return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusBadRequest, "请求参数无法转换为上游格式。")
			}
			values.Set(key, string(encoded))
		}
	}
	return values, nil
}

func readJustOneAPIBody(reader io.Reader, maxBytes int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "无法读取上游服务响应。")
	}
	if int64(len(payload)) > maxBytes {
		return nil, newConnectorError("UPSTREAM_RESPONSE_TOO_LARGE", http.StatusBadGateway, "上游服务响应超过大小限制。")
	}
	return payload, nil
}

func justOneAPIHTTPError(status int) error {
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

func decodeJustOneAPIEnvelope(contentType string, payload []byte) (justOneAPIEnvelope, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return justOneAPIEnvelope{}, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return justOneAPIEnvelope{}, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	var envelope justOneAPIEnvelope
	if err := common.Unmarshal(payload, &envelope); err != nil || envelope.Code == nil {
		return justOneAPIEnvelope{}, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	return envelope, nil
}

func justOneAPIBusinessCode(value any) (int, error) {
	switch typed := value.(type) {
	case float64:
		code := int(typed)
		if typed != float64(code) {
			return 0, errors.New("business code is not an integer")
		}
		return code, nil
	case string:
		return strconv.Atoi(strings.TrimSpace(typed))
	default:
		return 0, errors.New("business code has an unsupported type")
	}
}

func justOneAPIExplicitNonBillableCode(code int) bool {
	switch code {
	case 100, 301, 302, 303, 400, 500, 600, 601, 602:
		return true
	default:
		return false
	}
}

func justOneAPIBusinessError(code int) error {
	switch code {
	case 100:
		return newConnectorError("UPSTREAM_AUTH_FAILED", http.StatusBadGateway, "上游服务认证失败。")
	case 600:
		return newConnectorError("UPSTREAM_FORBIDDEN", http.StatusBadGateway, "上游密钥没有调用该接口的权限。")
	case 301:
		return newConnectorError("UPSTREAM_COLLECTION_FAILED", http.StatusBadGateway, "上游数据采集失败。")
	case 302:
		return newConnectorError("UPSTREAM_RATE_LIMITED", http.StatusBadGateway, "上游服务当前限流。")
	case 303:
		return newConnectorError("UPSTREAM_DAILY_QUOTA_EXHAUSTED", http.StatusBadGateway, "上游接口当日额度已用尽。")
	case 400:
		return newConnectorError("UPSTREAM_REQUEST_FAILED", http.StatusBadRequest, "上游服务拒绝了请求参数。")
	case 500:
		return newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "上游服务暂不可用。")
	case 601:
		return newConnectorError("UPSTREAM_CREDITS_UNAVAILABLE", http.StatusBadGateway, "上游服务额度不可用。")
	case 602:
		return newConnectorError("UPSTREAM_BUDGET_EXHAUSTED", http.StatusBadGateway, "上游密钥预算已用尽。")
	default:
		return newConnectorError("UPSTREAM_BUSINESS_ERROR", http.StatusBadGateway, "上游服务返回了业务错误。")
	}
}

func firstJustOneAPIHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func parseJustOneAPIRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		if seconds > int((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	duration := when.Sub(now)
	if duration > 24*time.Hour {
		return 24 * time.Hour
	}
	return duration
}

var _ ProviderAdapter = (*JustOneAPIAdapter)(nil)
