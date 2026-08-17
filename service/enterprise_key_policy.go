package service

import (
	"errors"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func GetEnterpriseKeyPolicySummary(enterpriseID int) (*model.EnterpriseKeyPolicySummary, error) {
	return model.GetEnterpriseKeyPolicySummary(enterpriseID)
}

func ApplyEnterpriseKeyPolicy(enterpriseID, initiatedBy int) (*model.EnterpriseKeyPolicyOperation, error) {
	return model.ApplyEnterpriseKeyPolicy(enterpriseID, initiatedBy)
}

func RollbackEnterpriseKeyPolicy(enterpriseID, operationID int) (*model.EnterpriseKeyPolicyOperation, error) {
	return model.RollbackEnterpriseKeyPolicy(enterpriseID, operationID)
}

func GetUserEnterpriseTokenGroupPolicy(userID int) (string, error) {
	membership, err := model.GetEnterpriseMembershipByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	enterprise, err := model.GetEnterpriseByID(membership.EnterpriseID)
	if err != nil {
		return "", err
	}
	return enterprise.TokenGroupPolicy, nil
}
