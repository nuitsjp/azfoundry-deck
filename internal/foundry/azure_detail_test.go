//go:build !e2e

package foundry

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func TestCapacityUnitsUseModelSpecificRatesAndRenewalPeriods(t *testing.T) {
	cases := []struct {
		name, sku, key, unit string
		capacity             int32
		count, period        float32
		multiplier           float64
	}{
		{"chat", "GlobalStandard", "token", "TPM", 50, 50000, 60, 1000},
		{"reasoning", "Standard", "token", "TPM", 20, 120000, 60, 6000},
		{"short renewal", "DataZoneStandard", "token", "TPM", 10, 5000, 10, 3000},
		{"images", "GlobalStandard", "request", "RPM", 8, 8, 60, 1},
		{"zero", "Standard", "token", "TPM", 0, 0, 60, 0},
		{"provisioned", "ProvisionedManaged", "", "PTU", 50, 0, 0, 1},
		{"global provisioned", "GlobalProvisionedManaged", "", "PTU", 50, 0, 0, 1},
		{"data zone provisioned", "DataZoneProvisionedManaged", "", "PTU", 50, 0, 0, 1},
		{"unknown unit", "GlobalBatch", "token", "", 10, 10000, 60, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deployment := armcognitiveservices.Deployment{
				SKU:        &armcognitiveservices.SKU{Capacity: to.Ptr(c.capacity)},
				Properties: &armcognitiveservices.DeploymentProperties{RateLimits: []*armcognitiveservices.ThrottlingRule{{Key: to.Ptr(c.key), Count: to.Ptr(c.count), RenewalPeriod: to.Ptr(c.period)}}},
			}
			unit, multiplier := capacityUnit(c.sku, deployment)
			if unit != c.unit || multiplier != c.multiplier {
				t.Fatalf("got %s x%v, want %s x%v", unit, multiplier, c.unit, c.multiplier)
			}
		})
	}
}

func TestCapacityUnitsPreferTokenRuleOverRequestRule(t *testing.T) {
	deployment := armcognitiveservices.Deployment{
		SKU: &armcognitiveservices.SKU{Capacity: to.Ptr(int32(20))},
		Properties: &armcognitiveservices.DeploymentProperties{RateLimits: []*armcognitiveservices.ThrottlingRule{
			{Key: to.Ptr("request"), Count: to.Ptr(float32(20)), RenewalPeriod: to.Ptr(float32(60))},
			{Key: to.Ptr("token"), Count: to.Ptr(float32(120000)), RenewalPeriod: to.Ptr(float32(60))},
		}},
	}
	unit, multiplier := capacityUnit("GlobalStandard", deployment)
	if unit != "TPM" || multiplier != 6000 {
		t.Fatalf("got %s x%v, want TPM x6000", unit, multiplier)
	}
}

func TestCapacityUnitsDoNotGuessWhenRateDefinitionIsMissing(t *testing.T) {
	deployment := armcognitiveservices.Deployment{SKU: &armcognitiveservices.SKU{Capacity: to.Ptr(int32(50))}, Properties: &armcognitiveservices.DeploymentProperties{}}
	unit, multiplier := capacityUnit("Standard", deployment)
	if unit != "" || multiplier != 0 {
		t.Fatalf("guessed unit %s x%v", unit, multiplier)
	}
}

func TestAllocatableCapacityRespectsDiscreteSettings(t *testing.T) {
	cases := []struct {
		name         string
		quotaMaximum float64
		config       armcognitiveservices.CapacityConfig
		want         float64
	}{
		{"PTU step", 87, armcognitiveservices.CapacityConfig{Minimum: to.Ptr(int32(50)), Step: to.Ptr(int32(50))}, 50},
		{"step anchored to minimum", 48, armcognitiveservices.CapacityConfig{Minimum: to.Ptr(int32(15)), Step: to.Ptr(int32(10))}, 45},
		{"below minimum", 49, armcognitiveservices.CapacityConfig{Minimum: to.Ptr(int32(50)), Step: to.Ptr(int32(50))}, 0},
		{"allowed values", 175, armcognitiveservices.CapacityConfig{AllowedValues: []*int32{to.Ptr(int32(200)), to.Ptr(int32(50)), to.Ptr(int32(100))}}, 100},
		{"below first allowed", 40, armcognitiveservices.CapacityConfig{AllowedValues: []*int32{to.Ptr(int32(50)), to.Ptr(int32(100))}}, 0},
		{"fractional quota remainder", 19.75, armcognitiveservices.CapacityConfig{}, 19},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := allowedCapacity(c.quotaMaximum, &c.config); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
func TestCapacityUnitsRejectIncompleteOrInvalidRates(t *testing.T) {
	cases := []*armcognitiveservices.ThrottlingRule{
		nil,
		{Count: to.Ptr(float32(1000)), RenewalPeriod: to.Ptr(float32(60))},
		{Key: to.Ptr("token"), RenewalPeriod: to.Ptr(float32(60))},
		{Key: to.Ptr("token"), Count: to.Ptr(float32(1000))},
		{Key: to.Ptr("token"), Count: to.Ptr(float32(1000)), RenewalPeriod: to.Ptr(float32(0))},
		{Key: to.Ptr("token"), Count: to.Ptr(float32(1000)), RenewalPeriod: to.Ptr(float32(-1))},
	}
	for index, rule := range cases {
		deployment := armcognitiveservices.Deployment{SKU: &armcognitiveservices.SKU{Capacity: to.Ptr(int32(50))}, Properties: &armcognitiveservices.DeploymentProperties{RateLimits: []*armcognitiveservices.ThrottlingRule{rule}}}
		unit, multiplier := capacityUnit("Standard", deployment)
		if unit != "" || multiplier != 0 {
			t.Errorf("case %d guessed %s x%v", index, unit, multiplier)
		}
	}
}
