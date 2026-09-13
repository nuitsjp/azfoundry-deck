package azurego

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

type fakeSubscriptionClient struct {
	accounts         []*armcognitiveservices.Account
	accountsErr      error
	deployments      map[string][]*armcognitiveservices.Deployment
	deploymentErrors map[string]error
}

func (c *fakeSubscriptionClient) listAccounts(ctx context.Context) ([]*armcognitiveservices.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.accounts, c.accountsErr
}

func (c *fakeSubscriptionClient) listDeployments(ctx context.Context, _, accountName string) ([]*armcognitiveservices.Deployment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.deploymentErrors[accountName]; err != nil {
		return nil, err
	}
	return c.deployments[accountName], nil
}

func testProvider(t *testing.T, cliOutput []cliSubscription, clients map[string]subscriptionClient) *Provider {
	t.Helper()
	output, err := json.Marshal(cliOutput)
	if err != nil {
		t.Fatalf("marshal CLI fixture: %v", err)
	}
	return newProvider(providerOptions{
		subscriptionConcurrency: 1,
		accountConcurrency:      1,
		runCommand: func(context.Context, ...string) ([]byte, []byte, error) {
			return output, nil, nil
		},
		newClient: func(subscription subscriptionInfo) (subscriptionClient, error) {
			client, ok := clients[subscription.id]
			if !ok {
				return nil, fmt.Errorf("missing fake client for %s", subscription.id)
			}
			return client, nil
		},
	})
}

func TestListSubscriptionsFiltersCloudStateAndDoesNotChangeAzureCLIContext(t *testing.T) {
	var gotArgs []string
	p := newProvider(providerOptions{
		runCommand: func(_ context.Context, args ...string) ([]byte, []byte, error) {
			gotArgs = append([]string(nil), args...)
			return []byte(`[
                {"id":"sub-a","name":"A","tenantId":"tenant-a","tenantDisplayName":"Tenant A","cloudName":"AzureCloud","state":"Enabled"},
                {"id":"sub-government","name":"Gov","tenantId":"tenant-a","cloudName":"AzureUSGovernment","state":"Enabled"},
                {"id":"sub-missing-cloud","name":"Missing cloud","tenantId":"tenant-a","state":"Enabled"},
                {"id":"sub-disabled","name":"Disabled","tenantId":"tenant-a","cloudName":"AzureCloud","state":"Disabled"},
                {"id":"sub-missing-tenant","name":"Missing","cloudName":"AzureCloud","state":"Enabled"},
                {"id":"tenant-a","name":"N/A(tenant level account)","tenantId":"tenant-a","cloudName":"AzureCloud","state":"Enabled"},
                {"id":"sub-a","name":"A duplicate","tenantId":"tenant-a","cloudName":"AzureCloud","state":"Enabled"}
            ]`), nil, nil
		},
	})

	items, err := p.listSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("listSubscriptions returned error: %v", err)
	}
	if len(items) != 1 || items[0].id != "sub-a" {
		t.Fatalf("subscriptions = %#v, want only sub-a", items)
	}
	if strings.Contains(strings.Join(gotArgs, " "), "set") {
		t.Fatalf("Azure CLI context was changed: %v", gotArgs)
	}
	wantArgs := "account list --all --only-show-errors --output json"
	if got := strings.Join(gotArgs, " "); got != wantArgs {
		t.Fatalf("Azure CLI args = %q, want %q", got, wantArgs)
	}
}

