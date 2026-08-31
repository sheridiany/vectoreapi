package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

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
	Id           int            `json:"id"`
	UserId       int            `json:"user_id" gorm:"not null;index"`
	EnterpriseID int            `json:"enterprise_id" gorm:"index"`
	Name         string         `json:"name" gorm:"size:64;not null"`
	KeyHash      string         `json:"-" gorm:"size:64;uniqueIndex"`
	KeyPrefix    string         `json:"key_prefix" gorm:"size:32;not null"`
	Status       int            `json:"status" gorm:"type:int;not null;index"`
	Scopes       string         `json:"-" gorm:"type:text;not null"`
	CreatedAt    int64          `json:"created_at" gorm:"autoCreateTime;index"`
	LastUsedAt   int64          `json:"last_used_at" gorm:"index"`
	ExpiresAt    int64          `json:"expires_at" gorm:"index"`
	UpdatedAt    int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
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
		UserId:       userID,
		EnterpriseID: enterpriseID,
		Name:         name,
		KeyHash:      HashSearchAgentKeySecret(secret),
		KeyPrefix:    secret[:15],
		Status:       SearchAgentKeyStatusActive,
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

func GenerateSearchAgentKeySecret() (string, error) {
	randomKey, err := common.GenerateKey()
	if err != nil {
		return "", err
	}
	return "vr_live_" + randomKey, nil
}
