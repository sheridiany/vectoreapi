package vsearch

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const (
	DefaultAgentKeyMCPURL   = "https://api.agentkey.app/v1/mcp"
	AgentKeyProtocolVersion = "2025-06-18"

	defaultAgentKeyTimeout          = 15 * time.Second
	defaultAgentKeyMaxResponseBytes = int64(4 << 20)
)

type UpstreamConnector interface {
	Account(ctx context.Context) (any, error)
	FindTools(ctx context.Context, query string, prefix string) (any, error)
	DescribeTool(ctx context.Context, name string) (any, error)
	ExecuteTool(ctx context.Context, name string, params map[string]any) (any, error)
}

type AgentKeyConnectorConfig struct {
	BaseURL           string
	Secret            string
	Timeout           time.Duration
	MaxResponseBytes  int64
	HTTPClient        *http.Client
	AllowLoopbackHTTP bool
}

type ConnectorError struct {
	Code       string
	HTTPStatus int
	Message    string
	cause      error
}

func (e *ConnectorError) Error() string {
	return e.Message
}

func (e *ConnectorError) Unwrap() error {
	return e.cause
}

func newConnectorError(code string, status int, message string) *ConnectorError {
	return &ConnectorError{Code: code, HTTPStatus: status, Message: message}
}

func newConnectorContextError(err error) *ConnectorError {
	if errors.Is(err, context.Canceled) {
		return &ConnectorError{
			Code:       "UPSTREAM_REQUEST_CANCELED",
			HTTPStatus: http.StatusRequestTimeout,
			Message:    "上游请求已取消。",
			cause:      context.Canceled,
		}
	}
	return &ConnectorError{
		Code:       "UPSTREAM_TIMEOUT",
		HTTPStatus: http.StatusGatewayTimeout,
		Message:    "上游服务响应超时。",
		cause:      context.DeadlineExceeded,
	}
}
