package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SearchCapabilityStatusEnabled  = 1
	SearchCapabilityStatusDisabled = 2

	SearchCapabilityAvailabilityLegacyPreserved = 0
	SearchCapabilityAvailabilityUpstream        = 1
	SearchCapabilityAvailabilityManual          = 2
	SearchCapabilityAvailabilityLegacyAdopted   = 3

	SearchCapabilitySchemaUnknown     = 0
	SearchCapabilitySchemaAvailable   = 1
	SearchCapabilitySchemaUnavailable = 2

	SearchCapabilityBindingStatusEnabled  = 1
	SearchCapabilityBindingStatusDisabled = 2

	SearchCapabilityGrantStatusEnabled  = 1
	SearchCapabilityGrantStatusDisabled = 2
)

type SearchCapability struct {
	Id                 int            `json:"id"`
	PublicID           string         `json:"public_id" gorm:"size:32;not null;uniqueIndex"`
	OperationKey       string         `json:"operation_key" gorm:"size:128;index"`
	ContractVersion    string         `json:"contract_version" gorm:"size:32"`
	Name               string         `json:"name" gorm:"size:128;not null;index"`
	Category           string         `json:"category" gorm:"size:64;not null;index"`
	Description        string         `json:"description" gorm:"type:text"`
	InputSchema        string         `json:"input_schema,omitempty" gorm:"type:text"`
	OutputSchema       string         `json:"output_schema,omitempty" gorm:"type:text"`
	SchemaStatus       int            `json:"schema_status" gorm:"type:int;not null"`
	Status             int            `json:"status" gorm:"type:int;not null;index"`
	AvailabilitySource int            `json:"-" gorm:"type:int;not null;default:0"`
	UpstreamCostMicros int64          `json:"upstream_cost_micros" gorm:"not null"`
	PriceMicros        int64          `json:"price_micros" gorm:"not null"`
	LastSyncedAt       int64          `json:"last_synced_at" gorm:"index"`
	CreatedAt          int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

type SearchCapabilityBinding struct {
	Id                  int    `json:"id"`
	CapabilityID        int    `json:"capability_id" gorm:"not null;uniqueIndex:idx_search_capability_binding,priority:1;index"`
	UpstreamAccountID   int    `json:"upstream_account_id" gorm:"not null;uniqueIndex:idx_search_capability_binding,priority:2;index"`
	ToolName            string `json:"-" gorm:"size:191;not null;uniqueIndex:idx_search_capability_binding,priority:3"`
	Platform            string `json:"platform" gorm:"size:32;index"`
	ProviderOperationID string `json:"provider_operation_id" gorm:"size:191"`
	HTTPMethod          string `json:"http_method" gorm:"size:8"`
	UpstreamPath        string `json:"upstream_path" gorm:"size:512"`
	AuthPlacement       string `json:"auth_placement" gorm:"size:16"`
	MappingKey          string `json:"mapping_key" gorm:"size:64"`
	MappingVersion      string `json:"mapping_version" gorm:"size:32"`
	ParameterMap        string `json:"-" gorm:"type:text"`
	FixedParams         string `json:"-" gorm:"type:text"`
	InputSchema         string `json:"-" gorm:"type:text"`
	OutputSchema        string `json:"-" gorm:"type:text"`
	CostCurrency        string `json:"cost_currency" gorm:"size:8"`
	ContractEquivalent  bool   `json:"contract_equivalent"`
	BillingReady        bool   `json:"billing_ready"`
	Status              int    `json:"status" gorm:"type:int;not null;index"`
	Weight              int    `json:"weight" gorm:"type:int;not null"`
	Priority            int    `json:"priority" gorm:"type:int;not null"`
	UpstreamCostMicros  int64  `json:"upstream_cost_micros" gorm:"not null"`
	LastSyncedAt        int64  `json:"last_synced_at" gorm:"index"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type SearchCapabilityGrant struct {
	Id           int   `json:"id"`
	CapabilityID int   `json:"capability_id" gorm:"not null;uniqueIndex:idx_search_capability_grant,priority:1;index"`
	EnterpriseID int   `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_search_capability_grant,priority:2;index"`
	UserID       int   `json:"user_id" gorm:"not null;uniqueIndex:idx_search_capability_grant,priority:3;index"`
	Status       int   `json:"status" gorm:"type:int;not null;index"`
	CreatedAt    int64 `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func GenerateSearchCapabilityPublicID(internalID string) (string, error) {
	internalID = strings.TrimSpace(internalID)
	if internalID == "" {
		return "", errors.New("search capability internal id is required")
	}
	digest := sha256.Sum256([]byte("vsearch-capability:v1:" + internalID))
	return "vr_svc_" + hex.EncodeToString(digest[:8]), nil
}

func normalizeSearchCapability(capability *SearchCapability) error {
	if capability == nil {
		return errors.New("search capability is required")
	}
	capability.PublicID = strings.TrimSpace(capability.PublicID)
	capability.OperationKey = strings.TrimSpace(capability.OperationKey)
	capability.ContractVersion = strings.TrimSpace(capability.ContractVersion)
	capability.Name = strings.TrimSpace(capability.Name)
	capability.Category = strings.TrimSpace(capability.Category)
	capability.Description = strings.TrimSpace(capability.Description)
	capability.InputSchema = strings.TrimSpace(capability.InputSchema)
	capability.OutputSchema = strings.TrimSpace(capability.OutputSchema)
	if capability.PublicID == "" || len(capability.PublicID) > 32 || !strings.HasPrefix(capability.PublicID, "vr_svc_") {
		return errors.New("search capability public id is invalid")
	}
	if capability.OperationKey == "" {
		capability.OperationKey = capability.PublicID
	}
	if capability.ContractVersion == "" {
		capability.ContractVersion = "legacy-v1"
	}
	if len(capability.OperationKey) > 128 || len(capability.ContractVersion) > 32 {
		return errors.New("search capability contract identity is invalid")
	}
	if capability.Name == "" || len([]rune(capability.Name)) > 128 {
		return errors.New("search capability name must be between 1 and 128 characters")
	}
	if capability.Category == "" || len([]rune(capability.Category)) > 64 {
		return errors.New("search capability category must be between 1 and 64 characters")
	}
	if capability.UpstreamCostMicros < 0 || capability.PriceMicros < 0 {
		return errors.New("search capability price is invalid")
	}
	if capability.Status == 0 {
		capability.Status = SearchCapabilityStatusDisabled
	}
	if capability.Status != SearchCapabilityStatusEnabled && capability.Status != SearchCapabilityStatusDisabled {
		return errors.New("search capability status is invalid")
	}
	if !validSearchCapabilityAvailabilitySource(capability.AvailabilitySource) {
		return errors.New("search capability availability source is invalid")
	}
	if capability.SchemaStatus == SearchCapabilitySchemaUnknown {
		if capability.InputSchema == "" {
			capability.SchemaStatus = SearchCapabilitySchemaUnavailable
		} else {
			capability.SchemaStatus = SearchCapabilitySchemaAvailable
		}
	}
	if capability.SchemaStatus != SearchCapabilitySchemaAvailable && capability.SchemaStatus != SearchCapabilitySchemaUnavailable {
		return errors.New("search capability schema status is invalid")
	}
	if capability.LastSyncedAt == 0 {
		capability.LastSyncedAt = common.GetTimestamp()
	}
	return nil
}

func validSearchCapabilityAvailabilitySource(source int) bool {
	return source == SearchCapabilityAvailabilityLegacyPreserved ||
		source == SearchCapabilityAvailabilityUpstream ||
		source == SearchCapabilityAvailabilityManual ||
		source == SearchCapabilityAvailabilityLegacyAdopted
}

func normalizeSearchCapabilityBinding(binding *SearchCapabilityBinding) error {
	if binding == nil || binding.CapabilityID <= 0 || binding.UpstreamAccountID <= 0 {
		return errors.New("search capability binding identity is invalid")
	}
	binding.ToolName = strings.TrimSpace(binding.ToolName)
	binding.Platform = strings.TrimSpace(binding.Platform)
	binding.ProviderOperationID = strings.TrimSpace(binding.ProviderOperationID)
	binding.HTTPMethod = strings.ToUpper(strings.TrimSpace(binding.HTTPMethod))
	binding.UpstreamPath = strings.TrimSpace(binding.UpstreamPath)
	binding.AuthPlacement = strings.TrimSpace(binding.AuthPlacement)
	binding.MappingKey = strings.TrimSpace(binding.MappingKey)
	binding.MappingVersion = strings.TrimSpace(binding.MappingVersion)
	binding.ParameterMap = strings.TrimSpace(binding.ParameterMap)
	binding.FixedParams = strings.TrimSpace(binding.FixedParams)
	binding.InputSchema = strings.TrimSpace(binding.InputSchema)
	binding.OutputSchema = strings.TrimSpace(binding.OutputSchema)
	binding.CostCurrency = strings.ToUpper(strings.TrimSpace(binding.CostCurrency))
	if binding.ProviderOperationID == "" {
		binding.ProviderOperationID = binding.ToolName
	}
	if binding.ToolName == "" {
		binding.ToolName = binding.ProviderOperationID
	}
	if binding.ToolName == "" || len(binding.ToolName) > 191 {
		return errors.New("search capability binding tool name is invalid")
	}
	if len(binding.Platform) > 32 || len(binding.ProviderOperationID) > 191 || len(binding.HTTPMethod) > 8 || len(binding.UpstreamPath) > 512 || len(binding.AuthPlacement) > 16 || len(binding.MappingKey) > 64 || len(binding.MappingVersion) > 32 || len(binding.CostCurrency) > 8 {
		return errors.New("search capability binding contract is invalid")
	}
	if binding.UpstreamCostMicros < 0 || binding.Priority < 0 {
		return errors.New("search capability binding numeric value is invalid")
	}
	if binding.Weight == 0 {
		binding.Weight = 1
	}
	if binding.Weight < 1 || binding.Weight > 100 {
		return errors.New("search capability binding weight is invalid")
	}
	if binding.Status == 0 {
		binding.Status = SearchCapabilityBindingStatusEnabled
	}
	if binding.Status != SearchCapabilityBindingStatusEnabled && binding.Status != SearchCapabilityBindingStatusDisabled {
		return errors.New("search capability binding status is invalid")
	}
	if binding.LastSyncedAt == 0 {
		binding.LastSyncedAt = common.GetTimestamp()
	}
	return nil
}

func IsSearchCapabilityBindingExecutable(contractVersion string, binding *SearchCapabilityBinding) bool {
	if binding == nil {
		return false
	}
	if strings.TrimSpace(contractVersion) == "legacy-v1" {
		return true
	}
	return binding.ContractEquivalent && binding.BillingReady && binding.CostCurrency == "CNY"
}

func CreateSearchCapability(capability *SearchCapability) error {
	if err := normalizeSearchCapability(capability); err != nil {
		return err
	}
	return DB.Create(capability).Error
}

func UpsertDiscoveredSearchCapability(capability *SearchCapability) error {
	if err := normalizeSearchCapability(capability); err != nil {
		return err
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "public_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"operation_key", "contract_version", "name", "category", "description", "input_schema", "output_schema", "schema_status",
			"upstream_cost_micros", "last_synced_at", "updated_at",
		}),
	}).Create(capability).Error
}

func ConfigureSearchCapability(id, status int, priceMicros int64, manualAvailability bool) error {
	if id <= 0 || priceMicros < 0 {
		return errors.New("search capability configuration is invalid")
	}
	if status != SearchCapabilityStatusEnabled && status != SearchCapabilityStatusDisabled {
		return errors.New("search capability status is invalid")
	}
	updates := map[string]any{
		"status":       status,
		"price_micros": priceMicros,
		"updated_at":   common.GetTimestamp(),
	}
	if manualAvailability {
		updates["availability_source"] = SearchCapabilityAvailabilityManual
	}
	result := DB.Model(&SearchCapability{}).Where("id = ?", id).Updates(updates)
	return searchUpdateError(result, &SearchCapability{}, id)
}

func RefreshSearchCapabilityPriceFloor(id int) error {
	return refreshSearchCapabilityPriceFloor(id, nil)
}

func RefreshSearchCapabilityPriceFloorForBindings(id int, allowedBindingIDs []int) error {
	if len(allowedBindingIDs) == 0 {
		return errors.New("search capability price floor bindings are invalid")
	}
	return refreshSearchCapabilityPriceFloor(id, allowedBindingIDs)
}

func refreshSearchCapabilityPriceFloor(id int, allowedBindingIDs []int) error {
	if id <= 0 {
		return errors.New("search capability id is invalid")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		priceFloor, err := searchCapabilityPriceFloor(tx, id, true, allowedBindingIDs)
		if err != nil {
			return err
		}
		result := tx.Model(&SearchCapability{}).Where("id = ?", id).Updates(map[string]any{
			"upstream_cost_micros": priceFloor,
			"price_micros": gorm.Expr(
				"CASE WHEN price_micros < ? THEN ? ELSE price_micros END",
				priceFloor, priceFloor,
			),
			"updated_at": common.GetTimestamp(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		var count int64
		if err := tx.Model(&SearchCapability{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func GetSearchCapabilityPriceFloor(id int) (int64, error) {
	if id <= 0 {
		return 0, errors.New("search capability id is invalid")
	}
	return searchCapabilityPriceFloor(DB, id, false, nil)
}

func searchCapabilityPriceFloor(tx *gorm.DB, id int, lockRows bool, allowedBindingIDs []int) (int64, error) {
	capability := SearchCapability{}
	capabilityQuery := tx.Select("id", "input_schema").Where("id = ?", id)
	if lockRows {
		capabilityQuery = lockForUpdate(capabilityQuery)
	}
	if err := capabilityQuery.First(&capability).Error; err != nil {
		return 0, err
	}

	bindings := make([]SearchCapabilityBinding, 0)
	bindingQuery := tx.Select("id", "input_schema", "upstream_cost_micros").
		Where("capability_id = ? AND status = ?", id, SearchCapabilityBindingStatusEnabled).
		Order("id asc")
	if len(allowedBindingIDs) > 0 {
		bindingQuery = bindingQuery.Where("id IN ?", allowedBindingIDs)
	}
	if lockRows {
		bindingQuery = lockForUpdate(bindingQuery)
	}
	if err := bindingQuery.Find(&bindings).Error; err != nil {
		return 0, err
	}

	var priceFloor int64
	for _, binding := range bindings {
		if binding.InputSchema == capability.InputSchema && binding.UpstreamCostMicros > priceFloor {
			priceFloor = binding.UpstreamCostMicros
		}
	}
	return priceFloor, nil
}

func GetSearchCapabilityByID(id int) (*SearchCapability, error) {
	if id <= 0 {
		return nil, errors.New("search capability id is invalid")
	}
	capability := &SearchCapability{}
	return capability, DB.First(capability, id).Error
}

func GetSearchCapabilityByPublicID(publicID string) (*SearchCapability, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, errors.New("search capability public id is required")
	}
	capability := &SearchCapability{}
	return capability, DB.Where("public_id = ?", publicID).First(capability).Error
}

func ListSearchCapabilities(includeDisabled bool) ([]*SearchCapability, error) {
	capabilities := make([]*SearchCapability, 0)
	query := DB.Model(&SearchCapability{})
	if !includeDisabled {
		query = query.Where("status = ?", SearchCapabilityStatusEnabled)
	}
	return capabilities, query.Order("category asc, name asc, id asc").Find(&capabilities).Error
}

func UpsertSearchCapabilityBinding(binding *SearchCapabilityBinding) error {
	if err := normalizeSearchCapabilityBinding(binding); err != nil {
		return err
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "capability_id"}, {Name: "upstream_account_id"}, {Name: "tool_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"platform", "provider_operation_id", "http_method", "upstream_path", "auth_placement", "mapping_key", "mapping_version",
			"parameter_map", "fixed_params", "input_schema", "output_schema", "cost_currency", "contract_equivalent", "billing_ready",
			"status", "weight", "priority", "upstream_cost_micros", "last_synced_at", "updated_at",
		}),
	}).Create(binding).Error
}

type SearchCapabilityBindingIdentity struct {
	CapabilityID        int
	ProviderOperationID string
	Platform            string
}

func DisableStaleSearchCapabilityBindings(accountID int, mappingKey string, expectedBindings []SearchCapabilityBindingIdentity) error {
	if accountID <= 0 || strings.TrimSpace(mappingKey) == "" {
		return errors.New("search capability binding scope is invalid")
	}
	expected := make(map[SearchCapabilityBindingIdentity]struct{}, len(expectedBindings))
	for _, identity := range expectedBindings {
		identity.ProviderOperationID = strings.TrimSpace(identity.ProviderOperationID)
		identity.Platform = strings.TrimSpace(identity.Platform)
		if identity.CapabilityID <= 0 || identity.ProviderOperationID == "" {
			return errors.New("search capability binding identity is invalid")
		}
		expected[identity] = struct{}{}
	}
	bindings := make([]SearchCapabilityBinding, 0)
	if err := DB.Where(
		"upstream_account_id = ? AND mapping_key = ? AND status = ?",
		accountID, strings.TrimSpace(mappingKey), SearchCapabilityBindingStatusEnabled,
	).Find(&bindings).Error; err != nil {
		return err
	}
	staleIDs := make([]int, 0)
	for _, binding := range bindings {
		identity := SearchCapabilityBindingIdentity{
			CapabilityID: binding.CapabilityID, ProviderOperationID: binding.ProviderOperationID, Platform: binding.Platform,
		}
		if _, keep := expected[identity]; !keep {
			staleIDs = append(staleIDs, binding.Id)
		}
	}
	if len(staleIDs) == 0 {
		return nil
	}
	return DB.Model(&SearchCapabilityBinding{}).Where("id IN ?", staleIDs).Updates(map[string]any{
		"status":     SearchCapabilityBindingStatusDisabled,
		"updated_at": common.GetTimestamp(),
	}).Error
}

func ListSearchCapabilityBindings(capabilityID int, enabledOnly bool) ([]*SearchCapabilityBinding, error) {
	if capabilityID <= 0 {
		return nil, errors.New("search capability id is invalid")
	}
	bindings := make([]*SearchCapabilityBinding, 0)
	query := DB.Where("capability_id = ?", capabilityID)
	if enabledOnly {
		query = query.Where("status = ?", SearchCapabilityBindingStatusEnabled)
	}
	return bindings, query.Order("priority asc, weight desc, id asc").Find(&bindings).Error
}

func ListSearchCapabilityBindingsForCapabilities(capabilityIDs []int, enabledOnly bool) ([]*SearchCapabilityBinding, error) {
	bindings := make([]*SearchCapabilityBinding, 0)
	if len(capabilityIDs) == 0 {
		return bindings, nil
	}
	query := DB.Where("capability_id IN ?", capabilityIDs)
	if enabledOnly {
		query = query.Where("status = ?", SearchCapabilityBindingStatusEnabled)
	}
	return bindings, query.Order("capability_id asc, priority asc, weight desc, id asc").Find(&bindings).Error
}

func ReplaceSearchCapabilityGrants(capabilityID int, grants []SearchCapabilityGrant) error {
	if capabilityID <= 0 {
		return errors.New("search capability id is invalid")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("capability_id = ?", capabilityID).Delete(&SearchCapabilityGrant{}).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].CapabilityID = capabilityID
			if grants[i].EnterpriseID < 0 || grants[i].UserID < 0 || (grants[i].EnterpriseID == 0 && grants[i].UserID == 0) {
				return errors.New("search capability grant scope is invalid")
			}
			if grants[i].Status == 0 {
				grants[i].Status = SearchCapabilityGrantStatusEnabled
			}
			if grants[i].Status != SearchCapabilityGrantStatusEnabled && grants[i].Status != SearchCapabilityGrantStatusDisabled {
				return errors.New("search capability grant status is invalid")
			}
		}
		if len(grants) == 0 {
			return nil
		}
		return tx.Create(&grants).Error
	})
}

func ListSearchCapabilityGrants(capabilityID int) ([]*SearchCapabilityGrant, error) {
	if capabilityID <= 0 {
		return nil, errors.New("search capability id is invalid")
	}
	grants := make([]*SearchCapabilityGrant, 0)
	return grants, DB.Where("capability_id = ?", capabilityID).Order("id asc").Find(&grants).Error
}

func ListSearchCapabilityGrantsForCapabilities(capabilityIDs []int) ([]*SearchCapabilityGrant, error) {
	grants := make([]*SearchCapabilityGrant, 0)
	if len(capabilityIDs) == 0 {
		return grants, nil
	}
	return grants, DB.Where("capability_id IN ?", capabilityIDs).
		Order("capability_id asc, id asc").Find(&grants).Error
}

func ReplaceSearchCapabilityEnterpriseGrants(capabilityID int, enterpriseIDs []int) error {
	seen := make(map[int]struct{}, len(enterpriseIDs))
	grants := make([]SearchCapabilityGrant, 0, len(enterpriseIDs))
	for _, enterpriseID := range enterpriseIDs {
		if enterpriseID <= 0 {
			return errors.New("search capability enterprise grant is invalid")
		}
		if _, exists := seen[enterpriseID]; exists {
			continue
		}
		seen[enterpriseID] = struct{}{}
		grants = append(grants, SearchCapabilityGrant{
			EnterpriseID: enterpriseID,
			Status:       SearchCapabilityGrantStatusEnabled,
		})
	}
	return ReplaceSearchCapabilityGrants(capabilityID, grants)
}

func ListSearchCapabilityEnterpriseGrants(capabilityID int) ([]*SearchCapabilityGrant, error) {
	if capabilityID <= 0 {
		return nil, errors.New("search capability id is invalid")
	}
	grants := make([]*SearchCapabilityGrant, 0)
	return grants, DB.Where(
		"capability_id = ? AND enterprise_id > 0 AND user_id = 0 AND status = ?",
		capabilityID,
		SearchCapabilityGrantStatusEnabled,
	).Order("enterprise_id asc, id asc").Find(&grants).Error
}

func IsSearchCapabilityGranted(capabilityID, enterpriseID, userID int) (bool, error) {
	if capabilityID <= 0 || enterpriseID < 0 || userID < 0 {
		return false, errors.New("search capability grant query is invalid")
	}
	var total int64
	if err := DB.Model(&SearchCapabilityGrant{}).
		Where("capability_id = ? AND enterprise_id > 0 AND user_id = 0 AND status = ?", capabilityID, SearchCapabilityGrantStatusEnabled).
		Count(&total).Error; err != nil {
		return false, err
	}
	if total == 0 {
		return true, nil
	}
	var matching int64
	err := DB.Model(&SearchCapabilityGrant{}).
		Where("capability_id = ? AND status = ? AND enterprise_id = ? AND user_id = 0", capabilityID, SearchCapabilityGrantStatusEnabled, enterpriseID).
		Count(&matching).Error
	return matching > 0, err
}
