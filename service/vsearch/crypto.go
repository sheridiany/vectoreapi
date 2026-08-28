package vsearch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	UpstreamSecretKeyEnv       = "VSEARCH_UPSTREAM_SECRET_KEY"
	LocalSecretFileEnv         = "VSEARCH_LOCAL_SECRET_FILE"
	upstreamSecretVersion      = 1
	upstreamSecretKeySize      = 32
	upstreamSecretAdditionalAD = "vsearch-upstream-secret:v1"
)

var (
	ErrUpstreamSecretKeyNotConfigured = errors.New("upstream secret key is not configured")
	ErrUpstreamSecretKeyInvalid       = errors.New("upstream secret key must contain exactly 32 bytes")
	ErrUpstreamSecretPayloadInvalid   = errors.New("upstream secret payload is invalid")
)

type ConfigurationError struct {
	Cause error
}

func (e *ConfigurationError) Error() string {
	return "vSearch configuration error: " + e.Cause.Error()
}

func (e *ConfigurationError) Unwrap() error {
	return e.Cause
}

type EncryptedSecret struct {
	Ciphertext string
	Nonce      string
	Version    int
}

func ResolveUpstreamSecretKey() ([]byte, error) {
	if configured := strings.TrimSpace(os.Getenv(UpstreamSecretKeyEnv)); configured != "" {
		key, err := decodeConfiguredSecretKey(configured)
		if err != nil {
			return nil, &ConfigurationError{Cause: err}
		}
		return key, nil
	}
	path := strings.TrimSpace(os.Getenv(LocalSecretFileEnv))
	if path == "" {
		return nil, &ConfigurationError{Cause: ErrUpstreamSecretKeyNotConfigured}
	}
	key, err := loadOrCreateLocalSecretKey(path)
	if err != nil {
		return nil, &ConfigurationError{Cause: err}
	}
	return key, nil
}

func decodeConfiguredSecretKey(configured string) ([]byte, error) {
	if len(configured) == upstreamSecretKeySize {
		return []byte(configured), nil
	}
	if len(configured) == hex.EncodedLen(upstreamSecretKeySize) {
		decoded, err := hex.DecodeString(configured)
		if err == nil && len(decoded) == upstreamSecretKeySize {
			return decoded, nil
		}
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(configured)
		if err == nil && len(decoded) == upstreamSecretKeySize {
			return decoded, nil
		}
	}
	return nil, ErrUpstreamSecretKeyInvalid
}

func loadOrCreateLocalSecretKey(path string) ([]byte, error) {
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("local secret file must be a regular file")
		}
		if info.Mode().Perm() != 0o600 {
			return nil, errors.New("local secret file permissions must be 0600")
		}
		key, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read local secret file: %w", readErr)
		}
		if len(key) != upstreamSecretKeySize {
			return nil, ErrUpstreamSecretKeyInvalid
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect local secret file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create local secret directory: %w", err)
	}
	key := make([]byte, upstreamSecretKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate local secret key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateLocalSecretKey(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create local secret file: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write local secret file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("sync local secret file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close local secret file: %w", err)
	}
	return key, nil
}

func EncryptUpstreamSecret(secret string) (EncryptedSecret, error) {
	key, err := ResolveUpstreamSecretKey()
	if err != nil {
		return EncryptedSecret{}, err
	}
	return encryptUpstreamSecretWithKey(secret, key)
}

func encryptUpstreamSecretWithKey(secret string, key []byte) (EncryptedSecret, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return EncryptedSecret{}, ErrUpstreamSecretPayloadInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedSecret{}, &ConfigurationError{Cause: ErrUpstreamSecretKeyInvalid}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedSecret{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedSecret{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(secret), []byte(upstreamSecretAdditionalAD))
	return EncryptedSecret{
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Version:    upstreamSecretVersion,
	}, nil
}

func DecryptUpstreamSecret(payload EncryptedSecret) (string, error) {
	key, err := ResolveUpstreamSecretKey()
	if err != nil {
		return "", err
	}
	return decryptUpstreamSecretWithKey(payload, key)
}

func decryptUpstreamSecretWithKey(payload EncryptedSecret, key []byte) (string, error) {
	if payload.Version != upstreamSecretVersion || payload.Ciphertext == "" || payload.Nonce == "" {
		return "", ErrUpstreamSecretPayloadInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", &ConfigurationError{Cause: ErrUpstreamSecretKeyInvalid}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(payload.Nonce)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return "", ErrUpstreamSecretPayloadInvalid
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return "", ErrUpstreamSecretPayloadInvalid
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(upstreamSecretAdditionalAD))
	if err != nil {
		return "", ErrUpstreamSecretPayloadInvalid
	}
	return string(plaintext), nil
}

func UpstreamSecretPrefix(secret string) string {
	runes := []rune(strings.TrimSpace(secret))
	visible := 12
	if len(runes) <= visible {
		visible = len(runes) / 2
	}
	return string(runes[:visible]) + "••••"
}
