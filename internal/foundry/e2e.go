//go:build e2e

package foundry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func (fixedSource) Discover(ctx context.Context, discovered func(Foundry), report func(DiscoveryProgress)) ([]Foundry, error) {
	foundries := []Foundry{
		{ID: "/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast", Name: "contoso-foundry-production-japaneast", SubscriptionName: "Contoso AI Production Subscription", ResourceGroupName: "rg-ai-production-japaneast"},
		{ID: "/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development", Name: "contoso-foundry-development", SubscriptionName: "Contoso Development", ResourceGroupName: "rg-ai-development"},
		{ID: "/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research", Name: "contoso-foundry-research", SubscriptionName: "Contoso Research", ResourceGroupName: "rg-ai-research"},
	}
	if err := waitForRelease(ctx, "discovery"); err != nil {
		return nil, err
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
	if err := waitForRelease(ctx, "start"); err != nil {
		return nil, err
	}
	progress.SubscriptionSearch = "completed"
	publish()
	if os.Getenv("AZFOUNDRYDECK_E2E_HOLD_FOUNDRY") == "1" {
		for i := range progress.Subscriptions {
			progress.Subscriptions[i].Phase = "running"
		}
		progress.Subscriptions[0].FoundryCount = 1
		publish()
		discovered(foundries[0])
		if err := waitForRelease(ctx, "second"); err != nil {
			return nil, err
		}
		progress.Subscriptions[1].FoundryCount = 1
		progress.Subscriptions[1].Phase = "completed"
		publish()
		discovered(foundries[1])
		if err := waitForRelease(ctx, "remaining"); err != nil {
			return nil, err
		}
		for _, i := range []int{0, 2} {
			progress.Subscriptions[i].FoundryCount = 1
			progress.Subscriptions[i].Phase = "completed"
			publish()
			if i == 2 {
				discovered(foundries[i])
			}
		}
		return foundries, ctx.Err()
	}
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
	if err := waitForRelease(ctx, "models"); err != nil {
		return nil, err
	}
	report(len(deployments))
	return deployments, ctx.Err()
}
