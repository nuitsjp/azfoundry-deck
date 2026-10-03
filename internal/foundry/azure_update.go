//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"math"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func (s *azureSource) DeploymentSettings(ctx context.Context, foundry Foundry, deployment Deployment) (DeploymentSettings, error) {
	detail, err := s.DeploymentDetail(ctx, foundry, deployment)
	if err != nil {
		return DeploymentSettings{}, err
	}
	versions, err := s.modelVersions(ctx, foundry, deployment, detail.ModelName, detail.Version)
	if err != nil {
		return DeploymentSettings{}, err
	}
	skuName := ""
	if detail.SKUName != nil {
		skuName = *detail.SKUName
	}
	policy := ""
	if detail.VersionUpgradePolicy != nil {
		policy = *detail.VersionUpgradePolicy
	}
	option := "Standard"
	var capacity, capacityMaximum *float64
	var capacityUnit *string
	if detail.CapacityUnit != nil && *detail.CapacityUnit != "" {
		capacity = detail.Capacity
		capacityMaximum = detail.CapacityMaximum
		capacityUnit = detail.CapacityUnit
	} else {
		option = "Pay-as-you-go"
	}
	return DeploymentSettings{
		DeploymentID:    detail.ID,
		DeploymentName:  detail.DeploymentName,
		ModelName:       detail.ModelName,
		SKUName:         skuName,
		Option:          option,
		Version:         detail.Version,
		Versions:        versions,
		Capacity:        capacity,
		CapacityMaximum: capacityMaximum,
		CapacityUnit:    capacityUnit,
		UpgradePolicy:   policy,
	}, nil
}

func (s *azureSource) modelVersions(ctx context.Context, foundry Foundry, deployment Deployment, modelName, current string) ([]string, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return nil, fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	deployments, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create deployments client: %w", err)
	}
	response, err := deployments.Get(ctx, foundry.ResourceGroupName, foundry.Name, deployment.DeploymentName, nil)
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	if response.Properties == nil || response.Properties.Model == nil || response.Properties.Model.Name == nil {
		return nil, fmt.Errorf("deployment response lacks model")
	}
	format := ""
	if response.Properties.Model.Format != nil {
		format = *response.Properties.Model.Format
	}
	accounts, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create accounts client: %w", err)
	}
	versions := make([]string, 0)
	seen := map[string]struct{}{}
	pager := accounts.NewListModelsPager(foundry.ResourceGroupName, foundry.Name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list model versions: %w", err)
		}
		for _, model := range page.Value {
			if model == nil || model.Version == nil || *model.Version == "" || !sameValue(model.Name, &modelName) {
				continue
			}
			if format != "" && !sameValue(model.Format, &format) {
				continue
			}
			if _, ok := seen[*model.Version]; ok {
				continue
			}
			seen[*model.Version] = struct{}{}
			versions = append(versions, *model.Version)
		}
	}
	if current != "" {
		if _, ok := seen[current]; !ok {
			versions = append([]string{current}, versions...)
		}
	}
	return versions, nil
}

func (s *azureSource) UpdateDeployment(ctx context.Context, foundry Foundry, deployment Deployment, spec DeploymentUpdateSpec) error {
	if spec.Version == "" {
		return fmt.Errorf("deployment version is empty")
	}
	if spec.UpgradePolicy == "" {
		return fmt.Errorf("deployment upgrade policy is empty")
	}
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	client, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return fmt.Errorf("create deployments client: %w", err)
	}
	response, err := client.Get(ctx, foundry.ResourceGroupName, foundry.Name, deployment.DeploymentName, nil)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}
	current := response.Deployment
	if current.Properties == nil || current.Properties.Model == nil || current.Properties.Model.Name == nil || current.SKU == nil || current.SKU.Name == nil {
		return fmt.Errorf("deployment response lacks model or SKU")
	}
	sku := &armcognitiveservices.SKU{
		Name:     current.SKU.Name,
		Capacity: current.SKU.Capacity,
		Family:   current.SKU.Family,
		Size:     current.SKU.Size,
		Tier:     current.SKU.Tier,
	}
	if spec.Capacity != nil {
		capacity, err := skuCapacity(*current.SKU.Name, current, *spec.Capacity)
		if err != nil {
			return err
		}
		sku.Capacity = &capacity
	}
	model := current.Properties.Model
	updated := armcognitiveservices.Deployment{
		SKU:  sku,
		Tags: current.Tags,
		Properties: &armcognitiveservices.DeploymentProperties{
			Model: &armcognitiveservices.DeploymentModel{
				Format:        model.Format,
				Name:          model.Name,
				Publisher:     model.Publisher,
				Source:        model.Source,
				SourceAccount: model.SourceAccount,
				Version:       to.Ptr(spec.Version),
			},
			VersionUpgradeOption:    to.Ptr(armcognitiveservices.DeploymentModelVersionUpgradeOption(spec.UpgradePolicy)),
			RaiPolicyName:           current.Properties.RaiPolicyName,
			SpilloverDeploymentName: current.Properties.SpilloverDeploymentName,
			ParentDeploymentName:    current.Properties.ParentDeploymentName,
		},
	}
	poller, err := client.BeginCreateOrUpdate(ctx, foundry.ResourceGroupName, foundry.Name, deployment.DeploymentName, updated, nil)
	if err != nil {
		return fmt.Errorf("begin update deployment %s: %w", deployment.DeploymentName, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for deployment update %s: %w", deployment.DeploymentName, err)
	}
	return nil
}

// skuCapacity converts the capacity shown in the detail into the SKU capacity integer.
// The detail shows SKU capacity multiplied by the per-unit rate from the deployment's rate limits.
func skuCapacity(name string, deployment armcognitiveservices.Deployment, displayed int64) (int32, error) {
	_, multiplier := capacityUnit(name, deployment)
	if multiplier == 0 {
		return 0, fmt.Errorf("deployment capacity unit is unknown")
	}
	units := math.Round(float64(displayed) / multiplier)
	if units < 1 || units > math.MaxInt32 {
		return 0, fmt.Errorf("deployment capacity %d is outside the SKU range", displayed)
	}
	return int32(units), nil
}
