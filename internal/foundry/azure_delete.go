//go:build !e2e

package foundry

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

func (s *azureSource) DeleteDeployment(ctx context.Context, foundry Foundry, deployment Deployment) error {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	client, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return fmt.Errorf("create deployments client: %w", err)
	}
	poller, err := client.BeginDelete(ctx, foundry.ResourceGroupName, foundry.Name, deployment.DeploymentName, nil)
	if err != nil {
		return fmt.Errorf("begin delete deployment %s: %w", deployment.DeploymentName, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for delete deployment %s: %w", deployment.DeploymentName, err)
	}
	return nil
}
