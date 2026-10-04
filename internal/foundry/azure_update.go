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
