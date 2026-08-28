package vsearch

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/model"
)

const searchUsageExportPageSize = 100

type UsageLog struct {
	Event          *model.SearchUsageEvent
	AgentKeyName   string
	UserName       string
	EnterpriseName string
	AccountName    string
}

func ListUsageLogs(ctx context.Context, query model.SearchUsageQuery, includeAdmin bool) ([]UsageLog, int64, error) {
	events, total, err := model.ListSearchUsageEvents(query)
	if err != nil {
		return nil, 0, err
	}
	logs, err := enrichUsageLogs(ctx, events, includeAdmin)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func ExportUsageLogs(ctx context.Context, query model.SearchUsageQuery, maxRows int) ([]UsageLog, error) {
	if maxRows <= 0 {
		return nil, errors.New("search usage export limit is invalid")
	}
	query.Offset = 0
	query.Limit = searchUsageExportPageSize
	events := make([]*model.SearchUsageEvent, 0, min(maxRows, searchUsageExportPageSize))
	for len(events) < maxRows {
		page, total, err := model.ListSearchUsageEvents(query)
		if err != nil {
			return nil, err
		}
		remaining := maxRows - len(events)
		if len(page) > remaining {
			page = page[:remaining]
		}
		events = append(events, page...)
		if int64(len(events)) >= total || len(page) == 0 {
			break
		}
		query.Offset += len(page)
	}
	return enrichUsageLogs(ctx, events, true)
}

func enrichUsageLogs(ctx context.Context, events []*model.SearchUsageEvent, includeAdmin bool) ([]UsageLog, error) {
	relations, err := model.LoadSearchUsageRelations(ctx, events, includeAdmin)
	if err != nil {
		return nil, err
	}
	logs := make([]UsageLog, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		logs = append(logs, UsageLog{
			Event:          event,
			AgentKeyName:   relations.AgentKeyNames[event.AgentKeyID],
			UserName:       relations.UserNames[event.UserID],
			EnterpriseName: relations.EnterpriseNames[event.EnterpriseID],
			AccountName:    relations.UpstreamAccounts[event.UpstreamAccountID],
		})
	}
	return logs, nil
}
