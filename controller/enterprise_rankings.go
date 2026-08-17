package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetEnterpriseRankings(c *gin.Context) {
	result, err := service.GetEnterpriseRankings(c.Query("period"), c.Query("start"), c.Query("end"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetEnterpriseMemberRankings(c *gin.Context) {
	enterpriseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetEnterpriseMemberRankings(enterpriseID, c.Query("period"), c.Query("start"), c.Query("end"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
