//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"log/slog"
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

					var skuCap *int64
					if option == "Standard" && sku.UsageName != nil && sku.Capacity != nil && sku.Capacity.Maximum != nil {
						if available, ok := usageLimits[strings.ToLower(*sku.UsageName)]; ok {
							maximum := math.Min(available, float64(*sku.Capacity.Maximum))
							// Keep the creation API's existing 1,000-unit conversion.
							capacity := int64(allowedCapacity(maximum, sku.Capacity)) * 1000
							skuCap = &capacity
						}
					}
					skus = append(skus, ModelSKUItem{
						Name:        sName,
						MaxCapacity: skuCap,
					})
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
		if spec.Capacity != nil {
			// Cognitive Services SKU capacity is in increments of 1k TPM
			skuCap := int32(*spec.Capacity / 1000)
			if skuCap < 1 {
				skuCap = 1
			}
			sku.Capacity = &skuCap
			slog.Info("create_deployment_sku_capacity", "spec.Capacity", *spec.Capacity, "skuCap", skuCap)
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
