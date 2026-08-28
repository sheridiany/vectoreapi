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
	Id                 int64                           `json:"id"`
	AgentKeyID         int                             `json:"agent_key_id" gorm:"not null;uniqueIndex:idx_search_execution_idempotency,priority:1"`
	KeyHash            string                          `json:"-" gorm:"type:char(64);not null;uniqueIndex:idx_search_execution_idempotency,priority:2"`
	RequestHash        string                          `json:"-" gorm:"type:char(64);not null"`
	ClaimToken         string                          `json:"-" gorm:"type:char(64);index"`
	Status             int                             `json:"status" gorm:"type:int;not null;index"`
	ResponseCiphertext SearchExecutionEncryptedPayload `json:"-"`
	ResponseNonce      string                          `json:"-" gorm:"size:64"`
	ResponseVersion    int                             `json:"-" gorm:"type:int;not null"`
	ExpiresAt          int64                           `json:"expires_at" gorm:"not null;index"`
	CreatedAt          int64                           `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          int64                           `json:"updated_at" gorm:"autoUpdateTime"`
}

type SearchExecutionIdempotencyState int

const (
	SearchExecutionIdempotencyAcquired SearchExecutionIdempotencyState = iota + 1
	SearchExecutionIdempotencyCached
	SearchExecutionIdempotencyPending
	SearchExecutionIdempotencyConflict
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
		ClaimToken: claimToken, Status: SearchExecutionIdempotencyStatusPending, ExpiresAt: expiresAt,
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
	if existing.ExpiresAt <= now {
		result := DB.Model(&SearchExecutionIdempotency{}).
			Where("id = ? AND expires_at <= ?", existing.Id, now).
			Updates(map[string]any{
				"request_hash": requestHash, "status": SearchExecutionIdempotencyStatusPending,
				"response_ciphertext": "", "response_nonce": "", "response_version": 0,
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
	default:
		return nil, 0, errors.New("search execution idempotency state is invalid")
	}
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
