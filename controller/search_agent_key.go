package controller

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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

type searchAgentInstallRequest struct {
	Token string `json:"token"`
	Label string `json:"label"`
}

type searchAgentActivationRequest struct {
	Token string `json:"token"`
}

type searchAgentInstallPayload struct {
	KeyID             int `json:"key_id"`
	CredentialVersion int `json:"credential_version"`
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

func CreateSearchAgentKeyInstallToken(c *gin.Context) {
	createSearchAgentKeyInstallToken(c, false)
}

// AdminCreateSearchAgentKeyInstallToken issues an install token for a key in
// the enterprise managed by the authenticated search administrator.
func AdminCreateSearchAgentKeyInstallToken(c *gin.Context) {
	createSearchAgentKeyInstallToken(c, true)
}

func createSearchAgentKeyInstallToken(c *gin.Context, allowManagedKey bool) {
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
	if key.UserId != c.GetInt("id") {
		if !allowManagedKey {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "only the key owner can create an install token"})
			return
		}
		if c.GetInt("role") < common.RoleRootUser {
			if enterpriseID := c.GetInt("enterprise_id"); enterpriseID == 0 || key.EnterpriseID != enterpriseID {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "key is outside the managed enterprise"})
				return
			}
		}
	}
	if key.Status != model.SearchAgentKeyStatusActive || (key.ExpiresAt > 0 && key.ExpiresAt <= common.GetTimestamp()) {
		common.ApiError(c, errors.New("search agent key is not active"))
		return
	}
	payload, err := common.Marshal(searchAgentInstallPayload{KeyID: key.Id, CredentialVersion: key.CredentialVersion})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, flow, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeSearchAgentInstall,
		UserId:    key.UserId,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"token":      "vr_search_install_" + token,
		"expires_at": flow.ExpiresAt.Unix(),
	})
}

// InstallSearchAgent prepares a pending credential while the current key stays
// active. The installer activates it only after every local config is durable.
func InstallSearchAgent(c *gin.Context) {
	var request searchAgentInstallRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, errors.New("install token is required"))
		return
	}
	token := strings.TrimSpace(request.Token)
	token = strings.TrimPrefix(token, "vr_search_install_")
	if token == "" {
		common.ApiError(c, errors.New("install token is required"))
		return
	}
	mcpURL := searchPublicMCPURL(c)
	if mcpURL == "" {
		common.ApiError(c, errors.New("vSearch public MCP URL must use HTTPS; loopback development may use HTTP"))
		return
	}
	var secret string
	var activationToken string
	installedKeyID := 0
	_, err := model.ConsumeAuthFlowWithAction(token, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeSearchAgentInstall,
	}, func(tx *gorm.DB, flow *model.AuthFlow) error {
		var payload searchAgentInstallPayload
		if err := common.Unmarshal([]byte(flow.Payload), &payload); err != nil || payload.KeyID <= 0 {
			return errors.New("install token payload is invalid")
		}
		installedKeyID = payload.KeyID
		var prepareErr error
		secret, activationToken, prepareErr = model.PrepareSearchAgentKeyRotationWithTx(tx, payload.KeyID, payload.CredentialVersion, time.Now().Add(10*time.Minute))
		return prepareErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if secret == "" || activationToken == "" {
		common.ApiError(c, errors.New("install token did not produce a key"))
		return
	}
	common.ApiSuccess(c, gin.H{
		"secret":           secret,
		"activation_token": "vr_search_activate_" + activationToken,
		"label":            strings.TrimSpace(request.Label),
		"mcp_url":          mcpURL,
		"mcpUrl":           mcpURL,
		"key_id":           installedKeyID,
		"installed":        false,
	})
}

func ActivateSearchAgentInstall(c *gin.Context) {
	var request searchAgentActivationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, errors.New("activation token is required"))
		return
	}
	token := strings.TrimPrefix(strings.TrimSpace(request.Token), "vr_search_activate_")
	if token == "" {
		common.ApiError(c, errors.New("activation token is required"))
		return
	}
	keyID, err := model.ActivatePreparedSearchAgentKeyRotation(token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"key_id": keyID, "installed": true})
}

func searchPublicMCPURL(c *gin.Context) string {
	if configured := strings.TrimSpace(os.Getenv("VSEARCH_PUBLIC_MCP_URL")); configured != "" {
		if strings.Contains(configured, "#") {
			return ""
		}
		if parsed, err := url.ParseRequestURI(configured); err == nil && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" {
			hostname := parsed.Hostname()
			ip := net.ParseIP(hostname)
			loopback := strings.EqualFold(hostname, "localhost") || (ip != nil && ip.IsLoopback())
			if strings.EqualFold(parsed.Scheme, "https") || (strings.EqualFold(parsed.Scheme, "http") && loopback) {
				return strings.TrimRight(parsed.String(), "/")
			}
		}
		return ""
	}
	requestURL := &url.URL{Scheme: "http", Host: c.Request.Host}
	hostname := requestURL.Hostname()
	ip := net.ParseIP(hostname)
	if !strings.EqualFold(hostname, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return ""
	}
	protocol := "http"
	if c.Request.TLS != nil {
		protocol = "https"
	} else if forwarded := strings.ToLower(strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])); forwarded == "http" || forwarded == "https" {
		protocol = forwarded
	}
	return protocol + "://" + c.Request.Host + "/v1/mcp"
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
