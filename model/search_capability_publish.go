package model

import (
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type SearchCapabilityPublishConfig struct {
	ID                   int
	PriceMicros          int64
	ExpectedInputSchema  string
	ExpectedSchemaStatus int
}

var ErrSearchCapabilityPublishStateChanged = errors.New("search capability publish state changed")

func ListSearchCapabilitiesByPublicIDs(publicIDs []string) ([]*SearchCapability, error) {
	capabilities := make([]*SearchCapability, 0)
	if len(publicIDs) == 0 {
		return capabilities, nil
	}
	return capabilities, DB.Where("public_id IN ?", publicIDs).
		Order("id asc").Find(&capabilities).Error
}

func PublishSearchCapabilities(configs []SearchCapabilityPublishConfig, clearGrants bool) error {
	if len(configs) == 0 {
		return nil
	}
	ids := make([]int, 0, len(configs))
	seen := make(map[int]struct{}, len(configs))
	for _, config := range configs {
		if config.ID <= 0 || config.PriceMicros < 0 || config.ExpectedSchemaStatus != SearchCapabilitySchemaAvailable {
			return errors.New("search capability publish configuration is invalid")
		}
		if _, exists := seen[config.ID]; exists {
			return errors.New("search capability publish configuration is duplicated")
		}
		seen[config.ID] = struct{}{}
		ids = append(ids, config.ID)
	}
	sortedConfigs := append([]SearchCapabilityPublishConfig(nil), configs...)
	sort.Slice(sortedConfigs, func(i, j int) bool { return sortedConfigs[i].ID < sortedConfigs[j].ID })

	return DB.Transaction(func(tx *gorm.DB) error {
		lockedConfigs := make([]SearchCapabilityPublishConfig, 0, len(sortedConfigs))
		for _, config := range sortedConfigs {
			var capability SearchCapability
			if err := lockForUpdate(tx).Where("id = ?", config.ID).First(&capability).Error; err != nil {
				return err
			}
			if capability.SchemaStatus != config.ExpectedSchemaStatus || capability.InputSchema != config.ExpectedInputSchema {
				return ErrSearchCapabilityPublishStateChanged
			}

			bindings := make([]SearchCapabilityBinding, 0)
			if err := lockForUpdate(tx).
				Where("capability_id = ? AND status = ?", config.ID, SearchCapabilityBindingStatusEnabled).
				Order("priority asc, weight desc, id asc").Find(&bindings).Error; err != nil {
				return err
			}
			accountIDs := make([]int, 0, len(bindings))
			accountIDSet := make(map[int]struct{}, len(bindings))
			for _, binding := range bindings {
				if _, exists := accountIDSet[binding.UpstreamAccountID]; exists {
					continue
				}
				accountIDSet[binding.UpstreamAccountID] = struct{}{}
				accountIDs = append(accountIDs, binding.UpstreamAccountID)
			}
			accounts := make([]SearchUpstreamAccount, 0, len(accountIDs))
			if len(accountIDs) > 0 {
				if err := lockForUpdate(tx).Where("id IN ?", accountIDs).Order("id asc").Find(&accounts).Error; err != nil {
					return err
				}
			}
			accountsByID := make(map[int]SearchUpstreamAccount, len(accounts))
			poolIDs := make([]int, 0, len(accounts))
			poolIDSet := make(map[int]struct{}, len(accounts))
			for _, account := range accounts {
				accountsByID[account.Id] = account
				if _, exists := poolIDSet[account.PoolID]; exists {
					continue
				}
				poolIDSet[account.PoolID] = struct{}{}
				poolIDs = append(poolIDs, account.PoolID)
			}
			pools := make([]SearchUpstreamPool, 0, len(poolIDs))
			if len(poolIDs) > 0 {
				if err := lockForUpdate(tx).Where("id IN ?", poolIDs).Order("id asc").Find(&pools).Error; err != nil {
					return err
				}
			}
			poolsByID := make(map[int]SearchUpstreamPool, len(pools))
			for _, pool := range pools {
				poolsByID[pool.Id] = pool
			}

			priceFloor := config.PriceMicros
			if capability.PriceMicros > priceFloor {
				priceFloor = capability.PriceMicros
			}
			if capability.UpstreamCostMicros > priceFloor {
				priceFloor = capability.UpstreamCostMicros
			}
			healthyRoute := false
			for _, binding := range bindings {
				account, accountExists := accountsByID[binding.UpstreamAccountID]
				pool, poolExists := poolsByID[account.PoolID]
				if !accountExists || !poolExists || account.Status != SearchUpstreamAccountStatusHealthy || pool.Status != SearchUpstreamPoolStatusEnabled {
					continue
				}
				if binding.InputSchema != capability.InputSchema {
					continue
				}
				healthyRoute = true
				if binding.UpstreamCostMicros > priceFloor {
					priceFloor = binding.UpstreamCostMicros
				}
			}
			if !healthyRoute {
				return ErrSearchCapabilityPublishStateChanged
			}
			config.PriceMicros = priceFloor
			lockedConfigs = append(lockedConfigs, config)
		}

		if clearGrants {
			if err := tx.Where("capability_id IN ?", ids).Delete(&SearchCapabilityGrant{}).Error; err != nil {
				return err
			}
		}
		now := common.GetTimestamp()
		for _, config := range lockedConfigs {
			result := tx.Model(&SearchCapability{}).Where("id = ?", config.ID).Updates(map[string]any{
				"status": SearchCapabilityStatusEnabled,
				"price_micros": gorm.Expr(
					"CASE WHEN price_micros < ? THEN ? ELSE price_micros END",
					config.PriceMicros, config.PriceMicros,
				),
				"updated_at": now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				var count int64
				if err := tx.Model(&SearchCapability{}).Where("id = ?", config.ID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					return gorm.ErrRecordNotFound
				}
			}
		}
		return nil
	})
}
