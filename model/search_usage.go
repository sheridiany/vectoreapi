package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SearchUsageStatusSucceeded     = 1
	SearchUsageStatusFailed        = 2
	SearchUsageStatusPending       = 3
	SearchUsageStatusIndeterminate = 4

	SearchUsagePendingTimeoutSeconds = int64(10 * 60)

	SearchUsagePhasePrepared    = "prepared"
	SearchUsagePhaseDispatching = "dispatching"
	SearchUsagePhaseCompleted   = "completed"

	SearchUsageBillingNotStarted     = "not_started"
	SearchUsageBillingReservePending = "reserve_pending"
	SearchUsageBillingReserved       = "reserved"
	SearchUsageBillingCommitPending  = "commit_pending"
	SearchUsageBillingLogPending     = "log_pending"
	SearchUsageBillingLogWriting     = "log_writing"
	SearchUsageBillingCommitted      = "committed"
	SearchUsageBillingRefundPending  = "refund_pending"
	SearchUsageBillingRefunded       = "refunded"
	SearchUsageBillingRefundFailed   = "refund_failed"

	SearchUsageActionCapabilities = "capabilities"
	SearchUsageActionDiscover     = "discover"
	SearchUsageActionDescribe     = "describe"
	SearchUsageActionExecute      = "execute"
)

type SearchUsageEvent struct {
	Id                        int64  `json:"id"`
	RequestID                 string `json:"request_id" gorm:"size:64;not null;uniqueIndex"`
	UpstreamRequestID         string `json:"-" gorm:"size:128;index"`
	UserID                    int    `json:"user_id" gorm:"not null;index"`
	EnterpriseID              int    `json:"enterprise_id" gorm:"not null;index"`
	AgentKeyID                int    `json:"agent_key_id" gorm:"not null;index"`
	UpstreamAccountID         int    `json:"-" gorm:"not null;index"`
	CapabilityID              int    `json:"-" gorm:"not null;index"`
	ServiceID                 string `json:"service_id" gorm:"size:32;index"`
	ServiceName               string `json:"service_name" gorm:"size:128;index"`
	Action                    string `json:"action" gorm:"size:24;not null;index"`
	Status                    int    `json:"status" gorm:"type:int;not null;index"`
	HTTPStatus                int    `json:"http_status" gorm:"type:int;not null"`
	LatencyMs                 int64  `json:"latency_ms" gorm:"not null"`
	InputBytes                int64  `json:"input_bytes" gorm:"not null"`
	OutputBytes               int64  `json:"output_bytes" gorm:"not null"`
	UpstreamCostMicros        int64  `json:"upstream_cost_micros" gorm:"not null"`
	ChargeMicros              int64  `json:"charge_micros" gorm:"not null"`
	PlannedUpstreamCostMicros int64  `json:"-"`
	PlannedChargeMicros       int64  `json:"-"`
	ExecutionPhase            string `json:"-" gorm:"size:24"`
	BillingState              string `json:"-" gorm:"size:24"`
	BillingSource             string `json:"-" gorm:"size:24"`
	ReservedQuota             int    `json:"-" gorm:"type:int"`
	ErrorCode                 string `json:"error_code,omitempty" gorm:"size:64;index"`
	SanitizedErrorMessage     string `json:"error_message,omitempty" gorm:"size:255"`
	CreatedAt                 int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt                 int64  `json:"-" gorm:"autoUpdateTime;index"`
}

type SearchUsageQuery struct {
	UserID       int
	EnterpriseID int
	AgentKeyID   int
	CapabilityID int
	ServiceID    string
	Action       string
	SearchText   string
	Status       int
	StartAt      int64
	EndAt        int64
	Offset       int
	Limit        int
}

type SearchUsageStat struct {
	RequestCount       int64 `json:"request_count"`
	SuccessCount       int64 `json:"success_count"`
	ErrorCount         int64 `json:"error_count"`
	PendingCount       int64 `json:"pending_count"`
	IndeterminateCount int64 `json:"indeterminate_count"`
	TotalLatencyMs     int64 `json:"total_latency_ms"`
	UpstreamCostMicros int64 `json:"upstream_cost_micros"`
	ChargeMicros       int64 `json:"charge_micros"`
	MarginMicros       int64 `json:"margin_micros"`
}

