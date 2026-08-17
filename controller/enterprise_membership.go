package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type adminAssignEnterpriseMemberRequest struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
}

type adminUpdateEnterpriseMemberRequest struct {
	Role   string `json:"role"`
	Status int    `json:"status"`
}

type adminCreateEnterpriseInvitationRequest struct {
	ExpiresAt int64 `json:"expires_at"`
	MaxUses   int   `json:"max_uses"`
}

type adminUpdateEnterpriseInvitationRequest struct {
	Status int `json:"status"`
}

func enterpriseIDParam(c *gin.Context) (int, error) {
	return strconv.Atoi(c.Param("id"))
}

func AdminListEnterpriseMembers(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	members, total, err := service.ListEnterpriseMembers(enterpriseID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(members)
	common.ApiSuccess(c, pageInfo)
}

func AdminListEnterpriseMemberCandidates(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	candidates, err := service.ListEnterpriseMemberCandidates(enterpriseID, c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, candidates)
}

func AdminAssignEnterpriseMember(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request adminAssignEnterpriseMemberRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Role = strings.ToLower(strings.TrimSpace(request.Role))
	if request.Role == model.EnterpriseMembershipRoleOwner && c.GetInt("role") < common.RoleRootUser {
		common.ApiError(c, errors.New("only root can appoint an enterprise owner"))
		return
	}
	membership, err := service.AssignEnterpriseMember(enterpriseID, request.UserID, c.GetInt("id"), request.Role)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Assign enterprise member",
		c.ClientIP(),
		"enterprise.member.assign",
		map[string]interface{}{"user_id": request.UserID, "role": request.Role},
		nil,
		nil,
	)
	common.ApiSuccess(c, membership)
}

func AdminUpdateEnterpriseMember(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request adminUpdateEnterpriseMemberRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Role = strings.ToLower(strings.TrimSpace(request.Role))
	if request.Role == model.EnterpriseMembershipRoleOwner && c.GetInt("role") < common.RoleRootUser {
		common.ApiError(c, errors.New("only root can appoint an enterprise owner"))
		return
	}
	membership, err := service.UpdateEnterpriseMember(enterpriseID, userID, request.Role, request.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Update enterprise member",
		c.ClientIP(),
		"enterprise.member.update",
		map[string]interface{}{"user_id": userID, "role": request.Role, "status": request.Status},
		nil,
		nil,
	)
	common.ApiSuccess(c, membership)
}

func AdminListEnterpriseInvitations(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	invitations, total, err := service.ListEnterpriseInvitations(enterpriseID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invitations)
	common.ApiSuccess(c, pageInfo)
}

func AdminCreateEnterpriseInvitation(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request adminCreateEnterpriseInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.CreateEnterpriseInvitation(enterpriseID, c.GetInt("id"), request.ExpiresAt, request.MaxUses)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Create enterprise invitation",
		c.ClientIP(),
		"enterprise.invitation.create",
		map[string]interface{}{"max_uses": request.MaxUses, "expires_at": request.ExpiresAt},
		nil,
		nil,
	)
	common.ApiSuccess(c, gin.H{
		"code":       result.Code,
		"invitation": result.Invitation,
	})
}

func AdminUpdateEnterpriseInvitation(c *gin.Context) {
	enterpriseID, err := enterpriseIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	invitationID, err := strconv.Atoi(c.Param("invitation_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request adminUpdateEnterpriseInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	invitation, err := service.UpdateEnterpriseInvitation(enterpriseID, invitationID, request.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordEnterpriseOperationAuditLog(
		enterpriseID,
		c.GetInt("id"),
		"Update enterprise invitation",
		c.ClientIP(),
		"enterprise.invitation.update",
		map[string]interface{}{"invitation_id": invitationID, "status": request.Status},
		nil,
		nil,
	)
	common.ApiSuccess(c, invitation)
}
