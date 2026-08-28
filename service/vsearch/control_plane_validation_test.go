package vsearch

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneSaveAccountRejectsRemoteHTTPBeforeCreatingStorage(t *testing.T) {
	openRuntimeTestDB(t)
	t.Setenv("VSEARCH_ALLOW_LOOPBACK_HTTP", "false")
	control := NewControlPlane(nil)

	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "insecure", BaseURL: "http://example.com/v1/mcp", Secret: "ak_live_insecure",
	})

	require.Error(t, err)
	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_URL_HTTPS_REQUIRED", publicErr.Code)
	accounts, listErr := model.ListSearchUpstreamAccounts()
	require.NoError(t, listErr)
	assert.Empty(t, accounts)
	pools, listErr := model.ListSearchUpstreamPools()
	require.NoError(t, listErr)
	assert.Empty(t, pools, "URL validation must happen before the default pool is persisted")
}

func TestControlPlaneSaveAccountAllowsExplicitLoopbackHTTP(t *testing.T) {
	openRuntimeTestDB(t)
	t.Setenv("VSEARCH_ALLOW_LOOPBACK_HTTP", "true")
	control := NewControlPlane(nil)

	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "local", BaseURL: "http://127.0.0.1:8080/v1/mcp", Secret: "ak_live_local",
	})

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8080/v1/mcp", account.BaseURL)
}
