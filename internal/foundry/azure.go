//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"golang.org/x/sync/errgroup"
)

type azureSource struct {
	credential    azcore.TokenCredential
	subscriptions *armsubscriptions.Client
}

func NewAzure(credential azcore.TokenCredential) (Source, error) {
	client, err := armsubscriptions.NewClient(credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create subscriptions client: %w", err)
	}
	return &azureSource{credential: credential, subscriptions: client}, nil
}

func (s *azureSource) Discover(ctx context.Context, discovered func(Foundry)) ([]Foundry, error) {
	foundries := make([]Foundry, 0)
	var mu sync.Mutex
	group, fetchCtx := errgroup.WithContext(ctx)
	// One subscription pager and at most eight account pagers run together.
	group.SetLimit(9)
	group.Go(func() error {
		pager := s.subscriptions.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(fetchCtx)
			if err != nil {
				return fmt.Errorf("list subscriptions: %w", err)
			}
			for _, subscription := range page.Value {
				group.Go(func() error {
					if subscription == nil || subscription.SubscriptionID == nil || *subscription.SubscriptionID == "" || subscription.DisplayName == nil || *subscription.DisplayName == "" {
						return fmt.Errorf("subscription response lacks ID or display name")
					}
					client, err := armcognitiveservices.NewAccountsClient(*subscription.SubscriptionID, s.credential, nil)
					if err != nil {
						return fmt.Errorf("create accounts client: %w", err)
					}
					accounts := client.NewListPager(nil)
					for accounts.More() {
						page, err := accounts.NextPage(fetchCtx)
						if err != nil {
							return fmt.Errorf("list accounts for subscription %s: %w", *subscription.SubscriptionID, err)
						}
						for _, account := range page.Value {
							if account == nil {
								return fmt.Errorf("accounts response contains a nil account")
							}
							if account.Kind == nil {
								return fmt.Errorf("account response lacks kind")
							}
							if !strings.EqualFold(*account.Kind, "AIServices") {
								continue
							}
							if account.ID == nil || account.Name == nil || *account.Name == "" {
								return fmt.Errorf("Foundry response lacks ID or name")
							}
							id, err := arm.ParseResourceID(*account.ID)
							if err != nil {
								return fmt.Errorf("parse Foundry resource ID: %w", err)
							}
							if id.ResourceGroupName == "" {
								return fmt.Errorf("Foundry response lacks name or resource group")
							}
							foundry := Foundry{
								ID:                *account.ID,
								Name:              *account.Name,
								SubscriptionName:  *subscription.DisplayName,
								ResourceGroupName: id.ResourceGroupName,
							}
							mu.Lock()
							foundries = append(foundries, foundry)
							discovered(foundry)
							mu.Unlock()
						}
					}
					return nil
				})
			}
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return foundries, nil
}

func (s *azureSource) Deployments(ctx context.Context, foundry Foundry) ([]Deployment, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return nil, fmt.Errorf("parse selected Foundry resource ID: %w", err)
	}
	client, err := armcognitiveservices.NewDeploymentsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create deployments client: %w", err)
	}
	deployments := make([]Deployment, 0)
	pager := client.NewListPager(foundry.ResourceGroupName, foundry.Name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list deployments for Foundry %s: %w", foundry.Name, err)
		}
		for _, deployment := range page.Value {
			if deployment == nil || deployment.ID == nil || *deployment.ID == "" || deployment.Name == nil || *deployment.Name == "" || deployment.Properties == nil || deployment.Properties.Model == nil || deployment.Properties.Model.Name == nil || *deployment.Properties.Model.Name == "" {
				return nil, fmt.Errorf("deployment response lacks ID, name, or model")
			}
			version := ""
			if deployment.Properties.Model.Version != nil {
				version = *deployment.Properties.Model.Version
			}
			deployments = append(deployments, Deployment{
				ID:             *deployment.ID,
				DeploymentName: *deployment.Name,
				ModelName:      *deployment.Properties.Model.Name,
				Version:        version,
			})
		}
	}
	return deployments, nil
}
