//go:build !e2e

package foundry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

const (
	graphPath       = "/providers/Microsoft.ResourceGraph/resources?api-version=2022-10-01"
	graphModuleName = "azfoundrydeck/internal/foundry"
	// A leftouter join keeps a Foundry whose subscription row is missing, which is then rejected below.
	foundriesQuery = `resources
| where type =~ 'microsoft.cognitiveservices/accounts' and kind =~ 'AIServices'
| join kind=leftouter (resourcecontainers | where type =~ 'microsoft.resources/subscriptions' | project subscriptionId, subscriptionName = name) on subscriptionId
| project id, name, resourceGroup, subscriptionName`
)

type azureSource struct {
	credential azcore.TokenCredential
	graph      *arm.Client
}

func NewAzure(credential azcore.TokenCredential) (Source, error) {
	graph, err := arm.NewClient(graphModuleName, "v1.0.0", credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create resource graph client: %w", err)
	}
	return &azureSource{credential: credential, graph: graph}, nil
}

type graphRequest struct {
	Query   string       `json:"query"`
	Options graphOptions `json:"options"`
}

type graphOptions struct {
	SkipToken string `json:"$skipToken,omitempty"`
}

type graphResponse struct {
	Data []struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		ResourceGroup    string `json:"resourceGroup"`
		SubscriptionName string `json:"subscriptionName"`
	} `json:"data"`
	SkipToken string `json:"$skipToken"`
}

// Foundries lists every Foundry the signed-in user can read in one Azure Resource Graph query.
func (s *azureSource) Foundries(ctx context.Context) ([]Foundry, error) {
	foundries := make([]Foundry, 0)
	skipToken := ""
	for {
		body, err := json.Marshal(graphRequest{Query: foundriesQuery, Options: graphOptions{SkipToken: skipToken}})
		if err != nil {
			return nil, fmt.Errorf("encode resource graph request: %w", err)
		}
		req, err := runtime.NewRequest(ctx, http.MethodPost, s.graph.Endpoint()+graphPath)
		if err != nil {
			return nil, fmt.Errorf("create resource graph request: %w", err)
		}
		if err := req.SetBody(streaming.NopCloser(bytes.NewReader(body)), "application/json"); err != nil {
			return nil, fmt.Errorf("set resource graph request body: %w", err)
		}
		resp, err := s.graph.Pipeline().Do(req)
		if err != nil {
			return nil, fmt.Errorf("query Foundries: %w", err)
		}
		if !runtime.HasStatusCode(resp, http.StatusOK) {
			return nil, fmt.Errorf("query Foundries: %w", runtime.NewResponseError(resp))
		}
		var page graphResponse
		if err := runtime.UnmarshalAsJSON(resp, &page); err != nil {
			return nil, fmt.Errorf("decode Foundries: %w", err)
		}
		for _, row := range page.Data {
			if row.ID == "" || row.Name == "" || row.ResourceGroup == "" || row.SubscriptionName == "" {
				return nil, fmt.Errorf("Foundry response lacks ID, name, resource group, or subscription name")
			}
			foundries = append(foundries, Foundry{ID: row.ID, Name: row.Name, SubscriptionName: row.SubscriptionName, ResourceGroupName: row.ResourceGroup})
		}
		if page.SkipToken == "" {
			return foundries, nil
		}
		skipToken = page.SkipToken
	}
}

func (s *azureSource) Deployments(ctx context.Context, foundry Foundry, report func(int)) ([]Deployment, error) {
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
			deployments = append(deployments, deploymentFrom(deployment))
		}
		report(len(deployments))
	}
	return deployments, nil
}
