package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/vsearch"
	"github.com/gin-gonic/gin"
)

const searchMCPProtocolVersion = "2025-06-18"

type searchMCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  searchMCPParams `json:"params"`
}

type searchMCPParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func HandleSearchMCP(c *gin.Context) {
	var request searchMCPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeSearchMCPError(c, http.StatusBadRequest, nil, -32700, "MCP 请求无法解析。")
		return
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		writeSearchMCPError(c, http.StatusBadRequest, request.ID, -32600, "MCP 请求格式无效。")
		return
	}
	if request.ID == nil {
		c.Status(http.StatusAccepted)
		c.Writer.WriteHeaderNow()
		return
	}
	principal, err := mcpSearchPrincipal(c)
	if err != nil {
		writeSearchMCPError(c, http.StatusUnauthorized, request.ID, -32001, "vSearch 密钥无效。")
		return
	}

	switch request.Method {
	case "initialize":
		c.Header("mcp-session-id", "vsearch-"+common.NewRequestId())
		writeSearchMCPResult(c, request.ID, gin.H{
			"protocolVersion": searchMCPProtocolVersion,
			"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
			"serverInfo":      gin.H{"name": "vsearch", "version": "1.0.0"},
			"instructions":    "先使用 vector_relay_find_tools 发现能力，再用 vector_relay_describe_tool 获取参数，最后通过 vector_relay_call 执行。",
		})
	case "ping":
		writeSearchMCPResult(c, request.ID, gin.H{})
	case "tools/list":
		writeSearchMCPResult(c, request.ID, gin.H{"tools": searchMCPTools()})
	case "tools/call":
		handleSearchMCPToolCall(c, request.ID, principal, request.Params)
	default:
		writeSearchMCPError(c, http.StatusOK, request.ID, -32601, "不支持的 MCP 方法。")
	}
}

