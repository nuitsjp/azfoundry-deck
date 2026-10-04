//go:build e2e

package foundry

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

var createdFoundries = struct {
	sync.Mutex
	items []Foundry
}{}

func fixedCreatedFoundries() []Foundry {
	createdFoundries.Lock()
	defer createdFoundries.Unlock()
	return append([]Foundry{}, createdFoundries.items...)
}

func fixedCreationWait(ctx context.Context, duration time.Duration) error {
	if os.Getenv("AZFOUNDRYDECK_E2E_FOUNDRY_ADD_REVIEW") != "1" {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return nil
	}
}

func (fixedSource) FoundrySubscriptions(ctx context.Context) ([]FoundrySubscription, error) {
	return []FoundrySubscription{
		{ID: "review-development", DisplayName: "Contoso Development"},
		{ID: "review-production", DisplayName: "Contoso AI Production Subscription"},
	}, ctx.Err()
}

func (fixedSource) FoundryRegions(ctx context.Context, subscriptionID string) ([]FoundryRegion, error) {
	return []FoundryRegion{
		{Name: "eastus2", DisplayName: "East US 2"},
		{Name: "japaneast", DisplayName: "Japan East"},
		{Name: "swedencentral", DisplayName: "Sweden Central"},
	}, ctx.Err()
}

func (fixedSource) CreateResourceGroup(ctx context.Context, spec FoundryCreateSpec) error {
	return fixedCreationWait(ctx, 2*time.Second)
}

func (source fixedSource) CreateFoundry(ctx context.Context, spec FoundryCreateSpec) (Foundry, error) {
	if err := fixedCreationWait(ctx, 4*time.Second); err != nil {
		return Foundry{}, err
	}
	subscriptions, err := source.FoundrySubscriptions(ctx)
	if err != nil {
		return Foundry{}, err
	}
	for _, subscription := range subscriptions {
		if subscription.ID == spec.SubscriptionID {
			created := Foundry{
				ID:   fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.CognitiveServices/accounts/%s", spec.SubscriptionID, spec.ResourceGroupName, spec.FoundryName),
				Name: spec.FoundryName, SubscriptionName: subscription.DisplayName, ResourceGroupName: spec.ResourceGroupName,
			}
			createdFoundries.Lock()
			createdFoundries.items = append(createdFoundries.items, created)
			createdFoundries.Unlock()
			return created, nil
		}
	}
	return Foundry{}, fmt.Errorf("subscription is not in the fixed creation catalog")
}
