package vsearch

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	MaxIdempotencyKeyCharacters         = 128
	searchExecutionIdempotencyTTL       = 24 * time.Hour
	searchExecutionResultVersion        = 1
	searchExecutionResultAdditionalData = "vsearch-execution-result:v1"
)

type encryptedExecutionResult struct {
	Ciphertext string
	Nonce      string
	Version    int
}

func (runtime *ExecutionRuntime) Execute(ctx context.Context, principal Principal, command ExecuteCommand) (ExecutionResult, error) {
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.IdempotencyKey == "" {
		return runtime.executeOnce(ctx, principal, command)
	}
	if utf8.RuneCountInString(command.IdempotencyKey) > MaxIdempotencyKeyCharacters {
		return ExecutionResult{}, &PublicError{
			Code: "IDEMPOTENCY_KEY_TOO_LONG", Message: "Idempotency-Key 不能超过 128 个字符。", HTTPStatus: http.StatusBadRequest,
		}
	}
	if principal.AgentKeyID <= 0 {
		return ExecutionResult{}, &PublicError{
			Code: "IDEMPOTENCY_IDENTITY_REQUIRED", Message: "当前请求无法使用幂等保护。", HTTPStatus: http.StatusUnauthorized,
		}
	}
	if command.Params == nil {
		command.Params = map[string]any{}
	}
	requestHash, err := hashIdempotentExecutionRequest(command)
	if err != nil {
		return ExecutionResult{}, &PublicError{
			Code: "INVALID_TOOL_PARAMS", Message: "vSearch 请求参数无法处理。", HTTPStatus: http.StatusBadRequest,
		}
	}
	keyHash := sha256Hex(command.IdempotencyKey)
	now := common.GetTimestamp()
	record, state, err := model.BeginSearchExecutionIdempotency(
		principal.AgentKeyID, keyHash, requestHash, now, now+int64(searchExecutionIdempotencyTTL/time.Second),
	)
	if err != nil {
		common.SysLog("failed to reserve vSearch idempotency key: " + err.Error())
		return ExecutionResult{}, idempotencyUnavailableError()
	}

	switch state {
	case model.SearchExecutionIdempotencyCached:
		if _, err := runtime.authorizedCapability(principal, command.ServiceID); err != nil {
			return ExecutionResult{}, err
		}
		result, err := decryptCachedExecutionResult(record)
		if err != nil {
			common.SysLog("failed to decrypt cached vSearch execution: " + err.Error())
			return ExecutionResult{}, idempotencyUnavailableError()
		}
		return result, nil
	case model.SearchExecutionIdempotencyPending:
		return ExecutionResult{}, &PublicError{
			Code: "IDEMPOTENCY_REQUEST_IN_PROGRESS", Message: "相同 Idempotency-Key 的请求正在处理中。", HTTPStatus: http.StatusConflict,
		}
	case model.SearchExecutionIdempotencyConflict:
		return ExecutionResult{}, &PublicError{
			Code: "IDEMPOTENCY_KEY_REUSED", Message: "该 Idempotency-Key 已用于不同请求。", HTTPStatus: http.StatusConflict,
		}
	case model.SearchExecutionIdempotencyAcquired:
	default:
		return ExecutionResult{}, idempotencyUnavailableError()
	}

	result, executeErr := runtime.executeOnce(ctx, principal, command)
	if executeErr != nil {
		var dispatchedErr *executionDispatchedError
		if errors.As(executeErr, &dispatchedErr) {
			// The upstream may have accepted the request. Keep the reservation pending
			// until expiry so a retry cannot trigger the tool a second time.
			return ExecutionResult{}, executeErr
		}
		if releaseErr := model.ReleaseSearchExecutionIdempotency(record.Id, requestHash, record.ClaimToken); releaseErr != nil {
			common.SysLog("failed to release vSearch idempotency key: " + releaseErr.Error())
		}
		return ExecutionResult{}, executeErr
	}
	encrypted, err := encryptExecutionResult(result)
	if err != nil {
		common.SysLog("failed to encrypt vSearch idempotency result: " + err.Error())
		// The upstream call and billing already succeeded, so preserve that success while the pending row blocks duplicate charges.
		return result, nil
	}
	if err := model.CompleteSearchExecutionIdempotency(
		record.Id, requestHash, record.ClaimToken, encrypted.Ciphertext, encrypted.Nonce, encrypted.Version,
	); err != nil {
		common.SysLog("failed to persist vSearch idempotency result: " + err.Error())
		// Never turn a completed and charged execution into a client-visible failure.
		return result, nil
	}
	return result, nil
}

func hashIdempotentExecutionRequest(command ExecuteCommand) (string, error) {
	data, err := common.Marshal(struct {
		ServiceID string         `json:"service_id"`
		Params    map[string]any `json:"params"`
	}{ServiceID: strings.TrimSpace(command.ServiceID), Params: command.Params})
	if err != nil {
		return "", err
	}
	return sha256Hex(string(data)), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func encryptExecutionResult(result ExecutionResult) (encryptedExecutionResult, error) {
	data, err := common.Marshal(result)
	if err != nil {
		return encryptedExecutionResult{}, err
	}
	key, err := ResolveUpstreamSecretKey()
	if err != nil {
		return encryptedExecutionResult{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedExecutionResult{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedExecutionResult{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedExecutionResult{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, data, []byte(searchExecutionResultAdditionalData))
	return encryptedExecutionResult{
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Version:    searchExecutionResultVersion,
	}, nil
}

func decryptCachedExecutionResult(record *model.SearchExecutionIdempotency) (ExecutionResult, error) {
	if record == nil || record.ResponseVersion != searchExecutionResultVersion {
		return ExecutionResult{}, errors.New("cached vSearch execution payload is invalid")
	}
	key, err := ResolveUpstreamSecretKey()
	if err != nil {
		return ExecutionResult{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ExecutionResult{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ExecutionResult{}, err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(record.ResponseNonce)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return ExecutionResult{}, errors.New("cached vSearch execution nonce is invalid")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(string(record.ResponseCiphertext))
	if err != nil {
		return ExecutionResult{}, errors.New("cached vSearch execution ciphertext is invalid")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(searchExecutionResultAdditionalData))
	if err != nil {
		return ExecutionResult{}, errors.New("cached vSearch execution authentication failed")
	}
	var result ExecutionResult
	if err := common.Unmarshal(plaintext, &result); err != nil || strings.TrimSpace(result.RequestID) == "" {
		return ExecutionResult{}, errors.New("cached vSearch execution result is invalid")
	}
	return result, nil
}

func idempotencyUnavailableError() *PublicError {
	return &PublicError{
		Code: "VSEARCH_IDEMPOTENCY_UNAVAILABLE", Message: "vSearch 幂等服务暂不可用，请稍后重试。", HTTPStatus: http.StatusServiceUnavailable,
	}
}
