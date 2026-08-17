package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type adminCreateEnterpriseRequest struct {
	Name                string `json:"name"`
	Code                string `json:"code"`
	RegistrationEnabled *bool  `json:"registration_enabled"`
	RegistrationMode    string `json:"registration_mode"`
}

type adminUpdateEnterpriseRequest struct {
	Name                *string `json:"name"`
	Code                *string `json:"code"`
	Status              *int    `json:"status"`
	RegistrationEnabled *bool   `json:"registration_enabled"`
	RegistrationMode    *string `json:"registration_mode"`
}

func GetEnterpriseRegistrationOptions(c *gin.Context) {
	options, err := service.ListRegistrationEnterprises()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, options)
}

func GetEnterprise(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := service.GetEnterprise(enterpriseID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, enterprise)
}

func AdminListEnterprises(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	enterprises, total, err := service.ListEnterprises(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(enterprises)
	common.ApiSuccess(c, pageInfo)
}

func AdminCreateEnterprise(c *gin.Context) {
	var request adminCreateEnterpriseRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}

	enterprise, err := service.CreateEnterprise(service.CreateEnterpriseInput{
		Name:                request.Name,
		Code:                request.Code,
		RegistrationEnabled: request.RegistrationEnabled,
		RegistrationMode:    request.RegistrationMode,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterprise.Id,
		c.GetInt("id"),
		"Create enterprise",
		c.ClientIP(),
		"enterprise.create",
		map[string]interface{}{"enterprise_id": enterprise.Id},
		nil,
		nil,
	)
	common.ApiSuccess(c, enterprise)
}

func AdminUpdateEnterprise(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request adminUpdateEnterpriseRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := service.UpdateEnterprise(service.UpdateEnterpriseInput{
		ID:                  enterpriseID,
		Name:                request.Name,
		Code:                request.Code,
		Status:              request.Status,
		RegistrationEnabled: request.RegistrationEnabled,
		RegistrationMode:    request.RegistrationMode,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Update enterprise",
		c.ClientIP(),
		"enterprise.update",
		map[string]interface{}{"enterprise_id": enterpriseID},
		nil,
		nil,
	)
	common.ApiSuccess(c, enterprise)
}

type updateEnterpriseRegistrationRequest struct {
	RegistrationEnabled *bool   `json:"registration_enabled"`
	RegistrationMode    *string `json:"registration_mode"`
}

func UpdateEnterpriseRegistration(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request updateEnterpriseRegistrationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := service.UpdateEnterprise(service.UpdateEnterpriseInput{
		ID: enterpriseID, RegistrationEnabled: request.RegistrationEnabled, RegistrationMode: request.RegistrationMode,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Update enterprise registration settings",
		c.ClientIP(),
		"enterprise.registration.update",
		map[string]interface{}{
			"registration_enabled": enterprise.RegistrationEnabled,
			"registration_mode":    enterprise.RegistrationMode,
		},
		nil,
		nil,
	)
	common.ApiSuccess(c, enterprise)
}
