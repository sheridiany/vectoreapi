package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSearchMCPUsesHTTPIdempotencyKeyHeader(t *testing.T) {
	previousDB := model.DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, db.AutoMigrate(&model.SearchExecutionIdempotency{}))

	const idempotencyKey = "mcp-idempotency-key"
	digest := sha256.Sum256([]byte(idempotencyKey))
	_, state, err := model.BeginSearchExecutionIdempotency(
		9, hex.EncodeToString(digest[:]), strings.Repeat("c", 64), common.GetTimestamp(), common.GetTimestamp()+86_400,
	)
	require.NoError(t, err)
	require.Equal(t, model.SearchExecutionIdempotencyAcquired, state)

	c, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"vector_relay_call","arguments":{"serviceId":"vr_svc_0123456789abcdef","params":{"query":"news"}}}}`)
	c.Request.Header.Set("Idempotency-Key", idempotencyKey)

	HandleSearchMCP(c)

	require.Equal(t, http.StatusOK, recorder.Code, "MCP tool errors stay inside the JSON-RPC result")
	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Result.IsError)
	assert.Equal(t, "IDEMPOTENCY_KEY_REUSED", payload.Result.StructuredContent.Error.Code)
}

func TestSearchMCPDerivesIdempotencyKeyFromJSONRPCRequest(t *testing.T) {
	previousDB := model.DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, db.AutoMigrate(&model.SearchExecutionIdempotency{}))

	params := map[string]any{"query": "news"}
	const sessionID = "vsearch-session-4"
	idempotencyKey, err := searchMCPIdempotencyKey(sessionID, float64(4), "vr_svc_0123456789abcdef", params)
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(idempotencyKey))
	_, state, err := model.BeginSearchExecutionIdempotency(
		9, hex.EncodeToString(digest[:]), strings.Repeat("d", 64), common.GetTimestamp(), common.GetTimestamp()+86_400,
	)
	require.NoError(t, err)
	require.Equal(t, model.SearchExecutionIdempotencyAcquired, state)

	c, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"vector_relay_call","arguments":{"serviceId":"vr_svc_0123456789abcdef","params":{"query":"news"}}}}`)
	c.Request.Header.Set("mcp-session-id", sessionID)
	HandleSearchMCP(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Result struct {
			StructuredContent struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "IDEMPOTENCY_KEY_REUSED", payload.Result.StructuredContent.Error.Code)
}

func TestSearchMCPRequiresExplicitOrSessionScopedIdempotency(t *testing.T) {
	c, recorder := newSearchMCPContext(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"vector_relay_call","arguments":{"serviceId":"vr_svc_0123456789abcdef","params":{"query":"news"}}}}`)

	HandleSearchMCP(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Result.IsError)
	assert.Equal(t, "IDEMPOTENCY_KEY_REQUIRED", payload.Result.StructuredContent.Error.Code)
}
