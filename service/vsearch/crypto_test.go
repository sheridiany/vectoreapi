package vsearch

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUpstreamSecretKeyRequiresExplicitConfiguration(t *testing.T) {
	t.Setenv(UpstreamSecretKeyEnv, "")
	t.Setenv(LocalSecretFileEnv, "")

	key, err := ResolveUpstreamSecretKey()
	require.Error(t, err)
	assert.Nil(t, key)
	assert.ErrorIs(t, err, ErrUpstreamSecretKeyNotConfigured)
	var configurationError *ConfigurationError
	assert.ErrorAs(t, err, &configurationError)
}

func TestResolveUpstreamSecretKeyPrefersEnvironmentKey(t *testing.T) {
	configuredKey := bytes.Repeat([]byte{0x5a}, upstreamSecretKeySize)
	secretFile := filepath.Join(t.TempDir(), "must-not-be-created", "vsearch.key")
	t.Setenv(UpstreamSecretKeyEnv, base64.StdEncoding.EncodeToString(configuredKey))
	t.Setenv(LocalSecretFileEnv, secretFile)

	resolved, err := ResolveUpstreamSecretKey()
	require.NoError(t, err)
	assert.Equal(t, configuredKey, resolved)
	_, statErr := os.Stat(secretFile)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestResolveUpstreamSecretKeyDoesNotFallbackFromInvalidEnvironmentKey(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "vsearch.key")
	t.Setenv(UpstreamSecretKeyEnv, "too-short")
	t.Setenv(LocalSecretFileEnv, secretFile)

	key, err := ResolveUpstreamSecretKey()
	require.Error(t, err)
	assert.Nil(t, key)
	assert.ErrorIs(t, err, ErrUpstreamSecretKeyInvalid)
	_, statErr := os.Stat(secretFile)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestResolveUpstreamSecretKeyCreatesStablePrivateLocalFile(t *testing.T) {
	t.Setenv(UpstreamSecretKeyEnv, "")
	secretFile := filepath.Join(t.TempDir(), "private", "vsearch.key")
	t.Setenv(LocalSecretFileEnv, secretFile)

	first, err := ResolveUpstreamSecretKey()
	require.NoError(t, err)
	require.Len(t, first, upstreamSecretKeySize)

	info, err := os.Stat(secretFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	second, err := ResolveUpstreamSecretKey()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestResolveUpstreamSecretKeyRejectsUnsafeLocalFileMode(t *testing.T) {
	t.Setenv(UpstreamSecretKeyEnv, "")
	secretFile := filepath.Join(t.TempDir(), "vsearch.key")
	require.NoError(t, os.WriteFile(secretFile, bytes.Repeat([]byte{0x41}, upstreamSecretKeySize), 0o644))
	require.NoError(t, os.Chmod(secretFile, 0o644))
	t.Setenv(LocalSecretFileEnv, secretFile)

	key, err := ResolveUpstreamSecretKey()
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "permissions must be 0600")
}

func TestUpstreamSecretEncryptionRoundTripIsRandomizedAndAuthenticated(t *testing.T) {
	key := bytes.Repeat([]byte{0x2c}, upstreamSecretKeySize)
	first, err := encryptUpstreamSecretWithKey("ak_live_secret", key)
	require.NoError(t, err)
	second, err := encryptUpstreamSecretWithKey("ak_live_secret", key)
	require.NoError(t, err)

	assert.NotEqual(t, first.Nonce, second.Nonce)
	assert.NotEqual(t, first.Ciphertext, second.Ciphertext)
	plaintext, err := decryptUpstreamSecretWithKey(first, key)
	require.NoError(t, err)
	assert.Equal(t, "ak_live_secret", plaintext)

	wrongKey := bytes.Repeat([]byte{0x3d}, upstreamSecretKeySize)
	_, err = decryptUpstreamSecretWithKey(first, wrongKey)
	assert.ErrorIs(t, err, ErrUpstreamSecretPayloadInvalid)

	tampered := first
	tamperedBytes, err := base64.RawStdEncoding.DecodeString(tampered.Ciphertext)
	require.NoError(t, err)
	tamperedBytes[0] ^= 0xff
	tampered.Ciphertext = base64.RawStdEncoding.EncodeToString(tamperedBytes)
	_, err = decryptUpstreamSecretWithKey(tampered, key)
	assert.ErrorIs(t, err, ErrUpstreamSecretPayloadInvalid)
}

func TestExportedUpstreamSecretEncryptionUsesConfiguredKey(t *testing.T) {
	configuredKey := bytes.Repeat([]byte{0x61}, upstreamSecretKeySize)
	t.Setenv(UpstreamSecretKeyEnv, base64.RawStdEncoding.EncodeToString(configuredKey))
	t.Setenv(LocalSecretFileEnv, "")

	payload, err := EncryptUpstreamSecret("ak_live_exported")
	require.NoError(t, err)
	assert.NotContains(t, payload.Ciphertext, "ak_live_exported")
	plaintext, err := DecryptUpstreamSecret(payload)
	require.NoError(t, err)
	assert.Equal(t, "ak_live_exported", plaintext)
}

func TestUpstreamSecretPrefixNeverReturnsFullSecret(t *testing.T) {
	assert.Equal(t, "ak_live_abc1••••", UpstreamSecretPrefix("ak_live_abc123456"))
	assert.Equal(t, "sh••••", UpstreamSecretPrefix(" short "))
	assert.Equal(t, "••••", UpstreamSecretPrefix("x"))
}
