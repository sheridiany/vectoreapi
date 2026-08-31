package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	WalletPreConsumeStatusConsumed = "consumed"
	WalletPreConsumeStatusRefunded = "refunded"
)

var ErrWalletPreConsumeInsufficient = errors.New("wallet quota insufficient")

// WalletPreConsumeRecord makes wallet reservation and compensation durable and
// idempotent for request-scoped billing flows.
type WalletPreConsumeRecord struct {
	Id          int64  `json:"id"`
	RequestID   string `json:"request_id" gorm:"size:64;not null;uniqueIndex"`
	UserID      int    `json:"user_id" gorm:"not null;index"`
	PreConsumed int    `json:"pre_consumed" gorm:"type:int;not null"`
	Status      string `json:"status" gorm:"size:24;not null;index"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func normalizeWalletPreConsume(requestID string, userID, amount int) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 64 || userID <= 0 || amount <= 0 {
		return "", errors.New("wallet pre-consume request is invalid")
	}
	return requestID, nil
}

// PreConsumeUserWallet atomically reserves wallet quota and records the
// request. Replaying the same request is a no-op; a refunded request cannot be
// consumed again.
func PreConsumeUserWallet(requestID string, userID, amount int) error {
	requestID, err := normalizeWalletPreConsume(requestID, userID, amount)
	if err != nil {
		return err
	}
	if err := invalidateUserCache(userID); err != nil {
		common.SysLog("failed to invalidate user quota cache before wallet pre-consume: " + err.Error())
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		var existing WalletPreConsumeRecord
		query := lockForUpdate(tx).Where("request_id = ?", requestID).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.UserID != userID || existing.PreConsumed != amount {
				return errors.New("wallet pre-consume request conflicts with existing record")
			}
			switch existing.Status {
			case WalletPreConsumeStatusConsumed:
				return nil
			case WalletPreConsumeStatusRefunded:
				return errors.New("wallet pre-consume already refunded")
			default:
				return errors.New("wallet pre-consume status is invalid")
			}
		}

		record := &WalletPreConsumeRecord{
			RequestID: requestID, UserID: userID, PreConsumed: amount,
			Status: WalletPreConsumeStatusConsumed,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		reserved := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", userID, amount).
			Update("quota", gorm.Expr("quota - ?", amount))
		if reserved.Error != nil {
			return reserved.Error
		}
		if reserved.RowsAffected != 1 {
			return ErrWalletPreConsumeInsufficient
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrWalletPreConsumeInsufficient) {
		var existing WalletPreConsumeRecord
		if lookupErr := DB.Where("request_id = ?", requestID).First(&existing).Error; lookupErr == nil &&
			existing.UserID == userID && existing.PreConsumed == amount && existing.Status == WalletPreConsumeStatusConsumed {
			err = nil
		}
	}
	if cacheErr := invalidateUserCache(userID); cacheErr != nil {
		common.SysLog("failed to invalidate user quota cache after wallet pre-consume: " + cacheErr.Error())
	}
	return err
}

// RefundUserWalletPreConsume atomically returns a request's reservation once.
func RefundUserWalletPreConsume(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 64 {
		return errors.New("wallet refund request is invalid")
	}
	var userID int
	if err := DB.Model(&WalletPreConsumeRecord{}).Select("user_id").Where("request_id = ?", requestID).Scan(&userID).Error; err != nil {
		return err
	}
	if userID > 0 {
		if err := invalidateUserCache(userID); err != nil {
			common.SysLog("failed to invalidate user quota cache before wallet refund: " + err.Error())
		}
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var record WalletPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", requestID).First(&record).Error; err != nil {
			return err
		}
		userID = record.UserID
		if record.Status == WalletPreConsumeStatusRefunded {
			return nil
		}
		if record.Status != WalletPreConsumeStatusConsumed || record.PreConsumed <= 0 {
			return errors.New("wallet pre-consume record cannot be refunded")
		}
		credited := tx.Model(&User{}).Where("id = ?", record.UserID).
			Update("quota", gorm.Expr("quota + ?", record.PreConsumed))
		if credited.Error != nil {
			return credited.Error
		}
		if credited.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		updated := tx.Model(&WalletPreConsumeRecord{}).
			Where("id = ? AND status = ?", record.Id, WalletPreConsumeStatusConsumed).
			Updates(map[string]any{"status": WalletPreConsumeStatusRefunded, "updated_at": common.GetTimestamp()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("wallet pre-consume refund lost ownership: %w", gorm.ErrRecordNotFound)
		}
		return nil
	})
	if userID > 0 {
		if cacheErr := invalidateUserCache(userID); cacheErr != nil {
			common.SysLog("failed to invalidate user quota cache after wallet refund: " + cacheErr.Error())
		}
	}
	return err
}

func GetWalletPreConsumeRecord(requestID string) (*WalletPreConsumeRecord, error) {
	record := &WalletPreConsumeRecord{}
	if err := DB.Where("request_id = ?", strings.TrimSpace(requestID)).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func CleanupWalletPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	pendingSearchRequests := DB.Model(&SearchUsageEvent{}).
		Select("request_id").Where("status = ? OR billing_state IN ?", SearchUsageStatusPending, []string{
		SearchUsageBillingReservePending, SearchUsageBillingReserved,
		SearchUsageBillingRefundPending, SearchUsageBillingRefundFailed,
	})
	result := DB.Where("updated_at < ? AND request_id NOT IN (?)", cutoff, pendingSearchRequests).
		Delete(&WalletPreConsumeRecord{})
	return result.RowsAffected, result.Error
}
