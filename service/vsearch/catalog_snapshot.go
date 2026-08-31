package vsearch

import "github.com/QuantumNous/new-api/model"

type catalogSnapshotItem struct {
	capability        *model.SearchCapability
	declaredBindings  []*model.SearchCapabilityBinding
	availableBindings []*model.SearchCapabilityBinding
	public            PublicCapability
}

func loadCatalogSnapshot(principal Principal, includeDisabled, enforceAccess bool) ([]catalogSnapshotItem, error) {
	capabilities, err := model.ListSearchCapabilities(includeDisabled)
	if err != nil {
		return nil, err
	}
	capabilityIDs := make([]int, 0, len(capabilities))
	for _, capability := range capabilities {
		capabilityIDs = append(capabilityIDs, capability.Id)
	}
	if len(capabilityIDs) == 0 {
		return []catalogSnapshotItem{}, nil
	}
	bindings, err := model.ListSearchCapabilityBindingsForCapabilities(capabilityIDs, false)
	if err != nil {
		return nil, err
	}
	accounts, err := model.ListSearchUpstreamAccounts()
	if err != nil {
		return nil, err
	}
	pools, err := model.ListSearchUpstreamPools()
	if err != nil {
		return nil, err
	}
	grants := make([]*model.SearchCapabilityGrant, 0)
	if enforceAccess {
		grants, err = model.ListSearchCapabilityGrantsForCapabilities(capabilityIDs)
		if err != nil {
			return nil, err
		}
	}

	bindingsByCapability := make(map[int][]*model.SearchCapabilityBinding, len(capabilities))
	for _, binding := range bindings {
		bindingsByCapability[binding.CapabilityID] = append(bindingsByCapability[binding.CapabilityID], binding)
	}
	accountsByID := make(map[int]*model.SearchUpstreamAccount, len(accounts))
	for _, account := range accounts {
		accountsByID[account.Id] = account
	}
	poolsByID := make(map[int]*model.SearchUpstreamPool, len(pools))
	for _, pool := range pools {
		poolsByID[pool.Id] = pool
	}
	restrictedCapabilities := make(map[int]struct{})
	grantedCapabilities := make(map[int]struct{})
	for _, grant := range grants {
		if grant.Status != model.SearchCapabilityGrantStatusEnabled || grant.EnterpriseID <= 0 || grant.UserID != 0 {
			continue
		}
		restrictedCapabilities[grant.CapabilityID] = struct{}{}
		if grant.EnterpriseID == principal.EnterpriseID {
			grantedCapabilities[grant.CapabilityID] = struct{}{}
		}
	}

	result := make([]catalogSnapshotItem, 0, len(capabilities))
	for _, capability := range capabilities {
		if enforceAccess {
			_, restricted := restrictedCapabilities[capability.Id]
			_, granted := grantedCapabilities[capability.Id]
			if (restricted && !granted) || !principalAllowsCategory(principal, capability.Category) {
				continue
			}
		}
		allCapabilityBindings := bindingsByCapability[capability.Id]
		declaredBindings := make([]*model.SearchCapabilityBinding, 0, len(allCapabilityBindings))
		for _, binding := range allCapabilityBindings {
			if binding.Status == model.SearchCapabilityBindingStatusEnabled {
				declaredBindings = append(declaredBindings, binding)
			}
		}
		capabilityBindings := make([]*model.SearchCapabilityBinding, 0, len(declaredBindings))
		for _, binding := range declaredBindings {
			account := accountsByID[binding.UpstreamAccountID]
			if searchToolAllowed(binding.ToolName) && (account == nil || account.Provider != model.SearchUpstreamProviderJustOneAPI) {
				capabilityBindings = append(capabilityBindings, binding)
			}
		}
		if !includeDisabled && len(declaredBindings) > 0 && len(capabilityBindings) == 0 {
			continue
		}
		_, parsedSchemaStatus := parseCapabilitySchema(capability.InputSchema)
		schemaStatus := "unavailable"
		if capability.SchemaStatus == model.SearchCapabilitySchemaAvailable && parsedSchemaStatus == "available" {
			schemaStatus = "available"
		}
		contractStatus := "verified"
		if capability.ContractVersion != "legacy-v1" {
			contractStatus = "unverified"
			for _, binding := range capabilityBindings {
				if binding.Status == model.SearchCapabilityBindingStatusEnabled && binding.ContractEquivalent {
					contractStatus = "verified"
					break
				}
			}
		}
		healthyBindings := make([]*model.SearchCapabilityBinding, 0, len(capabilityBindings))
		healthyRouteCostFloor := int64(0)
		for _, binding := range capabilityBindings {
			if binding.Status != model.SearchCapabilityBindingStatusEnabled {
				continue
			}
			if !model.IsSearchCapabilityBindingExecutable(capability.ContractVersion, binding) {
				continue
			}
			if !searchBindingMatchesCapabilitySchema(binding, capability) {
				continue
			}
			account := accountsByID[binding.UpstreamAccountID]
			if account == nil || account.Status != model.SearchUpstreamAccountStatusHealthy {
				continue
			}
			pool := poolsByID[account.PoolID]
			if pool != nil && pool.Status == model.SearchUpstreamPoolStatusEnabled {
				healthyBindings = append(healthyBindings, binding)
				if binding.UpstreamCostMicros > healthyRouteCostFloor {
					healthyRouteCostFloor = binding.UpstreamCostMicros
				}
			}
		}
		healthyRouteCount := int64(len(healthyBindings))
		pricingAvailable := capability.PriceMicros >= healthyRouteCostFloor
		interfaceCount := int64(0)
		if healthyRouteCount > 0 && schemaStatus == "available" && pricingAvailable {
			interfaceCount = 1
		}
		status := "disabled"
		if capability.Status == model.SearchCapabilityStatusEnabled {
			status = "unavailable"
			if healthyRouteCount > 0 && schemaStatus == "available" && pricingAvailable {
				status = "available"
			}
		}
		public := PublicCapability{
			ID: capability.PublicID, Name: capability.Name, Category: capability.Category,
			Description: capability.Description, SchemaStatus: schemaStatus, ContractStatus: contractStatus, Status: status,
			Enabled:        capability.Status == model.SearchCapabilityStatusEnabled && healthyRouteCount > 0 && schemaStatus == "available" && pricingAvailable,
			InterfaceCount: interfaceCount, CostLabel: formatMicros(capability.PriceMicros),
			Price: float64(capability.PriceMicros) / 1_000_000, PriceMicros: capability.PriceMicros,
			LastSyncedAt: capability.LastSyncedAt,
		}
		if includeDisabled {
			public.HealthyRouteCount = healthyRouteCount
			upstreamCost := float64(capability.UpstreamCostMicros) / 1_000_000
			public.UpstreamCost = &upstreamCost
			public.UpstreamCostMicros = capability.UpstreamCostMicros
		}
		availableBindings := make([]*model.SearchCapabilityBinding, 0, len(healthyBindings))
		if public.Enabled {
			availableBindings = append(availableBindings, healthyBindings...)
		}
		result = append(result, catalogSnapshotItem{
			capability: capability, declaredBindings: declaredBindings,
			availableBindings: availableBindings, public: public,
		})
	}
	return result, nil
}

func searchBindingMatchesCapabilitySchema(binding *model.SearchCapabilityBinding, capability *model.SearchCapability) bool {
	if binding == nil || capability == nil || binding.InputSchema != capability.InputSchema {
		return false
	}
	return capability.ContractVersion == "legacy-v1" || binding.OutputSchema == capability.OutputSchema
}