func handleSearchMCPToolCall(c *gin.Context, id any, principal vsearch.Principal, params searchMCPParams) {
	arguments := params.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	switch strings.TrimSpace(params.Name) {
	case "vector_relay_capabilities":
		catalog, err := searchRuntime.ListCatalog(c.Request.Context(), principal, false)
		if err != nil {
			writeSearchMCPToolError(c, id, err)
			return
		}
		category, _ := arguments["category"].(string)
		if category != "" {
			filtered := catalog[:0]
			for _, capability := range catalog {
				if strings.EqualFold(capability.Category, strings.TrimSpace(category)) {
					filtered = append(filtered, capability)
				}
			}
			catalog = filtered
		}
		writeSearchMCPToolResult(c, id, gin.H{
			"capabilities": catalog,
			"workflow":     []string{"discover", "describe", "execute"},
		})
	case "vector_relay_find_tools":
		query, _ := arguments["query"].(string)
		result, err := searchRuntime.Discover(c.Request.Context(), principal, query)
		if err != nil {
			writeSearchMCPToolError(c, id, err)
			return
		}
		writeSearchMCPToolResult(c, id, result)
	case "vector_relay_describe_tool":
		serviceID, _ := arguments["serviceId"].(string)
		if !validPublicSearchServiceID(serviceID) {
			writeSearchMCPError(c, http.StatusOK, id, -32602, "必须使用能力发现返回的 vSearch serviceId。")
			return
		}
		result, err := searchRuntime.Describe(c.Request.Context(), principal, serviceID)
		if err != nil {
			writeSearchMCPToolError(c, id, err)
			return
		}
		writeSearchMCPToolResult(c, id, result)
	case "vector_relay_call":
		serviceID, _ := arguments["serviceId"].(string)
		if !validPublicSearchServiceID(serviceID) {
			writeSearchMCPError(c, http.StatusOK, id, -32602, "必须使用能力发现返回的 vSearch serviceId。")
			return
		}
		toolParams, _ := arguments["params"].(map[string]any)
		idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if idempotencyKey == "" {
			sessionID := strings.TrimSpace(c.GetHeader("mcp-session-id"))
			if sessionID == "" {
				writeSearchMCPToolError(c, id, &vsearch.PublicError{
					Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "vSearch 执行请求必须携带 Idempotency-Key 或 mcp-session-id。", HTTPStatus: http.StatusBadRequest,
				})
				return
			}
			var keyErr error
			idempotencyKey, keyErr = searchMCPIdempotencyKey(sessionID, id, serviceID, toolParams)
			if keyErr != nil {
				writeSearchMCPToolError(c, id, keyErr)
				return
			}
		}
		result, err := searchRuntime.Execute(c.Request.Context(), principal, vsearch.ExecuteCommand{
			ServiceID: serviceID, Params: toolParams, IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			writeSearchMCPToolError(c, id, err)
			return
		}
		writeSearchMCPToolResult(c, id, result)
	default:
		writeSearchMCPError(c, http.StatusOK, id, -32601, "不支持的 vSearch 工具。")
	}
}

func searchMCPIdempotencyKey(sessionID string, id any, serviceID string, params map[string]any) (string, error) {
	payload, err := common.Marshal(struct {
		SessionID string         `json:"session_id"`
		ID        any            `json:"id"`
		ServiceID string         `json:"service_id"`
		Params    map[string]any `json:"params"`
	}{SessionID: strings.TrimSpace(sessionID), ID: id, ServiceID: serviceID, Params: params})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return "mcp-" + hex.EncodeToString(digest[:]), nil
}

func searchMCPTools() []gin.H {
	return []gin.H{
		{
			"name": "vector_relay_capabilities", "title": "vSearch 能力地图",
			"description": "查看当前 vSearch 密钥可使用的能力分类和调用流程。",
			"inputSchema": gin.H{"type": "object", "properties": gin.H{"category": gin.H{"type": "string"}}, "additionalProperties": false},
		},
		{
			"name": "vector_relay_find_tools", "title": "vSearch 能力发现",
			"description": "根据完整自然语言需求查找已同步并授权的 vSearch 能力。",
			"inputSchema": gin.H{"type": "object", "required": []string{"query"}, "properties": gin.H{"query": gin.H{"type": "string"}}, "additionalProperties": false},
		},
		{
			"name": "vector_relay_describe_tool", "title": "vSearch 能力说明",
			"description": "读取能力的准确参数定义和价格；执行前先调用。",
			"inputSchema": gin.H{"type": "object", "required": []string{"serviceId"}, "properties": gin.H{"serviceId": gin.H{"type": "string", "pattern": "^vr_svc_[a-f0-9]{16}$"}}, "additionalProperties": false},
		},
		{
			"name": "vector_relay_call", "title": "vSearch 能力调用",
			"description": "执行已经发现并描述过的 vSearch 能力。",
			"inputSchema": gin.H{"type": "object", "required": []string{"serviceId"}, "properties": gin.H{"serviceId": gin.H{"type": "string", "pattern": "^vr_svc_[a-f0-9]{16}$"}, "params": gin.H{"type": "object", "additionalProperties": true}}, "additionalProperties": false},
		},
	}
}

func validPublicSearchServiceID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != len("vr_svc_")+16 || !strings.HasPrefix(value, "vr_svc_") {
		return false
	}
	for _, character := range value[len("vr_svc_"):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func writeSearchMCPToolResult(c *gin.Context, id any, value any) {
	data, err := common.Marshal(value)
	if err != nil {
		writeSearchMCPToolError(c, id, err)
		return
	}
	writeSearchMCPResult(c, id, gin.H{
		"content":           []gin.H{{"type": "text", "text": string(data)}},
		"structuredContent": value,
		"isError":           false,
	})
}

func writeSearchMCPToolError(c *gin.Context, id any, err error) {
	publicErr := vsearchPublicError(err)
	publicValue := gin.H{"code": publicErr.Code, "message": publicErr.Message}
	if publicErr.RetryAfterSeconds > 0 {
		c.Header("Retry-After", strconv.Itoa(publicErr.RetryAfterSeconds))
		publicValue["retry_after_seconds"] = publicErr.RetryAfterSeconds
	}
	value := gin.H{"error": publicValue}
	writeSearchMCPResult(c, id, gin.H{
		"content":           []gin.H{{"type": "text", "text": publicErr.Message}},
		"structuredContent": value,
		"isError":           true,
	})
}

func vsearchPublicError(err error) *vsearch.PublicError {
	var publicErr *vsearch.PublicError
	if errors.As(err, &publicErr) {
		return publicErr
	}
	return &vsearch.PublicError{Code: "VSEARCH_INTERNAL_ERROR", Message: "vSearch 服务暂不可用，请稍后重试。", HTTPStatus: http.StatusInternalServerError}
}

func writeSearchMCPResult(c *gin.Context, id any, result any) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "result": result})
}

func writeSearchMCPError(c *gin.Context, status int, id any, code int, message string) {
	c.JSON(status, gin.H{"jsonrpc": "2.0", "id": id, "error": gin.H{"code": code, "message": message}})
}
