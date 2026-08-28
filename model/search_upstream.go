package model

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SearchUpstreamPoolStatusEnabled  = 1
	SearchUpstreamPoolStatusDisabled = 2

	SearchUpstreamAccountStatusHealthy = 1
	SearchUpstreamAccountStatusWarning = 2
	SearchUpstreamAccountStatusStandby = 3
	SearchUpstreamAccountStatusPaused  = 4

	SearchUpstreamPoolStrategyWeighted = "weighted"
	SearchUpstreamPoolStrategyFailover = "failover"
	SearchUpstreamPoolStrategySticky   = "sticky"

	SearchUpstreamProviderAgentKeyMCP = "agentkey_mcp"
)

var (
	ErrSearchUpstreamURLInvalid       = errors.New("search upstream base url is invalid")
	ErrSearchUpstreamURLHTTPSRequired = errors.New("search upstream base url must use https")
)

type SearchUpstreamPool struct {
	Id          int            `json:"id"`
	Name        string         `json:"name" gorm:"size:64;not null;uniqueIndex"`
	Strategy    string         `json:"strategy" gorm:"size:16;not null"`
	Description string         `json:"description" gorm:"size:255"`
	Status      int            `json:"status" gorm:"type:int;not null;index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type SearchUpstreamAccount struct {
	Id                 int            `json:"id"`
	PoolID             int            `json:"pool_id" gorm:"not null;index"`
	Provider           string         `json:"provider" gorm:"size:32;not null;index"`
	Name               string         `json:"name" gorm:"size:64;not null"`
	BaseURL            string         `json:"base_url" gorm:"size:512;not null"`
	SecretCiphertext   string         `json:"-" gorm:"type:text;not null"`
	SecretNonce        string         `json:"-" gorm:"size:64;not null"`
	SecretVersion      int            `json:"-" gorm:"type:int;not null"`
	SecretPrefix       string         `json:"secret_prefix" gorm:"size:32;not null"`
	Plan               string         `json:"plan" gorm:"size:64"`
	BalanceMicros      int64          `json:"balance_micros" gorm:"not null"`
	Weight             int            `json:"weight" gorm:"type:int;not null"`
	Priority           int            `json:"priority" gorm:"type:int;not null"`
	Status             int            `json:"status" gorm:"type:int;not null;index"`
	FailureCount       int            `json:"failure_count" gorm:"type:int;not null"`
	ConcurrentRequests int            `json:"concurrent_requests" gorm:"type:int;not null"`
	LastCheckedAt      int64          `json:"last_checked_at" gorm:"index"`
	LastErrorCode      string         `json:"last_error_code,omitempty" gorm:"size:64"`
	LastErrorMessage   string         `json:"last_error_message,omitempty" gorm:"size:255"`
	CreatedAt          int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

func searchUpdateError(result *gorm.DB, target any, id int) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := DB.Model(target).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func normalizeSearchUpstreamPool(pool *SearchUpstreamPool) error {
	if pool == nil {
		return errors.New("search upstream pool is required")
	}
	pool.Name = strings.TrimSpace(pool.Name)
	pool.Description = strings.TrimSpace(pool.Description)
	if pool.Name == "" || len([]rune(pool.Name)) > 64 {
		return errors.New("search upstream pool name must be between 1 and 64 characters")
	}
	if len([]rune(pool.Description)) > 255 {
		return errors.New("search upstream pool description is too long")
	}
	switch pool.Strategy {
	case "":
		pool.Strategy = SearchUpstreamPoolStrategyWeighted
	case SearchUpstreamPoolStrategyWeighted, SearchUpstreamPoolStrategyFailover, SearchUpstreamPoolStrategySticky:
	default:
		return errors.New("search upstream pool strategy is invalid")
	}
	if pool.Status == 0 {
		pool.Status = SearchUpstreamPoolStatusEnabled
	}
	if pool.Status != SearchUpstreamPoolStatusEnabled && pool.Status != SearchUpstreamPoolStatusDisabled {
		return errors.New("search upstream pool status is invalid")
	}
	return nil
}

func normalizeSearchUpstreamAccount(account *SearchUpstreamAccount) error {
	if account == nil {
		return errors.New("search upstream account is required")
	}
	account.Name = strings.TrimSpace(account.Name)
	account.Provider = strings.TrimSpace(account.Provider)
	account.BaseURL = strings.TrimSpace(account.BaseURL)
	account.SecretPrefix = strings.TrimSpace(account.SecretPrefix)
	account.Plan = strings.TrimSpace(account.Plan)
	account.LastErrorCode = strings.TrimSpace(account.LastErrorCode)
	account.LastErrorMessage = strings.TrimSpace(account.LastErrorMessage)
	if account.PoolID <= 0 {
		return errors.New("search upstream pool id is invalid")
	}
	if account.Name == "" || len([]rune(account.Name)) > 64 {
		return errors.New("search upstream account name must be between 1 and 64 characters")
	}
	if account.Provider == "" {
		account.Provider = SearchUpstreamProviderAgentKeyMCP
	}
	if account.Provider != SearchUpstreamProviderAgentKeyMCP {
		return errors.New("search upstream provider is unsupported")
	}
	parsedURL, err := ValidateSearchUpstreamBaseURL(account.BaseURL, SearchUpstreamLoopbackHTTPEnabled())
	if err != nil {
		return err
	}
	account.BaseURL = parsedURL.String()
	if account.SecretCiphertext == "" || account.SecretNonce == "" || account.SecretVersion <= 0 || account.SecretPrefix == "" {
		return errors.New("search upstream encrypted secret is required")
	}
	if len(account.SecretNonce) > 64 || len([]rune(account.SecretPrefix)) > 32 || len([]rune(account.Plan)) > 64 || len([]rune(account.LastErrorCode)) > 64 || len([]rune(account.LastErrorMessage)) > 255 {
		return errors.New("search upstream account text value is too long")
	}
	if account.BalanceMicros < 0 || account.FailureCount < 0 || account.ConcurrentRequests < 0 {
		return errors.New("search upstream account numeric value is invalid")
	}
	if account.Weight == 0 {
		account.Weight = 1
	}
	if account.Weight < 1 || account.Weight > 100 || account.Priority < 0 {
		return errors.New("search upstream routing configuration is invalid")
	}
	if account.Status == 0 {
		account.Status = SearchUpstreamAccountStatusStandby
	}
	switch account.Status {
	case SearchUpstreamAccountStatusHealthy, SearchUpstreamAccountStatusWarning, SearchUpstreamAccountStatusStandby, SearchUpstreamAccountStatusPaused:
	default:
		return errors.New("search upstream account status is invalid")
	}
	return nil
}

func SearchUpstreamLoopbackHTTPEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("VSEARCH_ALLOW_LOOPBACK_HTTP")), "true")
}

func ValidateSearchUpstreamBaseURL(rawURL string, allowLoopbackHTTP bool) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || len(rawURL) > 512 || strings.Contains(rawURL, "#") {
		return nil, ErrSearchUpstreamURLInvalid
	}
	endpoint, err := url.ParseRequestURI(rawURL)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, ErrSearchUpstreamURLInvalid
	}
	if endpoint.Scheme == "https" {
		return endpoint, nil
	}
	if endpoint.Scheme != "http" || !allowLoopbackHTTP || !isSearchUpstreamLoopbackHost(endpoint.Hostname()) {
		return nil, ErrSearchUpstreamURLHTTPSRequired
	}
	return endpoint, nil
}

func isSearchUpstreamLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func CreateSearchUpstreamPool(pool *SearchUpstreamPool) error {
	if err := normalizeSearchUpstreamPool(pool); err != nil {
		return err
	}
	return DB.Create(pool).Error
}

func UpdateSearchUpstreamPool(pool *SearchUpstreamPool) error {
	if pool == nil || pool.Id <= 0 {
		return errors.New("search upstream pool id is invalid")
	}
	if err := normalizeSearchUpstreamPool(pool); err != nil {
		return err
	}
	result := DB.Model(&SearchUpstreamPool{}).Where("id = ?", pool.Id).Updates(map[string]any{
		"name":        pool.Name,
		"strategy":    pool.Strategy,
		"description": pool.Description,
		"status":      pool.Status,
		"updated_at":  common.GetTimestamp(),
	})
	return searchUpdateError(result, &SearchUpstreamPool{}, pool.Id)
}

func GetSearchUpstreamPoolByID(id int) (*SearchUpstreamPool, error) {
	if id <= 0 {
		return nil, errors.New("search upstream pool id is invalid")
	}
	pool := &SearchUpstreamPool{}
	return pool, DB.First(pool, id).Error
}

func ListSearchUpstreamPools() ([]*SearchUpstreamPool, error) {
	pools := make([]*SearchUpstreamPool, 0)
	return pools, DB.Order("id asc").Find(&pools).Error
}

func DeleteSearchUpstreamPool(id int) error {
	if id <= 0 {
		return errors.New("search upstream pool id is invalid")
	}
	var accountCount int64
	if err := DB.Model(&SearchUpstreamAccount{}).Where("pool_id = ?", id).Count(&accountCount).Error; err != nil {
		return err
	}
	if accountCount > 0 {
		return errors.New("search upstream pool still has accounts")
	}
	result := DB.Delete(&SearchUpstreamPool{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func CreateSearchUpstreamAccount(account *SearchUpstreamAccount) error {
	if err := normalizeSearchUpstreamAccount(account); err != nil {
		return err
	}
	return DB.Create(account).Error
}

func UpdateSearchUpstreamAccount(account *SearchUpstreamAccount) error {
	if account == nil || account.Id <= 0 {
		return errors.New("search upstream account id is invalid")
	}
	if err := normalizeSearchUpstreamAccount(account); err != nil {
		return err
	}
	result := DB.Model(&SearchUpstreamAccount{}).Where("id = ?", account.Id).Updates(map[string]any{
		"pool_id":             account.PoolID,
		"provider":            account.Provider,
		"name":                account.Name,
		"base_url":            account.BaseURL,
		"secret_ciphertext":   account.SecretCiphertext,
		"secret_nonce":        account.SecretNonce,
		"secret_version":      account.SecretVersion,
		"secret_prefix":       account.SecretPrefix,
		"plan":                account.Plan,
		"balance_micros":      account.BalanceMicros,
		"weight":              account.Weight,
		"priority":            account.Priority,
		"status":              account.Status,
		"failure_count":       account.FailureCount,
		"concurrent_requests": account.ConcurrentRequests,
		"last_checked_at":     account.LastCheckedAt,
		"last_error_code":     account.LastErrorCode,
		"last_error_message":  account.LastErrorMessage,
		"updated_at":          common.GetTimestamp(),
	})
	return searchUpdateError(result, &SearchUpstreamAccount{}, account.Id)
}

func GetSearchUpstreamAccountByID(id int) (*SearchUpstreamAccount, error) {
	if id <= 0 {
		return nil, errors.New("search upstream account id is invalid")
	}
	account := &SearchUpstreamAccount{}
	return account, DB.First(account, id).Error
}

func ListSearchUpstreamAccounts() ([]*SearchUpstreamAccount, error) {
	accounts := make([]*SearchUpstreamAccount, 0)
	return accounts, DB.Order("priority asc, id asc").Find(&accounts).Error
}

func ListAvailableSearchUpstreamAccounts(poolID int) ([]*SearchUpstreamAccount, error) {
	if poolID <= 0 {
		return nil, errors.New("search upstream pool id is invalid")
	}
	accounts := make([]*SearchUpstreamAccount, 0)
	err := DB.Where("pool_id = ? AND status IN ?", poolID, []int{SearchUpstreamAccountStatusHealthy, SearchUpstreamAccountStatusStandby}).
		Order("priority asc, weight desc, id asc").Find(&accounts).Error
	return accounts, err
}

func DeleteSearchUpstreamAccount(id int) error {
	if id <= 0 {
		return errors.New("search upstream account id is invalid")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var account SearchUpstreamAccount
		if err := tx.First(&account, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&SearchCapabilityBinding{}).
			Where("upstream_account_id = ?", id).
			Update("status", SearchCapabilityBindingStatusDisabled).Error; err != nil {
			return err
		}
		return tx.Delete(&account).Error
	})
}

func UpdateSearchUpstreamAccountHealth(id, status int, balanceMicros int64, failureCount int, errorCode, errorMessage string) error {
	if id <= 0 || balanceMicros < 0 || failureCount < 0 {
		return errors.New("search upstream health update is invalid")
	}
	switch status {
	case SearchUpstreamAccountStatusHealthy, SearchUpstreamAccountStatusWarning, SearchUpstreamAccountStatusStandby, SearchUpstreamAccountStatusPaused:
	default:
		return errors.New("search upstream account status is invalid")
	}
	errorCodeRunes := []rune(strings.TrimSpace(errorCode))
	if len(errorCodeRunes) > 64 {
		errorCodeRunes = errorCodeRunes[:64]
	}
	errorMessageRunes := []rune(strings.TrimSpace(errorMessage))
	if len(errorMessageRunes) > 255 {
		errorMessageRunes = errorMessageRunes[:255]
	}
	now := common.GetTimestamp()
	result := DB.Model(&SearchUpstreamAccount{}).Where("id = ?", id).Updates(map[string]any{
		"status":             status,
		"balance_micros":     balanceMicros,
		"failure_count":      failureCount,
		"last_checked_at":    now,
		"last_error_code":    string(errorCodeRunes),
		"last_error_message": string(errorMessageRunes),
		"updated_at":         now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
