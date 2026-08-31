package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const retiredSearchUpstreamProviderCode = "PROVIDER_RETIRED"

func migrateUnsupportedSearchUpstreamProviders() error {
	supportedProviders := []string{
		SearchUpstreamProviderJustOneAPI,
		SearchUpstreamProviderTikHub,
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var unsupportedAccountIDs []int
		if err := tx.Model(&SearchUpstreamAccount{}).
			Where("provider IS NULL OR provider NOT IN ?", supportedProviders).
			Pluck("id", &unsupportedAccountIDs).Error; err != nil {
			return err
		}
		if len(unsupportedAccountIDs) == 0 {
			return nil
		}

		var affectedCapabilityIDs []int
		if err := tx.Model(&SearchCapabilityBinding{}).
			Where("upstream_account_id IN ?", unsupportedAccountIDs).
			Distinct("capability_id").Pluck("capability_id", &affectedCapabilityIDs).Error; err != nil {
			return err
		}

		if err := tx.Model(&SearchCapabilityBinding{}).
			Where("upstream_account_id IN ?", unsupportedAccountIDs).
			Where("status <> ?", SearchCapabilityBindingStatusDisabled).
			Updates(map[string]any{
				"status":     SearchCapabilityBindingStatusDisabled,
				"updated_at": common.GetTimestamp(),
			}).Error; err != nil {
			return err
		}
		if len(affectedCapabilityIDs) > 0 {
			var stillRoutableCapabilityIDs []int
			if err := tx.Model(&SearchCapabilityBinding{}).
				Where("capability_id IN ? AND status = ?", affectedCapabilityIDs, SearchCapabilityBindingStatusEnabled).
				Distinct("capability_id").Pluck("capability_id", &stillRoutableCapabilityIDs).Error; err != nil {
				return err
			}
			stillRoutable := make(map[int]struct{}, len(stillRoutableCapabilityIDs))
			for _, capabilityID := range stillRoutableCapabilityIDs {
				stillRoutable[capabilityID] = struct{}{}
			}
			retiredCapabilityIDs := make([]int, 0, len(affectedCapabilityIDs))
			for _, capabilityID := range affectedCapabilityIDs {
				if _, exists := stillRoutable[capabilityID]; !exists {
					retiredCapabilityIDs = append(retiredCapabilityIDs, capabilityID)
				}
			}
			if len(retiredCapabilityIDs) > 0 {
				if err := tx.Model(&SearchCapability{}).
					Where("id IN ? AND status <> ?", retiredCapabilityIDs, SearchCapabilityStatusDisabled).
					Updates(map[string]any{"status": SearchCapabilityStatusDisabled, "updated_at": common.GetTimestamp()}).Error; err != nil {
					return err
				}
				if err := tx.Where("id IN ?", retiredCapabilityIDs).Delete(&SearchCapability{}).Error; err != nil {
					return err
				}
			}
		}

		return tx.Model(&SearchUpstreamAccount{}).
			Where("id IN ?", unsupportedAccountIDs).
			Where("status <> ? OR last_error_code IS NULL OR last_error_code <> ?", SearchUpstreamAccountStatusPaused, retiredSearchUpstreamProviderCode).
			Updates(map[string]any{
				"status":             SearchUpstreamAccountStatusPaused,
				"last_error_code":    retiredSearchUpstreamProviderCode,
				"last_error_message": "旧上游类型已停用，请改用 JustOneAPI 或 TikHub。",
				"updated_at":         common.GetTimestamp(),
			}).Error
	})
}
