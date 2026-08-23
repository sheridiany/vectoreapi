package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const quotaPerUSD = 500000

type RemoteExport struct {
	Users       []RemoteUser       `json:"users"`
	APIKeys     []RemoteAPIKey     `json:"api_keys"`
	Accounts    []RemoteAccount    `json:"accounts"`
	Groups      []RemoteGroup      `json:"groups"`
	UsageLogs   []RemoteUsageLog   `json:"usage_logs"`
	BalanceLogs []RemoteBalanceLog `json:"balance_logs"`
}

type RemoteUser struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Username     string    `json:"username"`
	Balance      float64   `json:"balance"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type RemoteAPIKey struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	KeyEncrypted   string     `json:"key_encrypted"`
	IPWhitelist    []string   `json:"ip_whitelist"`
	IPBlacklist    []string   `json:"ip_blacklist"`
	QuotaUSD       float64    `json:"quota_usd"`
	UsedQuota      float64    `json:"used_quota"`
	MaxConcurrency int64      `json:"max_concurrency"`
	ExpiresAt      *time.Time `json:"expires_at"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	GroupID        *int64     `json:"group_api_keys"`
	UserID         int64      `json:"user_api_keys"`
}

type RemoteAccount struct {
	ID             int64                  `json:"id"`
	Name           string                 `json:"name"`
	Platform       string                 `json:"platform"`
	Type           string                 `json:"type"`
	Credentials    map[string]string      `json:"credentials"`
	State          string                 `json:"state"`
	Priority       int64                  `json:"priority"`
	MaxConcurrency int64                  `json:"max_concurrency"`
	CreatedAt      time.Time              `json:"created_at"`
	Extra          map[string]interface{} `json:"extra"`
}

type RemoteGroup struct {
	ID               int64              `json:"id"`
	Name             string             `json:"name"`
	Platform         string             `json:"platform"`
	SubscriptionType string             `json:"subscription_type"`
	ModelRouting     map[string][]int64 `json:"model_routing"`
}

type RemoteUsageLog struct {
	ID                int64     `json:"id"`
	Platform          string    `json:"platform"`
	Model             string    `json:"model"`
	InputTokens       int64     `json:"input_tokens"`
	OutputTokens      int64     `json:"output_tokens"`
	TotalCost         float64   `json:"total_cost"`
	ActualCost        float64   `json:"actual_cost"`
	BilledCost        float64   `json:"billed_cost"`
	Stream            bool      `json:"stream"`
	DurationMS        int64     `json:"duration_ms"`
	UserAgent         string    `json:"user_agent"`
	IPAddress         string    `json:"ip_address"`
	Endpoint          string    `json:"endpoint"`
	UserIDSnapshot    int64     `json:"user_id_snapshot"`
	UserEmailSnapshot string    `json:"user_email_snapshot"`
	CreatedAt         time.Time `json:"created_at"`
	APIKeyID          *int64    `json:"api_key_usage_logs"`
	AccountID         *int64    `json:"account_usage_logs"`
	GroupID           *int64    `json:"group_usage_logs"`
}

type RemoteBalanceLog struct {
	ID                int64     `json:"id"`
	Action            string    `json:"action"`
	Amount            float64   `json:"amount"`
	BeforeBalance     float64   `json:"before_balance"`
	AfterBalance      float64   `json:"after_balance"`
	Remark            string    `json:"remark"`
	UserIDSnapshot    int64     `json:"user_id_snapshot"`
	UserEmailSnapshot string    `json:"user_email_snapshot"`
	CreatedAt         time.Time `json:"created_at"`
}