func truncateSearchUsageText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func normalizeSearchUsageEvent(event *SearchUsageEvent) error {
	if event == nil || event.UserID <= 0 || event.AgentKeyID <= 0 {
		return errors.New("search usage identity is invalid")
	}
	if event.EnterpriseID < 0 || event.UpstreamAccountID < 0 || event.CapabilityID < 0 {
		return errors.New("search usage relation is invalid")
	}
	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.RequestID == "" {
		event.RequestID = common.NewRequestId()
	}
	if len(event.RequestID) > 64 {
		return errors.New("search usage request id is too long")
	}
	event.UpstreamRequestID = strings.TrimSpace(event.UpstreamRequestID)
	if len(event.UpstreamRequestID) > 128 {
		return errors.New("search usage upstream request id is too long")
	}
	event.ServiceID = strings.TrimSpace(event.ServiceID)
	event.ServiceName = truncateSearchUsageText(event.ServiceName, 128)
	event.Action = strings.TrimSpace(event.Action)
	switch event.Action {
	case SearchUsageActionCapabilities, SearchUsageActionDiscover, SearchUsageActionDescribe, SearchUsageActionExecute:
	default:
		return errors.New("search usage action is invalid")
	}
	if event.Status != SearchUsageStatusSucceeded && event.Status != SearchUsageStatusFailed && event.Status != SearchUsageStatusPending && event.Status != SearchUsageStatusIndeterminate {
		return errors.New("search usage status is invalid")
	}
	if event.HTTPStatus < 0 || event.HTTPStatus > 599 || event.LatencyMs < 0 || event.InputBytes < 0 || event.OutputBytes < 0 || event.UpstreamCostMicros < 0 || event.ChargeMicros < 0 || event.PlannedUpstreamCostMicros < 0 || event.PlannedChargeMicros < 0 || event.ReservedQuota < 0 {
		return errors.New("search usage numeric value is invalid")
	}
	event.ExecutionPhase = strings.TrimSpace(event.ExecutionPhase)
	if event.ExecutionPhase == "" {
		if event.Status == SearchUsageStatusPending {
			event.ExecutionPhase = SearchUsagePhasePrepared
		} else {
			event.ExecutionPhase = SearchUsagePhaseCompleted
		}
	}
	switch event.ExecutionPhase {
	case SearchUsagePhasePrepared, SearchUsagePhaseDispatching, SearchUsagePhaseCompleted:
	default:
		return errors.New("search usage execution phase is invalid")
	}
	event.BillingState = strings.TrimSpace(event.BillingState)
	if event.BillingState == "" {
		if event.ChargeMicros > 0 {
			event.BillingState = SearchUsageBillingCommitted
		} else {
			event.BillingState = SearchUsageBillingNotStarted
		}
	}
	switch event.BillingState {
	case SearchUsageBillingNotStarted, SearchUsageBillingReservePending, SearchUsageBillingReserved, SearchUsageBillingCommitPending, SearchUsageBillingLogPending, SearchUsageBillingLogWriting, SearchUsageBillingCommitted, SearchUsageBillingRefundPending, SearchUsageBillingRefunded, SearchUsageBillingRefundFailed:
	default:
		return errors.New("search usage billing state is invalid")
	}
	event.BillingSource = strings.TrimSpace(event.BillingSource)
	if len(event.BillingSource) > 24 {
		return errors.New("search usage billing source is too long")
	}
	event.ErrorCode = truncateSearchUsageText(event.ErrorCode, 64)
	event.SanitizedErrorMessage = truncateSearchUsageText(event.SanitizedErrorMessage, 255)
	return nil
}

func CreateSearchUsageEvent(event *SearchUsageEvent) error {
	if err := normalizeSearchUsageEvent(event); err != nil {
		return err
	}
	return DB.Create(event).Error
}

