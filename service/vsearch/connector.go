package vsearch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type AgentKeyConnector struct {
	endpoint         *url.URL
	secret           string
	timeout          time.Duration
	maxResponseBytes int64
	client           *http.Client
}

type agentKeySession struct {
	connector *AgentKeyConnector
	sessionID string
	requestID int
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      *int          `json:"id,omitempty"`
	Result  any           `json:"result"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewAgentKeyConnector(config AgentKeyConnectorConfig) (*AgentKeyConnector, error) {
	endpoint, err := validateAgentKeyURL(config.BaseURL, config.AllowLoopbackHTTP)
	if err != nil {
		return nil, err
	}
	secret := strings.TrimSpace(config.Secret)
	if secret == "" {
		return nil, newConnectorError("UPSTREAM_SECRET_REQUIRED", http.StatusInternalServerError, "上游服务密钥未配置。")
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultAgentKeyTimeout
	}
	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultAgentKeyMaxResponseBytes
	}

	client := &http.Client{Timeout: timeout}
	if config.HTTPClient != nil {
		clientCopy := *config.HTTPClient
		client = &clientCopy
	}
	if client.Timeout <= 0 || client.Timeout > timeout {
		client.Timeout = timeout
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &AgentKeyConnector{
		endpoint:         endpoint,
		secret:           secret,
		timeout:          timeout,
		maxResponseBytes: maxResponseBytes,
		client:           client,
	}, nil
}

func validateAgentKeyURL(rawURL string, allowLoopbackHTTP bool) (*url.URL, error) {
	if strings.TrimSpace(rawURL) == "" {
		rawURL = DefaultAgentKeyMCPURL
	}
	endpoint, err := model.ValidateSearchUpstreamBaseURL(rawURL, allowLoopbackHTTP)
	if errors.Is(err, model.ErrSearchUpstreamURLHTTPSRequired) {
		return nil, newConnectorError("UPSTREAM_URL_HTTPS_REQUIRED", http.StatusBadRequest, "上游服务地址必须使用 HTTPS。")
	}
	if err != nil {
		return nil, newConnectorError("UPSTREAM_URL_INVALID", http.StatusBadRequest, "上游服务地址无效。")
	}
	return endpoint, nil
}

func (c *AgentKeyConnector) Account(ctx context.Context) (any, error) {
	return c.withSession(ctx, func(ctx context.Context, session *agentKeySession) (any, error) {
		return session.callTool(ctx, "execute_tool", map[string]any{
			"name":   "agentkey_account",
			"params": map[string]any{},
		})
	})
}

func (c *AgentKeyConnector) FindTools(ctx context.Context, query string, prefix string) (any, error) {
	arguments := map[string]any{}
	if query = strings.TrimSpace(query); query != "" {
		arguments["q"] = query
	}
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		arguments["prefix"] = prefix
	}
	return c.withSession(ctx, func(ctx context.Context, session *agentKeySession) (any, error) {
		return session.callTool(ctx, "find_tools", arguments)
	})
}

func (c *AgentKeyConnector) DescribeTool(ctx context.Context, name string) (any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, newConnectorError("UPSTREAM_TOOL_NAME_REQUIRED", http.StatusBadRequest, "必须提供上游工具名称。")
	}
	return c.withSession(ctx, func(ctx context.Context, session *agentKeySession) (any, error) {
		return session.callTool(ctx, "describe_tool", map[string]any{"name": name})
	})
}

func (c *AgentKeyConnector) ExecuteTool(ctx context.Context, name string, params map[string]any) (any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, newConnectorError("UPSTREAM_TOOL_NAME_REQUIRED", http.StatusBadRequest, "必须提供上游工具名称。")
	}
	if params == nil {
		params = map[string]any{}
	}
	return c.withSession(ctx, func(ctx context.Context, session *agentKeySession) (any, error) {
		return session.callTool(ctx, "execute_tool", map[string]any{"name": name, "params": params})
	})
}

func (c *AgentKeyConnector) withSession(ctx context.Context, operation func(context.Context, *agentKeySession) (any, error)) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	operationContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	session := &agentKeySession{connector: c}
	if err := session.initialize(operationContext); err != nil {
		return nil, err
	}
	return operation(operationContext, session)
}

func (s *agentKeySession) initialize(ctx context.Context) error {
	result, err := s.request(ctx, "initialize", map[string]any{
		"protocolVersion": AgentKeyProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "vsearch",
			"version": "1.0.0",
		},
	}, false)
	if err != nil {
		return err
	}
	if _, ok := result.(map[string]any); !ok {
		return newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	_, err = s.request(ctx, "notifications/initialized", map[string]any{}, true)
	return err
}

func (s *agentKeySession) callTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	return s.request(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	}, false)
}

func (s *agentKeySession) request(ctx context.Context, method string, params any, notification bool) (any, error) {
	requestBody := jsonRPCRequest{JSONRPC: "2.0", Method: method, Params: params}
	if !notification {
		s.requestID++
		requestBody.ID = &s.requestID
	}
	body, err := common.Marshal(requestBody)
	if err != nil {
		return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusInternalServerError, "无法构造上游请求。")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.connector.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, newConnectorError("UPSTREAM_REQUEST_INVALID", http.StatusInternalServerError, "无法构造上游请求。")
	}
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+s.connector.secret)
	if s.sessionID != "" {
		request.Header.Set("mcp-session-id", s.sessionID)
	}
	var wroteRequest atomic.Bool
	if method == "tools/call" {
		trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wroteRequest.Store(true)
			}
		}}
		request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	}
	wrapDispatchError := func(err error, receivedResponse bool) error {
		if method == "tools/call" && (wroteRequest.Load() || receivedResponse) {
			return markExecutionDispatched(err)
		}
		return err
	}

	response, err := s.connector.client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return nil, wrapDispatchError(newConnectorContextError(context.Canceled), false)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return nil, wrapDispatchError(newConnectorContextError(context.DeadlineExceeded), false)
		}
		return nil, wrapDispatchError(newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "无法连接上游服务。"), false)
	}
	defer response.Body.Close()
	if sessionID := strings.TrimSpace(response.Header.Get("mcp-session-id")); sessionID != "" {
		s.sessionID = sessionID
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, wrapDispatchError(connectorHTTPError(response.StatusCode), true)
	}

	payload, err := readAgentKeyResponse(response, s.connector.maxResponseBytes)
	if err != nil {
		return nil, wrapDispatchError(err, true)
	}
	if notification && payload == nil {
		return nil, nil
	}
	if payload == nil {
		return nil, wrapDispatchError(newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。"), true)
	}
	if payload.JSONRPC != "2.0" {
		return nil, wrapDispatchError(newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。"), true)
	}
	if !notification && (payload.ID == nil || requestBody.ID == nil || *payload.ID != *requestBody.ID) {
		return nil, wrapDispatchError(newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。"), true)
	}
	if payload.Error != nil {
		return nil, wrapDispatchError(newConnectorError("UPSTREAM_TOOL_ERROR", http.StatusBadGateway, "上游工具调用失败。"), true)
	}
	return payload.Result, nil
}

func connectorHTTPError(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return newConnectorError("UPSTREAM_AUTH_FAILED", http.StatusBadGateway, "上游服务认证失败。")
	case http.StatusPaymentRequired:
		return newConnectorError("UPSTREAM_CREDITS_UNAVAILABLE", http.StatusBadGateway, "上游服务额度不可用。")
	case http.StatusTooManyRequests:
		return newConnectorError("UPSTREAM_RATE_LIMITED", http.StatusBadGateway, "上游服务当前限流。")
	default:
		if status >= http.StatusInternalServerError {
			return newConnectorError("UPSTREAM_UNAVAILABLE", http.StatusBadGateway, "上游服务暂不可用。")
		}
		return newConnectorError("UPSTREAM_REQUEST_FAILED", http.StatusBadGateway, "上游服务请求失败。")
	}
}

func readAgentKeyResponse(response *http.Response, maxBytes int64) (*jsonRPCResponse, error) {
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	body, err := readLimitedBody(response.Body, maxBytes)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	if mediaType == "text/event-stream" {
		body, err = lastSSEData(body)
		if err != nil {
			return nil, err
		}
	} else if mediaType != "application/json" {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	var payload jsonRPCResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
	}
	return &payload, nil
}

func readLimitedBody(reader io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "无法读取上游服务响应。")
	}
	if int64(len(body)) > maxBytes {
		return nil, newConnectorError("UPSTREAM_RESPONSE_TOO_LARGE", http.StatusBadGateway, "上游服务响应超过大小限制。")
	}
	return body, nil
}

func lastSSEData(body []byte) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	current := make([]string, 0)
	events := make([]string, 0)
	flush := func() {
		if len(current) == 0 {
			return
		}
		events = append(events, strings.Join(current, "\n"))
		current = current[:0]
	}
	for _, line := range lines {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "data:") {
			current = append(current, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	for index := len(events) - 1; index >= 0; index-- {
		payload := strings.TrimSpace(events[index])
		if payload != "" && payload != "[DONE]" {
			return []byte(payload), nil
		}
	}
	return nil, newConnectorError("UPSTREAM_INVALID_RESPONSE", http.StatusBadGateway, "上游服务返回了无效响应。")
}

var _ UpstreamConnector = (*AgentKeyConnector)(nil)
