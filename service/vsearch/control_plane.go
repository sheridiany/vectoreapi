package vsearch

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const maxSearchMoneyMicros = int64(9_000_000_000_000_000)

var fullCatalogQueries = []string{
	"web search API", "web page extraction and crawling", "social media public data",
	"news and research", "financial market data", "ecommerce product data",
	"company data", "weather and travel", "job search",
}

type AccountCommand struct {
	ID       int
	Name     string
	BaseURL  string
	Secret   string
	PoolID   int
	Weight   int
	Priority int
	Status   string
}

type AccountView struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	BaseURL       string  `json:"base_url"`
	KeyPrefix     string  `json:"key_prefix"`
	Plan          string  `json:"plan"`
	Balance       float64 `json:"balance"`
	BalanceMicros int64   `json:"balance_micros"`
	Weight        int     `json:"weight"`
	Priority      int     `json:"priority"`
	Pool          string  `json:"pool"`
	PoolID        int     `json:"pool_id"`
	Status        string  `json:"status"`
	LastCheck     int64   `json:"last_check"`
	LastError     string  `json:"last_error,omitempty"`
}

type SyncCommand struct {
	Queries []string
	Prefix  string
}

type SyncResult struct {
	Synced     int                `json:"synced"`
	Discovered int                `json:"discovered"`
	Accounts   int                `json:"accounts"`
	Failures   []string           `json:"failures"`
	Services   []PublicCapability `json:"services"`
}

type CapabilityCommand struct {
	ID          int
	Enabled     bool
	PriceMicros int64
}

type ControlPlane struct {
	connectorFactory ConnectorFactory
	runtime          *ExecutionRuntime
}

func NewControlPlane(factory ConnectorFactory) *ControlPlane {
	runtime := NewExecutionRuntime(factory)
	return &ControlPlane{connectorFactory: runtime.connectorFactory, runtime: runtime}
}

func (control *ControlPlane) ListAccounts(ctx context.Context) ([]AccountView, error) {
	accounts, err := model.ListSearchUpstreamAccounts()
	if err != nil {
		return nil, err
	}
	pools, err := model.ListSearchUpstreamPools()
	if err != nil {
		return nil, err
	}
	poolNames := make(map[int]string, len(pools))
	for _, pool := range pools {
		poolNames[pool.Id] = pool.Name
	}
	views := make([]AccountView, 0, len(accounts))
	for _, account := range accounts {
		views = append(views, toAccountView(account, poolNames[account.PoolID]))
	}
	return views, nil
}

func (control *ControlPlane) SaveAccount(ctx context.Context, command AccountCommand) (AccountView, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.BaseURL = strings.TrimSpace(command.BaseURL)
	command.Secret = strings.TrimSpace(command.Secret)
	if command.BaseURL == "" {
		command.BaseURL = DefaultAgentKeyMCPURL
	}
	endpoint, err := validateAgentKeyURL(
		command.BaseURL,
		model.SearchUpstreamLoopbackHTTPEnabled(),
	)
	if err != nil {
		return AccountView{}, safeRuntimeError(err)
	}
	command.BaseURL = endpoint.String()
	pool, err := control.resolvePool(command.PoolID)
	if err != nil {
		return AccountView{}, err
	}
	status, err := parseAccountStatus(command.Status)
	if err != nil {
		return AccountView{}, err
	}
	if command.Weight == 0 {
		command.Weight = 1
	}

	var account *model.SearchUpstreamAccount
	if command.ID > 0 {
		account, err = model.GetSearchUpstreamAccountByID(command.ID)
		if err != nil {
			return AccountView{}, err
		}
	} else {
		account = &model.SearchUpstreamAccount{Provider: model.SearchUpstreamProviderAgentKeyMCP}
		if command.Secret == "" {
			return AccountView{}, &PublicError{Code: "UPSTREAM_SECRET_REQUIRED", Message: "请输入 AgentKey 密钥。", HTTPStatus: http.StatusBadRequest}
		}
	}
	if command.Secret != "" {
		encrypted, encryptErr := EncryptUpstreamSecret(command.Secret)
		if encryptErr != nil {
			return AccountView{}, &PublicError{Code: "VSEARCH_NOT_CONFIGURED", Message: "vSearch 本地加密密钥尚未配置。", HTTPStatus: http.StatusServiceUnavailable}
		}
		account.SecretCiphertext = encrypted.Ciphertext
		account.SecretNonce = encrypted.Nonce
		account.SecretVersion = encrypted.Version
		account.SecretPrefix = UpstreamSecretPrefix(command.Secret)
	}
	account.PoolID = pool.Id
	account.Name = command.Name
	account.BaseURL = command.BaseURL
	account.Weight = command.Weight
	account.Priority = command.Priority
	account.Status = status
	if account.Id == 0 {
		err = model.CreateSearchUpstreamAccount(account)
	} else {
		err = model.UpdateSearchUpstreamAccount(account)
	}
	if err != nil {
		return AccountView{}, err
	}
	return toAccountView(account, pool.Name), nil
}

