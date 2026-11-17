package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestPublicPricingHidesMediaProviderWithoutChangingCachedMetadata(t *testing.T) {
	input := []model.Pricing{{ModelName: "seedance-2.0-fast-token", EnableGroup: []string{"default"}, BillingPluginVariants: []model.PricingPluginVariant{{PluginKey: "zhiqi", PluginName: "Zhiqi", Icon: "text:ZQ", BillingExpr: "price-expression"}}}}
	output := filterPricingByUsableGroups(input, map[string]string{"default": "Default"})
	assert.Equal(t, "media", output[0].BillingPluginVariants[0].PluginKey)
	assert.Equal(t, "Allin Media", output[0].BillingPluginVariants[0].PluginName)
	assert.Equal(t, "price-expression", output[0].BillingPluginVariants[0].BillingExpr)
	assert.Equal(t, "zhiqi", input[0].BillingPluginVariants[0].PluginKey)
}
