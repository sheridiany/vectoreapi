package model

type EnterpriseUsageAggregate struct {
	EnterpriseID int   `json:"enterprise_id"`
	NetQuota     int64 `json:"net_quota"`
	TotalTokens  int64 `json:"total_tokens"`
	RequestCount int64 `json:"request_count"`
	ActiveUsers  int64 `json:"active_users"`
}

type EnterpriseMemberUsageAggregate struct {
	UserID       int    `json:"user_id"`
	Username     string `json:"username"`
	NetQuota     int64  `json:"net_quota"`
	TotalTokens  int64  `json:"total_tokens"`
	RequestCount int64  `json:"request_count"`
}

func GetEnterpriseUsageAggregate(enterpriseID int, startTime, endTime int64) (*EnterpriseUsageAggregate, error) {
	aggregates, err := GetEnterpriseUsageAggregates(startTime, endTime)
	if err != nil {
		return nil, err
	}
	for _, aggregate := range aggregates {
		if aggregate.EnterpriseID == enterpriseID {
			result := aggregate
			return &result, nil
		}
	}
	return &EnterpriseUsageAggregate{EnterpriseID: enterpriseID}, nil
}

func GetEnterpriseUsageAggregates(startTime, endTime int64) ([]EnterpriseUsageAggregate, error) {
	rows := make([]EnterpriseUsageAggregate, 0)
	query := LOG_DB.Model(&Log{}).
		Select("enterprise_id, COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS net_quota, COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens, COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS request_count, COUNT(DISTINCT CASE WHEN type = ? THEN user_id END) AS active_users", LogTypeConsume, LogTypeRefund, LogTypeConsume, LogTypeConsume, LogTypeConsume).
		Where("enterprise_id > 0 AND created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("enterprise_id")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetEnterpriseMemberUsageAggregates(enterpriseID int, startTime, endTime int64) ([]EnterpriseMemberUsageAggregate, error) {
	rows := make([]EnterpriseMemberUsageAggregate, 0)
	query := LOG_DB.Model(&Log{}).
		Select("user_id, username, COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS net_quota, COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens, COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS request_count", LogTypeConsume, LogTypeRefund, LogTypeConsume, LogTypeConsume).
		Where("enterprise_id = ? AND created_at >= ? AND created_at <= ?", enterpriseID, startTime, endTime).
		Group("user_id, username")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetEnterprisesByIDs(ids []int) (map[int]*Enterprise, error) {
	enterprises := make([]*Enterprise, 0)
	if len(ids) == 0 {
		return map[int]*Enterprise{}, nil
	}
	if err := DB.Where("id IN ?", ids).Find(&enterprises).Error; err != nil {
		return nil, err
	}
	result := make(map[int]*Enterprise, len(enterprises))
	for _, enterprise := range enterprises {
		result[enterprise.Id] = enterprise
	}
	return result, nil
}
