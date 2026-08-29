package vsearch

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const (
	PublishAccessAllEnterprises = "all_enterprises"

	PublishSkipNotFound                = "not_found"
	PublishSkipSchemaUnavailable       = "schema_unavailable"
	PublishSkipHealthyRouteUnavailable = "healthy_route_unavailable"
)

type PublishCommand struct {
	ServiceIDs []string
	AccessMode string
}

type PublishSkippedService struct {
	ServiceID string `json:"service_id"`
	Reason    string `json:"reason"`
}

type PublishResult struct {
	Published           int                     `json:"published"`
	Skipped             int                     `json:"skipped"`
	PublishedServiceIDs []string                `json:"published_service_ids"`
	SkippedServices     []PublishSkippedService `json:"skipped_services"`
}

func (control *ControlPlane) PublishCatalog(_ context.Context, command PublishCommand) (PublishResult, error) {
	serviceIDs, err := normalizePublishServiceIDs(command.ServiceIDs)
	if err != nil {
		return PublishResult{}, err
	}
	if strings.TrimSpace(command.AccessMode) != PublishAccessAllEnterprises {
		return PublishResult{}, &PublicError{
			Code: "CATALOG_PUBLISH_INVALID", Message: "vSearch 发布范围无效。", HTTPStatus: http.StatusBadRequest,
		}
	}

	capabilities, err := model.ListSearchCapabilitiesByPublicIDs(serviceIDs)
	if err != nil {
		return PublishResult{}, err
	}
	capabilitiesByPublicID := make(map[string]*model.SearchCapability, len(capabilities))
	capabilityIDs := make([]int, 0, len(capabilities))
	for _, capability := range capabilities {
		capabilitiesByPublicID[capability.PublicID] = capability
		capabilityIDs = append(capabilityIDs, capability.Id)
	}
	bindings, err := model.ListSearchCapabilityBindingsForCapabilities(capabilityIDs, true)
	if err != nil {
		return PublishResult{}, err
	}
	accounts, err := model.ListSearchUpstreamAccounts()
	if err != nil {
		return PublishResult{}, err
	}
	pools, err := model.ListSearchUpstreamPools()
	if err != nil {
		return PublishResult{}, err
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

	result := PublishResult{
		PublishedServiceIDs: make([]string, 0, len(serviceIDs)),
		SkippedServices:     make([]PublishSkippedService, 0),
	}
	configs := make([]model.SearchCapabilityPublishConfig, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		capability := capabilitiesByPublicID[serviceID]
		if capability == nil {
			result.SkippedServices = append(result.SkippedServices, PublishSkippedService{ServiceID: serviceID, Reason: PublishSkipNotFound})
			continue
		}
		if _, status := parseCapabilitySchema(capability.InputSchema); capability.SchemaStatus != model.SearchCapabilitySchemaAvailable || status != "available" {
			result.SkippedServices = append(result.SkippedServices, PublishSkippedService{ServiceID: serviceID, Reason: PublishSkipSchemaUnavailable})
			continue
		}

		priceFloor := capability.UpstreamCostMicros
		healthyRoute := false
		for _, binding := range bindingsByCapability[capability.Id] {
			if !searchBindingMatchesCapabilitySchema(binding, capability) {
				continue
			}
			account := accountsByID[binding.UpstreamAccountID]
			if account == nil || account.Status != model.SearchUpstreamAccountStatusHealthy {
				continue
			}
			pool := poolsByID[account.PoolID]
			if pool == nil || pool.Status != model.SearchUpstreamPoolStatusEnabled {
				continue
			}
			healthyRoute = true
			if binding.UpstreamCostMicros > priceFloor {
				priceFloor = binding.UpstreamCostMicros
			}
		}
		if !healthyRoute {
			result.SkippedServices = append(result.SkippedServices, PublishSkippedService{ServiceID: serviceID, Reason: PublishSkipHealthyRouteUnavailable})
			continue
		}
		if capability.PriceMicros > priceFloor {
			priceFloor = capability.PriceMicros
		}
		configs = append(configs, model.SearchCapabilityPublishConfig{
			ID: capability.Id, PriceMicros: priceFloor,
			ExpectedInputSchema: capability.InputSchema, ExpectedSchemaStatus: capability.SchemaStatus,
		})
		result.PublishedServiceIDs = append(result.PublishedServiceIDs, serviceID)
	}
	if err := model.PublishSearchCapabilities(configs, true); err != nil {
		if errors.Is(err, model.ErrSearchCapabilityPublishStateChanged) {
			return PublishResult{}, &PublicError{
				Code: "CATALOG_PUBLISH_STATE_CHANGED", Message: "vSearch 目录状态已变化，请重新同步后发布。", HTTPStatus: http.StatusConflict,
			}
		}
		return PublishResult{}, err
	}
	result.Published = len(result.PublishedServiceIDs)
	result.Skipped = len(result.SkippedServices)
	return result, nil
}

func normalizePublishServiceIDs(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 500 {
		return nil, &PublicError{Code: "CATALOG_PUBLISH_INVALID", Message: "vSearch 发布能力数量无效。", HTTPStatus: http.StatusBadRequest}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 32 || !strings.HasPrefix(value, "vr_svc_") {
			return nil, &PublicError{Code: "CATALOG_PUBLISH_INVALID", Message: "vSearch 发布能力标识无效。", HTTPStatus: http.StatusBadRequest}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}
