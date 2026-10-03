//go:build e2e

package foundry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Opt-in gates hold the external source until the E2E test releases each stage.
// Ordinary E2E tests and screen review retain immediate fixed responses.
func waitForRelease(ctx context.Context, stage string) error {
	if os.Getenv("AZFOUNDRYDECK_E2E_HOLD_FOUNDRY") != "1" {
		return ctx.Err()
	}
	path := filepath.Join(os.Getenv("WAILS_DATA_DIR"), "e2e-foundry-"+stage+"-release")
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type fixedSource struct{}

func NewFixedSource() Source { return fixedSource{} }

func (fixedSource) Foundries(ctx context.Context) ([]Foundry, error) {
	if err := waitForRelease(ctx, "discovery"); err != nil {
		return nil, err
	}
	// AZFOUNDRYDECK_E2E_FOUNDRIES=none: no Foundry is readable.
	if os.Getenv("AZFOUNDRYDECK_E2E_FOUNDRIES") == "none" {
		return []Foundry{}, ctx.Err()
	}
	return []Foundry{
		{ID: "/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast", Name: "contoso-foundry-production-japaneast", SubscriptionName: "Contoso AI Production Subscription", ResourceGroupName: "rg-ai-production-japaneast"},
		{ID: "/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development", Name: "contoso-foundry-development", SubscriptionName: "Contoso Development", ResourceGroupName: "rg-ai-development"},
		{ID: "/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research", Name: "contoso-foundry-research", SubscriptionName: "Contoso Research", ResourceGroupName: "rg-ai-research"},
	}, ctx.Err()
}

func (fixedSource) Deployments(ctx context.Context, foundry Foundry, report func(int)) ([]Deployment, error) {
	deployments := []Deployment{
		{ID: foundry.ID + "/deployments/chat-production", DeploymentName: "chat-production", ModelName: "gpt-4.1", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/chat-mini", DeploymentName: "chat-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/embeddings", DeploymentName: "embeddings", ModelName: "text-embedding-3-large", Version: "1"},
	}
	switch foundry.Name {
	case "contoso-foundry-development":
		deployments = []Deployment{
			{ID: foundry.ID + "/deployments/development-chat", DeploymentName: "development-chat", ModelName: "gpt-4.1", Version: "2025-04-14"},
			{ID: foundry.ID + "/deployments/development-mini", DeploymentName: "development-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
			{ID: foundry.ID + "/deployments/development-embedding", DeploymentName: "development-embedding", ModelName: "text-embedding-3-large", Version: "1"},
		}
	case "contoso-foundry-research":
		deployments = []Deployment{
			{ID: foundry.ID + "/deployments/research-chat", DeploymentName: "research-chat", ModelName: "gpt-4.1", Version: "2025-04-14"},
		}
	}
	if err := waitForRelease(ctx, "models"); err != nil {
		return nil, err
	}
	report(len(deployments))
	return deployments, ctx.Err()
}

func (fixedSource) DeploymentDetail(ctx context.Context, foundry Foundry, deployment Deployment) (DeploymentDetail, error) {
	if err := waitForRelease(ctx, "detail"); err != nil {
		return DeploymentDetail{}, err
	}
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "detail" {
		return DeploymentDetail{}, fmt.Errorf("simulated deployment detail retrieval failure")
	}
	sku := "GlobalStandard"
	capacity := int32(50)
	state := "Succeeded"
	policy := "OnceNewDefaultVersionAvailable"
	switch deployment.ModelName {
	case "gpt-4.1-mini":
		capacity = 100
	case "text-embedding-3-large":
		sku = "Standard"
		capacity = 20
		policy = "NoAutoUpgrade"
	}
	return DeploymentDetail{
		ID: deployment.ID, DeploymentName: deployment.DeploymentName,
		ModelName: deployment.ModelName, Version: deployment.Version,
		SKUName: &sku, Capacity: &capacity, ProvisioningState: &state,
		VersionUpgradePolicy: &policy,
	}, ctx.Err()
}
