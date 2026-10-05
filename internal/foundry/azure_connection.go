//go:build !e2e

package foundry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

// openAIEndpoint names the Azure OpenAI entry of the account's endpoints. The
// screen shows the v1 API base URL under it.
const openAIEndpoint = "OpenAI Language Model Instance API"

// Connection reads the Foundry's Azure OpenAI endpoint and, in parallel, its key 1.
func (s *azureSource) Connection(ctx context.Context, foundry Foundry) (Connection, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return Connection{}, err
	}
	accounts, err := armcognitiveservices.NewAccountsClient(id.SubscriptionID, s.credential, nil)
	if err != nil {
		return Connection{}, err
	}
	var connection Connection
	var wg sync.WaitGroup
	var endpointErr, keyErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		account, err := accounts.Get(ctx, foundry.ResourceGroupName, foundry.Name, nil)
		if err != nil {
			endpointErr = fmt.Errorf("get Foundry endpoints: %w", err)
			return
		}
		if account.Properties == nil || account.Properties.Endpoints[openAIEndpoint] == nil {
			endpointErr = fmt.Errorf("Foundry response lacks the Azure OpenAI endpoint")
			return
		}
		connection.Endpoint = strings.TrimSuffix(*account.Properties.Endpoints[openAIEndpoint], "/") + "/openai/v1"
	}()
	go func() {
		defer wg.Done()
		keys, err := accounts.ListKeys(ctx, foundry.ResourceGroupName, foundry.Name, nil)
		if err != nil {
			keyErr = fmt.Errorf("list Foundry keys: %w", err)
			return
		}
		if keys.Key1 == nil {
			keyErr = fmt.Errorf("Foundry keys response lacks key 1")
			return
		}
		connection.Key = *keys.Key1
	}()
	wg.Wait()
	if endpointErr != nil {
		return Connection{}, endpointErr
	}
	if keyErr != nil {
		return Connection{}, keyErr
	}
	return connection, nil
}
