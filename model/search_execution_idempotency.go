package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	SearchExecutionIdempotencyStatusPending   = 1
	SearchExecutionIdempotencyStatusCompleted = 2
	SearchExecutionIdempotencyStatusResolved  = 3
	searchExecutionUsageTrackingVersion       = 1
)

type SearchExecutionEncryptedPayload string

func (SearchExecutionEncryptedPayload) GormDataType() string {
	return "text"
}

func (SearchExecutionEncryptedPayload) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}

type SearchExecutionIdempotency struct {
	Id                   int64                           `json:"id"`
	AgentKeyID           int                             `json:"agent_key_id" gorm:"not null;uniqueIndex:idx_search_execution_idempotency,priority:1"`
	KeyHash              string                          `json:"-" gorm:"type:char(64);not null;uniqueIndex:idx_search_execution_idempotency,priority:2"`
	RequestHash          string                          `json:"-" gorm:"type:char(64);not null"`
	ClaimToken           string                          `json:"-" gorm:"type:char(64);index"`
	UsageRequestID       string                          `json:"-" gorm:"size:64;index"`
	UsageTrackingVersion int                             `json:"-" gorm:"type:int"`
	Status               int                             `json:"status" gorm:"type:int;not null;index"`
	ResponseCiphertext   SearchExecutionEncryptedPayload `json:"-"`
	ResponseNonce        string                          `json:"-" gorm:"size:64"`
	ResponseVersion      int                             `json:"-" gorm:"type:int;not null"`
	ExpiresAt            int64                           `json:"expires_at" gorm:"not null;index"`
	CreatedAt            int64                           `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64                           `json:"updated_at" gorm:"autoUpdateTime"`
}

type SearchExecutionIdempotencyState int

const (
	SearchExecutionIdempotencyAcquired SearchExecutionIdempotencyState = iota + 1
	SearchExecutionIdempotencyCached
	SearchExecutionIdempotencyPending
	SearchExecutionIdempotencyConflict
	SearchExecutionIdempotencyResolved
)

func BeginSearchExecutionIdempotency(agentKeyID int, keyHash, requestHash string, now, expiresAt int64) (*SearchExecutionIdempotency, SearchExecutionIdempotencyState, error) {
	keyHash = strings.TrimSpace(keyHash)
	requestHash = strings.TrimSpace(requestHash)
	if agentKeyID <= 0 || len(keyHash) != 64 || len(requestHash) != 64 || now <= 0 || expiresAt <= now {
		return nil, 0, errors.New("search execution idempotency request is invalid")
	}
	claimToken, err := newSearchExecutionClaimToken()
	if err != nil {
		return nil, 0, err
	}
	record := &SearchExecutionIdempotency{
		AgentKeyID: agentKeyID, KeyHash: keyHash, RequestHash: requestHash,
		ClaimToken: claimToken, UsageTrackingVersion: searchExecutionUsageTrackingVersion,
		Status: SearchExecutionIdempotencyStatusPending, ExpiresAt: expiresAt,
	}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if created.Error != nil {
		return nil, 0, created.Error
	}
	existing := &SearchExecutionIdempotency{}
	if err := DB.Where("agent_key_id = ? AND key_hash = ?", agentKeyID, keyHash).First(existing).Error; err != nil {
		return nil, 0, err
	}
	if existing.ClaimToken == claimToken {
		return existing, SearchExecutionIdempotencyAcquired, nil
	}
	reclaimable := existing.Status == SearchExecutionIdempotencyStatusCompleted || existing.Status == SearchExecutionIdempotencyStatusResolved
	if existing.Status == SearchExecutionIdempotencyStatusPending && existing.ExpiresAt <= now {
		reclaimable, err = expiredSearchExecutionIdempotencyReclaimable(existing)
		if err != nil {
			return nil, 0, err
		}
	}
	if reclaimable && existing.ExpiresAt <= now {
		result := DB.Model(&SearchExecutionIdempotency{}).
			Where("id = ? AND status = ? AND claim_token = ? AND expires_at <= ?", existing.Id, existing.Status, existing.ClaimToken, now).
			Updates(map[string]any{
				"request_hash": requestHash, "status": SearchExecutionIdempotencyStatusPending,
				"response_ciphertext": "", "response_nonce": "", "response_version": 0,
				"usage_request_id": "", "usage_tracking_version": searchExecutionUsageTrackingVersion,
				"claim_token": claimToken, "expires_at": expiresAt, "updated_at": now,
			})
		if result.Error != nil {
			return nil, 0, result.Error
		}
		if err := DB.Where("agent_key_id = ? AND key_hash = ?", agentKeyID, keyHash).First(existing).Error; err != nil {
			return nil, 0, err
		}
		if existing.ClaimToken == claimToken {
			return existing, SearchExecutionIdempotencyAcquired, nil
		}
	}
	if existing.RequestHash != requestHash {
		return existing, SearchExecutionIdempotencyConflict, nil
	}
	switch existing.Status {
	case SearchExecutionIdempotencyStatusCompleted:
		return existing, SearchExecutionIdempotencyCached, nil
	case SearchExecutionIdempotencyStatusPending:
		return existing, SearchExecutionIdempotencyPending, nil
	case SearchExecutionIdempotencyStatusResolved:
		return existing, SearchExecutionIdempotencyResolved, nil
	default:
		return nil, 0, errors.New("search execution idempotency state is invalid")
	}
}

func AttachSearchExecutionUsage(id int64, requestHash, claimToken, usageRequestID string) error {
	requestHash = strings.TrimSpace(requestHash)
	claimToken = strings.TrimSpace(claimToken)
	usageRequestID = strings.TrimSpace(usageRequestID)
	if id <= 0 || len(requestHash) != 64 || len(claimToken) != 64 || usageRequestID == "" || len(usageRequestID) > 64 {
		return errors.New("search execution idempotency usage attachment is invalid")
	}
	result := DB.Model(&SearchExecutionIdempotency{}).
		Where("id = ? AND request_hash = ? AND claim_token = ? AND status = ? AND usage_tracking_version = ? AND usage_request_id = ?", id, requestHash, claimToken, SearchExecutionIdempotencyStatusPending, searchExecutionUsageTrackingVersion, "").
		Updates(map[string]any{"usage_request_id": usageRequestID, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var existing SearchExecutionIdempotency
	if err := DB.Where("id = ?", id).First(&existing).Error; err != nil {
		return err
	}
	if existing.RequestHash == requestHash && existing.ClaimToken == claimToken && existing.Status == SearchExecutionIdempotencyStatusPending && existing.UsageTrackingVersion == searchExecutionUsageTrackingVersion && existing.UsageRequestID == usageRequestID {
		return nil
	}
	return gorm.ErrRecordNotFound
}

func ResolveSearchExecutionIdempotencyByUsageRequestID(usageRequestID string) error {
	usageRequestID = strings.TrimSpace(usageRequestID)
	if usageRequestID == "" || len(usageRequestID) > 64 {
		return errors.New("search execution idempotency resolution is invalid")
	}
	return resolveSearchExecutionIdempotencyByUsageRequestIDTx(DB, usageRequestID)
}

func resolveSearchExecutionIdempotencyByUsageRequestIDTx(tx *gorm.DB, usageRequestID string) error {
	return tx.Model(&SearchExecutionIdempotency{}).
		Where("usage_request_id = ? AND status = ?", usageRequestID, SearchExecutionIdempotencyStatusPending).
		Updates(map[string]any{"status": SearchExecutionIdempotencyStatusResolved, "updated_at": common.GetTimestamp()}).Error
}

func expiredSearchExecutionIdempotencyReclaimable(record *SearchExecutionIdempotency) (bool, error) {
	if record == nil || record.Status != SearchExecutionIdempotencyStatusPending {
		return false, nil
	}
	if strings.TrimSpace(record.UsageRequestID) == "" {
		return record.UsageTrackingVersion == searchExecutionUsageTrackingVersion, nil
	}
	var usage SearchUsageEvent
	if err := DB.Where("request_id = ?", record.UsageRequestID).First(&usage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return usage.ExecutionPhase == SearchUsagePhasePrepared &&
		usage.Status == SearchUsageStatusFailed &&
		(usage.BillingState == SearchUsageBillingNotStarted || usage.BillingState == SearchUsageBillingRefunded), nil
}

func CompleteSearchExecutionIdempotency(id int64, requestHash, claimToken, ciphertext, nonce string, version int) error {
	requestHash = strings.TrimSpace(requestHash)
	claimToken = strings.TrimSpace(claimToken)
	if id <= 0 || len(requestHash) != 64 || len(claimToken) != 64 || strings.TrimSpace(ciphertext) == "" || strings.TrimSpace(nonce) == "" || version <= 0 {
		return errors.New("search execution idempotency completion is invalid")
	}
	result := DB.Model(&SearchExecutionIdempotency{}).
		Where("id = ? AND request_hash = ? AND claim_token = ? AND status = ?", id, requestHash, claimToken, SearchExecutionIdempotencyStatusPending).
		Updates(map[string]any{
			"status":              SearchExecutionIdempotencyStatusCompleted,
			"response_ciphertext": ciphertext, "response_nonce": nonce, "response_version": version,
			"updated_at": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ReleaseSearchExecutionIdempotency(id int64, requestHash, claimToken string) error {
	requestHash = strings.TrimSpace(requestHash)
	claimToken = strings.TrimSpace(claimToken)
	if id <= 0 || len(requestHash) != 64 || len(claimToken) != 64 {
		return errors.New("search execution idempotency release is invalid")
	}
	return DB.Where("id = ? AND request_hash = ? AND claim_token = ? AND status = ?", id, requestHash, claimToken, SearchExecutionIdempotencyStatusPending).
		Delete(&SearchExecutionIdempotency{}).Error
}

func newSearchExecutionClaimToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return hex.EncodeToString(random), nil
}
