package service

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type EnterpriseBudgetStatus struct {
	EnterpriseID    int     `json:"enterprise_id"`
	BudgetQuota     int64   `json:"budget_quota"`
	AlertThreshold  int     `json:"alert_threshold"`
	ConsumedQuota   int64   `json:"consumed_quota"`
	RemainingQuota  int64   `json:"remaining_quota"`
	UsagePercentage float64 `json:"usage_percentage"`
	AlertLevel      string  `json:"alert_level"`
	PeriodStart     int64   `json:"period_start"`
	PeriodEnd       int64   `json:"period_end"`
}

func GetEnterpriseBudgetStatus(enterpriseID int) (*EnterpriseBudgetStatus, error) {
	if enterpriseID <= 0 {
		return nil, errors.New("enterprise id is invalid")
	}
	enterprise, err := model.GetEnterpriseByID(enterpriseID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	periodEnd := now.Unix()
	aggregate, err := model.GetEnterpriseUsageAggregateByRange(enterpriseID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	consumed := aggregate.NetQuota
	if consumed < 0 {
		consumed = 0
	}
	remaining := enterprise.MonthlyQuotaBudget - consumed
	if remaining < 0 || enterprise.MonthlyQuotaBudget == 0 {
		remaining = 0
	}
	usagePercentage := float64(0)
	alertLevel := "none"
	if enterprise.MonthlyQuotaBudget > 0 {
		usagePercentage = float64(consumed) / float64(enterprise.MonthlyQuotaBudget) * 100
		if consumed >= enterprise.MonthlyQuotaBudget {
			alertLevel = "exceeded"
		} else if enterprise.BudgetAlertThreshold > 0 && usagePercentage >= float64(enterprise.BudgetAlertThreshold) {
			alertLevel = "warning"
		}
	}
	return &EnterpriseBudgetStatus{
		EnterpriseID: enterpriseID, BudgetQuota: enterprise.MonthlyQuotaBudget,
		AlertThreshold: enterprise.BudgetAlertThreshold, ConsumedQuota: consumed,
		RemainingQuota: remaining, UsagePercentage: usagePercentage,
		AlertLevel: alertLevel, PeriodStart: periodStart, PeriodEnd: periodEnd,
	}, nil
}

func UpdateEnterpriseBudget(enterpriseID int, budgetQuota int64, alertThreshold int) (*model.Enterprise, error) {
	enterprise, err := model.GetEnterpriseByID(enterpriseID)
	if err != nil {
		return nil, err
	}
	enterprise.MonthlyQuotaBudget = budgetQuota
	enterprise.BudgetAlertThreshold = alertThreshold
	if err := enterprise.Update(); err != nil {
		return nil, err
	}
	return enterprise, nil
}
