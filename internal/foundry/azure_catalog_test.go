//go:build !e2e

package foundry

import (
	"io"
	"net/http"
	"strings"
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
			limits := azureLimits{models: []*armcognitiveservices.AccountModel{{Name: to.Ptr("gpt-test"), Version: to.Ptr("v1"), SKUs: []*armcognitiveservices.ModelSKU{{Name: to.Ptr("Standard"), UsageName: to.Ptr("SHARED"), Capacity: &armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(50))}}}}}, quota: tc.quota,
				rates: map[string][]modelRate{modelRateKey("gpt-test", "", "v1", "Standard"): {{Key: "token", Count: 1000, RenewalPeriod: 60}}}}
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

func TestAzureCatalogUsesEachSKUsUnitMinimumAndStep(t *testing.T) {
	model := func(name, sku string, config armcognitiveservices.CapacityConfig) *armcognitiveservices.AccountModel {
		return &armcognitiveservices.AccountModel{Name: to.Ptr(name), Format: to.Ptr("Microsoft"), Version: to.Ptr("1"),
			SKUs: []*armcognitiveservices.ModelSKU{{Name: to.Ptr(sku), UsageName: to.Ptr("shared"), Capacity: &config}}}
	}
	limits := azureLimits{
		models: []*armcognitiveservices.AccountModel{
			model("decision", "GlobalStandard", armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(1000000))}),
			model("reasoning", "GlobalStandard", armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(1000000))}),
			model("provisioned", "GlobalProvisionedManaged", armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(1000000)), Minimum: to.Ptr(int32(15)), Step: to.Ptr(int32(5))}),
			model("unknown", "GlobalStandard", armcognitiveservices.CapacityConfig{Maximum: to.Ptr(int32(1000000))}),
		},
		rates: map[string][]modelRate{
			modelRateKey("decision", "Microsoft", "1", "GlobalStandard"):  {{Key: "request", Count: 1, RenewalPeriod: 60}},
			modelRateKey("reasoning", "Microsoft", "1", "GlobalStandard"): {{Key: "request", Count: 1, RenewalPeriod: 60}, {Key: "token", Count: 1000, RenewalPeriod: 10}},
		},
		quota: catalogUsage(150, 112),
	}
	want := map[string][4]int64{
		"decision":    {38, 1, 1, 1},
		"reasoning":   {228000, 6000, 6000, 6000},
		"provisioned": {35, 15, 5, 1},
	}
	units := map[string]string{"decision": "RPM", "reasoning": "TPM", "provisioned": "PTU"}
	for _, item := range limits.Catalog() {
		sku := item.SKUs[0]
		if item.Name == "unknown" {
			if sku.CapacityUnit != nil || sku.MaxCapacity != nil || sku.MinCapacity != nil {
				t.Fatalf("guessed capacity: %#v", sku)
			}
			continue
		}
		got := [4]int64{*sku.MaxCapacity, *sku.MinCapacity, *sku.CapacityStep, *sku.CapacityPerUnit}
		if got != want[item.Name] || *sku.CapacityUnit != units[item.Name] {
			t.Fatalf("%s = %v %s, want %v %s", item.Name, got, *sku.CapacityUnit, want[item.Name], units[item.Name])
		}
	}
}

func TestModelRatesKeepTheRuleKeysTheSDKDrops(t *testing.T) {
	body := `{"value":[{"name":"Microsoft-Decision-1","format":"Microsoft","version":"1","skus":[{"name":"GlobalStandard","rateLimits":[{"key":"request","count":1,"renewalPeriod":60}]}]}]}`
	rates := map[string][]modelRate{}
	if err := readModelRates(&http.Response{Body: io.NopCloser(strings.NewReader(body))}, rates); err != nil {
		t.Fatal(err)
	}
	got := rates[modelRateKey("microsoft-decision-1", "MICROSOFT", "1", "globalstandard")]
	if len(got) != 1 || got[0] != (modelRate{Key: "request", Count: 1, RenewalPeriod: 60}) {
		t.Fatalf("rates = %#v", rates)
	}
}
