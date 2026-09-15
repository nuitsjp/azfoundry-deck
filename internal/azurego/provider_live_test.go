package azurego

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

// The previous generated SDK pagers exist only in this opt-in comparison,
// never as a runtime fallback. They verify discovery and deployment parity.
type legacySDKClient struct {
	accounts    *armcognitiveservices.AccountsClient
	deployments *armcognitiveservices.DeploymentsClient
	models      *armcognitiveservices.ModelsClient
}

func newLegacySDKClient(sub subscriptionInfo) (subscriptionClient, error) {
	credential, err := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{Subscription: sub.id})
	if err != nil {
		return nil, err
	}
	factory, err := armcognitiveservices.NewClientFactory(sub.id, credential, &arm.ClientOptions{DisableRPRegistration: true})
	if err != nil {
		return nil, err
	}
	return &legacySDKClient{accounts: factory.NewAccountsClient(), deployments: factory.NewDeploymentsClient(), models: factory.NewModelsClient()}, nil
}

func (c *legacySDKClient) listResourceGroups(context.Context) ([]resourceGroupInfo, error) {
	return nil, nil
}

func (c *legacySDKClient) listRegionModels(ctx context.Context, location string) ([]*armcognitiveservices.Model, error) {
	var models []*armcognitiveservices.Model
	pager := c.models.NewListPager(location, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		models = append(models, page.Value...)
	}
	return models, nil
}

func (c *legacySDKClient) listAccounts(ctx context.Context) ([]*armcognitiveservices.Account, error) {
	var accounts []*armcognitiveservices.Account
	pager := c.accounts.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, page.Value...)
	}
	return accounts, nil
}

func (c *legacySDKClient) listDeployments(ctx context.Context, group, name string) ([]*armcognitiveservices.Deployment, error) {
	var deployments []*armcognitiveservices.Deployment
	pager := c.deployments.NewListPager(group, name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, page.Value...)
	}
	return deployments, nil
}

func (c *legacySDKClient) listModels(ctx context.Context, group, name string) ([]*armcognitiveservices.AccountModel, error) {
	var models []*armcognitiveservices.AccountModel
	pager := c.accounts.NewListModelsPager(group, name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		models = append(models, page.Value...)
	}
	return models, nil
}

type liveAccountObserver struct {
	sync.Mutex
	keys []string
}

type liveObservedClient struct {
	subscriptionClient
	observer *liveAccountObserver
}

func (c *liveObservedClient) listAccounts(ctx context.Context) ([]*armcognitiveservices.Account, error) {
	accounts, err := c.subscriptionClient.listAccounts(ctx)
	if err == nil {
		c.observer.Lock()
		defer c.observer.Unlock()
		for _, account := range accounts {
			if account != nil && isFoundryAccount(account.Kind) {
				c.observer.keys = append(c.observer.keys, strings.ToLower(value(account.ID))+"|"+value(account.Name)+"|"+value(account.Kind)+"|"+value(account.Location))
			}
		}
	}
	return accounts, err
}

