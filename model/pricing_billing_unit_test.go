package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestPricingBillingUnitSeparatesVideoPerSecondAndPerRequestModels(t *testing.T) {
	originalPatches := constant.TaskPricePatches
	constant.TaskPricePatches = []string{"seedance-2.0", "seedance-2.0-fast"}
	t.Cleanup(func() { constant.TaskPricePatches = originalPatches })

	videoEndpoint := []constant.EndpointType{constant.EndpointTypeOpenAIVideo}

	assert.Equal(t, "second", pricingBillingUnit("MiniMax-H3", videoEndpoint))
	assert.Equal(t, "second", pricingBillingUnit("seedance-2-official", videoEndpoint))
	assert.Equal(t, "request", pricingBillingUnit("seedance-2.0", videoEndpoint))
	assert.Equal(t, "request", pricingBillingUnit("seedance-2.0-fast", videoEndpoint))
	assert.Empty(t, pricingBillingUnit("gpt-5.5", []constant.EndpointType{constant.EndpointTypeOpenAI}))
}