func TestQuantityFromRateLimitsUsesRateLimitsAndNormalizesToMinute(t *testing.T) {
	tokenCount := float32(2_000_000)
	tokenPeriod := float32(60)
	requestCount := float32(20_000)
	requestPeriod := float32(60)
	capacity := int32(999)
	row := deploymentRow(discoveredAccount{
		subscription: subscriptionInfo{id: "sub", name: "Subscription", tenantID: "tenant", tenantName: "Tenant"},
		id:           "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account",
		name:         "account",
		region:       "japaneast",
	}, &armcognitiveservices.Deployment{
		ID:   stringPointer("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account/deployments/chat"),
		Name: stringPointer("chat"),
		Properties: &armcognitiveservices.DeploymentProperties{
			Model: &armcognitiveservices.DeploymentModel{Name: stringPointer("gpt-4o"), Version: stringPointer("2024-11-20")},
			RateLimits: []*armcognitiveservices.ThrottlingRule{
				{Key: stringPointer("token"), Count: &tokenCount, RenewalPeriod: &tokenPeriod},
				{Key: stringPointer("request"), Count: &requestCount, RenewalPeriod: &requestPeriod},
			},
		},
		SKU: &armcognitiveservices.SKU{Name: stringPointer("GlobalStandard"), Capacity: &capacity},
	})

	if row.TPM == nil || *row.TPM != 2_000_000 {
		t.Fatalf("TPM = %v, want 2000000", row.TPM)
	}
	if row.RPM == nil || *row.RPM != 20_000 {
		t.Fatalf("RPM = %v, want 20000", row.RPM)
	}
	if row.TPM != nil && *row.TPM == int64(capacity) {
		t.Fatal("TPM was taken from SKU capacity")
	}
	if row.Model != "gpt-4o" || row.ModelVersion != "2024-11-20" || row.SKU != "GlobalStandard" {
		t.Fatalf("deployment metadata = %#v", row)
	}

	thirtySecondCount := float32(10)
	thirtySecondPeriod := float32(30)
	if got := perMinuteQuantity(&thirtySecondCount, &thirtySecondPeriod); got == nil || *got != 20 {
		t.Fatalf("30-second quantity = %v, want 20", got)
	}
	unknownKey := "other"
	if tpm, rpm := quantityFromRateLimits([]*armcognitiveservices.ThrottlingRule{{Key: &unknownKey, Count: &tokenCount, RenewalPeriod: &tokenPeriod}}); tpm != nil || rpm != nil {
		t.Fatalf("unknown key quantities = %v, %v, want nil", tpm, rpm)
	}
}

func TestPerMinuteQuantityLeavesInvalidOrNonIntegralValuesUnknown(t *testing.T) {
	count := float32(1)
	period := float32(45)
	fractional := float32(1.5)
	zero := float32(0)
	cases := []struct {
		name   string
		count  *float32
		period *float32
	}{
		{name: "missing count", count: nil, period: &period},
		{name: "missing period", count: &count, period: nil},
		{name: "non integral per minute", count: &count, period: &period},
		{name: "fractional count", count: &fractional, period: &zero},
		{name: "zero period", count: &count, period: &zero},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := perMinuteQuantity(test.count, test.period); got != nil {
				t.Fatalf("quantity = %d, want nil", *got)
			}
		})
	}
}

func TestResourceIDAccountPartsUnescapesResourceGroupAndAccount(t *testing.T) {
	resourceGroup, accountName := resourceIDAccountParts("/subscriptions/sub/resourceGroups/rg%20one/providers/Microsoft.CognitiveServices/accounts/account%2Fname")
	if resourceGroup != "rg one" || accountName != "account/name" {
		t.Fatalf("parts = %q, %q", resourceGroup, accountName)
	}
}

