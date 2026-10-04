//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

// deploymentFrom reads the listed deployment's fields, including those the capacity limit needs.
func deploymentFrom(deployment *armcognitiveservices.Deployment) Deployment {
	properties := deployment.Properties
	d := Deployment{
		ID: *deployment.ID, DeploymentName: *deployment.Name,
		ModelName: *properties.Model.Name,
	}
	if properties.Model.Version != nil {
		d.Version = *properties.Model.Version
	}
	if properties.Model.Format != nil {
		d.format = *properties.Model.Format
	}
	if properties.ProvisioningState != nil {
		value := string(*properties.ProvisioningState)
		d.ProvisioningState = &value
	}
	if properties.VersionUpgradeOption != nil {
		value := string(*properties.VersionUpgradeOption)
		d.VersionUpgradePolicy = &value
	}
	if deployment.SKU == nil {
		return d
	}
	d.SKUName = deployment.SKU.Name
	if deployment.SKU.Name == nil || deployment.SKU.Capacity == nil {
		return d
	}
	if *deployment.SKU.Capacity == 0 {
		zero := float64(0)
		d.Capacity = &zero
	}
	unit, multiplier := capacityUnit(*deployment.SKU.Name, *deployment)
	if unit != "" {
		d.CapacityUnit = &unit
	}
	if multiplier == 0 {
		return d
	}
	d.skuCapacity = float64(*deployment.SKU.Capacity)
	d.multiplier = multiplier
	capacity := d.skuCapacity * multiplier
	d.Capacity = &capacity
	return d
}

type azureLimits struct {
	usages   *armcognitiveservices.UsagesClient
	location string
	models   []*armcognitiveservices.AccountModel
	quota    []*armcognitiveservices.Usage
}

// CapacityLimits fetches the Foundry's model definitions, and in parallel its
// region and that region's shared quota.
func (s *azureSource) CapacityLimits(ctx context.Context, foundry Foundry) (CapacityLimits, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return nil, err
	}
	accounts, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, err
	}
	usages, err := armcognitiveservices.NewUsagesClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, err
	}
	limits := &azureLimits{usages: usages}
	var wg sync.WaitGroup
	var modelsErr, quotaErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		pager := accounts.NewListModelsPager(foundry.ResourceGroupName, foundry.Name, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				modelsErr = fmt.Errorf("get model capacity definition: %w", err)
				return
			}
			limits.models = append(limits.models, page.Value...)
		}
	}()
	go func() {
		defer wg.Done()
		account, err := accounts.Get(ctx, foundry.ResourceGroupName, foundry.Name, nil)
		if err != nil {
			quotaErr = fmt.Errorf("get Foundry location: %w", err)
			return
		}
		if account.Location == nil {
			quotaErr = fmt.Errorf("Foundry response lacks location")
			return
		}
		limits.location = *account.Location
		limits.quota, quotaErr = listQuota(ctx, usages, limits.location)
	}()
	wg.Wait()
	if modelsErr != nil {
		return nil, modelsErr
	}
	if quotaErr != nil {
		return nil, quotaErr
	}
	return limits, nil
}

func listQuota(ctx context.Context, usages *armcognitiveservices.UsagesClient, location string) ([]*armcognitiveservices.Usage, error) {
	var quota []*armcognitiveservices.Usage
	pager := usages.NewListPager(location, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("get shared quota: %w", err)
		}
		quota = append(quota, page.Value...)
	}
	return quota, nil
}

func (l *azureLimits) RefreshQuota(ctx context.Context) (CapacityLimits, error) {
	quota, err := listQuota(ctx, l.usages, l.location)
	if err != nil {
		return nil, err
	}
	return &azureLimits{usages: l.usages, location: l.location, models: l.models, quota: quota}, nil
}

func (l *azureLimits) Maximum(d Deployment) *float64 {
	if d.multiplier == 0 || d.SKUName == nil {
		return nil
	}
	var sku *armcognitiveservices.ModelSKU
	for _, model := range l.models {
		if model == nil || !sameValue(model.Name, &d.ModelName) || !sameValue(model.Format, &d.format) || !sameValue(model.Version, &d.Version) {
			continue
		}
		for _, candidate := range model.SKUs {
			if candidate != nil && sameValue(candidate.Name, d.SKUName) {
				sku = candidate
				break
			}
		}
	}
	if sku == nil || sku.UsageName == nil || sku.Capacity == nil || sku.Capacity.Maximum == nil {
		return nil
	}
	for _, usage := range l.quota {
		if usage == nil || usage.Name == nil || !sameValue(usage.Name.Value, sku.UsageName) || usage.Limit == nil || usage.CurrentValue == nil || usage.Unit == nil || *usage.Unit != armcognitiveservices.UnitTypeCount {
			continue
		}
		maximum := math.Min(d.skuCapacity+math.Max(0, *usage.Limit-*usage.CurrentValue), float64(*sku.Capacity.Maximum))
		maximum = allowedCapacity(maximum, sku.Capacity) * d.multiplier
		return &maximum
	}
	return nil
}

func (l *azureLimits) Versions(d Deployment) []string {
	versions := make([]string, 0)
	seen := map[string]struct{}{}
	for _, model := range l.models {
		if model == nil || model.Version == nil || *model.Version == "" || !sameValue(model.Name, &d.ModelName) {
			continue
		}
		if d.format != "" && !sameValue(model.Format, &d.format) {
			continue
		}
		if _, ok := seen[*model.Version]; ok {
			continue
		}
		seen[*model.Version] = struct{}{}
		versions = append(versions, *model.Version)
	}
	if d.Version != "" {
		if _, ok := seen[d.Version]; !ok {
			versions = append([]string{d.Version}, versions...)
		}
	}
	return versions
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
