package azurego

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

type cacheCLIUser struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type cacheCLISubscription struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	TenantID          string       `json:"tenantId"`
	TenantDisplayName string       `json:"tenantDisplayName"`
	CloudName         string       `json:"cloudName"`
	State             string       `json:"state"`
	User              cacheCLIUser `json:"user"`
}

type cacheCLICommandResult struct {
	stdout []byte
	stderr []byte
	err    error
}

type cacheCountingSubscriptionClient struct {
	base             *fakeSubscriptionClient
	accountsCalls    atomic.Int32
	deploymentsCalls atomic.Int32
}

func (c *cacheCountingSubscriptionClient) listAccounts(ctx context.Context) ([]*armcognitiveservices.Account, error) {
	c.accountsCalls.Add(1)
	return c.base.listAccounts(ctx)
}

func (c *cacheCountingSubscriptionClient) listDeployments(ctx context.Context, resourceGroup, accountName string) ([]*armcognitiveservices.Deployment, error) {
	c.deploymentsCalls.Add(1)
	return c.base.listDeployments(ctx, resourceGroup, accountName)
}

func TestSubscriptionClientCacheReuseRefreshesAccountsDeploymentsAndDisplayName(t *testing.T) {
	firstCLI := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Old subscription", "tenant-a", "alice", "user"))
	secondCLI := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Renamed subscription", "tenant-a", "alice", "user"))
	client := newCacheClient("sub-a", "account-a", "old-deployment")
	var factoryCalls atomic.Int32

	p := newProvider(providerOptions{
		runCommand: cacheCLISequence(
			cacheCLICommandResult{stdout: firstCLI},
			cacheCLICommandResult{stdout: secondCLI},
		),
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			factoryCalls.Add(1)
			return client, nil
		},
	})

	firstResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, firstResult, "sub-a"); got != "old-deployment" {
		t.Fatalf("first deployment = %q, want old-deployment", got)
	}

	client.base.deployments["account-a"] = []*armcognitiveservices.Deployment{{Name: stringPointer("new-deployment")}}
	secondResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, secondResult, "sub-a"); got != "new-deployment" {
		t.Fatalf("second deployment = %q, want new-deployment", got)
	}
	if got := cacheSubscriptionName(t, secondResult, "sub-a"); got != "Renamed subscription" {
		t.Fatalf("second subscription name = %q, want Renamed subscription", got)
	}
	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	if got := client.accountsCalls.Load(); got != 2 {
		t.Fatalf("listAccounts calls = %d, want 2", got)
	}
	if got := client.deploymentsCalls.Load(); got != 2 {
		t.Fatalf("listDeployments calls = %d, want 2", got)
	}
}

func TestSubscriptionClientCacheSeparatesUserTypeTenantAndSubscription(t *testing.T) {
	entries := []cacheCLISubscription{
		cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"),
		cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "bob", "user"),
		cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "bob", "servicePrincipal"),
		cacheCLIEntry("sub-a", "Subscription A", "tenant-b", "bob", "servicePrincipal"),
		cacheCLIEntry("sub-b", "Subscription B", "tenant-b", "bob", "servicePrincipal"),
	}
	commands := make([]cacheCLICommandResult, 0, len(entries))
	for _, entry := range entries {
		commands = append(commands, cacheCLICommandResult{stdout: cacheCLIOutput(t, entry)})
	}

	var factoryCalls atomic.Int32
	p := newProvider(providerOptions{
		runCommand: cacheCLISequence(commands...),
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			call := factoryCalls.Add(1)
			return newCacheClient(subscription.id, fmt.Sprintf("account-%d", call), fmt.Sprintf("factory-%d", call)), nil
		},
	})

	for index, entry := range entries {
		result := fetchCacheResult(t, p)
		wantDeployment := fmt.Sprintf("factory-%d", index+1)
		if got := cacheDeploymentName(t, result, entry.ID); got != wantDeployment {
			t.Fatalf("fetch %d deployment = %q, want %q", index+1, got, wantDeployment)
		}
	}
	if got := factoryCalls.Load(); got != int32(len(entries)) {
		t.Fatalf("factory calls = %d, want %d", got, len(entries))
	}
}