func FinalizeSearchUsageEvent(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 || (event.Status != SearchUsageStatusSucceeded && event.Status != SearchUsageStatusFailed && event.Status != SearchUsageStatusIndeterminate) {
		return errors.New("search usage event finalization is invalid")
	}
	if err := normalizeSearchUsageEvent(event); err != nil {
		return err
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ?", event.Id, SearchUsageStatusPending).
		Updates(map[string]any{
			"status": event.Status, "http_status": event.HTTPStatus,
			"latency_ms": event.LatencyMs, "output_bytes": event.OutputBytes,
			"upstream_cost_micros": event.UpstreamCostMicros, "charge_micros": event.ChargeMicros,
			"execution_phase": event.ExecutionPhase, "billing_state": event.BillingState,
			"billing_source": event.BillingSource, "reserved_quota": event.ReservedQuota,
			"error_code": event.ErrorCode, "sanitized_error_message": event.SanitizedErrorMessage,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func MarkSearchUsageReservation(event *SearchUsageEvent, billingSource string, reservedQuota int) error {
	if event == nil || event.Id <= 0 || reservedQuota < 0 {
		return errors.New("search usage reservation is invalid")
	}
	billingSource = strings.TrimSpace(billingSource)
	if billingSource == "" {
		billingSource = "none"
	}
	if len(billingSource) > 24 {
		return errors.New("search usage billing source is too long")
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ? AND billing_state = ?", event.Id, SearchUsageStatusPending, SearchUsageBillingReservePending).
		Updates(map[string]any{
			"billing_state": SearchUsageBillingReserved, "billing_source": billingSource,
			"reserved_quota": reservedQuota, "updated_at": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	event.BillingState = SearchUsageBillingReserved
	event.BillingSource = billingSource
	event.ReservedQuota = reservedQuota
	return nil
}

func MarkSearchUsageCommitPending(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 || event.ExecutionPhase != SearchUsagePhaseCompleted || event.HTTPStatus != 200 || event.LatencyMs < 0 || event.OutputBytes < 0 || event.UpstreamCostMicros < 0 || event.ChargeMicros < 0 {
		return errors.New("search usage commit intent is invalid")
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ? AND billing_state = ?", event.Id, SearchUsageStatusPending, SearchUsageBillingReserved).
		Updates(map[string]any{
			"http_status": event.HTTPStatus, "latency_ms": event.LatencyMs,
			"output_bytes": event.OutputBytes, "upstream_cost_micros": event.UpstreamCostMicros,
			"charge_micros": event.ChargeMicros, "execution_phase": SearchUsagePhaseCompleted,
			"billing_state": SearchUsageBillingCommitPending, "error_code": "",
			"sanitized_error_message": "", "updated_at": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	event.BillingState = SearchUsageBillingCommitPending
	return nil
}

func MarkSearchUsageRefundPending(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 {
		return errors.New("search usage refund intent is invalid")
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ? AND billing_state IN ?", event.Id, SearchUsageStatusPending, []string{
			SearchUsageBillingReservePending, SearchUsageBillingReserved, SearchUsageBillingCommitPending, SearchUsageBillingRefundFailed,
		}).
		Updates(map[string]any{"billing_state": SearchUsageBillingRefundPending, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	event.BillingState = SearchUsageBillingRefundPending
	return nil
}

// CommitSearchUsageEvent atomically applies accumulated usage and moves a
// commit intent to its terminal billed state. Replays are no-ops.
func CommitSearchUsageEvent(eventID int64) (*SearchUsageEvent, error) {
	if eventID <= 0 {
		return nil, errors.New("search usage commit is invalid")
	}
	committed := &SearchUsageEvent{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", eventID).First(committed).Error; err != nil {
			return err
		}
		if committed.Status == SearchUsageStatusSucceeded && (committed.BillingState == SearchUsageBillingLogPending || committed.BillingState == SearchUsageBillingLogWriting || committed.BillingState == SearchUsageBillingCommitted) {
			return nil
		}
		if committed.Status != SearchUsageStatusPending || committed.BillingState != SearchUsageBillingCommitPending || committed.ExecutionPhase != SearchUsagePhaseCompleted {
			return errors.New("search usage event is not ready to commit")
		}
		userUpdate := tx.Model(&User{}).Where("id = ?", committed.UserID).Updates(map[string]any{
			"used_quota":    gorm.Expr("used_quota + ?", committed.ReservedQuota),
			"request_count": gorm.Expr("request_count + ?", 1),
		})
		if userUpdate.Error != nil {
			return userUpdate.Error
		}
		if userUpdate.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		terminal := tx.Model(&SearchUsageEvent{}).
			Where("id = ? AND status = ? AND billing_state = ?", committed.Id, SearchUsageStatusPending, SearchUsageBillingCommitPending).
			Updates(map[string]any{
				"status": SearchUsageStatusSucceeded, "billing_state": SearchUsageBillingLogPending,
				"updated_at": common.GetTimestamp(),
			})
		if terminal.Error != nil {
			return terminal.Error
		}
		if terminal.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		committed.Status = SearchUsageStatusSucceeded
		committed.BillingState = SearchUsageBillingLogPending
		return nil
	})
	if err != nil {
		return nil, err
	}
	return committed, nil
}

// EnsureSearchUsageConsumeLog materializes the gateway-wide consume log once.
// RequestID is the durable idempotency key; a crash after log insertion is
// recovered by observing the existing log before marking the outbox complete.
func EnsureSearchUsageConsumeLog(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 || event.Status != SearchUsageStatusSucceeded || (event.BillingState != SearchUsageBillingLogPending && event.BillingState != SearchUsageBillingLogWriting && event.BillingState != SearchUsageBillingCommitted) {
		return errors.New("search usage consume log request is invalid")
	}
	if event.BillingState == SearchUsageBillingCommitted {
		return nil
	}
	now := common.GetTimestamp()
	claimed := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ? AND billing_state = ?", event.Id, SearchUsageStatusSucceeded, SearchUsageBillingLogPending).
		Updates(map[string]any{"billing_state": SearchUsageBillingLogWriting, "updated_at": now})
	if claimed.Error != nil {
		return claimed.Error
	}

	var stored SearchUsageEvent
	if err := DB.Where("id = ?", event.Id).First(&stored).Error; err != nil {
		return err
	}
	if stored.Status != SearchUsageStatusSucceeded {
		return errors.New("search usage consume log status is invalid")
	}
	if stored.BillingState == SearchUsageBillingCommitted {
		event.BillingState = SearchUsageBillingCommitted
		return nil
	}
	if claimed.RowsAffected == 0 {
		if stored.BillingState != SearchUsageBillingLogWriting {
			return errors.New("search usage consume log state is invalid")
		}
		if stored.UpdatedAt > now-SearchUsagePendingTimeoutSeconds {
			return nil
		}
		takenOver := DB.Model(&SearchUsageEvent{}).
			Where("id = ? AND status = ? AND billing_state = ? AND updated_at = ?", stored.Id, SearchUsageStatusSucceeded, SearchUsageBillingLogWriting, stored.UpdatedAt).
			Update("updated_at", now)
		if takenOver.Error != nil {
			return takenOver.Error
		}
		if takenOver.RowsAffected == 0 {
			return nil
		}
		stored.UpdatedAt = now
	}

	if common.LogConsumeEnabled {
		var count int64
		if err := LOG_DB.Model(&Log{}).
			Where(&Log{RequestId: stored.RequestID, Type: LogTypeConsume, Group: "vsearch"}).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			var username string
			_ = DB.Model(&User{}).Where("id = ?", stored.UserID).Pluck("username", &username).Error
			log := &Log{
				UserId: stored.UserID, EnterpriseID: stored.EnterpriseID, Username: username,
				CreatedAt: stored.CreatedAt, Type: LogTypeConsume, Content: "vSearch capability execution",
				ModelName: "vsearch:" + stored.ServiceID, Quota: stored.ReservedQuota,
				Group: "vsearch", RequestId: stored.RequestID,
				Other: common.MapToJsonStr(map[string]any{
					"billing_source": stored.BillingSource, "charge_currency": "CNY",
					"charge_micros": stored.ChargeMicros,
				}),
			}
			if err := LOG_DB.Create(log).Error; err != nil {
				return err
			}
		}
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ? AND billing_state = ? AND updated_at = ?", stored.Id, SearchUsageStatusSucceeded, SearchUsageBillingLogWriting, stored.UpdatedAt).
		Updates(map[string]any{"billing_state": SearchUsageBillingCommitted, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("search usage consume log lost ownership: %w", gorm.ErrRecordNotFound)
	}
	event.BillingState = SearchUsageBillingCommitted
	return nil
}

func UpdateSearchUsageEventProgress(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 {
		return errors.New("search usage progress update is invalid")
	}
	phase := strings.TrimSpace(event.ExecutionPhase)
	billingState := strings.TrimSpace(event.BillingState)
	switch phase {
	case SearchUsagePhasePrepared, SearchUsagePhaseDispatching:
	default:
		return errors.New("search usage progress phase is invalid")
	}
	switch billingState {
	case SearchUsageBillingNotStarted, SearchUsageBillingReservePending, SearchUsageBillingReserved:
	default:
		return errors.New("search usage progress billing state is invalid")
	}
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ?", event.Id, SearchUsageStatusPending).
		Updates(map[string]any{"execution_phase": phase, "billing_state": billingState})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func applySearchUsageQuery(query SearchUsageQuery) *gorm.DB {
	db := DB.Model(&SearchUsageEvent{})
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.EnterpriseID > 0 {
		db = db.Where("enterprise_id = ?", query.EnterpriseID)
	}
	if query.AgentKeyID > 0 {
		db = db.Where("agent_key_id = ?", query.AgentKeyID)
	}
	if query.CapabilityID > 0 {
		db = db.Where("capability_id = ?", query.CapabilityID)
	}
	if serviceID := strings.TrimSpace(query.ServiceID); serviceID != "" {
		db = db.Where("service_id = ?", serviceID)
	}
	if action := strings.TrimSpace(query.Action); action != "" {
		db = db.Where("action = ?", action)
	}
	if searchText := strings.TrimSpace(query.SearchText); searchText != "" {
		searchRunes := []rune(searchText)
		if len(searchRunes) > 128 {
			searchText = string(searchRunes[:128])
		}
		escapedSearchText := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(searchText))
		pattern := "%" + escapedSearchText + "%"
		keyIDs := DB.Model(&SearchAgentKey{}).Select("id").Where("LOWER(name) LIKE ? ESCAPE '!'", pattern)
		userIDs := DB.Model(&User{}).Select("id").Where("LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!'", pattern, pattern, pattern)
		accountIDs := DB.Model(&SearchUpstreamAccount{}).Select("id").Where("LOWER(name) LIKE ? ESCAPE '!'", pattern)
		db = db.Where(
			"LOWER(service_name) LIKE ? ESCAPE '!' OR LOWER(service_id) LIKE ? ESCAPE '!' OR LOWER(action) LIKE ? ESCAPE '!' OR LOWER(request_id) LIKE ? ESCAPE '!' OR LOWER(error_code) LIKE ? ESCAPE '!' OR agent_key_id IN (?) OR user_id IN (?) OR upstream_account_id IN (?)",
			pattern, pattern, pattern, pattern, pattern, keyIDs, userIDs, accountIDs,
		)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	if query.StartAt > 0 {
		db = db.Where("created_at >= ?", query.StartAt)
	}
	if query.EndAt > 0 {
		db = db.Where("created_at <= ?", query.EndAt)
	}
	return db
}

func ListSearchUsageEvents(query SearchUsageQuery) ([]*SearchUsageEvent, int64, error) {
	events := make([]*SearchUsageEvent, 0)
	db := applySearchUsageQuery(query)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	err := db.Order("created_at desc, id desc").Offset(offset).Limit(limit).Find(&events).Error
	return events, total, err
}

func GetSearchUsageStat(query SearchUsageQuery) (*SearchUsageStat, error) {
	stat := &SearchUsageStat{}
	err := applySearchUsageQuery(query).Select(
		"COUNT(*) AS request_count, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_count, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS error_count, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending_count, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS indeterminate_count, "+
			"COALESCE(SUM(CASE WHEN status IN (?, ?) THEN latency_ms ELSE 0 END), 0) AS total_latency_ms, "+
			"COALESCE(SUM(upstream_cost_micros), 0) AS upstream_cost_micros, "+
			"COALESCE(SUM(charge_micros), 0) AS charge_micros",
		SearchUsageStatusSucceeded, SearchUsageStatusFailed, SearchUsageStatusPending, SearchUsageStatusIndeterminate,
		SearchUsageStatusSucceeded, SearchUsageStatusFailed,
	).Scan(stat).Error
	if err != nil {
		return nil, err
	}
	stat.MarginMicros = stat.ChargeMicros - stat.UpstreamCostMicros
	return stat, nil
}

func ReconcileStaleSearchUsageEvents(now int64) error {
	if now <= SearchUsagePendingTimeoutSeconds {
		return nil
	}
	cutoff := now - SearchUsagePendingTimeoutSeconds
	var stale []*SearchUsageEvent
	if err := DB.Where("updated_at <= ? AND (status = ? OR billing_state IN ?)", cutoff, SearchUsageStatusPending, []string{
		SearchUsageBillingRefundPending, SearchUsageBillingRefundFailed,
	}).
		Order("updated_at asc, id asc").Limit(100).Find(&stale).Error; err != nil {
		return err
	}
	var reconcileErrors []error
	for _, event := range stale {
		switch event.BillingState {
		case SearchUsageBillingCommitPending:
			committed, err := CommitSearchUsageEvent(event.Id)
			if err == nil {
				err = EnsureSearchUsageConsumeLog(committed)
			}
			if err != nil {
				reconcileErrors = append(reconcileErrors, fmt.Errorf("commit vSearch usage %s: %w", event.RequestID, err))
			}
		case SearchUsageBillingReservePending, SearchUsageBillingReserved, SearchUsageBillingRefundPending, SearchUsageBillingRefundFailed:
			if err := reconcileSearchUsageRefund(event); err != nil {
				reconcileErrors = append(reconcileErrors, fmt.Errorf("refund vSearch usage %s: %w", event.RequestID, err))
			}
		default:
			status := SearchUsageStatusIndeterminate
			errorCode := "VSEARCH_EXECUTION_INDETERMINATE"
			message := "vSearch execution did not reach a durable terminal state."
			if event.ExecutionPhase == SearchUsagePhasePrepared {
				status = SearchUsageStatusFailed
				errorCode = "VSEARCH_EXECUTION_RECOVERED"
				message = "vSearch execution stopped before upstream dispatch."
			}
			if err := finalizeReconciledSearchUsage(event.Id, status, event.BillingState, errorCode, message); err != nil {
				reconcileErrors = append(reconcileErrors, fmt.Errorf("finalize vSearch usage %s: %w", event.RequestID, err))
			}
		}
	}

	var pendingLogs []*SearchUsageEvent
	if err := DB.Where("status = ? AND (billing_state = ? OR (billing_state = ? AND updated_at <= ?))", SearchUsageStatusSucceeded, SearchUsageBillingLogPending, SearchUsageBillingLogWriting, cutoff).
		Order("id asc").Limit(100).Find(&pendingLogs).Error; err != nil {
		reconcileErrors = append(reconcileErrors, err)
	} else {
		for _, event := range pendingLogs {
			if err := EnsureSearchUsageConsumeLog(event); err != nil {
				reconcileErrors = append(reconcileErrors, fmt.Errorf("materialize vSearch consume log %s: %w", event.RequestID, err))
			}
		}
	}
	return errors.Join(reconcileErrors...)
}

func reconcileSearchUsageRefund(event *SearchUsageEvent) error {
	if event == nil || event.Id <= 0 {
		return errors.New("search usage refund recovery is invalid")
	}
	if event.BillingState != SearchUsageBillingRefundPending {
		result := DB.Model(&SearchUsageEvent{}).
			Where("id = ? AND billing_state IN ?", event.Id, []string{
				SearchUsageBillingReservePending, SearchUsageBillingReserved, SearchUsageBillingRefundFailed,
			}).
			Updates(map[string]any{"billing_state": SearchUsageBillingRefundPending, "updated_at": common.GetTimestamp()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var stored SearchUsageEvent
			if err := DB.Where("id = ?", event.Id).First(&stored).Error; err != nil {
				return err
			}
			if stored.BillingState == SearchUsageBillingRefunded {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
	}

	_, walletErr := GetWalletPreConsumeRecord(event.RequestID)
	if walletErr == nil {
		if err := RefundUserWalletPreConsume(event.RequestID); err != nil {
			_ = markSearchUsageRefundFailed(event.Id)
			return err
		}
	} else if !errors.Is(walletErr, gorm.ErrRecordNotFound) {
		_ = markSearchUsageRefundFailed(event.Id)
		return walletErr
	}

	var subscriptionRecord SubscriptionPreConsumeRecord
	subscriptionQuery := DB.Where("request_id = ?", event.RequestID).Limit(1).Find(&subscriptionRecord)
	if subscriptionQuery.Error != nil {
		_ = markSearchUsageRefundFailed(event.Id)
		return subscriptionQuery.Error
	}
	if subscriptionQuery.RowsAffected > 0 && subscriptionRecord.Status != "refunded" {
		if err := RefundSubscriptionPreConsume(event.RequestID); err != nil {
			_ = markSearchUsageRefundFailed(event.Id)
			return err
		}
	}
	if walletErr != nil && subscriptionQuery.RowsAffected == 0 && event.ReservedQuota > 0 && event.BillingState != SearchUsageBillingReservePending {
		_ = markSearchUsageRefundFailed(event.Id)
		return errors.New("durable reservation record is missing")
	}

	status := SearchUsageStatusIndeterminate
	errorCode := "VSEARCH_EXECUTION_INDETERMINATE"
	message := "vSearch execution state was recovered and its reservation was refunded."
	if event.ExecutionPhase == SearchUsagePhasePrepared {
		status = SearchUsageStatusFailed
		errorCode = "VSEARCH_EXECUTION_RECOVERED"
		message = "vSearch execution stopped before upstream dispatch and its reservation was refunded."
	}
	if event.Status != SearchUsageStatusPending {
		result := DB.Model(&SearchUsageEvent{}).
			Where("id = ? AND billing_state = ?", event.Id, SearchUsageBillingRefundPending).
			Updates(map[string]any{
				"billing_state": SearchUsageBillingRefunded, "charge_micros": 0,
				"error_code":              "VSEARCH_BILLING_COMPENSATION_RECOVERED",
				"sanitized_error_message": "vSearch billing compensation completed during recovery.",
				"updated_at":              common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var stored SearchUsageEvent
			if err := DB.Where("id = ?", event.Id).First(&stored).Error; err != nil {
				return err
			}
			if stored.BillingState != SearchUsageBillingRefunded {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	}
	return finalizeReconciledSearchUsage(event.Id, status, SearchUsageBillingRefunded, errorCode, message)
}

func markSearchUsageRefundFailed(eventID int64) error {
	return DB.Model(&SearchUsageEvent{}).Where("id = ? AND status = ?", eventID, SearchUsageStatusPending).
		Updates(map[string]any{"billing_state": SearchUsageBillingRefundFailed, "updated_at": common.GetTimestamp()}).Error
}

func finalizeReconciledSearchUsage(eventID int64, status int, billingState, errorCode, message string) error {
	result := DB.Model(&SearchUsageEvent{}).
		Where("id = ? AND status = ?", eventID, SearchUsageStatusPending).
		Updates(map[string]any{
			"status": status, "http_status": 500, "billing_state": billingState,
			"charge_micros": 0, "error_code": errorCode,
			"sanitized_error_message": message, "updated_at": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var stored SearchUsageEvent
		if err := DB.Where("id = ?", eventID).First(&stored).Error; err != nil {
			return err
		}
		if stored.Status == status && stored.BillingState == billingState {
			return nil
		}
		return gorm.ErrRecordNotFound
	}
	return nil
}
