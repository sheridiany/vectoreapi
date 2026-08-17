package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	EnterpriseKeyPolicyOperationApplied    = "applied"
	EnterpriseKeyPolicyOperationRolledBack = "rolled_back"
)

type EnterpriseKeyPolicyOperation struct {
	Id                   int    `json:"id"`
	EnterpriseID         int    `json:"enterprise_id" gorm:"not null;index"`
	InitiatedBy          int    `json:"initiated_by" gorm:"not null;index"`
	Status               string `json:"status" gorm:"size:16;not null;index"`
	ChangedCount         int    `json:"changed_count"`
	RollbackSkippedCount int    `json:"rollback_skipped_count"`
	CreatedAt            int64  `json:"created_at" gorm:"not null;index"`
	RolledBackAt         int64  `json:"rolled_back_at"`
}

type EnterpriseKeyPolicyTokenChange struct {
	Id                 int    `json:"id"`
	OperationID        int    `json:"operation_id" gorm:"not null;index"`
	EnterpriseID       int    `json:"enterprise_id" gorm:"not null;index"`
	UserID             int    `json:"user_id" gorm:"not null;index"`
	TokenID            int    `json:"token_id" gorm:"not null;index"`
	PreviousGroup      string `json:"previous_group" gorm:"size:64;not null"`
	PreviousCrossRetry bool   `json:"previous_cross_retry"`
	PreviousAutoGroups string `json:"-" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"not null;index"`
}

type EnterpriseKeyPolicySummary struct {
	EnterpriseID        int                           `json:"enterprise_id"`
	TokenGroupPolicy    string                        `json:"token_group_policy"`
	ActiveMemberCount   int64                         `json:"active_member_count"`
	MembersWithKeys     int64                         `json:"members_with_keys"`
	TotalKeyCount       int64                         `json:"total_key_count"`
	AutoKeyCount        int64                         `json:"auto_key_count"`
	ConvertibleKeyCount int64                         `json:"convertible_key_count"`
	LastOperation       *EnterpriseKeyPolicyOperation `json:"last_operation,omitempty"`
}

func GetEnterpriseKeyPolicySummary(enterpriseID int) (*EnterpriseKeyPolicySummary, error) {
	if enterpriseID <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	enterprise, err := GetEnterpriseByID(enterpriseID)
	if err != nil {
		return nil, err
	}
	userIDs, err := getActiveEnterpriseMemberUserIDs(DB, enterpriseID)
	if err != nil {
		return nil, err
	}

	summary := &EnterpriseKeyPolicySummary{
		EnterpriseID:      enterpriseID,
		TokenGroupPolicy:  enterprise.TokenGroupPolicy,
		ActiveMemberCount: int64(len(userIDs)),
	}
	if len(userIDs) > 0 {
		query := DB.Model(&Token{}).Where("user_id IN ?", userIDs)
		if err := query.Count(&summary.TotalKeyCount).Error; err != nil {
			return nil, err
		}
		if err := query.Where(commonGroupCol+" = ?", EnterpriseTokenGroupAuto).Count(&summary.AutoKeyCount).Error; err != nil {
			return nil, err
		}
		if err := query.Distinct("user_id").Count(&summary.MembersWithKeys).Error; err != nil {
			return nil, err
		}
	}
	summary.ConvertibleKeyCount = summary.TotalKeyCount - summary.AutoKeyCount
	if summary.ConvertibleKeyCount < 0 {
		summary.ConvertibleKeyCount = 0
	}
	summary.LastOperation, err = GetLatestEnterpriseKeyPolicyOperation(enterpriseID)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func GetLatestEnterpriseKeyPolicyOperation(enterpriseID int) (*EnterpriseKeyPolicyOperation, error) {
	if enterpriseID <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	operation := &EnterpriseKeyPolicyOperation{}
	err := DB.Where("enterprise_id = ?", enterpriseID).Order("id DESC").First(operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return operation, nil
}

func ApplyEnterpriseKeyPolicy(enterpriseID, initiatedBy int) (*EnterpriseKeyPolicyOperation, error) {
	if enterpriseID <= 0 || initiatedBy <= 0 {
		return nil, errors.New("enterprise key policy ids are invalid")
	}
	operation := &EnterpriseKeyPolicyOperation{}
	var userIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		enterprise, err := LockEnterpriseForUpdate(tx, enterpriseID)
		if err != nil {
			return err
		}
		if enterprise.TokenGroupPolicy == "" {
			enterprise.TokenGroupPolicy = EnterpriseTokenGroupAuto
			if err := tx.Model(enterprise).Update("token_group_policy", enterprise.TokenGroupPolicy).Error; err != nil {
				return err
			}
		}
		if enterprise.TokenGroupPolicy != EnterpriseTokenGroupAuto {
			return errors.New("enterprise token group policy is invalid")
		}
		userIDs, err = getActiveEnterpriseMemberUserIDs(tx, enterpriseID)
		if err != nil {
			return err
		}

		operation = &EnterpriseKeyPolicyOperation{
			EnterpriseID: enterpriseID,
			InitiatedBy:  initiatedBy,
			Status:       EnterpriseKeyPolicyOperationApplied,
			CreatedAt:    common.GetTimestamp(),
		}
		if err := tx.Create(operation).Error; err != nil {
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}

		var tokens []Token
		if err := lockForUpdate(tx).
			Where("user_id IN ? AND "+commonGroupCol+" <> ?", userIDs, EnterpriseTokenGroupAuto).
			Find(&tokens).Error; err != nil {
			return err
		}
		if len(tokens) == 0 {
			return nil
		}

		changes := make([]EnterpriseKeyPolicyTokenChange, 0, len(tokens))
		tokenIDs := make([]int, 0, len(tokens))
		for _, token := range tokens {
			changes = append(changes, EnterpriseKeyPolicyTokenChange{
				OperationID:        operation.Id,
				EnterpriseID:       enterpriseID,
				UserID:             token.UserId,
				TokenID:            token.Id,
				PreviousGroup:      token.Group,
				PreviousCrossRetry: token.CrossGroupRetry,
				PreviousAutoGroups: token.AutoGroups,
				CreatedAt:          common.GetTimestamp(),
			})
			tokenIDs = append(tokenIDs, token.Id)
		}
		if err := tx.Create(&changes).Error; err != nil {
			return err
		}
		result := tx.Model(&Token{}).Where("id IN ?", tokenIDs).Updates(map[string]interface{}{
			"group":             EnterpriseTokenGroupAuto,
			"cross_group_retry": true,
			"auto_groups":       "",
		})
		if result.Error != nil {
			return result.Error
		}
		if int(result.RowsAffected) != len(tokens) {
			return errors.New("enterprise key policy update count mismatch")
		}
		operation.ChangedCount = len(tokens)
		return tx.Model(operation).Updates(map[string]interface{}{"changed_count": operation.ChangedCount}).Error
	})
	if err != nil {
		return nil, err
	}
	for _, userID := range userIDs {
		if err := InvalidateUserTokensCache(userID); err != nil {
			common.SysLog("failed to invalidate enterprise token cache: " + err.Error())
		}
	}
	return operation, nil
}

