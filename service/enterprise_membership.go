package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrEnterpriseMemberAlreadyAssigned = errors.New("user is already assigned to another enterprise")
	ErrEnterpriseOwnerAlreadyAssigned  = errors.New("enterprise already has an owner")
)

type EnterpriseMemberCandidate struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type EnterpriseMemberUser struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type EnterpriseMember struct {
	ID           int                   `json:"id"`
	EnterpriseID int                   `json:"enterprise_id"`
	UserID       int                   `json:"user_id"`
	Role         string                `json:"role"`
	Status       int                   `json:"status"`
	InvitedBy    int                   `json:"invited_by"`
	JoinedAt     int64                 `json:"joined_at"`
	UpdatedAt    int64                 `json:"updated_at"`
	User         *EnterpriseMemberUser `json:"user,omitempty"`
}

func AssignEnterpriseMember(enterpriseID, userID, invitedBy int, role string) (*model.EnterpriseMembership, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if enterpriseID <= 0 || userID <= 0 || invitedBy <= 0 {
		return nil, errors.New("enterprise membership ids are invalid")
	}
	if !model.IsEnterpriseMembershipRole(role) {
		return nil, errors.New("enterprise membership role is invalid")
	}

	var membership model.EnterpriseMembership
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockEnterpriseForUpdate(tx, enterpriseID); err != nil {
			return err
		}
		var user model.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		findErr := tx.Unscoped().Where("user_id = ?", userID).First(&membership).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if findErr == nil && membership.EnterpriseID != enterpriseID {
			return ErrEnterpriseMemberAlreadyAssigned
		}

		if role == model.EnterpriseMembershipRoleOwner {
			var ownerCount int64
			query := tx.Model(&model.EnterpriseMembership{}).
				Where("enterprise_id = ? AND role = ? AND status = ?", enterpriseID, model.EnterpriseMembershipRoleOwner, model.EnterpriseMembershipStatusActive)
			if findErr == nil {
				query = query.Where("user_id <> ?", userID)
			}
			if err := query.Count(&ownerCount).Error; err != nil {
				return err
			}
			if ownerCount > 0 {
				return ErrEnterpriseOwnerAlreadyAssigned
			}
		}

		if findErr == nil {
			membership.Role = role
			membership.Status = model.EnterpriseMembershipStatusActive
			membership.InvitedBy = invitedBy
			membership.UpdatedAt = common.GetTimestamp()
			return tx.Model(&membership).Updates(map[string]interface{}{
				"role":       membership.Role,
				"status":     membership.Status,
				"invited_by": membership.InvitedBy,
				"updated_at": membership.UpdatedAt,
				"deleted_at": nil,
			}).Error
		}

		newMembership, err := model.NewEnterpriseMembership(enterpriseID, userID, role)
		if err != nil {
			return err
		}
		newMembership.InvitedBy = invitedBy
		membership = *newMembership
		return membership.InsertWithTx(tx)
	})
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

func ListEnterpriseMembers(enterpriseID, startIdx, num int) ([]EnterpriseMember, int64, error) {
	memberships, total, err := model.GetEnterpriseMemberships(enterpriseID, startIdx, num)
	if err != nil {
		return nil, 0, err
	}
	members := make([]EnterpriseMember, 0, len(memberships))
	for _, membership := range memberships {
		member := EnterpriseMember{
			ID: membership.Id, EnterpriseID: membership.EnterpriseID, UserID: membership.UserID,
			Role: membership.Role, Status: membership.Status, InvitedBy: membership.InvitedBy,
			JoinedAt: membership.JoinedAt, UpdatedAt: membership.UpdatedAt,
		}
		if user, userErr := model.GetUserById(membership.UserID, false); userErr == nil {
			member.User = &EnterpriseMemberUser{
				ID: user.Id, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email,
			}
		}
		members = append(members, member)
	}
	return members, total, nil
}

func ListEnterpriseMemberCandidates(enterpriseID int, keyword string) ([]EnterpriseMemberCandidate, error) {
	if enterpriseID <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	status := common.UserStatusEnabled
	users, _, err := model.SearchUsers(strings.TrimSpace(keyword), "", nil, &status, 0, 20)
	if err != nil {
		return nil, err
	}
	candidates := make([]EnterpriseMemberCandidate, 0, len(users))
	for _, user := range users {
		if _, membershipErr := model.GetEnterpriseMembershipByUserID(user.Id); membershipErr == nil {
			continue
		}
		candidates = append(candidates, EnterpriseMemberCandidate{
			ID: user.Id, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email,
		})
	}
	return candidates, nil
}

func UpdateEnterpriseMember(enterpriseID, userID int, role string, status int) (*model.EnterpriseMembership, error) {
	return model.UpdateEnterpriseMembership(enterpriseID, userID, role, status)
}

func JoinEnterpriseWithTx(tx *gorm.DB, enterpriseCode, invitationCode string, userID int) (*model.EnterpriseMembership, error) {
	return model.JoinEnterpriseWithTx(tx, enterpriseCode, invitationCode, userID)
}

type EnterpriseInvitationResult struct {
	Invitation *model.EnterpriseInvitation
	Code       string
}

func CreateEnterpriseInvitation(enterpriseID, createdBy int, expiresAt int64, maxUses int) (*EnterpriseInvitationResult, error) {
	if enterpriseID <= 0 || createdBy <= 0 || expiresAt < 0 {
		return nil, errors.New("enterprise invitation ids are invalid")
	}
	if maxUses < 0 {
		return nil, errors.New("enterprise invitation max uses is invalid")
	}
	enterprise, err := model.GetEnterpriseByID(enterpriseID)
	if err != nil {
		return nil, err
	}
	if !enterprise.IsRegistrationAvailable() {
		return nil, errors.New("enterprise registration is unavailable")
	}
	code, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	invitation, err := model.NewEnterpriseInvitation(enterpriseID, code, createdBy, expiresAt, maxUses)
	if err != nil {
		return nil, err
	}
	if err := invitation.Insert(); err != nil {
		return nil, err
	}
	return &EnterpriseInvitationResult{Invitation: invitation, Code: code}, nil
}

func ListEnterpriseInvitations(enterpriseID, startIdx, num int) ([]*model.EnterpriseInvitation, int64, error) {
	return model.GetEnterpriseInvitations(enterpriseID, startIdx, num)
}

func UpdateEnterpriseInvitation(enterpriseID, invitationID, status int) (*model.EnterpriseInvitation, error) {
	return model.UpdateEnterpriseInvitationStatus(enterpriseID, invitationID, status)
}
