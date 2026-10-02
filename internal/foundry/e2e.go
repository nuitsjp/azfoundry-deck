//go:build e2e

package foundry

import "context"

type fixedSource struct{}

func NewFixedSource() Source { return fixedSource{} }

func (fixedSource) Discover(ctx context.Context, discovered func(Foundry)) ([]Foundry, error) {
	foundries := []Foundry{
		{ID: "/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast", Name: "contoso-foundry-production-japaneast", SubscriptionName: "Contoso AI Production Subscription", ResourceGroupName: "rg-ai-production-japaneast"},
		{ID: "/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development", Name: "contoso-foundry-development", SubscriptionName: "Contoso Development", ResourceGroupName: "rg-ai-development"},
		{ID: "/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research", Name: "contoso-foundry-research", SubscriptionName: "Contoso Research", ResourceGroupName: "rg-ai-research"},
	}
	for _, foundry := range foundries {
		discovered(foundry)
	}
	return foundries, ctx.Err()
}

func (fixedSource) Deployments(ctx context.Context, foundry Foundry) ([]Deployment, error) {
	return []Deployment{
		{ID: foundry.ID + "/deployments/chat-production", DeploymentName: "chat-production", ModelName: "gpt-4.1", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/chat-mini", DeploymentName: "chat-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/embeddings", DeploymentName: "embeddings", ModelName: "text-embedding-3-large", Version: "1"},
	}, ctx.Err()
}
