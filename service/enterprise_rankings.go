package service

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type EnterpriseRanking struct {
	Rank         int     `json:"rank"`
	EnterpriseID int     `json:"enterprise_id"`
	Name         string  `json:"name"`
	NetQuota     int64   `json:"net_quota"`
	TotalTokens  int64   `json:"total_tokens"`
	RequestCount int64   `json:"request_count"`
	ActiveUsers  int64   `json:"active_users"`
	GrowthPct    float64 `json:"growth_pct"`
}

type EnterpriseMemberRanking struct {
	Rank         int     `json:"rank"`
	UserID       int     `json:"user_id"`
	Username     string  `json:"username"`
	NetQuota     int64   `json:"net_quota"`
	TotalTokens  int64   `json:"total_tokens"`
	RequestCount int64   `json:"request_count"`
	GrowthPct    float64 `json:"growth_pct"`
}

type EnterpriseRankingsResponse struct {
	EnterpriseID int                       `json:"enterprise_id,omitempty"`
	Period       string                    `json:"period"`
	StartAt      int64                     `json:"start_at"`
	EndAt        int64                     `json:"end_at"`
	Enterprises  []EnterpriseRanking       `json:"enterprises"`
	Enterprise   *EnterpriseRanking        `json:"enterprise,omitempty"`
	Members      []EnterpriseMemberRanking `json:"members,omitempty"`
}

type enterpriseRankingRange struct {
	period    string
	startTime int64
	endTime   int64
	duration  int64
}

func GetEnterpriseRankings(period, start, end string) (*EnterpriseRankingsResponse, error) {
	rangeConfig, err := parseEnterpriseRankingRange(period, start, end, time.Now())
	if err != nil {
		return nil, err
	}
	current, err := model.GetEnterpriseUsageAggregates(rangeConfig.startTime, rangeConfig.endTime)
	if err != nil {
		return nil, err
	}
	previous, err := model.GetEnterpriseUsageAggregates(rangeConfig.startTime-rangeConfig.duration, rangeConfig.startTime-1)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(current))
	for _, item := range current {
		ids = append(ids, item.EnterpriseID)
	}
	enterprises, err := model.GetEnterprisesByIDs(ids)
	if err != nil {
		return nil, err
	}
	previousByID := make(map[int]int64, len(previous))
	for _, item := range previous {
		previousByID[item.EnterpriseID] = item.NetQuota
	}
	rows := make([]EnterpriseRanking, 0, len(current))
	for _, item := range current {
		name := "Unknown enterprise"
		if enterprise, ok := enterprises[item.EnterpriseID]; ok {
			name = enterprise.Name
		}
		rows = append(rows, EnterpriseRanking{
			EnterpriseID: item.EnterpriseID,
			Name:         name,
			NetQuota:     item.NetQuota,
			TotalTokens:  item.TotalTokens,
			RequestCount: item.RequestCount,
			ActiveUsers:  item.ActiveUsers,
			GrowthPct:    enterpriseRankingGrowth(item.NetQuota, previousByID[item.EnterpriseID]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].NetQuota == rows[j].NetQuota {
			return rows[i].EnterpriseID < rows[j].EnterpriseID
		}
		return rows[i].NetQuota > rows[j].NetQuota
	})
	for index := range rows {
		rows[index].Rank = index + 1
	}
	return &EnterpriseRankingsResponse{
		Period:      rangeConfig.period,
		StartAt:     rangeConfig.startTime,
		EndAt:       rangeConfig.endTime,
		Enterprises: rows,
	}, nil
}

