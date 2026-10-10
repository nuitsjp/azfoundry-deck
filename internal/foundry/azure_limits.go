//go:build !e2e

package foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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
	accounts *armcognitiveservices.AccountsClient
	foundry  Foundry
	usages   *armcognitiveservices.UsagesClient
	location string
	models   []*armcognitiveservices.AccountModel
	// rates holds each model SKU's rate rules by modelRateKey; the SDK drops their keys.
	rates map[string][]modelRate
	quota []*armcognitiveservices.Usage
}

type modelRate struct {
	Key           string  `json:"key"`
	Count         float64 `json:"count"`
	RenewalPeriod float64 `json:"renewalPeriod"`
}

func modelRateKey(name, format, version, sku string) string {
	return strings.ToLower(name + "|" + format + "|" + version + "|" + sku)
}

// readModelRates reads the rate rule keys from the raw model list page.
func readModelRates(response *http.Response, rates map[string][]modelRate) error {
	body, err := runtime.Payload(response)
	if err != nil {
		return err
	}
	var page struct {
		Value []struct {
			Name, Format, Version string
			SKUs                  []struct {
				Name       string
				RateLimits []modelRate `json:"rateLimits"`
			} `json:"skus"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return err
	}
	for _, model := range page.Value {
		for _, sku := range model.SKUs {
			rates[modelRateKey(model.Name, model.Format, model.Version, sku.Name)] = sku.RateLimits
		}
	}
	return nil
}

// CapacityLimits fetches the Foundry's model definitions, and in parallel its
// region and that region's shared quota.
func (s *azureSource) CapacityLimits(ctx context.Context, foundry Foundry, publishModels func(CapacityLimits)) (CapacityLimits, error) {
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
	base := azureLimits{accounts: accounts, foundry: foundry, usages: usages}
	var models []*armcognitiveservices.AccountModel
	rates := map[string][]modelRate{}
	var quotaLimits CapacityLimits
	var wg sync.WaitGroup
	var modelsErr, quotaErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		pager := accounts.NewListModelsPager(foundry.ResourceGroupName, foundry.Name, nil)
		for pager.More() {
			var response *http.Response
			page, err := pager.NextPage(policy.WithCaptureResponse(ctx, &response))
			if err == nil {
				err = readModelRates(response, rates)
			}
			if err != nil {
				modelsErr = fmt.Errorf("get model capacity definition: %w", err)
				return
			}
			models = append(models, page.Value...)
		}
		published := base
		published.models, published.rates = models, rates
		publishModels(&published)
	}()
	go func() {
		defer wg.Done()
		quotaLimits, quotaErr = base.RefreshQuota(ctx)
	}()
	wg.Wait()
	if modelsErr != nil {
		return nil, modelsErr
	}
	limits := *quotaLimits.(*azureLimits)
	limits.models, limits.rates = models, rates
	return &limits, quotaErr
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
	refreshed := *l
	refreshed.quota = nil
	if refreshed.location == "" {
		account, err := refreshed.accounts.Get(ctx, refreshed.foundry.ResourceGroupName, refreshed.foundry.Name, nil)
		if err != nil {
			return &refreshed, fmt.Errorf("get Foundry location: %w", err)
		}
		if account.Location == nil {
			return &refreshed, fmt.Errorf("Foundry response lacks location")
		}
		refreshed.location = *account.Location
	}
	var err error
	refreshed.quota, err = listQuota(ctx, refreshed.usages, refreshed.location)
	return &refreshed, err
}

func (l *azureLimits) Maximum(d Deployment) *float64 {
	sku := l.sku(d)
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

// CapacityStep returns the smallest capacity and its increment in the deployment's unit.
// A SKU without a configured minimum and step changes by one SKU capacity unit.
func (l *azureLimits) CapacityStep(d Deployment) (float64, float64) {
	minimum, step := d.multiplier, d.multiplier
	if sku := l.sku(d); sku != nil && sku.Capacity != nil && sku.Capacity.Step != nil && *sku.Capacity.Step > 0 && sku.Capacity.Minimum != nil {
		minimum, step = float64(*sku.Capacity.Minimum)*d.multiplier, float64(*sku.Capacity.Step)*d.multiplier
	}
	return minimum, step
}

func (l *azureLimits) sku(d Deployment) *armcognitiveservices.ModelSKU {
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
	return sku
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
