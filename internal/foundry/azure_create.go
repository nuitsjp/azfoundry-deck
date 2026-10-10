//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

// Catalog derives the creation choices from the held definitions and quota.
func (l *azureLimits) Catalog() []ModelCatalogItem {
	usageLimits := make(map[string]float64)
	for _, usage := range l.quota {
		if usage == nil || usage.Name == nil || usage.Name.Value == nil || usage.Limit == nil || usage.CurrentValue == nil || usage.Unit == nil || *usage.Unit != armcognitiveservices.UnitTypeCount {
			continue
		}
		usageLimits[strings.ToLower(*usage.Name.Value)] = math.Max(0, *usage.Limit-*usage.CurrentValue)
	}
	modelMap := make(map[string]*ModelCatalogItem)
	for _, m := range l.models {
		if m == nil || m.Name == nil {
			continue
		}
		name := *m.Name
		item, exists := modelMap[name]
		if !exists {
			publisher := "OpenAI"
			if m.Publisher != nil && *m.Publisher != "" {
				publisher = *m.Publisher
			}
			option := "Standard"
			if strings.Contains(strings.ToLower(name), "claude") || strings.Contains(strings.ToLower(name), "llama") || strings.Contains(strings.ToLower(name), "mistral") {
				option = "Pay-as-you-go"
			}

			tasks := []string{"Chat"}
			if strings.Contains(strings.ToLower(name), "embedding") {
				tasks = []string{"Embeddings"}
			} else if strings.Contains(strings.ToLower(name), "vision") || strings.Contains(strings.ToLower(name), "4o") {
				tasks = []string{"Chat", "Multimodal"}
			}

			skus := make([]ModelSKUItem, 0, len(m.SKUs))
			skuSeen := make(map[string]bool)

			for _, sku := range m.SKUs {
				if sku != nil && sku.Name != nil {
					sName := *sku.Name
					if skuSeen[sName] {
						continue
					}
					skuSeen[sName] = true

					item := ModelSKUItem{Name: sName}
					unit, multiplier := l.catalogUnit(m, sName)
					if option == "Standard" && unit != "" && sku.Capacity != nil {
						item.CapacityUnit = &unit
						minimum, step := int64(1), int64(1)
						if sku.Capacity.Step != nil && *sku.Capacity.Step > 0 && sku.Capacity.Minimum != nil {
							minimum, step = int64(*sku.Capacity.Minimum), int64(*sku.Capacity.Step)
						}
						minimum, step = minimum*multiplier, step*multiplier
						item.MinCapacity, item.CapacityStep, item.CapacityPerUnit = &minimum, &step, &multiplier
						if available, ok := usageLimits[strings.ToLower(ptrValue(sku.UsageName))]; ok && sku.Capacity.Maximum != nil {
							maximum := math.Min(available, float64(*sku.Capacity.Maximum))
							capacity := int64(allowedCapacity(maximum, sku.Capacity)) * multiplier
							item.MaxCapacity = &capacity
						}
					}
					skus = append(skus, item)
				}
			}

			// Sort so GlobalStandard comes first if present
			sort.SliceStable(skus, func(i, j int) bool {
				return strings.EqualFold(skus[i].Name, "GlobalStandard")
			})

			var maxCap *int64
			if len(skus) > 0 && skus[0].MaxCapacity != nil {
				maxCap = skus[0].MaxCapacity
			}

			item = &ModelCatalogItem{
				Name:        name,
				Publisher:   publisher,
				Option:      option,
				Tasks:       tasks,
				Sub:         "128k context",
				MaxCapacity: maxCap,
				SKUs:        skus,
				Versions:    []string{},
			}
			modelMap[name] = item
		}

		if m.Version != nil && *m.Version != "" {
			v := *m.Version
			if !slicesContains(item.Versions, v) {
				item.Versions = append(item.Versions, v)
			}
		}
	}

	result := make([]ModelCatalogItem, 0, len(modelMap))
	for _, item := range modelMap {
		result = append(result, *item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// catalogUnit returns the SKU's capacity unit and the amount of that unit per SKU
// capacity. Token rates take precedence over request rates, as for deployments.
func (l *azureLimits) catalogUnit(model *armcognitiveservices.AccountModel, sku string) (string, int64) {
	switch sku {
	case "ProvisionedManaged", "GlobalProvisionedManaged", "DataZoneProvisionedManaged":
		return "PTU", 1
	}
	rates := l.rates[modelRateKey(ptrValue(model.Name), ptrValue(model.Format), ptrValue(model.Version), sku)]
	for _, key := range []string{"token", "request"} {
		for _, rate := range rates {
			if rate.Key != key || rate.RenewalPeriod <= 0 {
				continue
			}
			multiplier := int64(rate.Count * 60 / rate.RenewalPeriod)
			if multiplier <= 0 {
				return "", 0
			}
			if key == "request" {
				return "RPM", multiplier
			}
			return "TPM", multiplier
		}
	}
	return "", 0
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *azureSource) CreateDeployment(ctx context.Context, foundry Foundry, spec DeploymentCreateSpec) error {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	deployments, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return fmt.Errorf("create deployments client: %w", err)
	}

	deployment := armcognitiveservices.Deployment{
		Properties: &armcognitiveservices.DeploymentProperties{
			Model: &armcognitiveservices.DeploymentModel{
				Format:  to.Ptr("OpenAI"),
				Name:    to.Ptr(spec.ModelName),
				Version: to.Ptr(spec.Version),
			},
		},
	}

	if spec.UpgradePolicy != "" {
		deployment.Properties.VersionUpgradeOption = to.Ptr(armcognitiveservices.DeploymentModelVersionUpgradeOption(spec.UpgradePolicy))
	}

	if spec.SKU != "" {
		sku := &armcognitiveservices.SKU{
			Name: to.Ptr(spec.SKU),
		}
		if spec.SKUCapacity != nil {
			sku.Capacity = to.Ptr(int32(*spec.SKUCapacity))
		}
		deployment.SKU = sku
	}

	poller, err := deployments.BeginCreateOrUpdate(ctx, foundry.ResourceGroupName, foundry.Name, spec.DeploymentName, deployment, nil)
	if err != nil {
		return fmt.Errorf("begin create deployment %s: %w", spec.DeploymentName, err)
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for deployment %s: %w", spec.DeploymentName, err)
	}

	return nil
}

func slicesContains(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}