func TestFetchAggregatesPartialResultsAndReportsCompletionOrder(t *testing.T) {
	accountA := &armcognitiveservices.Account{
		ID:       stringPointer("/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/account-a"),
		Name:     stringPointer("account-a"),
		Kind:     stringPointer(accountKindAIServices),
		Location: stringPointer("japaneast"),
	}
	accountB := &armcognitiveservices.Account{
		ID:       stringPointer("/subscriptions/sub-b/resourceGroups/rg-b/providers/Microsoft.CognitiveServices/accounts/account-b"),
		Name:     stringPointer("account-b"),
		Kind:     stringPointer(accountKindOpenAI),
		Location: stringPointer("swedencentral"),
	}
	clientA := &fakeSubscriptionClient{
		accounts:         []*armcognitiveservices.Account{accountA},
		deployments:      map[string][]*armcognitiveservices.Deployment{"account-a": {{Name: stringPointer("deployment-a")}}},
		deploymentErrors: map[string]error{},
	}
	clientB := &fakeSubscriptionClient{
		accounts:         []*armcognitiveservices.Account{accountB},
		deployments:      map[string][]*armcognitiveservices.Deployment{},
		deploymentErrors: map[string]error{"account-b": &azcore.ResponseError{ErrorCode: "Forbidden", StatusCode: 403}},
	}
	p := testProvider(t, []cliSubscription{
		{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"},
		{ID: "sub-b", Name: "Subscription B", TenantID: "tenant-b", TenantDisplayName: "Tenant B", CloudName: azureCloudName, State: "Enabled"},
	}, map[string]subscriptionClient{"sub-a": clientA, "sub-b": clientB})

	progress := make([]service.DeploymentProgress, 0)
	result, err := p.Fetch(context.Background(), func(snapshot service.DeploymentProgress) {
		progress = append(progress, snapshot)
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Deployments) != 1 || result.Deployments[0].Name != "deployment-a" {
		t.Fatalf("deployments = %#v, want deployment-a only", result.Deployments)
	}
	if len(result.Failures) != 1 || result.Failures[0].Scope != "account" || result.Failures[0].Code != "Forbidden" {
		t.Fatalf("failures = %#v, want one account Forbidden failure", result.Failures)
	}
	if result.SuccessfulAccounts != 1 || result.TotalAccounts == nil || *result.TotalAccounts != 2 {
		t.Fatalf("account totals = successful %d, total %v; want 1/2", result.SuccessfulAccounts, result.TotalAccounts)
	}
	if len(progress) != 6 {
		t.Fatalf("progress count = %d, want initial + 2 discovery + fetch start + 2 account", len(progress))
	}
	if progress[0].Stage != service.DeploymentProgressStageDiscovering || progress[0].CompletedSubscriptions != 0 {
		t.Fatalf("initial progress = %#v", progress[0])
	}
	if progress[1].CompletedSubscriptions != 1 || progress[2].CompletedSubscriptions != 2 {
		t.Fatalf("discovery completion counts = %d, %d", progress[1].CompletedSubscriptions, progress[2].CompletedSubscriptions)
	}
	if progress[1].Result.TotalAccounts != nil || progress[2].Result.TotalAccounts == nil || *progress[2].Result.TotalAccounts != 2 {
		t.Fatalf("discovery totals = %v, %v; want nil then 2", progress[1].Result.TotalAccounts, progress[2].Result.TotalAccounts)
	}
	if progress[3].Stage != service.DeploymentProgressStageFetching || progress[3].CompletedAccounts != 0 {
		t.Fatalf("fetch start progress = %#v", progress[3])
	}
	if progress[4].CompletedAccounts != 1 || len(progress[4].Result.Deployments) != 1 || progress[5].CompletedAccounts != 2 {
		t.Fatalf("account progress = %#v, %#v", progress[4], progress[5])
	}
}

func TestFetchExcludesForbiddenSubscriptionDiscoveryFromTarget(t *testing.T) {
	client := &fakeSubscriptionClient{
		accountsErr:      &azcore.ResponseError{ErrorCode: "AuthorizationFailed", StatusCode: 403},
		deploymentErrors: map[string]error{},
	}
	p := testProvider(t, []cliSubscription{{ID: "sub", Name: "Subscription", TenantID: "tenant", TenantDisplayName: "Tenant", CloudName: azureCloudName, State: "Enabled"}}, map[string]subscriptionClient{"sub": client})

	result, err := p.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Failures) != 0 || result.TotalAccounts == nil || *result.TotalAccounts != 0 || result.SuccessfulAccounts != 0 {
		t.Fatalf("excluded discovery result = %#v, want clean zero-account result", result)
	}
}

func TestFetchReturnsStructuredAzureCLIFailure(t *testing.T) {
	p := newProvider(providerOptions{
		runCommand: func(context.Context, ...string) ([]byte, []byte, error) {
			return nil, []byte("Please run az login to setup account"), errors.New("exit status 1")
		},
	})

	result, err := p.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Failures) != 1 || result.Failures[0].Code != "azure-cli-not-logged-in" || result.Failures[0].Scope != "azure-cli" {
		t.Fatalf("CLI failure = %#v", result.Failures)
	}
}

func TestDescribeErrorClassifiesAzureCLITokenFailure(t *testing.T) {
	code, _, _ := describeError(errors.New("AzureCLICredential: ERROR: AADSTS700082 refresh token has expired; run az login"))
	if code != "azure-cli-not-logged-in" {
		t.Fatalf("error code = %q, want azure-cli-not-logged-in", code)
	}
}

func stringPointer(value string) *string { return &value }