func TestSubscriptionClientCacheDropsEntriesMissingFromLatestCLIList(t *testing.T) {
	firstCLI := cacheCLIOutput(t,
		cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"),
		cacheCLIEntry("sub-b", "Subscription B", "tenant-a", "alice", "user"),
	)
	secondCLI := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"))
	thirdCLI := cacheCLIOutput(t,
		cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"),
		cacheCLIEntry("sub-b", "Subscription B", "tenant-a", "alice", "user"),
	)

	var factoryMu sync.Mutex
	factoryCalls := make(map[string]int)
	p := newProvider(providerOptions{
		runCommand: cacheCLISequence(
			cacheCLICommandResult{stdout: firstCLI},
			cacheCLICommandResult{stdout: secondCLI},
			cacheCLICommandResult{stdout: thirdCLI},
		),
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			factoryMu.Lock()
			factoryCalls[subscription.id]++
			call := factoryCalls[subscription.id]
			factoryMu.Unlock()
			return newCacheClient(subscription.id, "account-"+subscription.id, fmt.Sprintf("%s-factory-%d", subscription.id, call)), nil
		},
	})

	firstResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, firstResult, "sub-a"); got != "sub-a-factory-1" {
		t.Fatalf("first sub-a deployment = %q, want sub-a-factory-1", got)
	}
	if got := cacheDeploymentName(t, firstResult, "sub-b"); got != "sub-b-factory-1" {
		t.Fatalf("first sub-b deployment = %q, want sub-b-factory-1", got)
	}

	secondResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, secondResult, "sub-a"); got != "sub-a-factory-1" {
		t.Fatalf("second sub-a deployment = %q, want sub-a-factory-1", got)
	}
	if len(secondResult.Deployments) != 1 {
		t.Fatalf("second deployments = %#v, want only sub-a", secondResult.Deployments)
	}

	thirdResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, thirdResult, "sub-a"); got != "sub-a-factory-1" {
		t.Fatalf("third sub-a deployment = %q, want sub-a-factory-1", got)
	}
	if got := cacheDeploymentName(t, thirdResult, "sub-b"); got != "sub-b-factory-2" {
		t.Fatalf("third sub-b deployment = %q, want sub-b-factory-2", got)
	}
	factoryMu.Lock()
	gotA, gotB := factoryCalls["sub-a"], factoryCalls["sub-b"]
	factoryMu.Unlock()
	if gotA != 1 || gotB != 2 {
		t.Fatalf("factory calls by subscription = %v, want sub-a=1 and sub-b=2", factoryCalls)
	}
}

func TestSubscriptionClientCacheDiscardedWhenCLIListFails(t *testing.T) {
	output := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"))
	var factoryCalls atomic.Int32
	p := newProvider(providerOptions{
		runCommand: cacheCLISequence(
			cacheCLICommandResult{stdout: output},
			cacheCLICommandResult{stderr: []byte("temporary CLI failure"), err: errors.New("exit status 1")},
			cacheCLICommandResult{stdout: output},
		),
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			call := factoryCalls.Add(1)
			return newCacheClient(subscription.id, fmt.Sprintf("account-%d", call), fmt.Sprintf("factory-%d", call)), nil
		},
	})

	firstResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, firstResult, "sub-a"); got != "factory-1" {
		t.Fatalf("first deployment = %q, want factory-1", got)
	}

	failedResult := fetchCacheResult(t, p)
	if len(failedResult.Deployments) != 0 || len(failedResult.Failures) != 1 || failedResult.Failures[0].Scope != "azure-cli" {
		t.Fatalf("CLI failure result = %#v, want one azure-cli failure and no deployments", failedResult)
	}

	thirdResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, thirdResult, "sub-a"); got != "factory-2" {
		t.Fatalf("third deployment = %q, want factory-2 after CLI failure", got)
	}
	if got := factoryCalls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2", got)
	}
}

func TestSubscriptionClientCacheRetriesFactoryError(t *testing.T) {
	output := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"))
	var factoryCalls atomic.Int32
	p := newProvider(providerOptions{
		runCommand: cacheCLISequence(
			cacheCLICommandResult{stdout: output},
			cacheCLICommandResult{stdout: output},
		),
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			if factoryCalls.Add(1) == 1 {
				return nil, errors.New("factory unavailable")
			}
			return newCacheClient(subscription.id, "account-a", "factory-retry-succeeded"), nil
		},
	})

	firstResult := fetchCacheResult(t, p)
	if len(firstResult.Deployments) != 0 || len(firstResult.Failures) != 1 || firstResult.Failures[0].Scope != "subscription" {
		t.Fatalf("factory error result = %#v, want one subscription failure and no deployments", firstResult)
	}

	secondResult := fetchCacheResult(t, p)
	if got := cacheDeploymentName(t, secondResult, "sub-a"); got != "factory-retry-succeeded" {
		t.Fatalf("retry deployment = %q, want factory-retry-succeeded", got)
	}
	if got := factoryCalls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2", got)
	}
}