func main() {
	baseURL := flag.String("remote-base-url", "https://gate.vectorepoch.com", "remote gateway base URL")
	localURL := flag.String("local-base-url", "http://new-api-local:3000", "local New API base URL")
	apply := flag.Bool("apply", false, "apply the migration")
	flag.Parse()
	if !*apply {
		fatal("refusing to migrate without --apply")
	}

	var dump RemoteExport
	dec := json.NewDecoder(os.Stdin)
	if err := dec.Decode(&dump); err != nil {
		fatal("read remote export: %v", err)
	}
	if len(dump.Users) == 0 || len(dump.APIKeys) == 0 {
		fatal("remote export has no users or API keys")
	}

	secret := os.Getenv("API_KEY_SECRET")
	if secret == "" {
		fatal("API_KEY_SECRET is required")
	}
	for i := range dump.APIKeys {
		plain, err := decryptAPIKey(dump.APIKeys[i].KeyEncrypted, secret)
		if err != nil {
			fatal("decrypt remote API key id=%d: %v", dump.APIKeys[i].ID, err)
		}
		dump.APIKeys[i].KeyEncrypted = plain
	}

	models, err := fetchModels(*baseURL, dump.APIKeys)
	if err != nil {
		fatal("fetch remote model catalog: %v", err)
	}
	if len(models) == 0 {
		fatal("remote model catalog is empty")
	}

	dsn := os.Getenv("NEW_API_DSN")
	if dsn == "" {
		dsn = "postgresql://root:123456@postgres:5432/new-api?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fatal("open local database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatal("ping local database: %v", err)
	}

	if err := migrate(db, dump, models); err != nil {
		fatal("apply migration: %v", err)
	}

	if err := validateLocal(*localURL, dump.APIKeys, models); err != nil {
		fatal("validate imported key: %v", err)
	}

	fmt.Printf("migrated users=%d api_keys=%d accounts=%d models=%d usage_logs=%d balance_logs=%d\n", len(dump.Users), len(dump.APIKeys), len(dump.Accounts), len(models), len(dump.UsageLogs), len(dump.BalanceLogs))
}

func decryptAPIKey(encoded, secret string) (string, error) {
	rawKey, err := hex.DecodeString(secret)
	if err != nil || len(rawKey) < 32 {
		return "", errors.New("invalid API_KEY_SECRET")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext: %w", err)
	}
	block, err := aes.NewCipher(rawKey[:32])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext is too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("ciphertext authentication failed")
	}
	return string(plain), nil
}

func fetchModels(baseURL string, keys []RemoteAPIKey) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	var lastErr error
	for _, key := range keys {
		if key.Status != "active" || key.KeyEncrypted == "" {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+key.KeyEncrypted)
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			lastErr = err
			continue
		}
		seen := make(map[string]bool)
		models := make([]string, 0, len(payload.Data))
		for _, item := range payload.Data {
			id := strings.TrimSpace(item.ID)
			if id != "" && !seen[id] {
				seen[id] = true
				models = append(models, id)
			}
		}
		sort.Strings(models)
		return models, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no active remote API key")
	}
	return nil, lastErr
}

