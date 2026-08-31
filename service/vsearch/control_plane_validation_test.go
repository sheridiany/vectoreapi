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

func TestControlPlaneSaveAccountRejectsNonProviderHTTPSHost(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)

	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "credential-exfiltration", Provider: ProviderTikHub,
		BaseURL: "https://attacker.example/api", Secret: "tk_live_must_not_leave",
	})

	var publicErr *PublicError
	require.ErrorAs(t, err, &publicErr)
	assert.Equal(t, "UPSTREAM_URL_INVALID", publicErr.Code)
	accounts, listErr := model.ListSearchUpstreamAccounts()
	require.NoError(t, listErr)
	assert.Empty(t, accounts)
}

func TestControlPlaneSaveAccountEnablesNewProviderByDefault(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)

	account, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "primary", Provider: ProviderJustOneAPI,
		BaseURL: DefaultJustOneAPIBaseURL, Secret: "ak_live_primary",
	})

	require.NoError(t, err)
	assert.Equal(t, "healthy", account.Status)
}

func TestControlPlaneListAccountsHidesUnsupportedProviders(t *testing.T) {
	openRuntimeTestDB(t)
	control := NewControlPlane(nil)

	_, err := control.SaveAccount(context.Background(), AccountCommand{
		Name: "tikhub-primary", Provider: ProviderTikHub,
		BaseURL: DefaultTikHubBaseURL, Secret: "tk_live_primary",
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.SearchUpstreamAccount{
		Name: "legacy-agentkey", Provider: "agentkey_mcp", BaseURL: "https://legacy.example/mcp",
		Status: model.SearchUpstreamAccountStatusPaused,
	}).Error)

	accounts, err := control.ListAccounts(context.Background())
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, ProviderTikHub, accounts[0].Provider)

	stored, err := model.ListSearchUpstreamAccounts()
	require.NoError(t, err)
	assert.Len(t, stored, 2, "unsupported accounts remain stored for historical relations")
}
