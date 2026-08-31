package vsearch

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	DefaultJustOneAPIBaseURL = "https://api.justoneapi.com"
	DefaultTikHubBaseURL     = "https://api.tikhub.io"

	ProviderJustOneAPI = "justoneapi_rest"
	ProviderTikHub     = "tikhub_rest"

	AuthPlacementBearer = "bearer"
	AuthPlacementQuery  = "query"
	AuthPlacementForm   = "form"
)

// ProviderAdapter contains provider-specific authentication, request mapping,
// and response normalization behind a provider-neutral runtime contract.
type ProviderAdapter interface {
	Probe(ctx context.Context) (AccountState, error)
	SnapshotCatalog(ctx context.Context) (CatalogSnapshot, error)
	Execute(ctx context.Context, operation ProviderOperation, request CanonicalRequest) (AttemptResult, error)
}

type AdapterConfig struct {
	Provider          string
	BaseURL           string
	Secret            string
	Timeout           time.Duration
	MaxResponseBytes  int64
	HTTPClient        *http.Client
	AllowLoopbackHTTP bool
}

type AccountState struct {
	Plan                string
	BalanceAmountMicros int64
	BalanceCurrency     string
}

type CatalogSnapshot struct {
	Provider   string
	Version    string
	SchemaHash string
	Operations []ProviderOperation
}

type ProviderOperation struct {
	OperationKey       string
	ContractVersion    string
	Platform           string
	OperationID        string
	Method             string
	Path               string
	AuthPlacement      string
	MappingKey         string
	MappingVersion     string
	ParameterMap       map[string]string
	FixedParams        map[string]any
	InputSchema        map[string]any
	OutputSchema       map[string]any
	CostAmountMicros   int64
	CostCurrency       string
	ContractEquivalent bool
	BillingReady       bool
}

type CanonicalRequest struct {
	OperationKey string
	Platform     string
	Params       map[string]any
	PageToken    string
}

type AttemptResult struct {
	Data              any
	NextPageToken     string
	HasMore           *bool
	ProviderRequestID string
	HTTPStatus        int
	BusinessCode      string
	Dispatched        bool
	Billable          *bool
	CostAmountMicros  int64
	CostCurrency      string
	RetryAfter        time.Duration
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

func sanitizeProviderRequestID(value, secret string) string {
	value = strings.TrimSpace(value)
	secret = strings.TrimSpace(secret)
	if value == "" || (secret != "" && strings.Contains(value, secret)) {
		return ""
	}
	if len(value) <= 128 {
		return value
	}
	for len(value) > 128 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func searchProviderReservedParam(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "token", "authorization", "headers", "cookie", "cookies", "proxy", "path", "method", "provider_params", "router", "raw":
		return true
	default:
		return false
	}
}
