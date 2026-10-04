//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

var _ FoundryDeleteSource = (*azureSource)(nil)

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

func (s *azureSource) ResourceGroupHoldsOnlyFoundry(ctx context.Context, foundry Foundry) (bool, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return false, fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	nextLink := s.graph.Endpoint() + "/subscriptions/" + url.PathEscape(id.SubscriptionID) + "/resourceGroups/" + url.PathEscape(foundry.ResourceGroupName) + "/resources?api-version=2021-04-01"
	for nextLink != "" {
		req, err := runtime.NewRequest(ctx, http.MethodGet, nextLink)
		if err != nil {
			return false, fmt.Errorf("create list resources request: %w", err)
		}
		resp, err := s.graph.Pipeline().Do(req)
		if err != nil {
			return false, fmt.Errorf("list resources in resource group: %w", err)
		}
		if !runtime.HasStatusCode(resp, http.StatusOK) {
			return false, fmt.Errorf("list resources: %w", runtime.NewResponseError(resp))
		}
		var page struct {
			Value []struct {
				Type string `json:"type"`
			} `json:"value"`
			NextLink string `json:"nextLink"`
		}
		if err := runtime.UnmarshalAsJSON(resp, &page); err != nil {
			return false, fmt.Errorf("decode list resources response: %w", err)
		}
		for _, r := range page.Value {
			t := strings.ToLower(r.Type)
			if t != "microsoft.cognitiveservices/accounts" && !strings.HasPrefix(t, "microsoft.cognitiveservices/accounts/") {
				return false, nil
			}
		}
		nextLink = page.NextLink
	}
	return true, nil
}

func (s *azureSource) DeleteFoundry(ctx context.Context, foundry Foundry) error {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	client, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return fmt.Errorf("create accounts client: %w", err)
	}
	poller, err := client.BeginDelete(ctx, foundry.ResourceGroupName, foundry.Name, nil)
	if err != nil {
		return fmt.Errorf("begin delete Foundry %s: %w", foundry.Name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for delete Foundry %s: %w", foundry.Name, err)
	}
	return nil
}

func (s *azureSource) PurgeFoundry(ctx context.Context, foundry Foundry) error {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	client, err := armcognitiveservices.NewDeletedAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return fmt.Errorf("create deleted accounts client: %w", err)
	}
	pager := client.NewListPager(nil)
	var location string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			break
		}
		for _, item := range page.Value {
			if item.ID != nil && item.Name != nil && strings.EqualFold(*item.Name, foundry.Name) {
				parsed, err := arm.ParseResourceID(*item.ID)
				if err == nil && strings.EqualFold(parsed.ResourceGroupName, foundry.ResourceGroupName) {
					if item.Location != nil {
						location = *item.Location
						break
					}
				}
			}
		}
		if location != "" {
			break
		}
	}
	if location == "" {
		return nil
	}
	poller, err := client.BeginPurge(ctx, location, foundry.ResourceGroupName, foundry.Name, nil)
	if err != nil {
		return fmt.Errorf("begin purge Foundry %s: %w", foundry.Name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for purge Foundry %s: %w", foundry.Name, err)
	}
	return nil
}

func (s *azureSource) DeleteResourceGroup(ctx context.Context, foundry Foundry) error {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return fmt.Errorf("parse Foundry resource ID: %w", err)
	}
	path := "/subscriptions/" + url.PathEscape(id.SubscriptionID) + "/resourcegroups/" + url.PathEscape(foundry.ResourceGroupName) + "?api-version=2021-04-01"
	req, err := runtime.NewRequest(ctx, http.MethodDelete, s.graph.Endpoint()+path)
	if err != nil {
		return fmt.Errorf("create delete resource group request: %w", err)
	}
	resp, err := s.graph.Pipeline().Do(req)
	if err != nil {
		return fmt.Errorf("delete resource group: %w", err)
	}
	if !runtime.HasStatusCode(resp, http.StatusOK, http.StatusAccepted, http.StatusNoContent, http.StatusNotFound) {
		return fmt.Errorf("delete resource group: %w", runtime.NewResponseError(resp))
	}
	resp.Body.Close()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			checkReq, err := runtime.NewRequest(ctx, http.MethodHead, s.graph.Endpoint()+path)
			if err != nil {
				return fmt.Errorf("check resource group request: %w", err)
			}
			checkResp, err := s.graph.Pipeline().Do(checkReq)
			if err != nil {
				return fmt.Errorf("check resource group: %w", err)
			}
			checkResp.Body.Close()
			if runtime.HasStatusCode(checkResp, http.StatusNotFound) {
				return nil
			}
		}
	}
}
