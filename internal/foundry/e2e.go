//go:build e2e

package foundry

import (
	"context"
	"strings"
)

type fixedSource struct{}

func NewFixedSource() Source { return fixedSource{} }

func (fixedSource) Discover(ctx context.Context, discovered func(Foundry), report func(DiscoveryProgress)) ([]Foundry, error) {
	foundries := []Foundry{
		{ID: "/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast", Name: "contoso-foundry-production-japaneast", SubscriptionName: "Contoso AI Production Subscription", ResourceGroupName: "rg-ai-production-japaneast"},
		{ID: "/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development", Name: "contoso-foundry-development", SubscriptionName: "Contoso Development", ResourceGroupName: "rg-ai-development"},
		{ID: "/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research", Name: "contoso-foundry-research", SubscriptionName: "Contoso Research", ResourceGroupName: "rg-ai-research"},
	}
	progress := DiscoveryProgress{SubscriptionSearch: "searching", Subscriptions: make([]SubscriptionProgress, len(foundries))}
	for i, foundry := range foundries {
		progress.Subscriptions[i] = SubscriptionProgress{ID: strings.Split(foundry.ID, "/")[2], Name: foundry.SubscriptionName, Phase: "waiting"}
	}
	publish := func() {
		snapshot := progress
		snapshot.Subscriptions = append([]SubscriptionProgress{}, progress.Subscriptions...)
		report(snapshot)
	}
	publish()
	progress.SubscriptionSearch = "completed"
	publish()
	for i, foundry := range foundries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress.Subscriptions[i].Phase = "running"
		publish()
		progress.Subscriptions[i].FoundryCount = 1
		publish()
		discovered(foundry)
		progress.Subscriptions[i].Phase = "completed"
		publish()
	}
	return foundries, ctx.Err()
}

func (fixedSource) Deployments(ctx context.Context, foundry Foundry, report func(int)) ([]Deployment, error) {
	deployments := []Deployment{
		{ID: foundry.ID + "/deployments/chat-production", DeploymentName: "chat-production", ModelName: "gpt-4.1", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/chat-mini", DeploymentName: "chat-mini", ModelName: "gpt-4.1-mini", Version: "2025-04-14"},
		{ID: foundry.ID + "/deployments/embeddings", DeploymentName: "embeddings", ModelName: "text-embedding-3-large", Version: "1"},
	}
	report(len(deployments))
	return deployments, ctx.Err()
}