func migrate(db *sql.DB, dump RemoteExport, models []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	usageByUser := make(map[int64]int64, len(dump.Users))
	requestCountByUser := make(map[int64]int64, len(dump.Users))
	for _, item := range dump.UsageLogs {
		usageByUser[item.UserIDSnapshot] += quotaFromUSD(item.BilledCost)
		requestCountByUser[item.UserIDSnapshot]++
	}
	groupNames := make(map[int64]string, len(dump.Groups))
	for _, group := range dump.Groups {
		if name := strings.TrimSpace(group.Name); name != "" {
			groupNames[group.ID] = name
		}
	}

	_, err = tx.Exec(`TRUNCATE TABLE users, tokens, channels, models, vendors, abilities, logs, quota_data, user_sessions, user_oauth_bindings, external_identity_claims, two_fas, two_fa_backup_codes, user_subscriptions, subscription_orders, checkins, passkey_credentials, redemptions, top_ups RESTART IDENTITY CASCADE`)
	if err != nil {
		return err
	}

	userIDs := make(map[int64]int64, len(dump.Users))
	for _, user := range dump.Users {
		username := normalizedUsername(user.Username, user.Email, user.ID)
		role := int64(1)
		if strings.EqualFold(user.Role, "admin") {
			// The remote admin is the platform owner. Preserve that capability
			// as New API's root role so all admin settings remain manageable.
			role = 100
		}
		status := int64(1)
		if !strings.EqualFold(user.Status, "active") {
			status = 2
		}
		if user.PasswordHash == "" {
			return fmt.Errorf("remote user id=%d has no password hash", user.ID)
		}
		quota := quotaFromUSD(user.Balance)
		_, err := tx.Exec(`INSERT INTO users (id, username, password, display_name, role, status, email, quota, used_quota, request_count, "group", aff_code, created_at, last_login_at, auth_version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'default',$11,$12,0,1)`, user.ID, username, user.PasswordHash, username, role, status, user.Email, quota, usageByUser[user.ID], requestCountByUser[user.ID], fmt.Sprintf("airgate-user-%d", user.ID), user.CreatedAt.Unix())
		if err != nil {
			return fmt.Errorf("insert user id=%d: %w", user.ID, err)
		}
		userIDs[user.ID] = user.ID
	}

	tokenNames := make(map[int64]string, len(dump.APIKeys))
	for _, key := range dump.APIKeys {
		if _, ok := userIDs[key.UserID]; !ok {
			return fmt.Errorf("API key id=%d points to missing user id=%d", key.ID, key.UserID)
		}
		status := int64(1)
		if key.Status != "active" {
			status = 2
		}
		expired := int64(-1)
		if key.ExpiresAt != nil {
			expired = key.ExpiresAt.Unix()
		}
		remain := int64(0)
		if key.QuotaUSD > 0 {
			remain = quotaFromUSD(key.QuotaUSD - key.UsedQuota)
			if remain < 0 {
				remain = 0
			}
		}
		allowIPs := strings.Join(key.IPWhitelist, "\n")
		if key.KeyEncrypted == "" {
			return fmt.Errorf("API key id=%d decrypted to empty", key.ID)
		}
		storedKey := strings.TrimPrefix(key.KeyEncrypted, "sk-")
		_, err := tx.Exec(`INSERT INTO tokens (id, user_id, key, status, name, created_time, accessed_time, expired_time, remain_quota, unlimited_quota, model_limits_enabled, model_limits, allow_ips, used_quota, "group", cross_group_retry, auto_groups) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,'',$11,$12,'default',true,'')`, key.ID, key.UserID, storedKey, status, key.Name, key.CreatedAt.Unix(), key.UpdatedAt.Unix(), expired, remain, key.QuotaUSD <= 0, allowIPs, quotaFromUSD(key.UsedQuota))
		if err != nil {
			return fmt.Errorf("insert API key id=%d: %w", key.ID, err)
		}
		tokenNames[key.ID] = key.Name
	}

	channelIDs := make(map[int64]int64, len(dump.Accounts))
	for _, account := range dump.Accounts {
		channelID := account.ID
		channelIDs[account.ID] = channelID
		name := account.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Airgate account %d", account.ID)
		}
		channelType := int64(1)
		key := account.Credentials["api_key"]
		if strings.EqualFold(account.Type, "oauth") {
			channelType = 57
			key = codexKey(account.Credentials)
		}
		if key == "" {
			return fmt.Errorf("account id=%d has no compatible credential", account.ID)
		}
		status := int64(1)
		if account.State != "active" {
			status = 2
		}
		baseURL := account.Credentials["base_url"]
		modelList := strings.Join(models, ",")
		otherInfo, _ := json.Marshal(map[string]interface{}{
			"airgate_account_id": account.ID,
			"airgate_platform":   account.Platform,
			"airgate_type":       account.Type,
			"max_concurrency":    account.MaxConcurrency,
		})
		_, err := tx.Exec(`INSERT INTO channels (id, type, key, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, "group", used_quota, priority, auto_ban, other_info, channel_info, settings) VALUES ($1,$2,$3,$4,$5,1,$6,0,0,$7,'',0,0,$8,'default',0,$9,1,$10,'{}','')`, channelID, channelType, key, status, name, account.CreatedAt.Unix(), baseURL, modelList, account.Priority, string(otherInfo))
		if err != nil {
			return fmt.Errorf("insert channel id=%d: %w", account.ID, err)
		}
		for _, model := range models {
			_, err = tx.Exec(`INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight) VALUES ('default',$1,$2,$3,$4,0)`, model, channelID, status == 1, account.Priority)
			if err != nil {
				return fmt.Errorf("insert ability account=%d model=%s: %w", account.ID, model, err)
			}
		}
	}

	for _, model := range models {
		_, err := tx.Exec(`INSERT INTO models (model_name, description, icon, tags, vendor_id, endpoints, status, sync_official, created_time, updated_time, name_rule) VALUES ($1,'','','',0,'',1,0,$2,$2,0)`, model, time.Now().UnixMilli())
		if err != nil {
			return fmt.Errorf("insert model %s: %w", model, err)
		}
	}

	logStmt, err := tx.Prepare(`INSERT INTO logs (user_id, created_at, type, content, username, token_name, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream, channel_id, token_id, "group", ip, request_id, other) VALUES ($1,$2,2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'default',$14,$15,$16)`)
	if err != nil {
		return err
	}
	defer logStmt.Close()
	quotaDataStmt, err := tx.Prepare(`INSERT INTO quota_data (user_id, username, model_name, created_at, use_group, token_id, channel_id, node_name, token_used, count, quota) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10)`)
	if err != nil {
		return err
	}
	defer quotaDataStmt.Close()
	for _, item := range dump.UsageLogs {
		userID := item.UserIDSnapshot
		if _, ok := userIDs[userID]; !ok {
			continue
		}
		channelID := int64(0)
		if item.AccountID != nil {
			channelID = channelIDs[*item.AccountID]
		}
		tokenID := int64(0)
		tokenName := ""
		if item.APIKeyID != nil {
			tokenID = *item.APIKeyID
			tokenName = tokenNames[tokenID]
		}
		other, _ := json.Marshal(map[string]interface{}{
			"airgate_usage_id": item.ID,
			"platform":         item.Platform,
			"endpoint":         item.Endpoint,
			"actual_cost_usd":  item.ActualCost,
			"billed_cost_usd":  item.BilledCost,
			"user_agent":       item.UserAgent,
		})
		if _, err := logStmt.Exec(userID, item.CreatedAt.Unix(), item.Endpoint, normalizedUsername("", item.UserEmailSnapshot, userID), tokenName, item.Model, quotaFromUSD(item.BilledCost), item.InputTokens, item.OutputTokens, item.DurationMS, item.Stream, channelID, tokenID, item.IPAddress, fmt.Sprintf("airgate-usage-%d", item.ID), string(other)); err != nil {
			return fmt.Errorf("insert usage log id=%d: %w", item.ID, err)
		}
		useGroup := "default"
		if item.GroupID != nil {
			if name := groupNames[*item.GroupID]; name != "" {
				useGroup = name
			}
		}
		createdAt := item.CreatedAt.Unix()
		createdAt -= createdAt % 3600
		if _, err := quotaDataStmt.Exec(userID, normalizedUsername("", item.UserEmailSnapshot, userID), item.Model, createdAt, useGroup, tokenID, channelID, "airgate", item.InputTokens+item.OutputTokens, quotaFromUSD(item.BilledCost)); err != nil {
			return fmt.Errorf("insert quota data id=%d: %w", item.ID, err)
		}
	}
	for _, item := range dump.BalanceLogs {
		userID := item.UserIDSnapshot
		if _, ok := userIDs[userID]; !ok {
			continue
		}
		logType := int64(1)
		if item.Action == "subtract" {
			logType = 6
		}
		other, _ := json.Marshal(map[string]interface{}{
			"airgate_balance_log_id": item.ID,
			"before_usd":             item.BeforeBalance,
			"after_usd":              item.AfterBalance,
		})
		if _, err := tx.Exec(`INSERT INTO logs (user_id, created_at, type, content, username, quota, request_id, other) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, userID, item.CreatedAt.Unix(), logType, item.Remark, normalizedUsername("", item.UserEmailSnapshot, userID), quotaFromUSD(item.Amount), fmt.Sprintf("airgate-balance-%d", item.ID), string(other)); err != nil {
			return fmt.Errorf("insert balance log id=%d: %w", item.ID, err)
		}
	}

	for _, table := range []string{"users", "tokens", "channels", "models"} {
		if _, err := tx.Exec(`SELECT setval(pg_get_serial_sequence($1, 'id'), COALESCE((SELECT MAX(id) FROM `+table+`), 1), true)`, table); err != nil {
			return fmt.Errorf("reset %s sequence: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func codexKey(credentials map[string]string) string {
	value := map[string]string{
		"access_token":  credentials["access_token"],
		"refresh_token": credentials["refresh_token"],
		"account_id":    credentials["chatgpt_account_id"],
		"email":         credentials["email"],
		"type":          "codex",
	}
	if credentials["id_token"] != "" {
		value["id_token"] = credentials["id_token"]
	}
	data, _ := json.Marshal(value)
	return string(data)
}

func validateLocal(baseURL string, keys []RemoteAPIKey, models []string) error {
	var key string
	for _, item := range keys {
		if item.Status == "active" {
			key = item.KeyEncrypted
			break
		}
	}
	if key == "" {
		return errors.New("no active imported key")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if len(payload.Data) == 0 {
		return fmt.Errorf("local model catalog is empty after importing %d models", len(models))
	}
	return nil
}

func normalizedUsername(username, email string, id int64) string {
	value := strings.TrimSpace(username)
	if value == "" {
		value = strings.TrimSpace(email)
		if at := strings.IndexByte(value, '@'); at > 0 {
			value = value[:at]
		}
	}
	if value == "" {
		value = fmt.Sprintf("airgate-%d", id)
	}
	value = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return '_'
		}
		return r
	}, value)
	if len(value) > 20 {
		suffix := fmt.Sprintf("-%d", id)
		value = value[:20-len(suffix)] + suffix
	}
	return value
}

func quotaFromUSD(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	value *= quotaPerUSD
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	if value < math.MinInt64 {
		return math.MinInt64
	}
	return int64(math.Round(value))
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
