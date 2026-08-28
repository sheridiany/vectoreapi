package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const SearchAgentKeyContextKey = "search_agent_key"

func SearchAgentKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret, ok := authorizationToken(c.GetHeader("Authorization"))
		if !ok || !strings.HasPrefix(secret, "vr_live_") {
			writeSearchAgentAuthError(c, http.StatusUnauthorized, "vSearch key is required")
			return
		}

		key, err := model.FindSearchAgentKeyBySecret(secret)
		if err != nil {
			writeSearchAgentAuthError(c, http.StatusUnauthorized, "vSearch key is invalid")
			return
		}
		user, err := model.GetUserById(key.UserId, false)
		if err != nil || user.Status != common.UserStatusEnabled {
			writeSearchAgentAuthError(c, http.StatusForbidden, "vSearch key owner is unavailable")
			return
		}
		if key.EnterpriseID > 0 {
			enterprise, enterpriseErr := model.GetEnterpriseByID(key.EnterpriseID)
			if enterpriseErr != nil || enterprise.Status != model.EnterpriseStatusEnabled {
				writeSearchAgentAuthError(c, http.StatusForbidden, "vSearch enterprise is unavailable")
				return
			}
			membership, membershipErr := model.GetEnterpriseMembership(key.EnterpriseID, key.UserId)
			if membershipErr != nil || membership.Status != model.EnterpriseMembershipStatusActive {
				writeSearchAgentAuthError(c, http.StatusForbidden, "vSearch key owner is no longer an enterprise member")
				return
			}
		}

		c.Set(SearchAgentKeyContextKey, key)
		c.Set("id", key.UserId)
		c.Set("enterprise_id", key.EnterpriseID)
		c.Set("search_agent_key_id", key.Id)
		if err := model.TouchSearchAgentKey(key.Id); err != nil {
			common.SysLog("failed to update vSearch AgentKey last-used time: " + err.Error())
		}
		c.Next()
	}
}

func writeSearchAgentAuthError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": gin.H{
			"code":    -32001,
			"message": message,
		},
	})
}

// SearchAdminAuth allows root users and active enterprise owners/admins to
// manage AgentKeys without granting access to model administration.
func SearchAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt("role") >= common.RoleRootUser {
			// Root users are platform-scoped even if they also belong to an enterprise.
			c.Set("enterprise_id", 0)
			c.Next()
			return
		}

		membership, err := model.GetEnterpriseMembershipByUserID(c.GetInt("id"))
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				common.SysLog("failed to load search admin membership: " + err.Error())
			}
			searchPermissionDenied(c)
			return
		}
		enterprise, err := model.GetEnterpriseByID(membership.EnterpriseID)
		if err != nil || enterprise.Status != model.EnterpriseStatusEnabled {
			searchPermissionDenied(c)
			return
		}
		if membership.Role != model.EnterpriseMembershipRoleOwner && membership.Role != model.EnterpriseMembershipRoleAdmin {
			searchPermissionDenied(c)
			return
		}
		c.Set("enterprise_id", membership.EnterpriseID)
		c.Next()
	}
}

func searchPermissionDenied(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
	})
}
