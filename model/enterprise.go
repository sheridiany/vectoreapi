package model

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrInvalidEnterpriseID = errors.New("enterprise id is invalid")

const (
	EnterpriseStatusEnabled  = 1
	EnterpriseStatusDisabled = 2

	EnterpriseTokenGroupAuto              = "auto"
	EnterpriseDefaultBudgetAlertThreshold = 80
	EnterpriseMaxMonthlyBudget            = int64(2_147_483_647)

	EnterpriseRegistrationModeOpen   = "open"
	EnterpriseRegistrationModeInvite = "invite"
	EnterpriseRegistrationModeClosed = "closed"
)

type Enterprise struct {
	Id                   int            `json:"id"`
	Name                 string         `json:"name" gorm:"size:128;not null;index"`
	Code                 string         `json:"code" gorm:"size:64;not null;uniqueIndex"`
	Status               int            `json:"status" gorm:"type:int;not null;index"`
	RegistrationEnabled  bool           `json:"registration_enabled"`
	RegistrationMode     string         `json:"registration_mode" gorm:"size:32;not null"`
	TokenGroupPolicy     string         `json:"token_group_policy" gorm:"size:16;index"`
	MonthlyQuotaBudget   int64          `json:"monthly_quota_budget" gorm:"index"`
	BudgetAlertThreshold int            `json:"budget_alert_threshold"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
}

func NewEnterprise(name, code string) (*Enterprise, error) {
	enterprise := &Enterprise{
		Name:                 strings.TrimSpace(name),
		Code:                 strings.ToLower(strings.TrimSpace(code)),
		Status:               EnterpriseStatusEnabled,
		RegistrationEnabled:  true,
		RegistrationMode:     EnterpriseRegistrationModeOpen,
		TokenGroupPolicy:     EnterpriseTokenGroupAuto,
		BudgetAlertThreshold: EnterpriseDefaultBudgetAlertThreshold,
	}
	if err := enterprise.Validate(); err != nil {
		return nil, err
	}
	return enterprise, nil
}

func (enterprise *Enterprise) Validate() error {
	if enterprise == nil {
		return errors.New("enterprise is nil")
	}
	enterprise.Name = strings.TrimSpace(enterprise.Name)
	enterprise.Code = strings.ToLower(strings.TrimSpace(enterprise.Code))
	if enterprise.RegistrationMode == "" {
		enterprise.RegistrationMode = EnterpriseRegistrationModeOpen
	}
	if enterprise.TokenGroupPolicy == "" {
		enterprise.TokenGroupPolicy = EnterpriseTokenGroupAuto
	}
	if enterprise.Name == "" || utf8.RuneCountInString(enterprise.Name) > 128 {
		return errors.New("enterprise name is invalid")
	}
	if !validEnterpriseCode(enterprise.Code) {
		return errors.New("enterprise code is invalid")
	}
	if enterprise.Status != EnterpriseStatusEnabled && enterprise.Status != EnterpriseStatusDisabled {
		return errors.New("enterprise status is invalid")
	}
	if enterprise.RegistrationMode != EnterpriseRegistrationModeOpen && enterprise.RegistrationMode != EnterpriseRegistrationModeInvite && enterprise.RegistrationMode != EnterpriseRegistrationModeClosed {
		return errors.New("enterprise registration mode is invalid")
	}
	if enterprise.TokenGroupPolicy != EnterpriseTokenGroupAuto {
		return errors.New("enterprise token group policy is invalid")
	}
	if enterprise.MonthlyQuotaBudget < 0 || enterprise.MonthlyQuotaBudget > EnterpriseMaxMonthlyBudget {
		return errors.New("enterprise monthly quota budget is invalid")
	}
	if enterprise.BudgetAlertThreshold < 0 || enterprise.BudgetAlertThreshold > 100 {
		return errors.New("enterprise budget alert threshold is invalid")
	}
	return nil
}

func validEnterpriseCode(code string) bool {
	if len(code) < 2 || len(code) > 64 {
		return false
	}
	for index, char := range code {
		isLetter := char >= 'a' && char <= 'z'
		isDigit := char >= '0' && char <= '9'
		if index == 0 || index == len(code)-1 {
			if !isLetter && !isDigit {
				return false
			}
			continue
		}
		if !isLetter && !isDigit && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func GetEnterpriseByID(id int) (*Enterprise, error) {
	if id <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	enterprise := &Enterprise{}
	if err := DB.First(enterprise, id).Error; err != nil {
		return nil, err
	}
	if enterprise.TokenGroupPolicy == "" {
		enterprise.TokenGroupPolicy = EnterpriseTokenGroupAuto
	}
	return enterprise, nil
}

func LockEnterpriseForUpdate(tx *gorm.DB, id int) (*Enterprise, error) {
	if tx == nil || id <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	enterprise := &Enterprise{}
	if err := lockForUpdate(tx).First(enterprise, id).Error; err != nil {
		return nil, err
	}
	return enterprise, nil
}

func GetEnterpriseByCode(code string) (*Enterprise, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if !validEnterpriseCode(code) {
		return nil, errors.New("enterprise code is invalid")
	}
	enterprise := &Enterprise{}
	if err := DB.Where("code = ?", code).First(enterprise).Error; err != nil {
		return nil, err
	}
	return enterprise, nil
}

func GetAllEnterprises(startIdx, num int) ([]*Enterprise, int64, error) {
	var total int64
	if err := DB.Model(&Enterprise{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	enterprises := make([]*Enterprise, 0)
	query := DB.Order("id DESC")
	if num > 0 {
		query = query.Offset(startIdx).Limit(num)
	}
	if err := query.Find(&enterprises).Error; err != nil {
		return nil, 0, err
	}
	return enterprises, total, nil
}

func GetRegistrationEnterprises() ([]*Enterprise, error) {
	enterprises := make([]*Enterprise, 0)
	if err := DB.Where("status = ? AND registration_enabled = ? AND registration_mode <> ?", EnterpriseStatusEnabled, true, EnterpriseRegistrationModeClosed).
		Order("name ASC, id ASC").Find(&enterprises).Error; err != nil {
		return nil, err
	}
	return enterprises, nil
}

func (enterprise *Enterprise) IsRegistrationAvailable() bool {
	return enterprise != nil && enterprise.Status == EnterpriseStatusEnabled && enterprise.RegistrationEnabled && enterprise.RegistrationMode != EnterpriseRegistrationModeClosed
}

func (enterprise *Enterprise) Insert() error {
	if err := enterprise.Validate(); err != nil {
		return err
	}
	return DB.Create(enterprise).Error
}

func (enterprise *Enterprise) Update() error {
	if err := enterprise.Validate(); err != nil {
		return err
	}
	return DB.Model(&Enterprise{}).Where("id = ?", enterprise.Id).Updates(map[string]interface{}{
		"name":                   enterprise.Name,
		"code":                   enterprise.Code,
		"status":                 enterprise.Status,
		"registration_enabled":   enterprise.RegistrationEnabled,
		"registration_mode":      enterprise.RegistrationMode,
		"token_group_policy":     enterprise.TokenGroupPolicy,
		"monthly_quota_budget":   enterprise.MonthlyQuotaBudget,
		"budget_alert_threshold": enterprise.BudgetAlertThreshold,
		"updated_at":             common.GetTimestamp(),
	}).Error
}