func RollbackEnterpriseKeyPolicy(enterpriseID, operationID int) (*EnterpriseKeyPolicyOperation, error) {
	if enterpriseID <= 0 || operationID <= 0 {
		return nil, errors.New("enterprise key policy operation ids are invalid")
	}
	operation := &EnterpriseKeyPolicyOperation{}
	var userIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := LockEnterpriseForUpdate(tx, enterpriseID); err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("id = ? AND enterprise_id = ?", operationID, enterpriseID).First(operation).Error; err != nil {
			return err
		}
		if operation.Status != EnterpriseKeyPolicyOperationApplied {
			return errors.New("enterprise key policy operation is not active")
		}
		var changes []EnterpriseKeyPolicyTokenChange
		if err := tx.Where("operation_id = ? AND enterprise_id = ?", operationID, enterpriseID).Find(&changes).Error; err != nil {
			return err
		}
		userIDs = make([]int, 0, len(changes))
		restored := 0
		skipped := 0
		for _, change := range changes {
			userIDs = append(userIDs, change.UserID)
			result := tx.Model(&Token{}).
				Where("id = ? AND user_id = ? AND "+commonGroupCol+" = ? AND cross_group_retry = ? AND auto_groups = ?", change.TokenID, change.UserID, EnterpriseTokenGroupAuto, true, "").
				Updates(map[string]interface{}{
					"group":             change.PreviousGroup,
					"cross_group_retry": change.PreviousCrossRetry,
					"auto_groups":       change.PreviousAutoGroups,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				restored++
			} else {
				skipped++
			}
		}
		operation.Status = EnterpriseKeyPolicyOperationRolledBack
		operation.RollbackSkippedCount = skipped
		operation.RolledBackAt = common.GetTimestamp()
		if err := tx.Model(operation).Updates(map[string]interface{}{
			"status":                 operation.Status,
			"rollback_skipped_count": operation.RollbackSkippedCount,
			"rolled_back_at":         operation.RolledBackAt,
		}).Error; err != nil {
			return err
		}
		operation.ChangedCount = restored
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, userID := range userIDs {
		if err := InvalidateUserTokensCache(userID); err != nil {
			common.SysLog("failed to invalidate enterprise token cache: " + err.Error())
		}
	}
	return operation, nil
}

func getActiveEnterpriseMemberUserIDs(tx *gorm.DB, enterpriseID int) ([]int, error) {
	if tx == nil || enterpriseID <= 0 {
		return nil, errors.New("enterprise key policy scope is invalid")
	}
	var userIDs []int
	err := tx.Model(&EnterpriseMembership{}).
		Where("enterprise_id = ? AND status = ?", enterpriseID, EnterpriseMembershipStatusActive).
		Pluck("user_id", &userIDs).Error
	return userIDs, err
}
