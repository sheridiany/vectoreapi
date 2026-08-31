package vsearch

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	coreservice "github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const searchMoneyMicrosPerCNY = int64(1_000_000)

func searchUpstreamCostToCNY(costMicros int64, currency string) (int64, string, error) {
	if costMicros < 0 || costMicros > maxSearchMoneyMicros {
		return 0, "", fmt.Errorf("vSearch upstream cost is invalid: %d", costMicros)
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "":
		return costMicros, "", nil
	case "CNY":
		return costMicros, "CNY", nil
	case "USD":
		rate := operation_setting.USDExchangeRate
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return 0, "", searchBillingConfigError("USDExchangeRate")
		}
		converted := decimal.NewFromInt(costMicros).Mul(decimal.NewFromFloat(rate)).Round(0)
		if converted.IsNegative() || converted.GreaterThan(decimal.NewFromInt(maxSearchMoneyMicros)) {
			return 0, "", fmt.Errorf("converted vSearch upstream cost is invalid")
		}
		return converted.IntPart(), "CNY", nil
	default:
		return 0, "", fmt.Errorf("unsupported vSearch upstream currency: %s", currency)
	}
}

type executionCharge interface {
	commit() error
	refund(context.Context) error
	potentialChargeMicros() int64
	billingSource() string
	reservedQuota() int
}

type executionChargeFactory func(context.Context, Principal, string, *model.SearchCapability) (executionCharge, error)

type coreExecutionCharge struct {
	session      *coreservice.BillingSession
	relayInfo    *relaycommon.RelayInfo
	quota        int
	chargeMicros int64
}

// searchChargeMicrosToQuota converts the vSearch contract's CNY micros into
// the gateway's USD-backed quota. The exchange rate and quota unit both come
// from the existing billing configuration; this path never owns a second rate.
func searchChargeMicrosToQuota(chargeMicros int64) (int, error) {
	if chargeMicros < 0 {
		return 0, fmt.Errorf("vSearch charge cannot be negative: %d", chargeMicros)
	}
	if chargeMicros == 0 {
		return 0, nil
	}
	rate := operation_setting.USDExchangeRate
	quotaPerUnit := common.QuotaPerUnit
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, searchBillingConfigError("USDExchangeRate")
	}
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
		return 0, searchBillingConfigError("QuotaPerUnit")
	}

	quotaValue := decimal.NewFromInt(chargeMicros).
		Div(decimal.NewFromInt(searchMoneyMicrosPerCNY)).
		Div(decimal.NewFromFloat(rate)).
		Mul(decimal.NewFromFloat(quotaPerUnit))
	quota, err := common.QuotaFromDecimalStrict(quotaValue)
	if err != nil {
		return 0, fmt.Errorf("convert vSearch charge to quota: %w", err)
	}
	if quota == 0 {
		quota = 1
	}
	return quota, nil
}

func searchBillingConfigError(name string) error {
	return fmt.Errorf("vSearch billing configuration %s is invalid", name)
}

func preConsumeExecutionCharge(ctx context.Context, principal Principal, requestID string, capability *model.SearchCapability) (*coreExecutionCharge, error) {
	if capability == nil {
		return nil, &PublicError{
			Code: "VSEARCH_BILLING_UNAVAILABLE", Message: "vSearch 计费服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	request := (&http.Request{}).WithContext(ctx)
	ginContext := &gin.Context{Request: request}
	ginContext.Set(common.RequestIdKey, requestID)
	relayInfo := &relaycommon.RelayInfo{
		UserId:                   principal.UserID,
		EnterpriseID:             principal.EnterpriseID,
		RequestId:                requestID,
		OriginModelName:          "vsearch:" + strings.TrimSpace(capability.PublicID),
		IsPlayground:             true,
		ForcePreConsume:          true,
		DurableWalletReservation: true,
	}
	charge := &coreExecutionCharge{
		relayInfo: relayInfo, chargeMicros: capability.PriceMicros,
	}
	if capability.PriceMicros == 0 {
		return charge, nil
	}
	quota, err := searchChargeMicrosToQuota(capability.PriceMicros)
	if err != nil {
		return nil, &PublicError{
			Code: "VSEARCH_BILLING_NOT_CONFIGURED", Message: "vSearch 计费配置暂不可用。", HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	userSetting, err := model.GetUserSetting(principal.UserID, false)
	if err != nil {
		return nil, &PublicError{
			Code: "VSEARCH_BILLING_UNAVAILABLE", Message: "vSearch 计费服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	relayInfo.UserSetting = userSetting

	session, apiErr := coreservice.NewBillingSession(ginContext, relayInfo, quota)
	if apiErr != nil {
		if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota || apiErr.GetErrorCode() == types.ErrorCodePreConsumeTokenQuotaFailed {
			return nil, &PublicError{
				Code: "INSUFFICIENT_QUOTA", Message: "vSearch 调用额度不足，请充值或检查订阅额度。", HTTPStatus: http.StatusPaymentRequired,
			}
		}
		return nil, &PublicError{
			Code: "VSEARCH_BILLING_UNAVAILABLE", Message: "vSearch 计费服务暂不可用。", HTTPStatus: http.StatusServiceUnavailable,
		}
	}
	relayInfo.Billing = session
	charge.session = session
	charge.quota = quota
	return charge, nil
}

func (charge *coreExecutionCharge) commit() error {
	if charge == nil {
		return nil
	}
	if charge.session != nil {
		if err := charge.session.Settle(charge.quota); err != nil {
			return err
		}
	}
	return nil
}

func (charge *coreExecutionCharge) refund(ctx context.Context) error {
	if charge == nil {
		return nil
	}
	if charge.session == nil {
		return nil
	}
	return charge.session.RefundNow(ctx)
}

func (charge *coreExecutionCharge) potentialChargeMicros() int64 {
	if charge == nil {
		return 0
	}
	return charge.chargeMicros
}

func (charge *coreExecutionCharge) billingSource() string {
	if charge == nil || charge.relayInfo == nil || strings.TrimSpace(charge.relayInfo.BillingSource) == "" {
		return "none"
	}
	return charge.relayInfo.BillingSource
}

func (charge *coreExecutionCharge) reservedQuota() int {
	if charge == nil {
		return 0
	}
	return charge.quota
}