func (control *ControlPlane) DeleteAccount(ctx context.Context, id int) error {
	return model.DeleteSearchUpstreamAccount(id)
}

func (control *ControlPlane) ProbeAccount(ctx context.Context, id int) (AccountView, error) {
	account, err := model.GetSearchUpstreamAccountByID(id)
	if err != nil {
		return AccountView{}, err
	}
	pool, err := model.GetSearchUpstreamPoolByID(account.PoolID)
	if err != nil {
		return AccountView{}, err
	}
	secret, err := DecryptUpstreamSecret(EncryptedSecret{Ciphertext: account.SecretCiphertext, Nonce: account.SecretNonce, Version: account.SecretVersion})
	if err != nil {
		return AccountView{}, &PublicError{Code: "VSEARCH_NOT_CONFIGURED", Message: "vSearch 上游密钥无法解密。", HTTPStatus: http.StatusServiceUnavailable}
	}
	connector, err := control.connectorFactory(account, secret)
	if err != nil {
		return AccountView{}, safeRuntimeError(err)
	}
	payload, probeErr := connector.Account(ctx)
	if probeErr != nil {
		failureCount := account.FailureCount + 1
		status := model.SearchUpstreamAccountStatusWarning
		if failureCount >= 3 {
			status = model.SearchUpstreamAccountStatusStandby
		}
		safeErr := safeRuntimeError(probeErr)
		_ = model.UpdateSearchUpstreamAccountHealth(account.Id, status, account.BalanceMicros, failureCount, safeErr.Code, safeErr.Message)
		return AccountView{}, safeErr
	}
	plan, balanceMicros := extractAccountMetadata(payload, []string{secret, account.SecretPrefix, account.BaseURL, account.Name})
	if err := model.UpdateSearchUpstreamAccountHealth(account.Id, model.SearchUpstreamAccountStatusHealthy, balanceMicros, 0, "", ""); err != nil {
		return AccountView{}, err
	}
	account.Plan = plan
	account.BalanceMicros = balanceMicros
	account.Status = model.SearchUpstreamAccountStatusHealthy
	account.FailureCount = 0
	account.LastCheckedAt = common.GetTimestamp()
	account.LastErrorCode = ""
	account.LastErrorMessage = ""
	if err := model.UpdateSearchUpstreamAccount(account); err != nil {
		return AccountView{}, err
	}
	return toAccountView(account, pool.Name), nil
}

