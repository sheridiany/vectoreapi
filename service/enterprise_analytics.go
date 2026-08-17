package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type EnterpriseUsageDaily struct {
	StartAt      int64 `json:"start_at"`
	EndAt        int64 `json:"end_at"`
	NetQuota     int64 `json:"net_quota"`
	TotalTokens  int64 `json:"total_tokens"`
	RequestCount int64 `json:"request_count"`
}

type EnterpriseUsageModel struct {
	ModelName    string `json:"model_name"`
	NetQuota     int64  `json:"net_quota"`
	TotalTokens  int64  `json:"total_tokens"`
	RequestCount int64  `json:"request_count"`
}

type EnterpriseAnalyticsResponse struct {
	EnterpriseID int                    `json:"enterprise_id"`
	Period       string                 `json:"period"`
	StartAt      int64                  `json:"start_at"`
	EndAt        int64                  `json:"end_at"`
	Daily        []EnterpriseUsageDaily `json:"daily"`
	Models       []EnterpriseUsageModel `json:"models"`
}

func GetEnterpriseAnalytics(enterpriseID int, period, start, end string) (*EnterpriseAnalyticsResponse, error) {
	if enterpriseID <= 0 {
		return nil, model.ErrInvalidEnterpriseID
	}
	rangeConfig, err := parseEnterpriseRankingRange(period, start, end, time.Now())
	if err != nil {
		return nil, err
	}
	if rangeConfig.endTime-rangeConfig.startTime > 90*24*60*60 {
		return nil, fmt.Errorf("enterprise analytics range cannot exceed 90 days")
	}

	daily := make([]EnterpriseUsageDaily, 0)
	for cursor := rangeConfig.startTime; cursor <= rangeConfig.endTime; cursor += 24 * 60 * 60 {
		windowEnd := cursor + 24*60*60 - 1
		if windowEnd > rangeConfig.endTime {
			windowEnd = rangeConfig.endTime
		}
		aggregate, err := model.GetEnterpriseUsageAggregateByRange(enterpriseID, cursor, windowEnd)
		if err != nil {
			return nil, err
		}
		daily = append(daily, EnterpriseUsageDaily{
			StartAt: cursor, EndAt: windowEnd, NetQuota: aggregate.NetQuota,
			TotalTokens: aggregate.TotalTokens, RequestCount: aggregate.RequestCount,
		})
		if windowEnd == rangeConfig.endTime {
			break
		}
	}

	modelRows, err := model.GetEnterpriseModelUsageAggregates(enterpriseID, rangeConfig.startTime, rangeConfig.endTime)
	if err != nil {
		return nil, err
	}
	models := make([]EnterpriseUsageModel, 0, len(modelRows))
	for _, row := range modelRows {
		models = append(models, EnterpriseUsageModel(row))
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].NetQuota == models[j].NetQuota {
			return models[i].ModelName < models[j].ModelName
		}
		return models[i].NetQuota > models[j].NetQuota
	})

	return &EnterpriseAnalyticsResponse{
		EnterpriseID: enterpriseID, Period: rangeConfig.period,
		StartAt: rangeConfig.startTime, EndAt: rangeConfig.endTime,
		Daily: daily, Models: models,
	}, nil
}
