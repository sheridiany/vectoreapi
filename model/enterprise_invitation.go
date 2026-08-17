package model

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	EnterpriseInvitationStatusEnabled  = 1
	EnterpriseInvitationStatusDisabled = 2
)

type EnterpriseInvitation struct {
	Id           int            `json:"id"`
	EnterpriseID int            `json:"enterprise_id" gorm:"not null;index"`
	CodeHash     string         `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Status       int            `json:"status" gorm:"type:int;not null;index"`
	ExpiresAt    int64          `json:"expires_at" gorm:"not null;default:0;index"`
	MaxUses      int            `json:"max_uses" gorm:"not null;default:0"`
	UsedCount    int            `json:"used_count" gorm:"not null;default:0"`
	CreatedBy    int            `json:"created_by" gorm:"not null;index"`
	CreatedAt    int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt    int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func NewEnterpriseInvitation(enterpriseID int, code string, createdBy int, expiresAt int64, maxUses int) (*EnterpriseInvitation, error) {
	if enterpriseID <= 0 || createdBy <= 0 || strings.TrimSpace(code) == "" || expiresAt < 0 || maxUses < 0 {
		return nil, errors.New("enterprise invitation is invalid")
	}
	return &EnterpriseInvitation{
		EnterpriseID: enterpriseID,
		CodeHash:     HashEnterpriseInvitationCode(code),
		Status:       EnterpriseInvitationStatusEnabled,
		ExpiresAt:    expiresAt,
		MaxUses:      maxUses,
		CreatedBy:    createdBy,
	}, nil
}

func HashEnterpriseInvitationCode(code string) string {
	return hex.EncodeToString(common.Sha256Raw([]byte(strings.TrimSpace(code))))
}

func (invitation *EnterpriseInvitation) CanUse(now int64) bool {
	if invitation == nil || invitation.Status != EnterpriseInvitationStatusEnabled {
		return false
	}
	if invitation.ExpiresAt > 0 && invitation.ExpiresAt <= now {
		return false
	}
	return invitation.MaxUses <= 0 || invitation.UsedCount < invitation.MaxUses
}

func (invitation *EnterpriseInvitation) Insert() error {
	if invitation == nil || invitation.EnterpriseID <= 0 || invitation.CodeHash == "" || invitation.CreatedBy <= 0 {
		return errors.New("enterprise invitation is invalid")
	}
	if invitation.Status == 0 {
		invitation.Status = EnterpriseInvitationStatusEnabled
	}
	return DB.Create(invitation).Error
}

func GetEnterpriseInvitations(enterpriseID, startIdx, num int) ([]*EnterpriseInvitation, int64, error) {
	if enterpriseID <= 0 {
		return nil, 0, errors.New("enterprise id is invalid")
	}
	var total int64
	if err := DB.Model(&EnterpriseInvitation{}).Where("enterprise_id = ?", enterpriseID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	invites := make([]*EnterpriseInvitation, 0)
	query := DB.Where("enterprise_id = ?", enterpriseID).Order("id DESC")
	if num > 0 {
		query = query.Offset(startIdx).Limit(num)
	}
	if err := query.Find(&invites).Error; err != nil {
		return nil, 0, err
	}
	return invites, total, nil
}

func UpdateEnterpriseInvitationStatus(enterpriseID, invitationID, status int) (*EnterpriseInvitation, error) {
	if enterpriseID <= 0 || invitationID <= 0 {
		return nil, errors.New("enterprise invitation ids are invalid")
	}
	if status != EnterpriseInvitationStatusEnabled && status != EnterpriseInvitationStatusDisabled {
		return nil, errors.New("enterprise invitation status is invalid")
	}
	invitation := &EnterpriseInvitation{}
	if err := DB.Where("id = ? AND enterprise_id = ?", invitationID, enterpriseID).First(invitation).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(invitation).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": common.GetTimestamp(),
	}).Error; err != nil {
		return nil, err
	}
	invitation.Status = status
	return invitation, nil
}

func ConsumeEnterpriseInvitationWithTx(tx *gorm.DB, enterpriseCode, code string, userID int) (*EnterpriseMembership, error) {
	enterpriseCode = strings.ToLower(strings.TrimSpace(enterpriseCode))
	code = strings.TrimSpace(code)
	if tx == nil || !validEnterpriseCode(enterpriseCode) || code == "" || userID <= 0 {
		return nil, errors.New("enterprise registration context is invalid")
	}
	var enterprise Enterprise
	if err := tx.Where("code = ?", enterpriseCode).First(&enterprise).Error; err != nil {
		return nil, err
	}
	if !enterprise.IsRegistrationAvailable() {
		return nil, errors.New("enterprise registration is unavailable")
	}
	var invitation EnterpriseInvitation
	if err := lockForUpdate(tx).Where("enterprise_id = ? AND code_hash = ?", enterprise.Id, HashEnterpriseInvitationCode(code)).First(&invitation).Error; err != nil {
		return nil, err
	}
	if !invitation.CanUse(time.Now().Unix()) {
		return nil, errors.New("enterprise invitation is invalid or expired")
	}
	var existingMembership EnterpriseMembership
	if err := tx.Where("user_id = ? AND status = ?", userID, EnterpriseMembershipStatusActive).First(&existingMembership).Error; err == nil {
		return nil, errors.New("user already belongs to an enterprise")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	membership, err := NewEnterpriseMembership(enterprise.Id, userID, EnterpriseMembershipRoleMember)
	if err != nil {
		return nil, err
	}
	if err := membership.InsertWithTx(tx); err != nil {
		return nil, err
	}
	result := tx.Model(&EnterpriseInvitation{}).
		Where("id = ? AND status = ?", invitation.Id, EnterpriseInvitationStatusEnabled).
		UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("enterprise invitation is no longer available")
	}
	return membership, nil
}