func (control *ControlPlane) SyncCatalog(ctx context.Context, command SyncCommand) (SyncResult, error) {
	queries, err := normalizeSyncQueries(command.Queries)
	if err != nil {
		return SyncResult{}, err
	}
	accounts, err := model.ListSearchUpstreamAccounts()
	if err != nil {
		return SyncResult{}, err
	}
	pools, err := model.ListSearchUpstreamPools()
	if err != nil {
		return SyncResult{}, err
	}
	enabledPoolIDs := make(map[int]struct{}, len(pools))
	for _, pool := range pools {
		if pool.Status == model.SearchUpstreamPoolStatusEnabled {
			enabledPoolIDs[pool.Id] = struct{}{}
		}
	}
	healthy := make([]*model.SearchUpstreamAccount, 0, len(accounts))
	for _, account := range accounts {
		if _, enabled := enabledPoolIDs[account.PoolID]; account.Status == model.SearchUpstreamAccountStatusHealthy && enabled {
			healthy = append(healthy, account)
		}
	}
	if len(healthy) == 0 {
		return SyncResult{}, &PublicError{Code: "UPSTREAM_ACCOUNT_UNAVAILABLE", Message: "请先接入并通过健康检查的 AgentKey 账号。", HTTPStatus: http.StatusServiceUnavailable}
	}

	result := SyncResult{Failures: make([]string, 0)}
	for _, account := range healthy {
		accountSyncedAt := common.GetTimestamp()
		accountSyncComplete := true
		secret, decryptErr := DecryptUpstreamSecret(EncryptedSecret{Ciphertext: account.SecretCiphertext, Nonce: account.SecretNonce, Version: account.SecretVersion})
		if decryptErr != nil {
			result.Failures = append(result.Failures, account.Name+"：密钥无法解密")
			continue
		}
		connector, connectorErr := control.connectorFactory(account, secret)
		if connectorErr != nil {
			result.Failures = append(result.Failures, account.Name+"：连接器配置无效")
			continue
		}
		discovered := make(map[string]discoveredTool)
		for _, query := range queries {
			payload, findErr := connector.FindTools(ctx, query, command.Prefix)
			if findErr != nil {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：目录发现失败")
				continue
			}
			for _, tool := range flattenToolRecords(payload) {
				discovered[strings.ToLower(tool.Name)] = tool
				if len(discovered) >= 500 {
					accountSyncComplete = false
					break
				}
			}
		}
		if len(discovered) == 0 {
			continue
		}
		result.Accounts++
		result.Discovered += len(discovered)
		toolNames := make([]string, 0, len(discovered))
		for toolName := range discovered {
			toolNames = append(toolNames, toolName)
		}
		sort.Strings(toolNames)
		for _, normalizedName := range toolNames {
			tool := discovered[normalizedName]
			descriptionPayload, describeErr := connector.DescribeTool(ctx, tool.Name)
			if describeErr == nil {
				mergeToolDescription(&tool, descriptionPayload)
			} else {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：能力描述读取失败")
			}
			publicID, idErr := model.GenerateSearchCapabilityPublicID(tool.Name)
			if idErr != nil {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：能力标识生成失败")
				continue
			}
			forbidden := []string{secret, account.SecretPrefix, account.BaseURL, account.Name, tool.Name}
			schemaText := ""
			if tool.Schema != nil {
				safeSchema := sanitizePublicValueWithForbidden(tool.Schema, forbidden)
				if schemaData, marshalErr := common.Marshal(safeSchema); marshalErr == nil {
					schemaText = string(schemaData)
				} else {
					accountSyncComplete = false
					result.Failures = append(result.Failures, account.Name+"：能力结构保存失败")
				}
			}
			costMicros := numberToMicros(tool.Cost)
			publicName := sanitizePublicTextWithForbidden(publicToolName(tool), forbidden)
			publicDescription := sanitizePublicTextWithForbidden(publicToolDescription(tool), forbidden)
			publicCategory := sanitizePublicTextWithForbidden(classifyTool(tool), forbidden)
			capability := &model.SearchCapability{
				PublicID: publicID, Name: publicName, Category: publicCategory,
				Description: publicDescription, InputSchema: schemaText,
				Status: model.SearchCapabilityStatusDisabled, UpstreamCostMicros: costMicros,
				PriceMicros: costMicros, LastSyncedAt: accountSyncedAt,
			}
			if upsertErr := model.UpsertDiscoveredSearchCapability(capability); upsertErr != nil {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：能力保存失败")
				continue
			}
			persisted, getErr := model.GetSearchCapabilityByPublicID(publicID)
			if getErr != nil {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：能力读取失败")
				continue
			}
			binding := &model.SearchCapabilityBinding{
				CapabilityID: persisted.Id, UpstreamAccountID: account.Id, ToolName: tool.Name,
				InputSchema: schemaText, Status: model.SearchCapabilityBindingStatusEnabled,
				Weight: account.Weight, Priority: account.Priority,
				UpstreamCostMicros: costMicros, LastSyncedAt: accountSyncedAt,
			}
			if bindingErr := model.UpsertSearchCapabilityBinding(binding); bindingErr != nil {
				accountSyncComplete = false
				result.Failures = append(result.Failures, account.Name+"：能力绑定失败")
				continue
			}
			result.Synced++
		}
		if !accountSyncComplete {
			result.Failures = append(result.Failures, account.Name+"：目录不完整，已保留原有路由")
		}
	}
	if result.Synced == 0 {
		return result, &PublicError{Code: "CATALOG_SYNC_FAILED", Message: "没有同步到可用能力，请检查 AgentKey 账号。", HTTPStatus: http.StatusBadGateway}
	}
	result.Services, err = control.runtime.ListCatalog(ctx, Principal{}, true)
	return result, err
}

