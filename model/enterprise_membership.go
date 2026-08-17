package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	EnterpriseMembershipStatusActive   = 1
	EnterpriseMembershipStatusDisabled = 2

	EnterpriseMembershipRoleOwner   = "owner"
	EnterpriseMembershipRoleAdmin   = "admin"
	EnterpriseMembershipRoleMember  = "member"
	EnterpriseMembershipRoleAuditor = "auditor"
)

type EnterpriseMembership struct {
	Id           int            `json:"id"`
	EnterpriseID int            `json:"enterprise_id" gorm:"not null;index"`
	UserID       int            `json:"user_id" gorm:"not null;uniqueIndex"`
	Role         string         `json:"role" gorm:"size:16;not null"`
	Status       int            `json:"status" gorm:"type:int;not null;index"`
	InvitedBy    int            `json:"invited_by" gorm:"not null;default:0;index"`
	JoinedAt     int64          `json:"joined_at" gorm:"not null;index"`
	UpdatedAt    int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func NewEnterpriseMembership(enterpriseID, userID int, role string) (*EnterpriseMembership, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if enterpriseID <= 0 || userID <= 0 {
		return nil, errors.New("enterprise membership ids are invalid")
	}
	if !IsEnterpriseMembershipRole(role) {
		return nil, errors.New("enterprise membership role is invalid")
	}
	return &EnterpriseMembership{
		EnterpriseID: enterpriseID,
		UserID:       userID,
		Role:         role,
		Status:       EnterpriseMembershipStatusActive,
		JoinedAt:     common.GetTimestamp(),
	}, nil
}

func IsEnterpriseMembershipRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case EnterpriseMembershipRoleOwner, EnterpriseMembershipRoleAdmin, EnterpriseMembershipRoleMember, EnterpriseMembershipRoleAuditor:
		return true
	default:
		return false
	}
}

func (membership *EnterpriseMembership) Insert() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return membership.InsertWithTx(tx)
	})
}

func (membership *EnterpriseMembership) InsertWithTx(tx *gorm.DB) error {
	if membership == nil || membership.EnterpriseID <= 0 || membership.UserID <= 0 || !IsEnterpriseMembershipRole(membership.Role) {
		return errors.New("enterprise membership is invalid")
	}
	if membership.Status == 0 {
		membership.Status = EnterpriseMembershipStatusActive
	}
	if membership.JoinedAt == 0 {
		membership.JoinedAt = common.GetTimestamp()
	}
	if membership.Role == EnterpriseMembershipRoleOwner {
		if _, err := LockEnterpriseForUpdate(tx, membership.EnterpriseID); err != nil {
			return err
		}
		var ownerCount int64
		if err := tx.Model(&EnterpriseMembership{}).
			Where("enterprise_id = ? AND role = ? AND status = ?", membership.EnterpriseID, EnterpriseMembershipRoleOwner, EnterpriseMembershipStatusActive).
			Count(&ownerCount).Error; err != nil {
			return err
		}
		if ownerCount > 0 {
			return errors.New("enterprise already has an owner")
		}
	}
	return tx.Create(membership).Error
}

func GetEnterpriseMembershipByUserID(userID int) (*EnterpriseMembership, error) {
	if userID <= 0 {
		return nil, errors.New("user id is invalid")
	}
	membership := &EnterpriseMembership{}
	if err := DB.Where("user_id = ? AND status = ?", userID, EnterpriseMembershipStatusActive).First(membership).Error; err != nil {
		return nil, err
	}
	return membership, nil
}

func GetEnterpriseMembership(enterpriseID, userID int) (*EnterpriseMembership, error) {
	if enterpriseID <= 0 || userID <= 0 {
		return nil, errors.New("enterprise membership ids are invalid")
	}
	membership := &EnterpriseMembership{}
	if err := DB.Where("enterprise_id = ? AND user_id = ? AND status = ?", enterpriseID, userID, EnterpriseMembershipStatusActive).First(membership).Error; err != nil {
		return nil, err
	}
	return membership, nil
}

