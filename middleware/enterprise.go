package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func EnterpriseAdminAuth() func(c *gin.Context) {
	return enterpriseAuth(true)
}

func EnterpriseMemberAuth() func(c *gin.Context) {
	return enterpriseAuth(false)
}

func EnterpriseViewerAuth() func(c *gin.Context) {
	return enterpriseAuth(true, model.EnterpriseMembershipRoleAuditor)
}

func enterpriseAuth(adminOnly bool, viewerRole ...string) func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.GetInt("role") >= common.RoleRootUser {
			c.Next()
			return
		}
		enterpriseID, err := strconv.Atoi(c.Param("id"))
		if err != nil || enterpriseID <= 0 {
			enterprisePermissionDenied(c)
			return
		}
		enterprise, err := model.GetEnterpriseByID(enterpriseID)
		if err != nil || enterprise.Status != model.EnterpriseStatusEnabled {
			enterprisePermissionDenied(c)
			return
		}
		membership, err := model.GetEnterpriseMembership(enterpriseID, c.GetInt("id"))
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				common.SysLog("failed to load enterprise membership: " + err.Error())
			}
			enterprisePermissionDenied(c)
			return
		}
		allowed := membership.Role == model.EnterpriseMembershipRoleOwner || membership.Role == model.EnterpriseMembershipRoleAdmin
		if len(viewerRole) > 0 {
			allowed = allowed || membership.Role == viewerRole[0]
		}
		if adminOnly && !allowed {
			enterprisePermissionDenied(c)
			return
		}
		c.Next()
	}
}

func enterprisePermissionDenied(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
	})
}
