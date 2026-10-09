//go:build !e2e

package foundry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"azfoundrydeck/internal/fault"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

var _ FoundryCreateSource = (*azureSource)(nil)

func (s *azureSource) FoundrySubscriptions(ctx context.Context) ([]FoundrySubscription, error) {
	client, err := armsubscriptions.NewClient(s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create subscriptions client: %w", err)
	}
	subscriptions := make([]FoundrySubscription, 0)
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list Foundry subscriptions: %w", err)
		}
		for _, subscription := range page.Value {
			if subscription == nil || subscription.State == nil || *subscription.State != armsubscriptions.SubscriptionStateEnabled {
				continue
			}
			if subscription.SubscriptionID == nil || *subscription.SubscriptionID == "" || subscription.DisplayName == nil || *subscription.DisplayName == "" {
				return nil, fmt.Errorf("subscription response lacks ID or display name")
			}
			subscriptions = append(subscriptions, FoundrySubscription{ID: *subscription.SubscriptionID, DisplayName: *subscription.DisplayName})
		}
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		if subscriptions[i].DisplayName == subscriptions[j].DisplayName {
			return subscriptions[i].ID < subscriptions[j].ID
		}
		return subscriptions[i].DisplayName < subscriptions[j].DisplayName
	})
	return subscriptions, nil
}

func (s *azureSource) FoundryRegions(ctx context.Context, subscriptionID string) ([]FoundryRegion, error) {
	skus, err := armcognitiveservices.NewResourceSKUsClient(subscriptionID, s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create resource SKUs client: %w", err)
	}
	available := make(map[string]bool)
	skuPager := skus.NewListPager(nil)
	for skuPager.More() {
		page, err := skuPager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list Foundry resource SKUs: %w", err)
		}
		for _, sku := range page.Value {
			if sku == nil || sku.Kind == nil || !strings.EqualFold(*sku.Kind, "AIServices") || sku.Name == nil || !strings.EqualFold(*sku.Name, "S0") {
				continue
			}
			for _, location := range sku.Locations {
				if location == nil || *location == "" {
					return nil, fmt.Errorf("Foundry resource SKU response lacks location")
				}
				if !foundryLocationRestricted(sku.Restrictions, *location) {
					available[strings.ToLower(*location)] = true
				}
			}
		}
	}
	client, err := armsubscriptions.NewClient(s.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create subscriptions client: %w", err)
	}
	regions := make([]FoundryRegion, 0)
	pager := client.NewListLocationsPager(subscriptionID, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list subscription locations: %w", err)
		}
		for _, location := range page.Value {
			if location == nil || location.Name == nil || *location.Name == "" {
				return nil, fmt.Errorf("subscription location response lacks name")
			}
			if !available[strings.ToLower(*location.Name)] {
				continue
			}
			if location.DisplayName == nil || *location.DisplayName == "" {
				return nil, fmt.Errorf("subscription location response lacks display name")
			}
			regions = append(regions, FoundryRegion{Name: *location.Name, DisplayName: *location.DisplayName})
		}
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].DisplayName < regions[j].DisplayName })
	return regions, nil
}

func foundryLocationRestricted(restrictions []*armcognitiveservices.ResourceSKURestrictions, location string) bool {
	for _, restriction := range restrictions {
		if restriction == nil || restriction.Type == nil || *restriction.Type != armcognitiveservices.ResourceSKURestrictionsTypeLocation {
			continue
		}
		for _, value := range restriction.Values {
			if value != nil && strings.EqualFold(*value, location) {
				return true
			}
		}
		if restriction.RestrictionInfo != nil {
			for _, value := range restriction.RestrictionInfo.Locations {
				if value != nil && strings.EqualFold(*value, location) {
					return true
				}
			}
		}
	}
	return false
}

