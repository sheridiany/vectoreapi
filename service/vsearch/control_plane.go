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

type AccountCommand struct {
	ID       int
	Name     string
	Provider string
	BaseURL  string
	Secret   string
	PoolID   int
	Weight   int
	Priority int
	Status   string
}

type AccountView struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Provider        string  `json:"provider"`
	BaseURL         string  `json:"base_url"`
	KeyPrefix       string  `json:"key_prefix"`
	Plan            string  `json:"plan"`
	Balance         float64 `json:"balance"`
	BalanceMicros   int64   `json:"balance_micros"`
	BalanceCurrency string  `json:"balance_currency"`
	Weight          int     `json:"weight"`
	Priority        int     `json:"priority"`
	Pool            string  `json:"pool"`
	PoolID          int     `json:"pool_id"`
	Status          string  `json:"status"`
	LastCheck       int64   `json:"last_check"`
	LastError       string  `json:"last_error,omitempty"`
}

type SyncResult struct {
	Synced           int                `json:"synced"`
	Published        int                `json:"published"`
	Skipped          int                `json:"skipped"`
	Discovered       int                `json:"discovered"`
	Accounts         int                `json:"accounts"`
	Failures         []string           `json:"failures"`
	Services         []PublicCapability `json:"services"`
	SyncedServiceIDs []string           `json:"synced_service_ids"`
}

type CapabilityCommand struct {
	ID                   int
	Enabled              bool
	PriceMicros          int64
	AvailabilityOverride bool
}

type ControlPlane struct {
	adapterFactory AdapterFactory
	runtime        *ExecutionRuntime
}

func NewControlPlane(factory AdapterFactory) *ControlPlane {
	runtime := NewExecutionRuntime(factory)
	return &ControlPlane{adapterFactory: runtime.adapterFactory, runtime: runtime}
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
		if account.Provider != model.SearchUpstreamProviderJustOneAPI && account.Provider != model.SearchUpstreamProviderTikHub {
			continue
		}
		views = append(views, toAccountView(account, poolNames[account.PoolID]))
	}
	return views, nil
}

