//go:build !e2e

package foundry

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func TestAzureCatalogDistinguishesUnknownQuotaFromZeroAndClampsCapacity(t *testing.T) {
	cases := []struct {
		name    string
		quota   []*armcognitiveservices.Usage
		maximum *int64
	}{
		{"unknown", nil, nil},
		{"missing current usage", []*armcognitiveservices.Usage{{Name: &armcognitiveservices.MetricName{Value: to.Ptr("shared")}, Limit: to.Ptr(float64(100)), Unit: to.Ptr(armcognitiveservices.UnitTypeCount)}}, nil},
		{"wrong unit", []*armcognitiveservices.Usage{{Name: &armcognitiveservices.MetricName{Value: to.Ptr("shared")}, Limit: to.Ptr(float64(100)), CurrentValue: to.Ptr(float64(0)), Unit: to.Ptr(armcognitiveservices.UnitTypeBytes)}}, nil},
		{"fully used", catalogUsage(100, 100), to.Ptr(int64(0))},
		{"overused", catalogUsage(100, 110), to.Ptr(int64(0))},
		{"quota bound", catalogUsage(100, 70), to.Ptr(int64(30000))},
		{"SKU bound", catalogUsage(100, 10), to.Ptr(int64(50000))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limits := azureLimits{models: []*armcognitiveservices.AccountModel{{Name: to.Ptr("gpt-test"), Version: to.Ptr("v1"), SKUs: []*armcognitiveservices.ModelSKU{{Name: to.Ptr("Standard"), UsageName: to.Ptr("SHARED"), Capacity: &armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(50))}}}}}, quota: tc.quota}
			items := limits.Catalog()
			if len(items) != 1 || len(items[0].SKUs) != 1 {
				t.Fatalf("catalog = %#v", items)
			}
			for _, got := range []*int64{items[0].MaxCapacity, items[0].SKUs[0].MaxCapacity} {
				if (got == nil) != (tc.maximum == nil) || (got != nil && *got != *tc.maximum) {
					t.Fatalf("capacity = %v, want %v", got, tc.maximum)
				}
			}
		})
	}
}

func catalogUsage(limit, current float64) []*armcognitiveservices.Usage {
	return []*armcognitiveservices.Usage{{Name: &armcognitiveservices.MetricName{Value: to.Ptr("shared")}, Limit: &limit, CurrentValue: &current, Unit: to.Ptr(armcognitiveservices.UnitTypeCount)}}
}

func TestAzureCatalogDoesNotInventVersionSKUOrCapacity(t *testing.T) {
	limits := azureLimits{models: []*armcognitiveservices.AccountModel{
		{Name: to.Ptr("missing-options")},
		{Name: to.Ptr("missing-capacity"), Version: to.Ptr("v1"), SKUs: []*armcognitiveservices.ModelSKU{{Name: to.Ptr("Standard"), UsageName: to.Ptr("shared")}}},
	}, quota: catalogUsage(100, 0)}
	items := limits.Catalog()
	if len(items) != 2 {
		t.Fatalf("catalog = %#v", items)
	}
	for _, item := range items {
		if item.MaxCapacity != nil {
			t.Fatalf("invented capacity for %s: %d", item.Name, *item.MaxCapacity)
		}
		if item.Name == "missing-options" && (len(item.Versions) != 0 || len(item.SKUs) != 0) {
			t.Fatalf("invented options: %#v", item)
		}
		if item.Name == "missing-capacity" && (len(item.SKUs) != 1 || item.SKUs[0].MaxCapacity != nil) {
			t.Fatalf("invented SKU capacity: %#v", item)
		}
	}
}
