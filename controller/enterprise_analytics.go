package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetEnterpriseAnalytics(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetEnterpriseAnalytics(enterpriseID, c.Query("period"), c.Query("start"), c.Query("end"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetEnterpriseBudget(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetEnterpriseBudgetStatus(enterpriseID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

type updateEnterpriseBudgetRequest struct {
	BudgetQuota    int64 `json:"budget_quota"`
	AlertThreshold int   `json:"alert_threshold"`
}

func UpdateEnterpriseBudget(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request updateEnterpriseBudgetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := service.UpdateEnterpriseBudget(enterpriseID, request.BudgetQuota, request.AlertThreshold)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Update enterprise budget settings",
		c.ClientIP(),
		"enterprise.budget.update",
		map[string]interface{}{
			"enterprise_id":   enterpriseID,
			"budget_quota":    enterprise.MonthlyQuotaBudget,
			"alert_threshold": enterprise.BudgetAlertThreshold,
		},
		nil,
		nil,
	)
	common.ApiSuccess(c, enterprise)
}
