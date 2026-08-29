package vsearch

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type Principal struct {
	UserID       int
	EnterpriseID int
	AgentKeyID   int
	Scopes       []string
}

type PublicCapability struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Category           string   `json:"category"`
	Description        string   `json:"description"`
	SchemaStatus       string   `json:"schema_status"`
	Status             string   `json:"status"`
	Enabled            bool     `json:"enabled"`
	InterfaceCount     int64    `json:"interface_count"`
	HealthyRouteCount  int64    `json:"healthy_route_count,omitempty"`
	CostLabel          string   `json:"cost_label"`
	Price              float64  `json:"price"`
	PriceMicros        int64    `json:"price_micros,omitempty"`
	UpstreamCost       *float64 `json:"upstream_cost,omitempty"`
	UpstreamCostMicros int64    `json:"upstream_cost_micros,omitempty"`
	RecentLatencyMs    int64    `json:"recent_latency_ms"`
	LastSyncedAt       int64    `json:"last_synced_at"`
}

type PublicTool struct {
	ServiceID   string `json:"serviceId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Available   bool   `json:"available"`
}

type Discovery struct {
	Query string       `json:"query"`
	Tools []PublicTool `json:"tools"`
}

type ToolContractService struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Description    string `json:"description"`
	InterfaceCount int64  `json:"interfaceCount"`
	PriceMicros    int64  `json:"priceMicros"`
	Enabled        bool   `json:"enabled"`
}

type ToolContractTool struct {
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	SchemaStatus string         `json:"schemaStatus"`
}

type ToolContract struct {
	Service ToolContractService `json:"service"`
	Tool    ToolContractTool    `json:"tool"`
	Cost    int64               `json:"costMicros"`
}

type ExecuteCommand struct {
	ServiceID      string
	Params         map[string]any
	IdempotencyKey string
}

type ExecutionResult struct {
	RequestID string `json:"requestId"`
	Data      any    `json:"data"`
}

type PublicError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *PublicError) Error() string { return e.Message }

type executionDispatchedError struct {
	cause error
}

func (e *executionDispatchedError) Error() string { return e.cause.Error() }
func (e *executionDispatchedError) Unwrap() error { return e.cause }

func markExecutionDispatched(err error) error {
	if err == nil {
		return nil
	}
	return &executionDispatchedError{cause: err}
}

type ConnectorFactory func(account *model.SearchUpstreamAccount, secret string) (UpstreamConnector, error)

type ExecutionRuntime struct {
	connectorFactory ConnectorFactory
	chargeFactory    executionChargeFactory
}

func NewExecutionRuntime(factory ConnectorFactory) *ExecutionRuntime {
	if factory == nil {
		factory = defaultConnectorFactory
	}
	return &ExecutionRuntime{connectorFactory: factory, chargeFactory: func(ctx context.Context, principal Principal, requestID string, capability *model.SearchCapability) (executionCharge, error) {
		return preConsumeExecutionCharge(ctx, principal, requestID, capability)
	}}
}

func defaultConnectorFactory(account *model.SearchUpstreamAccount, secret string) (UpstreamConnector, error) {
	return NewAgentKeyConnector(AgentKeyConnectorConfig{
		BaseURL:           account.BaseURL,
		Secret:            secret,
		AllowLoopbackHTTP: strings.EqualFold(strings.TrimSpace(os.Getenv("VSEARCH_ALLOW_LOOPBACK_HTTP")), "true"),
	})
}

func (runtime *ExecutionRuntime) ListCatalog(ctx context.Context, principal Principal, includeDisabled bool) ([]PublicCapability, error) {
	catalog, err := loadCatalogSnapshot(principal, includeDisabled)
	if err != nil {
		return nil, err
	}
	result := make([]PublicCapability, 0, len(catalog))
	for _, entry := range catalog {
		result = append(result, entry.public)
	}
	return result, nil
}

func (runtime *ExecutionRuntime) Discover(ctx context.Context, principal Principal, query string) (Discovery, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Discovery{}, &PublicError{Code: "DISCOVERY_QUERY_REQUIRED", Message: "请输入完整的搜索需求。", HTTPStatus: http.StatusBadRequest}
	}
	catalog, err := runtime.ListCatalog(ctx, principal, false)
	if err != nil {
		return Discovery{}, err
	}
	terms := strings.Fields(strings.ToLower(query))
	type rankedTool struct {
		tool  PublicTool
		score int
	}
	ranked := make([]rankedTool, 0, len(catalog))
	for _, capability := range catalog {
		text := strings.ToLower(capability.Name + " " + capability.Category + " " + capability.Description)
		score := 0
		for _, term := range terms {
			if strings.Contains(text, term) {
				score++
			}
		}
		if score == 0 && len(terms) > 0 {
			continue
		}
		ranked = append(ranked, rankedTool{score: score, tool: PublicTool{
			ServiceID: capability.ID, Title: capability.Name, Description: capability.Description,
			Category: capability.Category, Available: capability.Enabled,
		}})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	tools := make([]PublicTool, 0, len(ranked))
	for _, item := range ranked {
		tools = append(tools, item.tool)
		if len(tools) == 20 {
			break
		}
	}
	return Discovery{Query: query, Tools: tools}, nil
}

func (runtime *ExecutionRuntime) Describe(ctx context.Context, principal Principal, publicID string) (ToolContract, error) {
	capability, err := runtime.authorizedCapability(principal, publicID)
	if err != nil {
		return ToolContract{}, err
	}
	bindings, err := listHealthySearchBindingsForCapability(capability)
	if err != nil {
		return ToolContract{}, err
	}
	if len(bindings) == 0 {
		return ToolContract{}, &PublicError{Code: "CAPABILITY_UNAVAILABLE", Message: "该能力暂不可用。", HTTPStatus: http.StatusServiceUnavailable}
	}
	schema, schemaStatus := parseCapabilitySchema(capability.InputSchema)
	if schemaStatus != "available" {
		return ToolContract{}, &PublicError{Code: "CAPABILITY_SCHEMA_UNAVAILABLE", Message: "该能力的参数定义尚未同步。", HTTPStatus: http.StatusServiceUnavailable}
	}
	if capability.PriceMicros < healthySearchBindingsPriceFloor(bindings) {
		return ToolContract{}, &PublicError{
			Code: "CAPABILITY_PRICING_STALE", Message: "该能力价格正在同步，请稍后重试。", HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	return ToolContract{
		Service: ToolContractService{
			ID: capability.PublicID, Name: capability.Name, Category: capability.Category,
			Description: capability.Description, InterfaceCount: 1,
			PriceMicros: capability.PriceMicros, Enabled: true,
		},
		Tool: ToolContractTool{Title: capability.Name, Description: capability.Description, InputSchema: schema, SchemaStatus: schemaStatus},
		Cost: capability.PriceMicros,
	}, nil
}

func (runtime *ExecutionRuntime) executeOnce(ctx context.Context, principal Principal, command ExecuteCommand) (ExecutionResult, error) {
	startedAt := time.Now()
	requestID := common.NewRequestId()
	inputBytes := serializedSize(command.Params)
	capability, err := runtime.authorizedCapability(principal, command.ServiceID)
	if err != nil {
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, nil, nil, nil, command.ServiceID, inputBytes, startedAt, nil, err)
	}
	if command.Params == nil {
		command.Params = map[string]any{}
		inputBytes = serializedSize(command.Params)
	}
	if validationErr := validateCapabilityParams(command.Params, capability.InputSchema); validationErr != nil {
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, nil, nil, command.ServiceID, inputBytes, startedAt, nil, validationErr)
	}
	binding, account, priceFloor, err := selectExecutionTarget(capability)
	if err != nil {
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, nil, nil, command.ServiceID, inputBytes, startedAt, nil, err)
	}
	if capability.PriceMicros < priceFloor {
		pricingErr := &PublicError{
			Code: "CAPABILITY_PRICING_STALE", Message: "该能力价格正在同步，请稍后重试。", HTTPStatus: http.StatusServiceUnavailable,
		}
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, nil, command.ServiceID, inputBytes, startedAt, nil, pricingErr)
	}
	secret, err := DecryptUpstreamSecret(EncryptedSecret{
		Ciphertext: account.SecretCiphertext, Nonce: account.SecretNonce, Version: account.SecretVersion,
	})
	if err != nil {
		configErr := &PublicError{Code: "VSEARCH_NOT_CONFIGURED", Message: "vSearch 上游密钥尚未正确配置。", HTTPStatus: http.StatusServiceUnavailable}
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, nil, command.ServiceID, inputBytes, startedAt, nil, configErr)
	}
	connector, err := runtime.connectorFactory(account, secret)
	if err != nil {
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, nil, command.ServiceID, inputBytes, startedAt, nil, err)
	}
	usageEvent := &model.SearchUsageEvent{
		RequestID: requestID, UserID: principal.UserID, EnterpriseID: principal.EnterpriseID,
		AgentKeyID: principal.AgentKeyID, UpstreamAccountID: account.Id, CapabilityID: capability.Id,
		ServiceID: capability.PublicID, ServiceName: capability.Name, Action: model.SearchUsageActionExecute,
		Status: model.SearchUsageStatusPending, HTTPStatus: 0, InputBytes: inputBytes,
		PlannedUpstreamCostMicros: binding.UpstreamCostMicros, PlannedChargeMicros: capability.PriceMicros,
		ExecutionPhase: model.SearchUsagePhasePrepared, BillingState: model.SearchUsageBillingReservePending,
	}
	if err := model.CreateSearchUsageEvent(usageEvent); err != nil {
		auditErr := &PublicError{Code: "VSEARCH_AUDIT_UNAVAILABLE", Message: "vSearch 审计服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable}
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, nil, command.ServiceID, inputBytes, startedAt, nil, auditErr)
	}
	charge, err := runtime.chargeFactory(ctx, principal, requestID, capability)
	if err != nil {
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, nil, command.ServiceID, inputBytes, startedAt, usageEvent, err)
	}
	if err := model.MarkSearchUsageReservation(usageEvent, charge.billingSource(), charge.reservedQuota()); err != nil {
		auditErr := &PublicError{Code: "VSEARCH_AUDIT_UNAVAILABLE", Message: "vSearch 审计服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable}
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, charge, command.ServiceID, inputBytes, startedAt, usageEvent, auditErr)
	}
	usageEvent.ExecutionPhase = model.SearchUsagePhaseDispatching
	if err := model.UpdateSearchUsageEventProgress(usageEvent); err != nil {
		auditErr := &PublicError{Code: "VSEARCH_AUDIT_UNAVAILABLE", Message: "vSearch 审计服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable}
		return ExecutionResult{}, failSearchExecution(ctx, requestID, principal, capability, account, charge, command.ServiceID, inputBytes, startedAt, usageEvent, auditErr)
	}
	result, executeErr := connector.ExecuteTool(ctx, binding.ToolName, command.Params)
	if executeErr != nil {
		var dispatchedErr *executionDispatchedError
		dispatched := errors.As(executeErr, &dispatchedErr)
		if !dispatched {
			usageEvent.ExecutionPhase = model.SearchUsagePhasePrepared
		}
		err := failSearchExecution(ctx, requestID, principal, capability, account, charge, command.ServiceID, inputBytes, startedAt, usageEvent, executeErr)
		if dispatched {
			return ExecutionResult{}, markExecutionDispatched(err)
		}
		return ExecutionResult{}, err
	}
	usageEvent.ExecutionPhase = model.SearchUsagePhaseCompleted
	usageEvent.UpstreamCostMicros = binding.UpstreamCostMicros
	publicResult := sanitizePublicValueWithForbidden(result, []string{
		binding.ToolName, account.Name, account.SecretPrefix, account.BaseURL, secret,
	})
	latency := time.Since(startedAt).Milliseconds()
	usageEvent.HTTPStatus = http.StatusOK
	usageEvent.LatencyMs = latency
	usageEvent.OutputBytes = serializedSize(publicResult)
	usageEvent.UpstreamCostMicros = binding.UpstreamCostMicros
	usageEvent.ChargeMicros = capability.PriceMicros
	if err := model.MarkSearchUsageCommitPending(usageEvent); err != nil {
		auditErr := &PublicError{Code: "VSEARCH_AUDIT_UNAVAILABLE", Message: "vSearch 审计服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable}
		return ExecutionResult{}, markExecutionDispatched(failSearchExecution(ctx, requestID, principal, capability, account, charge, command.ServiceID, inputBytes, startedAt, usageEvent, markExecutionDispatched(auditErr)))
	}
	if err := charge.commit(); err != nil {
		err := failSearchExecution(ctx, requestID, principal, capability, account, charge, command.ServiceID, inputBytes, startedAt, usageEvent,
			markExecutionDispatched(&PublicError{Code: "VSEARCH_BILLING_UNAVAILABLE", Message: "vSearch 计费服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable}))
		return ExecutionResult{}, markExecutionDispatched(err)
	}
	committed, err := model.CommitSearchUsageEvent(usageEvent.Id)
	if err != nil {
		common.SysLog("failed to commit vSearch usage: " + err.Error())
		return ExecutionResult{}, markExecutionDispatched(&PublicError{
			Code: "VSEARCH_EXECUTION_INDETERMINATE", Message: "vSearch 请求已执行，计费状态正在恢复，请勿自动重试。", HTTPStatus: http.StatusInternalServerError,
		})
	}
	if err := model.EnsureSearchUsageConsumeLog(committed); err != nil {
		common.SysLog("failed to materialize vSearch consume log: " + err.Error())
	}
	return ExecutionResult{RequestID: requestID, Data: publicResult}, nil
}

func failSearchExecution(
	ctx context.Context,
	requestID string,
	principal Principal,
	capability *model.SearchCapability,
	account *model.SearchUpstreamAccount,
	charge executionCharge,
	requestedServiceID string,
	inputBytes int64,
	startedAt time.Time,
	usageEvent *model.SearchUsageEvent,
	err error,
) error {
	safeErr := safeRuntimeError(err)
	var dispatchedErr *executionDispatchedError
	indeterminate := errors.As(err, &dispatchedErr)
	preventRetry := false
	residualChargeMicros := int64(0)
	if charge != nil {
		if usageEvent != nil {
			if markErr := model.MarkSearchUsageRefundPending(usageEvent); markErr != nil {
				common.SysLog("failed to persist vSearch refund intent: " + markErr.Error())
			}
		}
		if refundErr := charge.refund(ctx); refundErr != nil {
			preventRetry = true
			residualChargeMicros = charge.potentialChargeMicros()
			if usageEvent != nil {
				usageEvent.BillingState = model.SearchUsageBillingRefundFailed
			}
			common.SysLog("failed to compensate vSearch charge: " + refundErr.Error())
			safeErr = &PublicError{
				Code: "VSEARCH_BILLING_COMPENSATION_FAILED", Message: "vSearch 计费补偿失败，请联系管理员。", HTTPStatus: http.StatusInternalServerError,
			}
		} else if usageEvent != nil {
			usageEvent.BillingState = model.SearchUsageBillingRefunded
		}
	}
	if principal.UserID <= 0 || principal.AgentKeyID <= 0 {
		return safeErr
	}

	event := usageEvent
	if event == nil {
		event = &model.SearchUsageEvent{
			RequestID: requestID, UserID: principal.UserID, EnterpriseID: principal.EnterpriseID,
			AgentKeyID: principal.AgentKeyID, ServiceID: strings.TrimSpace(requestedServiceID),
			Action: model.SearchUsageActionExecute,
		}
	}
	event.Status = model.SearchUsageStatusFailed
	if indeterminate {
		event.Status = model.SearchUsageStatusIndeterminate
	}
	event.HTTPStatus = safeErr.HTTPStatus
	event.LatencyMs = time.Since(startedAt).Milliseconds()
	event.InputBytes = inputBytes
	event.ChargeMicros = residualChargeMicros
	event.ErrorCode = safeErr.Code
	if indeterminate {
		event.ErrorCode = "VSEARCH_EXECUTION_INDETERMINATE"
	}
	event.SanitizedErrorMessage = safeErr.Message
	if capability != nil {
		event.CapabilityID = capability.Id
		event.ServiceID = capability.PublicID
		event.ServiceName = capability.Name
	}
	if account != nil {
		event.UpstreamAccountID = account.Id
	}
	var logErr error
	if usageEvent == nil {
		logErr = model.CreateSearchUsageEvent(event)
	} else {
		logErr = model.FinalizeSearchUsageEvent(event)
	}
	if logErr != nil {
		common.SysLog("failed to record vSearch failure usage: " + logErr.Error())
	}
	if preventRetry {
		return markExecutionDispatched(safeErr)
	}
	return safeErr
}

func (runtime *ExecutionRuntime) authorizedCapability(principal Principal, publicID string) (*model.SearchCapability, error) {
	capability, err := model.GetSearchCapabilityByPublicID(strings.TrimSpace(publicID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &PublicError{Code: "CAPABILITY_NOT_FOUND", Message: "该能力不存在或未开放。", HTTPStatus: http.StatusNotFound}
	}
	if err != nil {
		return nil, err
	}
	if capability.Status != model.SearchCapabilityStatusEnabled || !principalAllowsCategory(principal, capability.Category) {
		return nil, &PublicError{Code: "CAPABILITY_NOT_ALLOWED", Message: "该能力未开放给当前密钥。", HTTPStatus: http.StatusForbidden}
	}
	granted, err := model.IsSearchCapabilityGranted(capability.Id, principal.EnterpriseID, principal.UserID)
	if err != nil {
		return nil, err
	}
	if !granted {
		return nil, &PublicError{Code: "CAPABILITY_NOT_ALLOWED", Message: "该能力未开放给当前企业。", HTTPStatus: http.StatusForbidden}
	}
	return capability, nil
}

func selectExecutionTarget(capability *model.SearchCapability) (*model.SearchCapabilityBinding, *model.SearchUpstreamAccount, int64, error) {
	bindings, err := listHealthySearchBindingsForCapability(capability)
	if err != nil {
		return nil, nil, 0, err
	}
	priceFloor := healthySearchBindingsPriceFloor(bindings)
	type candidate struct {
		binding *model.SearchCapabilityBinding
		account *model.SearchUpstreamAccount
		weight  int
	}
	candidates := make([]candidate, 0, len(bindings))
	selectedPriority := 0
	for _, healthy := range bindings {
		binding := healthy.binding
		account := healthy.account
		if len(candidates) == 0 {
			selectedPriority = binding.Priority
		}
		if binding.Priority != selectedPriority {
			break
		}
		weight := account.Weight
		if weight < 1 {
			weight = 1
		}
		candidates = append(candidates, candidate{binding: binding, account: account, weight: weight})
	}
	if len(candidates) == 0 {
		return nil, nil, 0, &PublicError{Code: "CAPABILITY_UNAVAILABLE", Message: "该能力当前没有可用上游账号。", HTTPStatus: http.StatusServiceUnavailable}
	}
	totalWeight := 0
	for _, item := range candidates {
		totalWeight += item.weight
	}
	selectedWeight, randomErr := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(totalWeight)))
	if randomErr != nil {
		return candidates[0].binding, candidates[0].account, priceFloor, nil
	}
	position := int(selectedWeight.Int64())
	for _, item := range candidates {
		if position < item.weight {
			return item.binding, item.account, priceFloor, nil
		}
		position -= item.weight
	}
	return candidates[len(candidates)-1].binding, candidates[len(candidates)-1].account, priceFloor, nil
}

type healthySearchBinding struct {
	binding *model.SearchCapabilityBinding
	account *model.SearchUpstreamAccount
}

func healthySearchBindingsPriceFloor(bindings []healthySearchBinding) int64 {
	priceFloor := int64(0)
	for _, healthy := range bindings {
		if healthy.binding.UpstreamCostMicros > priceFloor {
			priceFloor = healthy.binding.UpstreamCostMicros
		}
	}
	return priceFloor
}

func listHealthySearchBindings(capabilityID int) ([]healthySearchBinding, error) {
	capability, err := model.GetSearchCapabilityByID(capabilityID)
	if err != nil {
		return nil, err
	}
	return listHealthySearchBindingsForCapability(capability)
}

func listHealthySearchBindingsForCapability(capability *model.SearchCapability) ([]healthySearchBinding, error) {
	bindings, err := model.ListSearchCapabilityBindings(capability.Id, true)
	if err != nil {
		return nil, err
	}
	healthy := make([]healthySearchBinding, 0, len(bindings))
	for _, binding := range bindings {
		if !searchBindingMatchesCapabilitySchema(binding, capability) {
			continue
		}
		account, accountErr := model.GetSearchUpstreamAccountByID(binding.UpstreamAccountID)
		if errors.Is(accountErr, gorm.ErrRecordNotFound) {
			continue
		}
		if accountErr != nil {
			return nil, accountErr
		}
		pool, poolErr := model.GetSearchUpstreamPoolByID(account.PoolID)
		if errors.Is(poolErr, gorm.ErrRecordNotFound) {
			continue
		}
		if poolErr != nil {
			return nil, poolErr
		}
		if account.Status == model.SearchUpstreamAccountStatusHealthy && pool.Status == model.SearchUpstreamPoolStatusEnabled {
			healthy = append(healthy, healthySearchBinding{binding: binding, account: account})
		}
	}
	return healthy, nil
}

func principalAllowsCategory(principal Principal, category string) bool {
	if len(principal.Scopes) == 0 {
		return true
	}
	required := categoryScope(category)
	for _, scope := range principal.Scopes {
		if strings.EqualFold(strings.TrimSpace(scope), required) {
			return true
		}
	}
	return false
}

func categoryScope(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	switch {
	case strings.Contains(category, "抓取"), strings.Contains(category, "extract"), strings.Contains(category, "crawl"):
		return "extract"
	case strings.Contains(category, "社交"), strings.Contains(category, "social"):
		return "social"
	case strings.Contains(category, "金融"), strings.Contains(category, "finance"), strings.Contains(category, "电商"), strings.Contains(category, "commerce"), strings.Contains(category, "crypto"):
		return "finance"
	case strings.Contains(category, "新闻"), strings.Contains(category, "news"), strings.Contains(category, "research"):
		return "news"
	case strings.Contains(category, "企业"), strings.Contains(category, "company"), strings.Contains(category, "industry"):
		return "company"
	case strings.Contains(category, "旅行"), strings.Contains(category, "travel"), strings.Contains(category, "天气"), strings.Contains(category, "weather"):
		return "travel"
	case strings.Contains(category, "招聘"), strings.Contains(category, "job"):
		return "jobs"
	default:
		return "web-search"
	}
}

func parseCapabilitySchema(raw string) (map[string]any, string) {
	if strings.TrimSpace(raw) == "" {
		return nil, "unavailable"
	}
	var schema map[string]any
	if err := common.UnmarshalJsonStr(raw, &schema); err != nil {
		return nil, "unavailable"
	}
	return schema, "available"
}

func validateCapabilityParams(params map[string]any, rawSchema string) error {
	schema, status := parseCapabilitySchema(rawSchema)
	if status != "available" {
		return &PublicError{Code: "CAPABILITY_SCHEMA_UNAVAILABLE", Message: "该能力的参数定义尚未同步。", HTTPStatus: http.StatusServiceUnavailable}
	}
	if err := validateSchemaValue(params, schema, "$"); err != nil {
		return &PublicError{Code: "INVALID_TOOL_PARAMS", Message: "参数无效：" + err.Error(), HTTPStatus: http.StatusBadRequest}
	}
	return nil
}

func validateSchemaValue(value any, schema map[string]any, path string) error {
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s 必须是对象", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, item := range required {
				name, _ := item.(string)
				if _, exists := object[name]; name != "" && !exists {
					return fmt.Errorf("缺少 %s.%s", path, name)
				}
			}
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			for name := range object {
				if _, exists := properties[name]; !exists {
					return fmt.Errorf("%s.%s 不是支持的参数", path, name)
				}
			}
		}
		for name, item := range object {
			child, ok := properties[name].(map[string]any)
			if ok {
				if err := validateSchemaValue(item, child, path+"."+name); err != nil {
					return err
				}
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s 必须是字符串", path)
		}
	case "integer":
		number, ok := runtimeNumber(value)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("%s 必须是整数", path)
		}
	case "number":
		if _, ok := runtimeNumber(value); !ok {
			return fmt.Errorf("%s 必须是数字", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s 必须是布尔值", path)
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s 必须是数组", path)
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for index, item := range items {
				if err := validateSchemaValue(item, itemSchema, fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
			}
		}
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		matched := false
		for _, option := range enum {
			if schemaValuesEqual(value, option) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s 不是支持的选项", path)
		}
	}
	if number, ok := runtimeNumber(value); ok {
		if minimum, valid := schemaNumber(schema["minimum"]); valid && number < minimum {
			return fmt.Errorf("%s 不能小于 %v", path, minimum)
		}
		if maximum, valid := schemaNumber(schema["maximum"]); valid && number > maximum {
			return fmt.Errorf("%s 不能大于 %v", path, maximum)
		}
	}
	return nil
}

func schemaValuesEqual(left, right any) bool {
	leftNumber, leftIsNumber := runtimeNumber(left)
	rightNumber, rightIsNumber := runtimeNumber(right)
	if leftIsNumber || rightIsNumber {
		return leftIsNumber && rightIsNumber && leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func runtimeNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		number := float64(typed)
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func safeRuntimeError(err error) *PublicError {
	var publicErr *PublicError
	if errors.As(err, &publicErr) {
		return publicErr
	}
	var connectorErr *ConnectorError
	if errors.As(err, &connectorErr) {
		return &PublicError{Code: connectorErr.Code, Message: connectorErr.Message, HTTPStatus: connectorErr.HTTPStatus}
	}
	return &PublicError{Code: "VSEARCH_INTERNAL_ERROR", Message: "vSearch 服务暂不可用，请稍后重试。", HTTPStatus: http.StatusInternalServerError}
}

func formatMicros(value int64) string {
	if value <= 0 {
		return "未计费"
	}
	return fmt.Sprintf("¥%.4f / 次", float64(value)/1_000_000)
}

func serializedSize(value any) int64 {
	data, err := common.Marshal(value)
	if err != nil {
		return 0
	}
	return int64(len(data))
}

func sanitizePublicValue(value any) any {
	return sanitizePublicValueWithForbidden(value, nil)
}

func sanitizePublicValueWithForbidden(value any, forbidden []string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "agentkey") {
				continue
			}
			blockedKey := false
			for _, secret := range forbidden {
				secret = strings.TrimSpace(secret)
				if len(secret) >= 3 && strings.Contains(lowerKey, strings.ToLower(secret)) {
					blockedKey = true
					break
				}
			}
			if blockedKey {
				continue
			}
			normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			switch normalized {
			case "account", "accountid", "upstreamaccount", "upstreamaccountid",
				"apikey", "key", "keyprefix", "secret", "clientsecret", "token", "accesstoken", "authorization",
				"nonce", "ciphertext", "provider", "providerid", "toolname", "upstreamtool", "upstreamtoolname",
				"cost", "costmicros", "upstreamcost", "upstreamcostmicros":
				continue
			}
			result[key] = sanitizePublicValueWithForbidden(item, forbidden)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = sanitizePublicValueWithForbidden(item, forbidden)
		}
		return result
	case string:
		text := strings.ReplaceAll(typed, "https://api.agentkey.app", "vSearch upstream")
		text = strings.ReplaceAll(text, "AgentKey", "vSearch upstream")
		text = strings.ReplaceAll(text, "agentkey", "vSearch upstream")
		for _, secret := range forbidden {
			secret = strings.TrimSpace(secret)
			if len(secret) >= 3 {
				text = strings.ReplaceAll(text, secret, "vSearch upstream")
			}
		}
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var nested any
			if err := common.UnmarshalJsonStr(trimmed, &nested); err == nil {
				if sanitized, marshalErr := common.Marshal(sanitizePublicValueWithForbidden(nested, forbidden)); marshalErr == nil {
					return string(sanitized)
				}
			}
		}
		return text
	default:
		return value
	}
}
