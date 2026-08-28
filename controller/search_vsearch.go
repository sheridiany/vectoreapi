package controller

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/vsearch"
	"github.com/gin-gonic/gin"
)

var (
	searchRuntime      = vsearch.NewExecutionRuntime(nil)
	searchControlPlane = vsearch.NewControlPlane(nil)
)

type searchUpstreamAccountRequest struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Secret   string `json:"secret"`
	APIKey   string `json:"api_key"`
	PoolID   int    `json:"pool_id"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

type searchCatalogSyncRequest struct {
	Queries []string `json:"queries"`
	Prefix  string   `json:"prefix"`
}

type searchCapabilityConfigRequest struct {
	Enabled     *bool    `json:"enabled"`
	Price       *float64 `json:"price"`
	PriceMicros *int64   `json:"price_micros"`
}

type searchCapabilityEnterpriseGrantsRequest struct {
	EnterpriseIDs []int `json:"enterprise_ids"`
}

type searchCapabilityEnterpriseGrantsResponse struct {
	CapabilityID  string `json:"capability_id"`
	AccessMode    string `json:"access_mode"`
	EnterpriseIDs []int  `json:"enterprise_ids"`
}

type searchExecuteRequest struct {
	ServiceID string         `json:"service_id"`
	Params    map[string]any `json:"params"`
}

type searchLogResponse struct {
	ID                        int64   `json:"id"`
	CreatedAt                 int64   `json:"created_at"`
	Service                   string  `json:"service"`
	Endpoint                  string  `json:"endpoint"`
	Status                    string  `json:"status"`
	LatencyMs                 int64   `json:"latency_ms"`
	AgentKeyName              string  `json:"agent_key_name"`
	RequestID                 string  `json:"request_id"`
	Charge                    float64 `json:"charge"`
	ChargeMicros              int64   `json:"charge_micros"`
	UserID                    int     `json:"user_id,omitempty"`
	UserName                  string  `json:"user_name,omitempty"`
	EnterpriseID              int     `json:"enterprise_id,omitempty"`
	EnterpriseName            string  `json:"enterprise_name,omitempty"`
	Account                   string  `json:"account,omitempty"`
	UpstreamCost              float64 `json:"upstream_cost,omitempty"`
	UpstreamCostMicros        int64   `json:"upstream_cost_micros,omitempty"`
	Profit                    float64 `json:"profit,omitempty"`
	ProfitMicros              int64   `json:"profit_micros,omitempty"`
	ExecutionPhase            string  `json:"execution_phase,omitempty"`
	BillingState              string  `json:"billing_state,omitempty"`
	PlannedChargeMicros       int64   `json:"planned_charge_micros,omitempty"`
	PlannedUpstreamCostMicros int64   `json:"planned_upstream_cost_micros,omitempty"`
	ErrorCode                 string  `json:"error_code,omitempty"`
}

type searchLogStatResponse struct {
	TotalRequests         int64   `json:"total_requests"`
	SuccessRequests       int64   `json:"success_requests"`
	ErrorRequests         int64   `json:"error_requests"`
	PendingRequests       int64   `json:"pending_requests"`
	IndeterminateRequests int64   `json:"indeterminate_requests"`
	SuccessRate           float64 `json:"success_rate"`
	AverageLatencyMs      int64   `json:"average_latency_ms"`
	Quota                 float64 `json:"quota"`
	QuotaMicros           int64   `json:"quota_micros"`
	UpstreamCost          float64 `json:"upstream_cost"`
	UpstreamCostMicros    int64   `json:"upstream_cost_micros"`
	Revenue               float64 `json:"revenue"`
	RevenueMicros         int64   `json:"revenue_micros"`
	Profit                float64 `json:"profit"`
	ProfitMicros          int64   `json:"profit_micros"`
}

func GetSearchCatalog(c *gin.Context) {
	catalog, err := searchRuntime.ListCatalogGroups(c.Request.Context(), dashboardSearchPrincipal(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, catalog)
}

func DiscoverSearchCapabilities(c *gin.Context) {
	result, err := searchRuntime.Discover(c.Request.Context(), dashboardSearchPrincipal(c), c.Query("q"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func DescribeSearchCapability(c *gin.Context) {
	result, err := searchRuntime.Describe(c.Request.Context(), dashboardSearchPrincipal(c), c.Param("service_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ExecuteSearchCapability(c *gin.Context) {
	var request searchExecuteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "vSearch execution request is invalid")
		return
	}
	principal, err := dashboardSearchExecutionPrincipal(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := searchRuntime.Execute(c.Request.Context(), principal, vsearch.ExecuteCommand{
		ServiceID: request.ServiceID, Params: request.Params, IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		var publicErr *vsearch.PublicError
		if errors.As(err, &publicErr) {
			c.JSON(publicErr.HTTPStatus, gin.H{"success": false, "message": publicErr.Message, "code": publicErr.Code})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetSearchLogs(c *gin.Context) {
	writeSearchLogs(c, false)
}

func GetSearchLogStat(c *gin.Context) {
	writeSearchLogStat(c, false)
}

func AdminListSearchUpstreamAccounts(c *gin.Context) {
	accounts, err := searchControlPlane.ListAccounts(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, accounts)
}

func AdminCreateSearchUpstreamAccount(c *gin.Context) {
	adminSaveSearchUpstreamAccount(c, 0)
}

func AdminUpdateSearchUpstreamAccount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "vSearch upstream account id is invalid")
		return
	}
	adminSaveSearchUpstreamAccount(c, id)
}

func adminSaveSearchUpstreamAccount(c *gin.Context, id int) {
	var request searchUpstreamAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "vSearch upstream account request is invalid")
		return
	}
	if strings.TrimSpace(request.Secret) == "" {
		request.Secret = request.APIKey
	}
	account, err := searchControlPlane.SaveAccount(c.Request.Context(), vsearch.AccountCommand{
		ID: id, Name: request.Name, BaseURL: request.BaseURL, Secret: request.Secret,
		PoolID: request.PoolID, Weight: request.Weight, Priority: request.Priority, Status: request.Status,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, account)
}

func AdminDeleteSearchUpstreamAccount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "vSearch upstream account id is invalid")
		return
	}
	if err := searchControlPlane.DeleteAccount(c.Request.Context(), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminTestSearchUpstreamAccount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "vSearch upstream account id is invalid")
		return
	}
	account, err := searchControlPlane.ProbeAccount(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, account)
}

func AdminGetSearchCatalog(c *gin.Context) {
	catalog, err := searchRuntime.ListCatalog(c.Request.Context(), vsearch.Principal{}, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, catalog)
}

func AdminSyncSearchCatalog(c *gin.Context) {
	var request searchCatalogSyncRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			common.ApiErrorMsg(c, "vSearch catalog sync request is invalid")
			return
		}
	}
	result, err := searchControlPlane.SyncCatalog(c.Request.Context(), vsearch.SyncCommand{Queries: request.Queries, Prefix: request.Prefix})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminConfigureSearchCapability(c *gin.Context) {
	capability, err := model.GetSearchCapabilityByPublicID(strings.TrimSpace(c.Param("id")))
	if err != nil {
		common.ApiErrorMsg(c, "vSearch capability id is invalid")
		return
	}
	var request searchCapabilityConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil || (request.Enabled == nil && request.Price == nil && request.PriceMicros == nil) {
		common.ApiErrorMsg(c, "vSearch capability configuration is invalid")
		return
	}
	enabled := capability.Status == model.SearchCapabilityStatusEnabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	priceMicros := capability.PriceMicros
	if request.PriceMicros != nil {
		priceMicros = *request.PriceMicros
	}
	if request.Price != nil {
		if math.IsNaN(*request.Price) || math.IsInf(*request.Price, 0) || *request.Price < 0 || *request.Price > 9_000_000_000 {
			common.ApiErrorMsg(c, "vSearch capability price is invalid")
			return
		}
		priceMicros = int64(math.Round(*request.Price * 1_000_000))
	}
	result, err := searchControlPlane.ConfigureCapability(c.Request.Context(), vsearch.CapabilityCommand{ID: capability.Id, Enabled: enabled, PriceMicros: priceMicros})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminGetSearchCapabilityEnterpriseGrants(c *gin.Context) {
	capability, err := model.GetSearchCapabilityByPublicID(strings.TrimSpace(c.Param("id")))
	if err != nil {
		common.ApiErrorMsg(c, "vSearch capability id is invalid")
		return
	}
	response, err := searchCapabilityEnterpriseGrants(capability)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func AdminSetSearchCapabilityEnterpriseGrants(c *gin.Context) {
	capability, err := model.GetSearchCapabilityByPublicID(strings.TrimSpace(c.Param("id")))
	if err != nil {
		common.ApiErrorMsg(c, "vSearch capability id is invalid")
		return
	}
	var request searchCapabilityEnterpriseGrantsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "vSearch capability enterprise grants are invalid")
		return
	}
	enterpriseIDs := make([]int, 0, len(request.EnterpriseIDs))
	seen := make(map[int]struct{}, len(request.EnterpriseIDs))
	for _, enterpriseID := range request.EnterpriseIDs {
		if enterpriseID <= 0 {
			common.ApiErrorMsg(c, "vSearch capability enterprise grant is invalid")
			return
		}
		if _, exists := seen[enterpriseID]; exists {
			continue
		}
		if _, err := model.GetEnterpriseByID(enterpriseID); err != nil {
			common.ApiErrorMsg(c, "vSearch capability enterprise does not exist")
			return
		}
		seen[enterpriseID] = struct{}{}
		enterpriseIDs = append(enterpriseIDs, enterpriseID)
	}
	if err := model.ReplaceSearchCapabilityEnterpriseGrants(capability.Id, enterpriseIDs); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := searchCapabilityEnterpriseGrants(capability)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func searchCapabilityEnterpriseGrants(capability *model.SearchCapability) (*searchCapabilityEnterpriseGrantsResponse, error) {
	grants, err := model.ListSearchCapabilityEnterpriseGrants(capability.Id)
	if err != nil {
		return nil, err
	}
	enterpriseIDs := make([]int, 0, len(grants))
	for _, grant := range grants {
		enterpriseIDs = append(enterpriseIDs, grant.EnterpriseID)
	}
	accessMode := "all_enterprises"
	if len(enterpriseIDs) > 0 {
		accessMode = "selected_enterprises"
	}
	return &searchCapabilityEnterpriseGrantsResponse{
		CapabilityID:  capability.PublicID,
		AccessMode:    accessMode,
		EnterpriseIDs: enterpriseIDs,
	}, nil
}

func AdminGetSearchUsageLogs(c *gin.Context) {
	writeSearchLogs(c, true)
}

func AdminGetSearchUsageStat(c *gin.Context) {
	writeSearchLogStat(c, true)
}

func AdminExportSearchUsageLogs(c *gin.Context) {
	query := searchUsageQuery(c, true)
	rows, err := vsearch.ExportUsageLogs(c.Request.Context(), query, 10_000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=vsearch-usage.csv")
	_, _ = c.Writer.WriteString("\ufeff")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"time", "request_id", "user_id", "user", "enterprise_id", "enterprise", "service", "upstream_account", "status", "latency_ms", "upstream_cost", "revenue", "profit", "error_code"})
	for _, row := range rows {
		event := row.Event
		_ = writer.Write([]string{
			time.Unix(event.CreatedAt, 0).Format(time.RFC3339), sanitizeSearchCSVText(event.RequestID),
			strconv.Itoa(event.UserID), sanitizeSearchCSVText(row.UserName), strconv.Itoa(event.EnterpriseID), sanitizeSearchCSVText(row.EnterpriseName), sanitizeSearchCSVText(event.ServiceName), sanitizeSearchCSVText(row.AccountName),
			searchUsageStatusName(event.Status), strconv.FormatInt(event.LatencyMs, 10),
			formatSearchMoney(event.UpstreamCostMicros), formatSearchMoney(event.ChargeMicros),
			formatSearchMoney(event.ChargeMicros - event.UpstreamCostMicros), sanitizeSearchCSVText(event.ErrorCode),
		})
	}
	writer.Flush()
}

func writeSearchLogs(c *gin.Context, admin bool) {
	pageInfo := common.GetPageQuery(c)
	query := searchUsageQuery(c, admin)
	query.Offset = pageInfo.GetStartIdx()
	query.Limit = pageInfo.GetPageSize()
	logs, total, err := vsearch.ListUsageLogs(c.Request.Context(), query, admin)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]searchLogResponse, 0, len(logs))
	for _, log := range logs {
		event := log.Event
		item := searchLogResponse{
			ID: event.Id, CreatedAt: event.CreatedAt, Service: event.ServiceName,
			Endpoint: event.Action, Status: searchUsageStatusName(event.Status),
			LatencyMs: event.LatencyMs, RequestID: event.RequestID,
			Charge: float64(event.ChargeMicros) / 1_000_000, ChargeMicros: event.ChargeMicros,
			ErrorCode: event.ErrorCode,
		}
		item.AgentKeyName = log.AgentKeyName
		if admin {
			item.UserID = event.UserID
			item.EnterpriseID = event.EnterpriseID
			item.UserName = log.UserName
			item.EnterpriseName = log.EnterpriseName
			item.UpstreamCost = float64(event.UpstreamCostMicros) / 1_000_000
			item.UpstreamCostMicros = event.UpstreamCostMicros
			item.ProfitMicros = event.ChargeMicros - event.UpstreamCostMicros
			item.Profit = float64(item.ProfitMicros) / 1_000_000
			item.ExecutionPhase = event.ExecutionPhase
			item.BillingState = event.BillingState
			item.PlannedChargeMicros = event.PlannedChargeMicros
			item.PlannedUpstreamCostMicros = event.PlannedUpstreamCostMicros
			item.Account = log.AccountName
		}
		items = append(items, item)
	}
	pageInfo.SetItems(items)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

func writeSearchLogStat(c *gin.Context, admin bool) {
	stat, err := model.GetSearchUsageStat(searchUsageQuery(c, admin))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := searchLogStatResponse{
		TotalRequests: stat.RequestCount, SuccessRequests: stat.SuccessCount, ErrorRequests: stat.ErrorCount,
		PendingRequests: stat.PendingCount, IndeterminateRequests: stat.IndeterminateCount,
		Quota: float64(stat.ChargeMicros) / 1_000_000, QuotaMicros: stat.ChargeMicros,
		Revenue: float64(stat.ChargeMicros) / 1_000_000, RevenueMicros: stat.ChargeMicros,
	}
	completedRequests := stat.SuccessCount + stat.ErrorCount
	if completedRequests > 0 {
		result.SuccessRate = mathPercent(stat.SuccessCount, completedRequests)
	}
	if completedRequests > 0 {
		result.AverageLatencyMs = stat.TotalLatencyMs / completedRequests
	}
	if admin {
		result.UpstreamCost = float64(stat.UpstreamCostMicros) / 1_000_000
		result.UpstreamCostMicros = stat.UpstreamCostMicros
		result.Profit = float64(stat.MarginMicros) / 1_000_000
		result.ProfitMicros = stat.MarginMicros
	}
	common.ApiSuccess(c, result)
}

func searchUsageQuery(c *gin.Context, admin bool) model.SearchUsageQuery {
	query := model.SearchUsageQuery{}
	if !admin {
		query.UserID = c.GetInt("id")
	}
	query.StartAt, _ = strconv.ParseInt(c.Query("start_at"), 10, 64)
	query.EndAt, _ = strconv.ParseInt(c.Query("end_at"), 10, 64)
	if query.StartAt == 0 {
		daysValue := c.Query("days")
		if daysValue == "" {
			daysValue = c.DefaultQuery("range", "30")
		}
		days, _ := strconv.Atoi(daysValue)
		if days < 1 || days > 365 {
			days = 30
		}
		query.StartAt = time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	}
	query.ServiceID = strings.TrimSpace(c.Query("service_id"))
	query.Action = strings.TrimSpace(c.Query("action"))
	query.SearchText = strings.TrimSpace(c.Query("query"))
	switch strings.ToLower(strings.TrimSpace(c.Query("status"))) {
	case "success", "succeeded":
		query.Status = model.SearchUsageStatusSucceeded
	case "error", "failed":
		query.Status = model.SearchUsageStatusFailed
	case "pending":
		query.Status = model.SearchUsageStatusPending
	case "indeterminate":
		query.Status = model.SearchUsageStatusIndeterminate
	}
	return query
}

func dashboardSearchPrincipal(c *gin.Context) vsearch.Principal {
	return vsearch.Principal{UserID: c.GetInt("id"), EnterpriseID: c.GetInt("enterprise_id")}
}

func dashboardSearchExecutionPrincipal(c *gin.Context) (vsearch.Principal, error) {
	userID := c.GetInt("id")
	enterpriseID := c.GetInt("enterprise_id")
	keys, err := model.GetSearchAgentKeysByUserID(userID)
	if err != nil {
		return vsearch.Principal{}, err
	}
	for _, key := range keys {
		if key.EnterpriseID != enterpriseID || key.Status != model.SearchAgentKeyStatusActive || (key.ExpiresAt > 0 && key.ExpiresAt <= common.GetTimestamp()) {
			continue
		}
		scopes, scopeErr := key.GetScopes()
		if scopeErr != nil {
			return vsearch.Principal{}, scopeErr
		}
		return vsearch.Principal{UserID: userID, EnterpriseID: enterpriseID, AgentKeyID: key.Id, Scopes: scopes}, nil
	}
	return vsearch.Principal{}, errors.New("create a vSearch AgentKey before executing capabilities")
}

func mcpSearchPrincipal(c *gin.Context) (vsearch.Principal, error) {
	value, ok := c.Get("search_agent_key")
	if !ok {
		return vsearch.Principal{}, errors.New("vSearch AgentKey context is missing")
	}
	key, ok := value.(*model.SearchAgentKey)
	if !ok || key == nil {
		return vsearch.Principal{}, errors.New("vSearch AgentKey context is invalid")
	}
	scopes, err := key.GetScopes()
	if err != nil {
		return vsearch.Principal{}, err
	}
	return vsearch.Principal{UserID: key.UserId, EnterpriseID: key.EnterpriseID, AgentKeyID: key.Id, Scopes: scopes}, nil
}

func searchUsageStatusName(status int) string {
	if status == model.SearchUsageStatusSucceeded {
		return "success"
	}
	if status == model.SearchUsageStatusPending {
		return "pending"
	}
	if status == model.SearchUsageStatusIndeterminate {
		return "indeterminate"
	}
	return "error"
}

func mathPercent(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	percentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(value)*100/float64(total)), 64)
	return percentage
}

func formatSearchMoney(value int64) string {
	return strconv.FormatFloat(float64(value)/1_000_000, 'f', 6, 64)
}

func sanitizeSearchCSVText(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}