func (control *ControlPlane) ConfigureCapability(ctx context.Context, command CapabilityCommand) (PublicCapability, error) {
	if command.PriceMicros < 0 || command.PriceMicros > maxSearchMoneyMicros {
		return PublicCapability{}, &PublicError{Code: "CAPABILITY_PRICE_INVALID", Message: "能力售价无效。", HTTPStatus: http.StatusBadRequest}
	}
	status := model.SearchCapabilityStatusDisabled
	if command.Enabled {
		status = model.SearchCapabilityStatusEnabled
	}
	if err := model.ConfigureSearchCapability(command.ID, status, command.PriceMicros); err != nil {
		return PublicCapability{}, err
	}
	capability, err := model.GetSearchCapabilityByID(command.ID)
	if err != nil {
		return PublicCapability{}, err
	}
	catalog, err := control.runtime.ListCatalog(ctx, Principal{}, true)
	if err != nil {
		return PublicCapability{}, err
	}
	for _, item := range catalog {
		if item.ID == capability.PublicID {
			return item, nil
		}
	}
	return PublicCapability{}, gorm.ErrRecordNotFound
}

func (control *ControlPlane) resolvePool(id int) (*model.SearchUpstreamPool, error) {
	if id > 0 {
		return model.GetSearchUpstreamPoolByID(id)
	}
	pools, err := model.ListSearchUpstreamPools()
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Status == model.SearchUpstreamPoolStatusEnabled {
			return pool, nil
		}
	}
	pool := &model.SearchUpstreamPool{Name: "default", Strategy: model.SearchUpstreamPoolStrategyWeighted, Description: "vSearch default pool", Status: model.SearchUpstreamPoolStatusEnabled}
	if err := model.CreateSearchUpstreamPool(pool); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, err
		}
		pools, err = model.ListSearchUpstreamPools()
		if err != nil || len(pools) == 0 {
			return nil, err
		}
		return pools[0], nil
	}
	return pool, nil
}

func toAccountView(account *model.SearchUpstreamAccount, poolName string) AccountView {
	return AccountView{
		ID: account.Id, Name: account.Name, Provider: account.Provider, BaseURL: account.BaseURL,
		KeyPrefix: account.SecretPrefix, Plan: account.Plan,
		Balance: float64(account.BalanceMicros) / 1_000_000, BalanceMicros: account.BalanceMicros,
		Weight: account.Weight, Priority: account.Priority, Pool: poolName, PoolID: account.PoolID,
		Status: accountStatusName(account.Status), LastCheck: account.LastCheckedAt,
		LastError: account.LastErrorMessage,
	}
}

func parseAccountStatus(value string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "standby":
		return model.SearchUpstreamAccountStatusStandby, nil
	case "healthy", "active":
		return model.SearchUpstreamAccountStatusHealthy, nil
	case "warning":
		return model.SearchUpstreamAccountStatusWarning, nil
	case "paused", "disabled":
		return model.SearchUpstreamAccountStatusPaused, nil
	default:
		return 0, &PublicError{Code: "UPSTREAM_STATUS_INVALID", Message: "上游账号状态无效。", HTTPStatus: http.StatusBadRequest}
	}
}

func accountStatusName(status int) string {
	switch status {
	case model.SearchUpstreamAccountStatusHealthy:
		return "healthy"
	case model.SearchUpstreamAccountStatusWarning:
		return "warning"
	case model.SearchUpstreamAccountStatusPaused:
		return "paused"
	default:
		return "standby"
	}
}

