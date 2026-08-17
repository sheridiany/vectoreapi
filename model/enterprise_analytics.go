package model

type EnterpriseModelUsageAggregate struct {
	ModelName    string `json:"model_name"`
	NetQuota     int64  `json:"net_quota"`
	TotalTokens  int64  `json:"total_tokens"`
	RequestCount int64  `json:"request_count"`
}

func GetEnterpriseUsageAggregateByRange(enterpriseID int, startTime, endTime int64) (*EnterpriseUsageAggregate, error) {
	result := &EnterpriseUsageAggregate{EnterpriseID: enterpriseID}
	err := LOG_DB.Model(&Log{}).
		Select("enterprise_id, COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS net_quota, COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens, COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS request_count, COUNT(DISTINCT CASE WHEN type = ? THEN user_id END) AS active_users", LogTypeConsume, LogTypeRefund, LogTypeConsume, LogTypeConsume, LogTypeConsume).
		Where("enterprise_id = ? AND created_at >= ? AND created_at <= ?", enterpriseID, startTime, endTime).
		Scan(result).Error
	result.EnterpriseID = enterpriseID
	return result, err
}

func GetEnterpriseModelUsageAggregates(enterpriseID int, startTime, endTime int64) ([]EnterpriseModelUsageAggregate, error) {
	rows := make([]EnterpriseModelUsageAggregate, 0)
	err := LOG_DB.Model(&Log{}).
		Select("model_name, COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS net_quota, COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens, COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS request_count", LogTypeConsume, LogTypeRefund, LogTypeConsume, LogTypeConsume).
		Where("enterprise_id = ? AND created_at >= ? AND created_at <= ?", enterpriseID, startTime, endTime).
		Group("model_name").
		Scan(&rows).Error
	return rows, err
}