// TestLiveAzureProvider is opt-in because it uses the caller's signed-in Azure
// CLI session and performs read-only ARM requests.
func TestLiveAzureProvider(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LIVE") != "1" {
		t.Skip("set AZFOUNDRY_LIVE=1 to run the live Azure verification")
	}
	observer := &liveAccountObserver{}
	observe := func(factory subscriptionClientFactory) subscriptionClientFactory {
		return func(sub subscriptionInfo) (subscriptionClient, error) {
			client, err := factory(sub)
			if err != nil {
				return nil, err
			}
			return &liveObservedClient{subscriptionClient: client, observer: observer}, nil
		}
	}
	parallel := newProvider(providerOptions{newClient: observe(newSDKSubscriptionClient)})
	runs := []struct {
		name string
		p    *Provider
	}{
		{name: "previous_api_before", p: newProvider(providerOptions{newClient: observe(newLegacySDKClient)})},
		{name: "parallel_cold", p: parallel},
		{name: "parallel_refresh_1", p: parallel},
		{name: "parallel_refresh_2", p: parallel},
		{name: "previous_api_after", p: newProvider(providerOptions{newClient: observe(newLegacySDKClient)})},
		{name: "serial_cold", p: newProvider(providerOptions{subscriptionConcurrency: 1, accountConcurrency: 1, newClient: observe(newSDKSubscriptionClient)})},
	}
	var reference service.DeploymentResult
	var referenceAccounts []string
	for _, run := range runs {
		run := run
		t.Run(run.name, func(t *testing.T) {
			observer.keys = nil
			started := time.Now()
			progress := make([]service.DeploymentProgress, 0)
			var firstRows time.Duration
			result, err := run.p.Fetch(context.Background(), func(snapshot service.DeploymentProgress) {
				progress = append(progress, snapshot)
				if len(snapshot.Result.Deployments) > 0 && firstRows == 0 {
					firstRows = time.Since(started)
				}
			})
			elapsed := time.Since(started)
			if err != nil {
				t.Fatalf("Fetch returned error: %v", err)
			}
			if result.TotalAccounts == nil && len(result.Failures) == 0 {
				t.Fatal("TotalAccounts is nil without a discovery failure")
			}
			if result.SuccessfulAccounts == 0 {
				t.Fatal("no successful accounts; live data parity cannot be verified")
			}
			result.FetchedAt = ""
			sort.Slice(result.Deployments, func(i, j int) bool { return result.Deployments[i].ID < result.Deployments[j].ID })
			sort.Slice(result.Failures, func(i, j int) bool { return fmt.Sprint(result.Failures[i]) < fmt.Sprint(result.Failures[j]) })
			sort.Strings(observer.keys)
			if run.name == "previous_api_before" {
				reference, referenceAccounts = result, append([]string(nil), observer.keys...)
			}
			if !reflect.DeepEqual(reference, result) || !reflect.DeepEqual(referenceAccounts, observer.keys) {
				t.Error("account metadata, deployment fields, totals or failures differ from the previous API (resource values omitted)")
			}

			maxSubscriptions := 0
			maxAccounts := 0
			fetchEvents := 0
			knownTPM := 0
			knownRPM := 0
			failureCodes := make(map[string]int)
			for _, snapshot := range progress {
				if snapshot.CompletedSubscriptions > maxSubscriptions {
					maxSubscriptions = snapshot.CompletedSubscriptions
				}
				if snapshot.CompletedAccounts > maxAccounts {
					maxAccounts = snapshot.CompletedAccounts
				}
				if snapshot.Stage == service.DeploymentProgressStageFetching {
					fetchEvents++
				}
			}
			for _, deployment := range result.Deployments {
				if deployment.TPM != nil {
					knownTPM++
				}
				if deployment.RPM != nil {
					knownRPM++
				}
			}
			for _, failure := range result.Failures {
				failureCodes[failure.Code]++
			}
			if result.TotalAccounts != nil && maxAccounts != *result.TotalAccounts {
				t.Fatalf("progress completed accounts = %d, result total accounts = %d", maxAccounts, *result.TotalAccounts)
			}
			if fetchEvents == 0 {
				t.Fatal("live fetch progress was not reported")
			}
			totalAccounts := "unknown"
			if result.TotalAccounts != nil {
				totalAccounts = fmt.Sprintf("%d", *result.TotalAccounts)
			}
			t.Logf("subscription concurrency=%d, account concurrency=%d, elapsed=%s, first rows=%s, subscriptions completed=%d, account completions=%d, successful accounts=%d, accounts=%s, deployments=%d, failures=%d (%v), known TPM=%d, known RPM=%d, progress events=%d, parity=%t", run.p.subscriptionConcurrency, run.p.accountConcurrency, elapsed.Round(time.Millisecond), firstRows.Round(time.Millisecond), maxSubscriptions, maxAccounts, result.SuccessfulAccounts, totalAccounts, len(result.Deployments), len(result.Failures), failureCodes, knownTPM, knownRPM, len(progress), !t.Failed())
		})
	}
}
