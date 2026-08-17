package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetEnterpriseKeyPolicy(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := service.GetEnterpriseKeyPolicySummary(enterpriseID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func ApplyEnterpriseKeyPolicy(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	operation, err := service.ApplyEnterpriseKeyPolicy(enterpriseID, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Apply enterprise Auto key policy",
		c.ClientIP(),
		"enterprise.key_policy.apply",
		map[string]interface{}{
			"enterprise_id": enterpriseID,
			"operation_id":  operation.Id,
			"changed_count": operation.ChangedCount,
		},
		nil,
		nil,
	)
	common.ApiSuccess(c, operation)
}

func RollbackEnterpriseKeyPolicy(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	operationID, err := strconv.Atoi(c.Param("operation_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	operation, err := service.RollbackEnterpriseKeyPolicy(enterpriseID, operationID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Rollback enterprise Auto key policy",
		c.ClientIP(),
		"enterprise.key_policy.rollback",
		map[string]interface{}{
			"enterprise_id":          enterpriseID,
			"operation_id":           operation.Id,
			"restored_count":         operation.ChangedCount,
			"rollback_skipped_count": operation.RollbackSkippedCount,
		},
		nil,
		nil,
	)
	common.ApiSuccess(c, operation)
}