func TestSubscriptionClientCacheConcurrentReuseCreatesFactoryOnce(t *testing.T) {
	output := cacheCLIOutput(t, cacheCLIEntry("sub-a", "Subscription A", "tenant-a", "alice", "user"))
	client := newCacheClient("sub-a", "account-a", "shared-client")
	var factoryCalls atomic.Int32
	factoryStarted := make(chan struct{})
	releaseFactory := make(chan struct{})
	var startedOnce sync.Once

	p := newProvider(providerOptions{
		runCommand: func(context.Context, ...string) ([]byte, []byte, error) {
			return output, nil, nil
		},
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			call := factoryCalls.Add(1)
			if call == 1 {
				startedOnce.Do(func() { close(factoryStarted) })
				<-releaseFactory
			}
			return client, nil
		},
	})

	type outcome struct {
		result service.DeploymentResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	go func() {
		result, err := p.Fetch(context.Background(), nil)
		outcomes <- outcome{result: result, err: err}
	}()
	<-factoryStarted
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		result, err := p.Fetch(context.Background(), nil)
		outcomes <- outcome{result: result, err: err}
	}()
	<-secondStarted
	close(releaseFactory)

	for i := 0; i < 2; i++ {
		got := <-outcomes
		if got.err != nil {
			t.Fatalf("concurrent Fetch returned error: %v", got.err)
		}
		if gotName := cacheDeploymentName(t, got.result, "sub-a"); gotName != "shared-client" {
			t.Fatalf("concurrent deployment = %q, want shared-client", gotName)
		}
	}
	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	if got := client.accountsCalls.Load(); got != 2 {
		t.Fatalf("listAccounts calls = %d, want 2", got)
	}
	if got := client.deploymentsCalls.Load(); got != 2 {
		t.Fatalf("listDeployments calls = %d, want 2", got)
	}
}

func cacheCLIEntry(id, name, tenantID, userName, userType string) cacheCLISubscription {
	return cacheCLISubscription{
		ID:                id,
		Name:              name,
		TenantID:          tenantID,
		TenantDisplayName: tenantID,
		CloudName:         azureCloudName,
		State:             "Enabled",
		User:              cacheCLIUser{Name: userName, Type: userType},
	}
}

func cacheCLIOutput(t *testing.T, entries ...cacheCLISubscription) []byte {
	t.Helper()
	output, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal cache CLI fixture: %v", err)
	}
	return output
}

func cacheCLISequence(commands ...cacheCLICommandResult) commandRunner {
	var call atomic.Int32
	return func(context.Context, ...string) ([]byte, []byte, error) {
		index := int(call.Add(1)) - 1
		if index >= len(commands) {
			return nil, nil, fmt.Errorf("unexpected CLI call %d", index+1)
		}
		command := commands[index]
		return command.stdout, command.stderr, command.err
	}
}

func newCacheClient(subscriptionID, accountName, deploymentName string) *cacheCountingSubscriptionClient {
	accountID := fmt.Sprintf("/subscriptions/%s/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/%s", subscriptionID, accountName)
	return &cacheCountingSubscriptionClient{
		base: &fakeSubscriptionClient{
			accounts: []*armcognitiveservices.Account{{
				ID:       stringPointer(accountID),
				Name:     stringPointer(accountName),
				Kind:     stringPointer(accountKindAIServices),
				Location: stringPointer("japaneast"),
			}},
			deployments: map[string][]*armcognitiveservices.Deployment{
				accountName: {{Name: stringPointer(deploymentName)}},
			},
			deploymentErrors: map[string]error{},
		},
	}
}

func fetchCacheResult(t *testing.T, provider *Provider) service.DeploymentResult {
	t.Helper()
	result, err := provider.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	return result
}

func cacheDeploymentName(t *testing.T, result service.DeploymentResult, subscriptionID string) string {
	t.Helper()
	for _, deployment := range result.Deployments {
		if deployment.SubscriptionID == subscriptionID {
			return deployment.Name
		}
	}
	t.Fatalf("deployment for subscription %q not found in %#v", subscriptionID, result.Deployments)
	return ""
}

func cacheSubscriptionName(t *testing.T, result service.DeploymentResult, subscriptionID string) string {
	t.Helper()
	for _, deployment := range result.Deployments {
		if deployment.SubscriptionID == subscriptionID {
			return deployment.SubscriptionName
		}
	}
	t.Fatalf("deployment for subscription %q not found in %#v", subscriptionID, result.Deployments)
	return ""
}