func (control *ControlPlane) SaveAccount(ctx context.Context, command AccountCommand) (AccountView, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Provider = strings.TrimSpace(command.Provider)
	command.BaseURL = strings.TrimSpace(command.BaseURL)
	command.Secret = strings.TrimSpace(command.Secret)
	var account *model.SearchUpstreamAccount
	var err error
	providerChanged := false
	if command.ID > 0 {
		account, err = model.GetSearchUpstreamAccountByID(command.ID)
		if err != nil {
			return AccountView{}, err
		}
		if command.Provider == "" {
			command.Provider = account.Provider
		}
		providerChanged = command.Provider != account.Provider
		if providerChanged && command.Secret == "" {
			return AccountView{}, &PublicError{Code: "UPSTREAM_SECRET_REQUIRED", Message: "切换上游服务时必须重新输入 API 密钥。", HTTPStatus: http.StatusBadRequest}
		}
		if command.BaseURL == "" && !providerChanged {
			command.BaseURL = account.BaseURL
		}
	} else {
		if command.Provider == "" {
			command.Provider = ProviderTikHub
		}
		account = &model.SearchUpstreamAccount{}
	}
	if command.BaseURL == "" {
		switch command.Provider {
		case ProviderJustOneAPI:
			command.BaseURL = DefaultJustOneAPIBaseURL
		case ProviderTikHub:
			command.BaseURL = DefaultTikHubBaseURL
		}
	}
	endpoint, err := model.ValidateSearchUpstreamProviderBaseURL(command.Provider, command.BaseURL, model.SearchUpstreamLoopbackHTTPEnabled())
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

	if command.ID == 0 {
		if command.Secret == "" {
			return AccountView{}, &PublicError{Code: "UPSTREAM_SECRET_REQUIRED", Message: "请输入上游服务 API 密钥。", HTTPStatus: http.StatusBadRequest}
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
	account.Provider = command.Provider
	account.Name = command.Name
	account.BaseURL = command.BaseURL
	account.Weight = command.Weight
	account.Priority = command.Priority
	account.Status = status
	if providerChanged {
		account.Plan = ""
		account.BalanceMicros = 0
		account.BalanceCurrency = ""
		account.FailureCount = 0
		account.ConcurrentRequests = 0
		account.LastCheckedAt = 0
		account.LastErrorCode = ""
		account.LastErrorMessage = ""
	}
	if account.Id == 0 {
		err = model.CreateSearchUpstreamAccount(account)
	} else if providerChanged {
		err = model.UpdateSearchUpstreamAccountAndDisableBindings(account)
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
	adapter, err := control.adapterFactory(account, secret)
	if err != nil {
		return AccountView{}, safeRuntimeError(err)
	}
	state, probeErr := adapter.Probe(ctx)
	if probeErr != nil {
		safeErr := safeRuntimeError(probeErr)
		if safeErr.Code == "UPSTREAM_PROBE_UNSUPPORTED" {
			_ = model.UpdateSearchUpstreamAccountHealth(
				account.Id, account.Status, account.BalanceMicros, account.FailureCount, safeErr.Code, safeErr.Message,
			)
			return AccountView{}, safeErr
		}
		failureCount := account.FailureCount + 1
		status := model.SearchUpstreamAccountStatusWarning
		if failureCount >= 3 {
			status = model.SearchUpstreamAccountStatusStandby
		}
		_ = model.UpdateSearchUpstreamAccountHealth(account.Id, status, account.BalanceMicros, failureCount, safeErr.Code, safeErr.Message)
		return AccountView{}, safeErr
	}
	if err := model.UpdateSearchUpstreamAccountHealth(account.Id, model.SearchUpstreamAccountStatusHealthy, state.BalanceAmountMicros, 0, "", ""); err != nil {
		return AccountView{}, err
	}
	account.Plan = state.Plan
	account.BalanceMicros = state.BalanceAmountMicros
	account.BalanceCurrency = state.BalanceCurrency
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

func (control *ControlPlane) SyncCatalog(ctx context.Context) (SyncResult, error) {
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
	result := SyncResult{Failures: make([]string, 0), SyncedServiceIDs: make([]string, 0)}
	syncedServiceIDs := make(map[string]struct{})
	definitions := standardCapabilityRegistry()
	definitionsByKey := make(map[string]standardCapabilityDefinition, len(definitions))
	for _, definition := range definitions {
		definitionsByKey[definition.OperationKey] = definition
	}
	for _, account := range accounts {
		if _, enabled := enabledPoolIDs[account.PoolID]; !enabled || account.Status == model.SearchUpstreamAccountStatusPaused {
			continue
		}
		if account.Provider != model.SearchUpstreamProviderJustOneAPI && account.Provider != model.SearchUpstreamProviderTikHub {
			continue
		}
		secret, decryptErr := DecryptUpstreamSecret(EncryptedSecret{
			Ciphertext: account.SecretCiphertext, Nonce: account.SecretNonce, Version: account.SecretVersion,
		})
		if decryptErr != nil {
			result.Failures = append(result.Failures, account.Name+"：密钥无法解密")
			continue
		}
		adapter, adapterErr := control.adapterFactory(account, secret)
		if adapterErr != nil {
			result.Failures = append(result.Failures, account.Name+"：适配器配置无效")
			continue
		}
		snapshot, snapshotErr := adapter.SnapshotCatalog(ctx)
		if snapshotErr != nil {
			result.Failures = append(result.Failures, account.Name+"：标准目录读取失败")
			continue
		}
		result.Accounts++
		result.Discovered += len(snapshot.Operations)
		syncedAt := common.GetTimestamp()
		expectedMappingKey := mappingKeyForProvider(account.Provider)
		expectedBindings := make([]model.SearchCapabilityBindingIdentity, 0, len(snapshot.Operations))
		for _, operation := range snapshot.Operations {
			definition, exists := definitionsByKey[operation.OperationKey]
			if !exists || operation.ContractVersion != definition.ContractVersion || operation.MappingKey != expectedMappingKey {
				result.Skipped++
				continue
			}
			normalizedCostMicros, normalizedCostCurrency, costErr := searchUpstreamCostToCNY(operation.CostAmountMicros, operation.CostCurrency)
			if costErr != nil {
				result.Failures = append(result.Failures, account.Name+"：上游价格换算失败")
				continue
			}
			publicID, idErr := model.GenerateSearchCapabilityPublicID(definition.OperationKey + "@" + definition.ContractVersion)
			if idErr != nil {
				result.Failures = append(result.Failures, account.Name+"：标准能力标识生成失败")
				continue
			}
			inputSchema, inputErr := common.Marshal(definition.InputSchema)
			outputSchema, outputErr := common.Marshal(definition.OutputSchema)
			parameterMap, parameterErr := common.Marshal(operation.ParameterMap)
			fixedParams, fixedErr := common.Marshal(operation.FixedParams)
			if inputErr != nil || outputErr != nil || parameterErr != nil || fixedErr != nil {
				result.Failures = append(result.Failures, account.Name+"：标准能力合同保存失败")
				continue
			}
			capability := &model.SearchCapability{
				PublicID: publicID, OperationKey: definition.OperationKey, ContractVersion: definition.ContractVersion,
				Name: definition.Name, Category: definition.Category, Description: definition.Description,
				InputSchema: string(inputSchema), OutputSchema: string(outputSchema), SchemaStatus: model.SearchCapabilitySchemaAvailable,
				Status: model.SearchCapabilityStatusDisabled, AvailabilitySource: model.SearchCapabilityAvailabilityUpstream,
				UpstreamCostMicros: normalizedCostMicros, PriceMicros: normalizedCostMicros, LastSyncedAt: syncedAt,
			}
			if upsertErr := model.UpsertDiscoveredSearchCapability(capability); upsertErr != nil {
				result.Failures = append(result.Failures, account.Name+"：标准能力保存失败")
				continue
			}
			persisted, getErr := model.GetSearchCapabilityByPublicID(publicID)
			if getErr != nil {
				result.Failures = append(result.Failures, account.Name+"：标准能力读取失败")
				continue
			}
			if operation.ContractEquivalent && operation.BillingReady && persisted.AvailabilitySource != model.SearchCapabilityAvailabilityManual {
				if priceErr := model.ConfigureSearchCapability(persisted.Id, persisted.Status, normalizedCostMicros, false); priceErr != nil {
					result.Failures = append(result.Failures, account.Name+"：标准能力同价设置失败")
					continue
				}
			}
			binding := &model.SearchCapabilityBinding{
				CapabilityID: persisted.Id, UpstreamAccountID: account.Id, ToolName: operation.OperationID,
				Platform: operation.Platform, ProviderOperationID: operation.OperationID,
				HTTPMethod: operation.Method, UpstreamPath: operation.Path, AuthPlacement: operation.AuthPlacement,
				MappingKey: operation.MappingKey, MappingVersion: operation.MappingVersion,
				ParameterMap: string(parameterMap), FixedParams: string(fixedParams),
				InputSchema: string(inputSchema), OutputSchema: string(outputSchema),
				CostCurrency: normalizedCostCurrency, ContractEquivalent: operation.ContractEquivalent,
				BillingReady: operation.BillingReady,
				Status:       model.SearchCapabilityBindingStatusEnabled, Weight: account.Weight, Priority: account.Priority,
				UpstreamCostMicros: normalizedCostMicros, LastSyncedAt: syncedAt,
			}
			if bindingErr := model.UpsertSearchCapabilityBinding(binding); bindingErr != nil {
				result.Failures = append(result.Failures, account.Name+"：标准能力绑定失败")
				continue
			}
			expectedBindings = append(expectedBindings, model.SearchCapabilityBindingIdentity{
				CapabilityID: persisted.Id, ProviderOperationID: operation.OperationID, Platform: operation.Platform,
			})
			result.Synced++
			if _, exists := syncedServiceIDs[publicID]; !exists {
				syncedServiceIDs[publicID] = struct{}{}
				result.SyncedServiceIDs = append(result.SyncedServiceIDs, publicID)
			}
		}
		if staleErr := model.DisableStaleSearchCapabilityBindings(account.Id, expectedMappingKey, expectedBindings); staleErr != nil {
			result.Failures = append(result.Failures, account.Name+"：旧目录绑定停用失败")
		}
	}
	sort.Strings(result.SyncedServiceIDs)
	for _, publicID := range result.SyncedServiceIDs {
		capability, getErr := model.GetSearchCapabilityByPublicID(publicID)
		if getErr != nil {
			result.Failures = append(result.Failures, publicID+"：标准能力读取失败")
			continue
		}
		bindings, bindingErr := model.ListSearchCapabilityBindings(capability.Id, true)
		if bindingErr != nil {
			result.Failures = append(result.Failures, capability.Name+"：标准能力绑定读取失败")
			continue
		}
		executableBindingIDs := make([]int, 0, len(bindings))
		for _, binding := range bindings {
			if model.IsSearchCapabilityBindingExecutable(capability.ContractVersion, binding) {
				executableBindingIDs = append(executableBindingIDs, binding.Id)
			}
		}
		if len(executableBindingIDs) == 0 {
			continue
		}
		if priceErr := model.RefreshSearchCapabilityPriceFloorForBindings(capability.Id, executableBindingIDs); priceErr != nil {
			result.Failures = append(result.Failures, capability.Name+"：上游成本刷新失败")
		}
	}
	if result.Synced == 0 {
		return result, &PublicError{Code: "CATALOG_SYNC_FAILED", Message: "没有同步到标准能力，请检查 TikHub 账号配置。", HTTPStatus: http.StatusBadGateway}
	}
	result.Services, err = control.runtime.ListCatalog(ctx, Principal{}, true)
	return result, err
}

func (control *ControlPlane) ConfigureCapability(ctx context.Context, command CapabilityCommand) (PublicCapability, error) {
	if command.PriceMicros < 0 || command.PriceMicros > maxSearchMoneyMicros {
		return PublicCapability{}, &PublicError{Code: "CAPABILITY_PRICE_INVALID", Message: "能力售价无效。", HTTPStatus: http.StatusBadRequest}
	}
	capability, err := model.GetSearchCapabilityByID(command.ID)
	if err != nil {
		return PublicCapability{}, err
	}
	if command.Enabled {
		if _, schemaStatus := parseCapabilitySchema(capability.InputSchema); capability.SchemaStatus != model.SearchCapabilitySchemaAvailable || schemaStatus != "available" {
			return PublicCapability{}, &PublicError{Code: "CAPABILITY_SCHEMA_UNAVAILABLE", Message: "该能力的参数定义尚未同步。", HTTPStatus: http.StatusServiceUnavailable}
		}
		if capability.ContractVersion != "legacy-v1" {
			bindings, bindingErr := model.ListSearchCapabilityBindings(capability.Id, true)
			if bindingErr != nil {
				return PublicCapability{}, bindingErr
			}
			contractVerified := false
			billingReady := false
			for _, binding := range bindings {
				if binding.ContractEquivalent {
					contractVerified = true
				}
				if model.IsSearchCapabilityBindingExecutable(capability.ContractVersion, binding) {
					billingReady = true
				}
			}
			if !contractVerified {
				return PublicCapability{}, &PublicError{Code: "CAPABILITY_CONTRACT_UNVERIFIED", Message: "该能力的上游返回结构尚未完成标准合同验证。", HTTPStatus: http.StatusServiceUnavailable}
			}
			if !billingReady {
				return PublicCapability{}, &PublicError{Code: "CAPABILITY_PRICING_UNVERIFIED", Message: "该能力的上游成本币种与人民币结算尚未完成验证。", HTTPStatus: http.StatusServiceUnavailable}
			}
		}
		healthyBindings, listErr := listHealthySearchBindings(command.ID)
		if listErr != nil {
			return PublicCapability{}, listErr
		}
		if len(healthyBindings) == 0 {
			return PublicCapability{}, &PublicError{Code: "CAPABILITY_UNAVAILABLE", Message: "该能力当前没有可用上游账号。", HTTPStatus: http.StatusServiceUnavailable}
		}
		priceFloor := healthySearchBindingsPriceFloor(healthyBindings)
		if command.PriceMicros < priceFloor {
			return PublicCapability{}, &PublicError{Code: "CAPABILITY_PRICE_BELOW_COST", Message: "能力售价不能低于上游成本。", HTTPStatus: http.StatusBadRequest}
		}
	}
	status := model.SearchCapabilityStatusDisabled
	if command.Enabled {
		status = model.SearchCapabilityStatusEnabled
	}
	if err := model.ConfigureSearchCapability(command.ID, status, command.PriceMicros, command.AvailabilityOverride); err != nil {
		return PublicCapability{}, err
	}
	capability, err = model.GetSearchCapabilityByID(command.ID)
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
		Balance: float64(account.BalanceMicros) / 1_000_000, BalanceMicros: account.BalanceMicros, BalanceCurrency: account.BalanceCurrency,
		Weight: account.Weight, Priority: account.Priority, Pool: poolName, PoolID: account.PoolID,
		Status: accountStatusName(account.Status), LastCheck: account.LastCheckedAt,
		LastError: account.LastErrorMessage,
	}
}

func parseAccountStatus(value string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return model.SearchUpstreamAccountStatusHealthy, nil
	case "standby":
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

func schemaNumber(value any) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}