func normalizeSyncQueries(values []string) ([]string, error) {
	if len(values) == 0 {
		return append([]string(nil), fullCatalogQueries...), nil
	}
	if len(values) > 20 {
		return nil, &PublicError{Code: "CATALOG_QUERY_INVALID", Message: "同步查询词不能超过 20 个。", HTTPStatus: http.StatusBadRequest}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > 120 {
			return nil, &PublicError{Code: "CATALOG_QUERY_INVALID", Message: "同步查询词无效。", HTTPStatus: http.StatusBadRequest}
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

type discoveredTool struct {
	Name        string
	Title       string
	Description string
	Category    string
	Cost        float64
	Schema      map[string]any
}

func flattenToolRecords(value any) []discoveredTool {
	result := make([]discoveredTool, 0)
	seen := map[string]struct{}{}
	visited := 0
	var visit func(any, int)
	visit = func(current any, depth int) {
		if depth > 12 || visited > 20_000 || current == nil {
			return
		}
		visited++
		switch typed := current.(type) {
		case string:
			var parsed any
			if common.UnmarshalJsonStr(typed, &parsed) == nil {
				visit(parsed, depth+1)
			}
		case []any:
			for _, item := range typed {
				visit(item, depth+1)
			}
		case map[string]any:
			name := firstString(typed, "name", "toolName", "tool_name")
			lowerName := strings.ToLower(strings.TrimSpace(name))
			if name != "" && lowerName != "find_tools" && lowerName != "describe_tool" && lowerName != "execute_tool" && lowerName != "agentkey_account" {
				if _, ok := seen[lowerName]; !ok {
					seen[lowerName] = struct{}{}
					result = append(result, discoveredTool{
						Name: name, Title: firstString(typed, "title", "displayName", "display_name"),
						Description: firstString(typed, "description", "summary"), Category: firstString(typed, "category"),
						Cost: firstNumber(typed, "cost", "price", "credits"), Schema: extractInputSchema(typed),
					})
				}
			}
			for _, nested := range typed {
				visit(nested, depth+1)
			}
		}
	}
	visit(value, 0)
	return result
}

func mergeToolDescription(tool *discoveredTool, payload any) {
	if tool == nil {
		return
	}
	var visit func(any, int)
	visit = func(current any, depth int) {
		if depth > 12 || current == nil {
			return
		}
		switch typed := current.(type) {
		case string:
			var parsed any
			if common.UnmarshalJsonStr(typed, &parsed) == nil {
				visit(parsed, depth+1)
			}
		case []any:
			for _, item := range typed {
				visit(item, depth+1)
			}
		case map[string]any:
			if description := firstString(typed, "description", "summary"); description != "" {
				tool.Description = description
			}
			if schema := extractInputSchema(typed); schema != nil {
				tool.Schema = schema
			}
			if cost := firstNumber(typed, "cost", "price", "credits"); cost > 0 {
				tool.Cost = cost
			}
			for _, nested := range typed {
				visit(nested, depth+1)
			}
		}
	}
	visit(payload, 0)
}

func extractInputSchema(value map[string]any) map[string]any {
	for _, key := range []string{"inputSchema", "input_schema", "schema", "parametersSchema", "parameters"} {
		if schema, ok := value[key].(map[string]any); ok {
			if _, hasType := schema["type"]; hasType || schema["properties"] != nil {
				return schema
			}
		}
	}
	return nil
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if result, ok := value[key].(string); ok && strings.TrimSpace(result) != "" {
			return strings.TrimSpace(result)
		}
	}
	return ""
}

func firstNumber(value map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch typed := value[key].(type) {
		case float64:
			if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
				return typed
			}
		case string:
			if result, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil && !math.IsNaN(result) && !math.IsInf(result, 0) {
				return result
			}
		case map[string]any:
			if result := firstNumber(typed, "amount", "value", "credits", "remaining", "balance"); result != 0 {
				return result
			}
		}
	}
	return 0
}