func GetEnterpriseMemberships(enterpriseID, startIdx, num int) ([]*EnterpriseMembership, int64, error) {
	if enterpriseID <= 0 {
		return nil, 0, errors.New("enterprise id is invalid")
	}
	var total int64
	if err := DB.Model(&EnterpriseMembership{}).Where("enterprise_id = ?", enterpriseID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	memberships := make([]*EnterpriseMembership, 0)
	query := DB.Where("enterprise_id = ?", enterpriseID).Order("id DESC")
	if num > 0 {
		query = query.Offset(startIdx).Limit(num)
	}
	if err := query.Find(&memberships).Error; err != nil {
		return nil, 0, err
	}
	return memberships, total, nil
}

func JoinEnterpriseWithTx(tx *gorm.DB, enterpriseCode, invitationCode string, userID int) (*EnterpriseMembership, error) {
	enterpriseCode = strings.ToLower(strings.TrimSpace(enterpriseCode))
	invitationCode = strings.TrimSpace(invitationCode)
	if tx == nil || !validEnterpriseCode(enterpriseCode) || userID <= 0 {
		return nil, errors.New("enterprise registration context is invalid")
	}

	var enterprise Enterprise
	if err := tx.Where("code = ?", enterpriseCode).First(&enterprise).Error; err != nil {
		return nil, err
	}
	if !enterprise.IsRegistrationAvailable() {
		return nil, errors.New("enterprise registration is unavailable")
	}
	if enterprise.RegistrationMode == EnterpriseRegistrationModeInvite && invitationCode == "" {
		return nil, errors.New("enterprise invitation code is required")
	}
	if invitationCode != "" {
		return ConsumeEnterpriseInvitationWithTx(tx, enterpriseCode, invitationCode, userID)
	}

	if _, err := LockEnterpriseForUpdate(tx, enterprise.Id); err != nil {
		return nil, err
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
	return membership, nil
}

func UpdateEnterpriseMembership(enterpriseID, userID int, roleName string, status int) (*EnterpriseMembership, error) {
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if enterpriseID <= 0 || userID <= 0 {
		return nil, errors.New("enterprise membership ids are invalid")
	}
	if !IsEnterpriseMembershipRole(roleName) {
		return nil, errors.New("enterprise membership role is invalid")
	}
	if status != EnterpriseMembershipStatusActive && status != EnterpriseMembershipStatusDisabled {
		return nil, errors.New("enterprise membership status is invalid")
	}
	var membership EnterpriseMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := LockEnterpriseForUpdate(tx, enterpriseID); err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("enterprise_id = ? AND user_id = ?", enterpriseID, userID).First(&membership).Error; err != nil {
			return err
		}
		if roleName == EnterpriseMembershipRoleOwner && membership.Role != EnterpriseMembershipRoleOwner {
			var ownerCount int64
			if err := tx.Model(&EnterpriseMembership{}).
				Where("enterprise_id = ? AND role = ? AND status = ? AND user_id <> ?", enterpriseID, EnterpriseMembershipRoleOwner, EnterpriseMembershipStatusActive, userID).
				Count(&ownerCount).Error; err != nil {
				return err
			}
			if ownerCount > 0 {
				return errors.New("enterprise already has an owner")
			}
		}
		removesOwner := membership.Role == EnterpriseMembershipRoleOwner && (roleName != EnterpriseMembershipRoleOwner || status != EnterpriseMembershipStatusActive)
		if removesOwner {
			var ownerCount int64
			if err := tx.Model(&EnterpriseMembership{}).
				Where("enterprise_id = ? AND role = ? AND status = ? AND user_id <> ?", enterpriseID, EnterpriseMembershipRoleOwner, EnterpriseMembershipStatusActive, userID).
				Count(&ownerCount).Error; err != nil {
				return err
			}
			if ownerCount == 0 {
				return errors.New("enterprise must keep an active owner")
			}
		}
		membership.Role = roleName
		membership.Status = status
		membership.UpdatedAt = common.GetTimestamp()
		return tx.Model(&membership).Updates(map[string]interface{}{
			"role":       membership.Role,
			"status":     membership.Status,
			"updated_at": membership.UpdatedAt,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &membership, nil
}
