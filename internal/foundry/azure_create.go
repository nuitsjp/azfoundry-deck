//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func (s *azureSource) ListModels(ctx context.Context, foundry Foundry) ([]ModelCatalogItem, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return nil, fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	accounts, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create accounts client: %w", err)
	}

	// Fetch regional quota usages to constrain capacity slider to actual available quota
	usageLimits := make(map[string]int64)
	account, err := accounts.Get(ctx, foundry.ResourceGroupName, foundry.Name, nil)
	if err == nil && account.Location != nil {
		usagesClient, err := armcognitiveservices.NewUsagesClient(id.SubscriptionID, s.credential, nil)
		if err == nil {
			pager := usagesClient.NewListPager(*account.Location, nil)
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					break
				}
				for _, u := range page.Value {
					if u != nil && u.Name != nil && u.Name.Value != nil && u.Limit != nil && u.CurrentValue != nil {
						avail := int64(*u.Limit - *u.CurrentValue)
						if avail < 0 {
							avail = 0
						}
						// Azure quota count is in increments of 1k TPM
						usageLimits[strings.ToLower(*u.Name.Value)] = avail * 1000
					}
				}
			}
		}
	}

	pager := accounts.NewListModelsPager(foundry.ResourceGroupName, foundry.Name, nil)
	modelMap := make(map[string]*ModelCatalogItem)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list models from Azure: %w", err)
		}
		for _, m := range page.Value {
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
						if option == "Standard" {
							c := int64(160000)
							found := false
							if sku.UsageName != nil {
								if avail, ok := usageLimits[strings.ToLower(*sku.UsageName)]; ok {
									c = avail
									found = true
								}
							}
							if !found && sku.Capacity != nil && sku.Capacity.Maximum != nil {
								maxVal := int64(*sku.Capacity.Maximum)
								if maxVal > 10000000 {
									maxVal = 10000000
								}
								c = maxVal
							}
							if sku.Capacity != nil && sku.Capacity.Maximum != nil {
								maxVal := int64(*sku.Capacity.Maximum)
								if maxVal > 10000000 {
									maxVal = 10000000
								}
								if c > maxVal {
									c = maxVal
								}
							}
							if c < 1000 {
								c = 1000
							}
							skuCap = &c
						}
						skus = append(skus, ModelSKUItem{
							Name:        sName,
							MaxCapacity: skuCap,
						})
					}
				}

				if len(skus) == 0 {
					defName := "GlobalStandard"
					if option == "Pay-as-you-go" {
						defName = "GlobalProvisioned"
					}
					defCap := int64(160000)
					skus = append(skus, ModelSKUItem{
						Name:        defName,
						MaxCapacity: &defCap,
					})
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
	}

	result := make([]ModelCatalogItem, 0, len(modelMap))
	for _, item := range modelMap {
		if len(item.Versions) == 0 {
			item.Versions = []string{"1"}
		}
		result = append(result, *item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
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
