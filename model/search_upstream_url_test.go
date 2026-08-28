package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSearchUpstreamAccountEnforcesConnectorURLPolicy(t *testing.T) {
	openSearchDataTestDB(t)
	t.Setenv("VSEARCH_ALLOW_LOOPBACK_HTTP", "false")
	pool := &SearchUpstreamPool{Name: "default"}
	require.NoError(t, CreateSearchUpstreamPool(pool))
	account := &SearchUpstreamAccount{
		PoolID: pool.Id, Name: "insecure", BaseURL: "http://example.com/v1/mcp",
		SecretCiphertext: "ciphertext", SecretNonce: "nonce", SecretVersion: 1,
		SecretPrefix: "ak_live_test", Status: SearchUpstreamAccountStatusHealthy,
	}

	err := CreateSearchUpstreamAccount(account)

	assert.ErrorIs(t, err, ErrSearchUpstreamURLHTTPSRequired)
	accounts, listErr := ListSearchUpstreamAccounts()
	require.NoError(t, listErr)
	assert.Empty(t, accounts)
}

func TestValidateSearchUpstreamBaseURLAllowsOnlyExplicitLoopbackHTTP(t *testing.T) {
	_, err := ValidateSearchUpstreamBaseURL("http://localhost:8080/v1/mcp", false)
	assert.ErrorIs(t, err, ErrSearchUpstreamURLHTTPSRequired)

	endpoint, err := ValidateSearchUpstreamBaseURL("http://[::1]:8080/v1/mcp", true)
	require.NoError(t, err)
	assert.Equal(t, "http://[::1]:8080/v1/mcp", endpoint.String())
}
