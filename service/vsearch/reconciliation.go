package vsearch

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type UsageReconciliationCommand struct {
	EventID int64
	Action  string
	AdminID int
	Note    string
}

type UsageReconciliationResult struct {
	Event   *model.SearchUsageEvent
	Started bool
}

func ReconcileUsage(ctx context.Context, command UsageReconciliationCommand) (*UsageReconciliationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command.Action = strings.ToLower(strings.TrimSpace(command.Action))
	command.Note = strings.TrimSpace(command.Note)
	if command.EventID <= 0 || command.AdminID <= 0 || command.Note == "" || len([]rune(command.Note)) > 255 ||
		(command.Action != model.SearchUsageReconciliationSettle && command.Action != model.SearchUsageReconciliationRefund) {
		return nil, &PublicError{
			Code: "VSEARCH_RECONCILIATION_INVALID", Message: "vSearch 对账请求无效。", HTTPStatus: http.StatusBadRequest,
		}
	}
	event, started, err := model.ReconcileIndeterminateSearchUsage(command.EventID, command.Action, command.AdminID, command.Note)
	if err == nil {
		return &UsageReconciliationResult{Event: event, Started: started}, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &PublicError{
			Code: "VSEARCH_USAGE_NOT_FOUND", Message: "vSearch 调用记录不存在。", HTTPStatus: http.StatusNotFound,
		}
	}
	if errors.Is(err, model.ErrSearchUsageReconciliationConflict) {
		return nil, &PublicError{
			Code: "VSEARCH_RECONCILIATION_CONFLICT", Message: "该调用记录已按另一种方式完成对账。", HTTPStatus: http.StatusConflict,
		}
	}
	if errors.Is(err, model.ErrSearchUsageNotReconcilable) {
		return nil, &PublicError{
			Code: "VSEARCH_USAGE_NOT_RECONCILABLE", Message: "该调用记录当前不允许人工对账。", HTTPStatus: http.StatusConflict,
		}
	}
	return nil, err
}
