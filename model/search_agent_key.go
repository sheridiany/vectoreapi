package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SearchAgentKeyStatusActive   = 1
	SearchAgentKeyStatusDisabled = 2
	SearchAgentKeyStatusRevoked  = 3
)

var searchAgentKeyScopes = map[string]struct{}{
	"web-search": {},
	"extract":    {},
	"social":     {},
	"finance":    {},
	"news":       {},
	"company":    {},
	"travel":     {},
	"jobs":       {},
}

// SearchAgentKey is the credential for search.vectorepoch capabilities.
// The raw secret is deliberately never persisted.
type SearchAgentKey struct {
	Id                int            `json:"id"`
	UserId            int            `json:"user_id" gorm:"not null;index"`
	EnterpriseID      int            `json:"enterprise_id" gorm:"index"`
	Name              string         `json:"name" gorm:"size:64;not null"`
	KeyHash           string         `json:"-" gorm:"size:64;uniqueIndex"`
	KeyPrefix         string         `json:"key_prefix" gorm:"size:32;not null"`
	CredentialVersion int            `json:"-" gorm:"not null;default:0"`
	Status            int            `json:"status" gorm:"type:int;not null;index"`
	Scopes            string         `json:"-" gorm:"type:text;not null"`
	CreatedAt         int64          `json:"created_at" gorm:"autoCreateTime;index"`
	LastUsedAt        int64          `json:"last_used_at" gorm:"index"`
	ExpiresAt         int64          `json:"expires_at" gorm:"index"`
	UpdatedAt         int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

func NormalizeSearchAgentKeyScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{"web-search", "extract", "social", "finance", "news", "company", "travel", "jobs"}, nil
	}
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if _, ok := searchAgentKeyScopes[scope]; !ok {
			return nil, fmt.Errorf("invalid search agent key scope: %s", scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	if len(result) == 0 {
		return nil, errors.New("at least one search agent key scope is required")
	}
	return result, nil
}

func (key *SearchAgentKey) SetScopes(scopes []string) error {
	normalized, err := NormalizeSearchAgentKeyScopes(scopes)
	if err != nil {
		return err
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	key.Scopes = string(data)
	return nil
}

func (key *SearchAgentKey) GetScopes() ([]string, error) {
	if strings.TrimSpace(key.Scopes) == "" {
		return NormalizeSearchAgentKeyScopes(nil)
	}
	var scopes []string
	if err := common.UnmarshalJsonStr(key.Scopes, &scopes); err != nil {
		return nil, err
	}
	return NormalizeSearchAgentKeyScopes(scopes)
}

func (key *SearchAgentKey) HasScope(scope string) bool {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		return false
	}
	scopes, err := key.GetScopes()
	if err != nil {
		return false
	}
	for _, allowed := range scopes {
		if allowed == scope {
			return true
		}
	}
	return false
}

func HashSearchAgentKeySecret(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}

func NewSearchAgentKey(userID, enterpriseID int, name string, scopes []string) (*SearchAgentKey, string, error) {
	name = strings.TrimSpace(name)
	if userID <= 0 {
		return nil, "", errors.New("user id is invalid")
	}
	if name == "" || len([]rune(name)) > 64 {
		return nil, "", errors.New("search agent key name must be between 1 and 64 characters")
	}
	keyScopes, err := NormalizeSearchAgentKeyScopes(scopes)
	if err != nil {
		return nil, "", err
	}
	secret, err := GenerateSearchAgentKeySecret()
	if err != nil {
		return nil, "", err
	}
	key := &SearchAgentKey{
		UserId:            userID,
		EnterpriseID:      enterpriseID,
		Name:              name,
		KeyHash:           HashSearchAgentKeySecret(secret),
		KeyPrefix:         secret[:15],
		CredentialVersion: 1,
		Status:            SearchAgentKeyStatusActive,
	}
	if err := key.SetScopes(keyScopes); err != nil {
		return nil, "", err
	}
	return key, secret, nil
}

func CreateSearchAgentKey(userID, enterpriseID int, name string, scopes []string) (*SearchAgentKey, string, error) {
	key, secret, err := NewSearchAgentKey(userID, enterpriseID, name, scopes)
	if err != nil {
		return nil, "", err
	}
	if err := DB.Create(key).Error; err != nil {
		return nil, "", err
	}
	return key, secret, nil
}

func GetSearchAgentKeysByUserID(userID int) ([]*SearchAgentKey, error) {
	if userID <= 0 {
		return nil, errors.New("user id is invalid")
	}
	keys := make([]*SearchAgentKey, 0)
	err := DB.Where("user_id = ?", userID).Order("id desc").Find(&keys).Error
	return keys, err
}

func GetSearchAgentKeysByEnterpriseID(enterpriseID int) ([]*SearchAgentKey, error) {
	if enterpriseID <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	keys := make([]*SearchAgentKey, 0)
	err := DB.Where("enterprise_id = ?", enterpriseID).Order("id desc").Find(&keys).Error
	return keys, err
}

func GetAllSearchAgentKeys() ([]*SearchAgentKey, error) {
	keys := make([]*SearchAgentKey, 0)
	err := DB.Order("id desc").Find(&keys).Error
	return keys, err
}

func GetSearchAgentKeyByID(id int) (*SearchAgentKey, error) {
	if id <= 0 {
		return nil, errors.New("search agent key id is invalid")
	}
	key := &SearchAgentKey{}
	if err := DB.First(key, id).Error; err != nil {
		return nil, err
	}
	return key, nil
}

func RevokeSearchAgentKey(id int, userID int) error {
	if id <= 0 || userID <= 0 {
		return errors.New("search agent key ownership is invalid")
	}
	result := DB.Model(&SearchAgentKey{}).
		Where("id = ? AND user_id = ? AND status <> ?", id, userID, SearchAgentKeyStatusRevoked).
		Updates(map[string]any{"status": SearchAgentKeyStatusRevoked, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func RevokeSearchAgentKeyByID(id int) error {
	if id <= 0 {
		return errors.New("search agent key id is invalid")
	}
	result := DB.Model(&SearchAgentKey{}).
		Where("id = ? AND status <> ?", id, SearchAgentKeyStatusRevoked).
		Updates(map[string]any{"status": SearchAgentKeyStatusRevoked, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func FindSearchAgentKeyBySecret(secret string) (*SearchAgentKey, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("search agent key secret is required")
	}
	key := &SearchAgentKey{}
	if err := DB.Where("key_hash = ?", HashSearchAgentKeySecret(secret)).First(key).Error; err != nil {
		return nil, err
	}
	if key.Status != SearchAgentKeyStatusActive || (key.ExpiresAt > 0 && key.ExpiresAt <= common.GetTimestamp()) {
		return nil, errors.New("search agent key is invalid")
	}
	return key, nil
}

func TouchSearchAgentKey(id int) error {
	if id <= 0 {
		return errors.New("search agent key id is invalid")
	}
	return DB.Model(&SearchAgentKey{}).
		Where("id = ? AND status = ?", id, SearchAgentKeyStatusActive).
		Update("last_used_at", common.GetTimestamp()).Error
}

func InitializeSearchAgentKeyCredentialVersions() error {
	return DB.Model(&SearchAgentKey{}).
		Where("credential_version IS NULL OR credential_version < ?", 1).
		Update("credential_version", 1).Error
}

type searchAgentKeyActivationPayload struct {
	KeyID             int    `json:"key_id"`
	CredentialVersion int    `json:"credential_version"`
	KeyHash           string `json:"key_hash"`
	KeyPrefix         string `json:"key_prefix"`
}

// PrepareSearchAgentKeyRotationWithTx creates a short-lived pending credential
// without invalidating the currently active key. Activation is a separate,
// idempotent transaction after the installer has written every local config.
func PrepareSearchAgentKeyRotationWithTx(tx *gorm.DB, id, expectedCredentialVersion int, expiresAt time.Time) (string, string, error) {
	if tx == nil || id <= 0 || expectedCredentialVersion < 1 || !expiresAt.After(time.Now()) {
		return "", "", errors.New("search agent key activation is invalid")
	}
	now := time.Now()
	var key SearchAgentKey
	if err := lockForUpdate(tx).
		Where("id = ? AND status = ? AND credential_version = ? AND (expires_at = 0 OR expires_at > ?)", id, SearchAgentKeyStatusActive, expectedCredentialVersion, now.Unix()).
		First(&key).Error; err != nil {
		return "", "", err
	}
	secret, err := GenerateSearchAgentKeySecret()
	if err != nil {
		return "", "", err
	}
	payload, err := common.Marshal(searchAgentKeyActivationPayload{
		KeyID:             id,
		CredentialVersion: expectedCredentialVersion,
		KeyHash:           HashSearchAgentKeySecret(secret),
		KeyPrefix:         secret[:15],
	})
	if err != nil {
		return "", "", err
	}
	activationToken, _, err := CreateAuthFlowWithTx(tx, AuthFlowCreate{
		Purpose:   AuthFlowPurposeSearchAgentActivate,
		UserId:    key.UserId,
		Payload:   string(payload),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", "", err
	}
	return secret, activationToken, nil
}

// ActivatePreparedSearchAgentKeyRotation switches to a prepared credential.
// Repeating the same activation token after a lost response returns success
// when the intended credential is already active.
func ActivatePreparedSearchAgentKeyRotation(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, ErrAuthFlowInvalid
	}
	activatedKeyID := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var flow AuthFlow
		query := applyAuthFlowMatch(lockForUpdate(tx), token, AuthFlowMatch{Purpose: AuthFlowPurposeSearchAgentActivate})
		if err := query.First(&flow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuthFlowInvalid
			}
			return err
		}
		var payload searchAgentKeyActivationPayload
		if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil || payload.KeyID <= 0 || payload.CredentialVersion < 1 || len(payload.KeyHash) != 64 || payload.KeyPrefix == "" {
			return ErrAuthFlowInvalid
		}
		var key SearchAgentKey
		if err := lockForUpdate(tx).First(&key, payload.KeyID).Error; err != nil {
			return err
		}
		activatedKeyID = payload.KeyID
		now := time.Now()
		keyUsable := key.Status == SearchAgentKeyStatusActive && (key.ExpiresAt == 0 || key.ExpiresAt > now.Unix())
		if flow.ConsumedAt != nil {
			if keyUsable && key.CredentialVersion == payload.CredentialVersion+1 && key.KeyHash == payload.KeyHash {
				return nil
			}
			return ErrAuthFlowConsumed
		}
		if !flow.ExpiresAt.After(now) {
			return ErrAuthFlowExpired
		}
		result := tx.Model(&SearchAgentKey{}).
			Where("id = ? AND status = ? AND credential_version = ? AND (expires_at = 0 OR expires_at > ?)", payload.KeyID, SearchAgentKeyStatusActive, payload.CredentialVersion, now.Unix()).
			Updates(map[string]any{
				"key_hash":           payload.KeyHash,
				"key_prefix":         payload.KeyPrefix,
				"credential_version": payload.CredentialVersion + 1,
				"updated_at":         now.Unix(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		result = tx.Model(&AuthFlow{}).
			Where("id = ? AND consumed_at IS NULL AND expires_at > ?", flow.Id, now).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAuthFlowConsumed
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return activatedKeyID, nil
}

func GenerateSearchAgentKeySecret() (string, error) {
	randomKey, err := common.GenerateKey()
	if err != nil {
		return "", err
	}
	return "vr_live_" + randomKey, nil
}
