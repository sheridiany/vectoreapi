package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type searchAgentKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type adminSearchAgentKeyRequest struct {
	UserID int      `json:"user_id"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type searchAgentKeyResponse struct {
	ID           int      `json:"id"`
	UserID       int      `json:"user_id"`
	EnterpriseID int      `json:"enterprise_id"`
	Label        string   `json:"label"`
	Prefix       string   `json:"prefix"`
	Owner        string   `json:"owner,omitempty"`
	Status       string   `json:"status"`
	Scopes       []string `json:"scopes"`
	CreatedAt    int64    `json:"created_at"`
	LastUsedAt   int64    `json:"last_used_at,omitempty"`
	ExpiresAt    int64    `json:"expires_at,omitempty"`
}

type createdSearchAgentKeyResponse struct {
	searchAgentKeyResponse
	Secret string `json:"secret"`
}

func toSearchAgentKeyResponse(key *model.SearchAgentKey, owner string) (searchAgentKeyResponse, error) {
	scopes, err := key.GetScopes()
	if err != nil {
		return searchAgentKeyResponse{}, err
	}
	return searchAgentKeyResponse{
		ID: key.Id, UserID: key.UserId, EnterpriseID: key.EnterpriseID,
		Label: key.Name, Prefix: key.KeyPrefix, Owner: owner,
		Status: searchAgentKeyStatusName(key.Status), Scopes: scopes,
		CreatedAt: key.CreatedAt, LastUsedAt: key.LastUsedAt, ExpiresAt: key.ExpiresAt,
	}, nil
}

func searchAgentKeyStatusName(status int) string {
	switch status {
	case model.SearchAgentKeyStatusDisabled:
		return "disabled"
	case model.SearchAgentKeyStatusRevoked:
		return "revoked"
	default:
		return "active"
	}
}

func buildSearchAgentKeyResponses(keys []*model.SearchAgentKey, includeOwner bool) ([]searchAgentKeyResponse, error) {
	responses := make([]searchAgentKeyResponse, 0, len(keys))
	for _, key := range keys {
		owner := ""
		if includeOwner {
			if user, err := model.GetUserById(key.UserId, false); err == nil {
				owner = user.Username
			}
		}
		response, err := toSearchAgentKeyResponse(key, owner)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func GetSearchAgentKeys(c *gin.Context) {
	keys, err := model.GetSearchAgentKeysByUserID(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses, err := buildSearchAgentKeyResponses(keys, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, responses)
}

func CreateSearchAgentKey(c *gin.Context) {
	var request searchAgentKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, errors.New("invalid search agent key request"))
		return
	}
	enterpriseID := 0
	if membership, err := model.GetEnterpriseMembershipByUserID(c.GetInt("id")); err == nil {
		enterpriseID = membership.EnterpriseID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	key, secret, err := model.CreateSearchAgentKey(c.GetInt("id"), enterpriseID, request.Name, request.Scopes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := toSearchAgentKeyResponse(key, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, createdSearchAgentKeyResponse{searchAgentKeyResponse: response, Secret: secret})
}

func RevokeSearchAgentKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("search agent key id is invalid"))
		return
	}
	if err := model.RevokeSearchAgentKey(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminGetSearchAgentKeys(c *gin.Context) {
	var (
		keys []*model.SearchAgentKey
		err  error
	)
	if enterpriseID := c.GetInt("enterprise_id"); enterpriseID > 0 {
		keys, err = model.GetSearchAgentKeysByEnterpriseID(enterpriseID)
	} else {
		keys, err = model.GetAllSearchAgentKeys()
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses, err := buildSearchAgentKeyResponses(keys, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, responses)
}

func AdminCreateSearchAgentKey(c *gin.Context) {
	var request adminSearchAgentKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.UserID <= 0 {
		common.ApiError(c, errors.New("user id and search agent key details are required"))
		return
	}
	user, err := model.GetUserById(request.UserID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterpriseID := 0
	if membership, membershipErr := model.GetEnterpriseMembershipByUserID(user.Id); membershipErr == nil {
		enterpriseID = membership.EnterpriseID
	} else if !errors.Is(membershipErr, gorm.ErrRecordNotFound) {
		common.ApiError(c, membershipErr)
		return
	}
	if managerEnterpriseID := c.GetInt("enterprise_id"); managerEnterpriseID > 0 && enterpriseID != managerEnterpriseID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "user is outside the managed enterprise"})
		return
	}
	key, secret, err := model.CreateSearchAgentKey(user.Id, enterpriseID, request.Name, request.Scopes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := toSearchAgentKeyResponse(key, user.Username)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, createdSearchAgentKeyResponse{searchAgentKeyResponse: response, Secret: secret})
}

func AdminRevokeSearchAgentKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, errors.New("search agent key id is invalid"))
		return
	}
	key, err := model.GetSearchAgentKeyByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if enterpriseID := c.GetInt("enterprise_id"); enterpriseID > 0 && key.EnterpriseID != enterpriseID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "key is outside the managed enterprise"})
		return
	}
	if err := model.RevokeSearchAgentKeyByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