func extractAccountMetadata(payload any, forbidden []string) (string, int64) {
	plan := "已连接"
	balance := float64(0)
	var visit func(any, int)
	visit = func(current any, depth int) {
		if depth > 10 || current == nil {
			return
		}
		switch typed := current.(type) {
		case string:
			var parsed any
			if common.UnmarshalJsonStr(typed, &parsed) == nil {
				visit(parsed, depth+1)
			}
		case []any:
			for _, item := range typed {
				visit(item, depth+1)
			}
		case map[string]any:
			if candidate := firstString(typed, "plan", "tier"); candidate != "" {
				plan = candidate
			}
			if candidate := firstNumber(typed, "balance", "credits"); candidate > 0 {
				balance = candidate
			}
			for _, nested := range typed {
				visit(nested, depth+1)
			}
		}
	}
	visit(payload, 0)
	return truncatePublicText(sanitizePublicTextWithForbidden(plan, forbidden), 64), numberToMicros(balance)
}

func sanitizePublicTextWithForbidden(value string, forbidden []string) string {
	sanitized, ok := sanitizePublicValueWithForbidden(value, forbidden).(string)
	if !ok {
		return ""
	}
	return sanitized
}

func publicToolName(tool discoveredTool) string {
	title := strings.TrimSpace(tool.Title)
	normalizeIdentifier := strings.NewReplacer("/", "", "_", "", "-", "", ".", "", " ", "")
	normalizedTitle := strings.ToLower(normalizeIdentifier.Replace(title))
	normalizedToolName := strings.ToLower(normalizeIdentifier.Replace(tool.Name))
	privateTitle := normalizedToolName != "" && strings.Contains(normalizedTitle, normalizedToolName)
	if title != "" && !privateTitle && !strings.Contains(title, "/") && !strings.Contains(strings.ToLower(title), "agentkey") && len([]rune(title)) <= 64 {
		return truncatePublicText(title, 128)
	}
	provider := strings.Split(tool.Name, "/")[0]
	provider = strings.TrimSpace(strings.ReplaceAll(provider, "_", " "))
	if provider == "" {
		return "vSearch 数据能力"
	}
	return truncatePublicText(provider, 128)
}

func publicToolDescription(tool discoveredTool) string {
	description := truncatePublicText(tool.Description, 600)
	if description == "" || strings.Contains(strings.ToLower(description), strings.ToLower(tool.Name)) {
		return "可由 vSearch 按需调用的企业数据能力。"
	}
	return description
}

func classifyTool(tool discoveredTool) string {
	if category := truncatePublicText(tool.Category, 64); category != "" {
		return category
	}
	name := strings.ToLower(tool.Name + " " + tool.Description)
	switch {
	case strings.Contains(name, "crawl"), strings.Contains(name, "scrape"), strings.Contains(name, "extract"), strings.Contains(name, "firecrawl"), strings.Contains(name, "jina"):
		return "抓取"
	case strings.Contains(name, "wechat"), strings.Contains(name, "douyin"), strings.Contains(name, "tiktok"), strings.Contains(name, "youtube"), strings.Contains(name, "reddit"), strings.Contains(name, "social"):
		return "社交媒体"
	case strings.Contains(name, "stock"), strings.Contains(name, "finance"), strings.Contains(name, "market"), strings.Contains(name, "crypto"):
		return "金融"
	case strings.Contains(name, "amazon"), strings.Contains(name, "taobao"), strings.Contains(name, "ecommerce"), strings.Contains(name, "product"):
		return "电商"
	case strings.Contains(name, "company"), strings.Contains(name, "crunchbase"):
		return "企业工商"
	case strings.Contains(name, "weather"):
		return "天气"
	case strings.Contains(name, "travel"), strings.Contains(name, "booking"):
		return "旅行"
	case strings.Contains(name, "job"), strings.Contains(name, "recruit"):
		return "招聘"
	case strings.Contains(name, "news"), strings.Contains(name, "research"):
		return "新闻研究"
	default:
		return "搜索"
	}
}

func truncatePublicText(value string, limit int) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "https://api.agentkey.app", "vSearch upstream")
	value = strings.ReplaceAll(value, "AgentKey", "vSearch upstream")
	value = strings.ReplaceAll(value, "agentkey", "vSearch upstream")
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

func numberToMicros(value float64) int64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value >= float64(maxSearchMoneyMicros)/1_000_000 {
		return maxSearchMoneyMicros
	}
	return int64(math.Round(value * 1_000_000))
}
