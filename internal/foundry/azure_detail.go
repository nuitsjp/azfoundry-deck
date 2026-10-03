//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func (s *azureSource) DeploymentDetail(ctx context.Context, foundry Foundry, selected Deployment) (DeploymentDetail, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return DeploymentDetail{}, err
	}
	deployments, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return DeploymentDetail{}, err
	}
	response, err := deployments.Get(ctx, foundry.ResourceGroupName, foundry.Name, selected.DeploymentName, nil)
	if err != nil {
		return DeploymentDetail{}, fmt.Errorf("get deployment: %w", err)
	}
	deployment := response.Deployment
	if deployment.ID == nil || deployment.Name == nil || deployment.Properties == nil || deployment.Properties.Model == nil || deployment.Properties.Model.Name == nil {
		return DeploymentDetail{}, fmt.Errorf("deployment response lacks identity or model")
	}
	properties := deployment.Properties
	detail := DeploymentDetail{ID: *deployment.ID, DeploymentName: *deployment.Name, ModelName: *properties.Model.Name}
	if properties.Model.Version != nil {
		detail.Version = *properties.Model.Version
	}
	if properties.ProvisioningState != nil {
		value := string(*properties.ProvisioningState)
		detail.ProvisioningState = &value
	}
	if properties.VersionUpgradeOption != nil {
		value := string(*properties.VersionUpgradeOption)
		detail.VersionUpgradePolicy = &value
	}
	if deployment.SKU == nil {
		return detail, nil
	}
	detail.SKUName = deployment.SKU.Name
	if deployment.SKU.Name == nil || deployment.SKU.Capacity == nil {
		return detail, nil
	}
	if *deployment.SKU.Capacity == 0 {
		zero := float64(0)
		detail.Capacity = &zero
	}
	unit, multiplier := capacityUnit(*deployment.SKU.Name, deployment)
	if unit != "" {
		detail.CapacityUnit = &unit
	}
	if multiplier == 0 {
		return detail, nil
	}
	current := float64(*deployment.SKU.Capacity)
	capacity := current * multiplier
	detail.Capacity, detail.CapacityUnit = &capacity, &unit
	accounts, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return DeploymentDetail{}, err
	}
	account, err := accounts.Get(ctx, foundry.ResourceGroupName, foundry.Name, nil)
	if err != nil {
		return DeploymentDetail{}, fmt.Errorf("get Foundry location: %w", err)
	}
	if account.Location == nil {
		return DeploymentDetail{}, fmt.Errorf("Foundry response lacks location")
	}
	var sku *armcognitiveservices.ModelSKU
	models := accounts.NewListModelsPager(foundry.ResourceGroupName, foundry.Name, nil)
	for models.More() {
		page, err := models.NextPage(ctx)
		if err != nil {
			return DeploymentDetail{}, fmt.Errorf("get model capacity definition: %w", err)
		}
		for _, model := range page.Value {
			if model == nil || !sameValue(model.Name, properties.Model.Name) || !sameValue(model.Format, properties.Model.Format) || !sameValue(model.Version, properties.Model.Version) {
				continue
			}
			for _, candidate := range model.SKUs {
				if candidate != nil && sameValue(candidate.Name, deployment.SKU.Name) {
					sku = candidate
					break
				}
			}
		}
	}
	if sku == nil {
		return detail, nil
	}
	if sku.UsageName == nil || sku.Capacity == nil || sku.Capacity.Maximum == nil {
		return detail, nil
	}
	usages, err := armcognitiveservices.NewUsagesClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return DeploymentDetail{}, err
	}
	pager := usages.NewListPager(*account.Location, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return DeploymentDetail{}, fmt.Errorf("get shared quota: %w", err)
		}
		for _, usage := range page.Value {
			if usage == nil || usage.Name == nil || !sameValue(usage.Name.Value, sku.UsageName) || usage.Limit == nil || usage.CurrentValue == nil || usage.Unit == nil || *usage.Unit != armcognitiveservices.UnitTypeCount {
				continue
			}
			maximum := math.Min(current+math.Max(0, *usage.Limit-*usage.CurrentValue), float64(*sku.Capacity.Maximum))
			maximum = allowedCapacity(maximum, sku.Capacity) * multiplier
			detail.CapacityMaximum = &maximum
			return detail, nil
		}
	}
	return detail, nil
}

func sameValue(a, b *string) bool { return a != nil && b != nil && strings.EqualFold(*a, *b) }

func capacityUnit(name string, deployment armcognitiveservices.Deployment) (string, float64) {
	switch name {
	case "ProvisionedManaged", "GlobalProvisionedManaged", "DataZoneProvisionedManaged":
		return "PTU", 1
	case "Standard", "GlobalStandard", "DataZoneStandard":
		// Token-based models use TPM; request-only models (such as images) use RPM.
		// Read deployment rules because this SDK drops the key on ModelSKU rate limits.
		for _, key := range []string{"token", "request"} {
			for _, rule := range deployment.Properties.RateLimits {
				if rule == nil || rule.Key == nil || *rule.Key != key || rule.Count == nil || rule.RenewalPeriod == nil || *rule.RenewalPeriod <= 0 {
					continue
				}
				unit := "TPM"
				if key == "request" {
					unit = "RPM"
				}
				if *deployment.SKU.Capacity <= 0 {
					return unit, 0
				}
				return unit, float64(*rule.Count) * 60 / float64(*rule.RenewalPeriod) / float64(*deployment.SKU.Capacity)
			}
		}
	}
	return "", 0
}

func allowedCapacity(maximum float64, config *armcognitiveservices.CapacityConfig) float64 {
	if len(config.AllowedValues) > 0 {
		allowed := float64(0)
		for _, value := range config.AllowedValues {
			if value != nil && float64(*value) <= maximum {
				allowed = math.Max(allowed, float64(*value))
			}
		}
		return allowed
	}
	if config.Step != nil && *config.Step > 0 && config.Minimum != nil {
		minimum := float64(*config.Minimum)
		if maximum < minimum {
			return 0
		}
		return minimum + math.Floor((maximum-minimum)/float64(*config.Step))*float64(*config.Step)
	}
	return math.Floor(maximum)
}