func (s *azureSource) CreateResourceGroup(ctx context.Context, spec FoundryCreateSpec) error {
	// Resource Groups has no create-only operation. Reject an existing group before PUT.
	path := "/subscriptions/" + url.PathEscape(spec.SubscriptionID) + "/resourcegroups/" + url.PathEscape(spec.ResourceGroupName) + "?api-version=2021-04-01"
	req, err := runtime.NewRequest(ctx, http.MethodHead, s.graph.Endpoint()+path)
	if err != nil {
		return fmt.Errorf("create resource group existence request: %w", err)
	}
	resp, err := s.graph.Pipeline().Do(req)
	if err != nil {
		return fmt.Errorf("check resource group existence: %w", err)
	}
	if runtime.HasStatusCode(resp, http.StatusNoContent) {
		resp.Body.Close() //nolint:errcheck // Only the HEAD response status is needed.
		return fault.New("FOUNDRY_CREATE_FAILED", "A resource group with this name already exists. Choose a new resource group name.")
	}
	if !runtime.HasStatusCode(resp, http.StatusNotFound) {
		return fmt.Errorf("check resource group existence: %w", runtime.NewResponseError(resp))
	}
	resp.Body.Close() //nolint:errcheck // Only the HEAD response status is needed.
	req, err = runtime.NewRequest(ctx, http.MethodPut, s.graph.Endpoint()+path)
	if err != nil {
		return fmt.Errorf("create resource group request: %w", err)
	}
	if err := runtime.MarshalAsJSON(req, struct {
		Location string `json:"location"`
	}{Location: spec.Region}); err != nil {
		return fmt.Errorf("encode resource group request: %w", err)
	}
	resp, err = s.graph.Pipeline().Do(req)
	if err != nil {
		return fmt.Errorf("create resource group: %w", err)
	}
	if !runtime.HasStatusCode(resp, http.StatusOK, http.StatusCreated) {
		return fmt.Errorf("create resource group: %w", runtime.NewResponseError(resp))
	}
	defer resp.Body.Close() //nolint:errcheck // Closing a read-only response does not change the accepted result.
	var created struct {
		Properties struct {
			ProvisioningState string `json:"provisioningState"`
		} `json:"properties"`
	}
	if err := runtime.UnmarshalAsJSON(resp, &created); err != nil {
		return fmt.Errorf("decode created resource group: %w", err)
	}
	if !strings.EqualFold(created.Properties.ProvisioningState, "Succeeded") {
		return fmt.Errorf("resource group creation did not succeed: %s", created.Properties.ProvisioningState)
	}
	return nil
}

func (s *azureSource) CreateFoundry(ctx context.Context, spec FoundryCreateSpec) (Foundry, error) {
	client, err := armcognitiveservices.NewAccountsClient(spec.SubscriptionID, s.credential, nil)
	if err != nil {
		return Foundry{}, fmt.Errorf("create accounts client: %w", err)
	}
	_, err = client.Get(ctx, spec.ResourceGroupName, spec.FoundryName, nil)
	if err == nil {
		return Foundry{}, fault.New("FOUNDRY_CREATE_FAILED", "A Foundry with this name already exists in the resource group. Choose a new Foundry name.")
	}
	var responseError *azcore.ResponseError
	if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusNotFound {
		return Foundry{}, fmt.Errorf("check Foundry existence: %w", err)
	}
	subscriptions, err := armsubscriptions.NewClient(s.credential, nil)
	if err != nil {
		return Foundry{}, fmt.Errorf("create subscriptions client: %w", err)
	}
	subscription, err := subscriptions.Get(ctx, spec.SubscriptionID, nil)
	if err != nil {
		return Foundry{}, fmt.Errorf("get Foundry subscription: %w", err)
	}
	if subscription.DisplayName == nil || *subscription.DisplayName == "" {
		return Foundry{}, fmt.Errorf("subscription response lacks display name")
	}
	poller, err := client.BeginCreate(ctx, spec.ResourceGroupName, spec.FoundryName, armcognitiveservices.Account{
		Kind: to.Ptr("AIServices"), Location: to.Ptr(spec.Region),
		SKU:      &armcognitiveservices.SKU{Name: to.Ptr("S0")},
		Identity: &armcognitiveservices.Identity{Type: to.Ptr(armcognitiveservices.ResourceIdentityTypeSystemAssigned)},
		Properties: &armcognitiveservices.AccountProperties{
			AllowProjectManagement: to.Ptr(true),
			CustomSubDomainName:    to.Ptr(spec.FoundryName),
		},
	}, nil)
	if err != nil {
		return Foundry{}, fmt.Errorf("begin Foundry creation: %w", err)
	}
	created, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return Foundry{}, fmt.Errorf("complete Foundry creation: %w", err)
	}
	if created.ID == nil || *created.ID == "" || created.Name == nil || *created.Name == "" {
		return Foundry{}, fmt.Errorf("created Foundry response lacks ID or name")
	}
	return Foundry{ID: *created.ID, Name: *created.Name, ResourceGroupName: spec.ResourceGroupName, SubscriptionName: *subscription.DisplayName}, nil
}