func GetEnterpriseMemberRankings(enterpriseID int, period, start, end string) (*EnterpriseRankingsResponse, error) {
	if enterpriseID <= 0 {
		return nil, fmt.Errorf("enterprise id is invalid")
	}
	rangeConfig, err := parseEnterpriseRankingRange(period, start, end, time.Now())
	if err != nil {
		return nil, err
	}
	current, err := model.GetEnterpriseMemberUsageAggregates(enterpriseID, rangeConfig.startTime, rangeConfig.endTime)
	if err != nil {
		return nil, err
	}
	previous, err := model.GetEnterpriseMemberUsageAggregates(enterpriseID, rangeConfig.startTime-rangeConfig.duration, rangeConfig.startTime-1)
	if err != nil {
		return nil, err
	}
	enterprise, err := model.GetEnterpriseByID(enterpriseID)
	if err != nil {
		return nil, err
	}
	currentEnterprise, err := model.GetEnterpriseUsageAggregate(enterpriseID, rangeConfig.startTime, rangeConfig.endTime)
	if err != nil {
		return nil, err
	}
	previousEnterprise, err := model.GetEnterpriseUsageAggregate(enterpriseID, rangeConfig.startTime-rangeConfig.duration, rangeConfig.startTime-1)
	if err != nil {
		return nil, err
	}
	previousByID := make(map[int]int64, len(previous))
	for _, item := range previous {
		previousByID[item.UserID] = item.NetQuota
	}
	rows := make([]EnterpriseMemberRanking, 0, len(current))
	for _, item := range current {
		rows = append(rows, EnterpriseMemberRanking{
			UserID:       item.UserID,
			Username:     item.Username,
			NetQuota:     item.NetQuota,
			TotalTokens:  item.TotalTokens,
			RequestCount: item.RequestCount,
			GrowthPct:    enterpriseRankingGrowth(item.NetQuota, previousByID[item.UserID]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].NetQuota == rows[j].NetQuota {
			return rows[i].UserID < rows[j].UserID
		}
		return rows[i].NetQuota > rows[j].NetQuota
	})
	for index := range rows {
		rows[index].Rank = index + 1
	}
	return &EnterpriseRankingsResponse{
		EnterpriseID: enterpriseID,
		Period:       rangeConfig.period,
		StartAt:      rangeConfig.startTime,
		EndAt:        rangeConfig.endTime,
		Enterprise: &EnterpriseRanking{
			Rank:         1,
			EnterpriseID: enterpriseID,
			Name:         enterprise.Name,
			NetQuota:     currentEnterprise.NetQuota,
			TotalTokens:  currentEnterprise.TotalTokens,
			RequestCount: currentEnterprise.RequestCount,
			ActiveUsers:  currentEnterprise.ActiveUsers,
			GrowthPct:    enterpriseRankingGrowth(currentEnterprise.NetQuota, previousEnterprise.NetQuota),
		},
		Members: rows,
	}, nil
}

func parseEnterpriseRankingRange(period, start, end string, now time.Time) (enterpriseRankingRange, error) {
	if period == "" {
		period = "week"
	}
	endTime := now.Unix()
	var startTime int64
	switch period {
	case "today":
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		startTime = midnight.Unix()
	case "week":
		startTime = now.Add(-7 * 24 * time.Hour).Unix()
	case "month":
		startTime = now.Add(-30 * 24 * time.Hour).Unix()
	case "custom":
		parsedStart, parseStartErr := strconv.ParseInt(start, 10, 64)
		parsedEnd, parseEndErr := strconv.ParseInt(end, 10, 64)
		if parseStartErr != nil || parseEndErr != nil {
			return enterpriseRankingRange{}, fmt.Errorf("custom ranking range requires valid start and end timestamps")
		}
		startTime, endTime = parsedStart, parsedEnd
	default:
		return enterpriseRankingRange{}, fmt.Errorf("invalid enterprise ranking period: %s", period)
	}
	if startTime < 0 || endTime <= startTime {
		return enterpriseRankingRange{}, fmt.Errorf("enterprise ranking range is invalid")
	}
	return enterpriseRankingRange{period: period, startTime: startTime, endTime: endTime, duration: endTime - startTime + 1}, nil
}

func enterpriseRankingGrowth(current, previous int64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return float64(current-previous) / float64(previous) * 100
}
