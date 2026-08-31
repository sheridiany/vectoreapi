package vsearch

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchChargeMicrosToQuotaUsesCoreBillingConfiguration(t *testing.T) {
	previousRate := operation_setting.USDExchangeRate
	previousQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.USDExchangeRate = previousRate
		common.QuotaPerUnit = previousQuotaPerUnit
	})
	operation_setting.USDExchangeRate = 7.5
	common.QuotaPerUnit = 600_000

	quota, err := searchChargeMicrosToQuota(1_250_000)
	require.NoError(t, err)
	assert.Equal(t, 100_000, quota, "CNY 1.25 / 7.5 CNY per USD * 600k quota per USD")

	quota, err = searchChargeMicrosToQuota(1)
	require.NoError(t, err)
	assert.Equal(t, 1, quota, "a positive configured charge must never become a free call after rounding")

	quota, err = searchChargeMicrosToQuota(0)
	require.NoError(t, err)
	assert.Zero(t, quota)
}

func TestSearchUpstreamCostToCNYUsesConfiguredRateWithoutMarkup(t *testing.T) {
	previousRate := operation_setting.USDExchangeRate
	t.Cleanup(func() { operation_setting.USDExchangeRate = previousRate })
	operation_setting.USDExchangeRate = 7.5

	cost, currency, err := searchUpstreamCostToCNY(2_000, "usd")
	require.NoError(t, err)
	assert.Equal(t, int64(15_000), cost)
	assert.Equal(t, "CNY", currency)

	cost, currency, err = searchUpstreamCostToCNY(15_000, "CNY")
	require.NoError(t, err)
	assert.Equal(t, int64(15_000), cost)
	assert.Equal(t, "CNY", currency)
}

func TestSearchUpstreamCostToCNYRejectsInvalidCurrencyAndRate(t *testing.T) {
	previousRate := operation_setting.USDExchangeRate
	t.Cleanup(func() { operation_setting.USDExchangeRate = previousRate })

	_, _, err := searchUpstreamCostToCNY(2_000, "EUR")
	require.Error(t, err)
	operation_setting.USDExchangeRate = 0
	_, _, err = searchUpstreamCostToCNY(2_000, "USD")
	require.Error(t, err)
	_, _, err = searchUpstreamCostToCNY(-1, "CNY")
	require.Error(t, err)
}

func TestSearchChargeMicrosToQuotaRejectsInvalidOrUnrepresentableValues(t *testing.T) {
	previousRate := operation_setting.USDExchangeRate
	previousQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.USDExchangeRate = previousRate
		common.QuotaPerUnit = previousQuotaPerUnit
	})

	_, err := searchChargeMicrosToQuota(-1)
	require.Error(t, err)

	operation_setting.USDExchangeRate = 0
	_, err = searchChargeMicrosToQuota(1_000_000)
	require.Error(t, err)

	operation_setting.USDExchangeRate = 7.5
	common.QuotaPerUnit = math.MaxFloat64
	_, err = searchChargeMicrosToQuota(math.MaxInt64)
	require.Error(t, err, "overflow must fail before any pre-consume reaches the database")
}
