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
	models           map[string][]*armcognitiveservices.AccountModel
	modelErrors      map[string]error
	regionModels     map[string][]*armcognitiveservices.Model
	regionModelErrs  map[string]error
	groups           []resourceGroupInfo
	groupsErr        error
}

func (c *fakeSubscriptionClient) listResourceGroups(ctx context.Context) ([]resourceGroupInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.groups, c.groupsErr
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

func (c *fakeSubscriptionClient) listRegionModels(ctx context.Context, location string) ([]*armcognitiveservices.Model, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.regionModelErrs[location]; err != nil {
		return nil, err
	}
	return c.regionModels[location], nil
}

func (c *fakeSubscriptionClient) listModels(ctx context.Context, _, accountName string) ([]*armcognitiveservices.AccountModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.modelErrors[accountName]; err != nil {
		return nil, err
	}
	return c.models[accountName], nil
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
                {"id":"sub-missing-state","name":"Missing state","tenantId":"tenant-a","cloudName":"AzureCloud"},
                {"id":"sub-empty-state","name":"Empty state","tenantId":"tenant-a","cloudName":"AzureCloud","state":""},
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

func TestFetchModelsTargetsSelectedAccountAndPreservesOptionalFields(t *testing.T) {
	accountID := "/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/account-a"
	lifecycle := armcognitiveservices.ModelLifecycleStatusGenerallyAvailable
	defaultVersion := true
	client := &fakeSubscriptionClient{
		models: map[string][]*armcognitiveservices.AccountModel{
			"account-a": {
				{Name: stringPointer("gpt-4o"), Format: stringPointer("OpenAI"), Version: stringPointer("2024-11-20"), LifecycleStatus: &lifecycle, IsDefaultVersion: &defaultVersion, SKUs: []*armcognitiveservices.ModelSKU{{Name: stringPointer("GlobalStandard"), Capacity: &armcognitiveservices.CapacityConfig{Default: int32Pointer(10), Minimum: int32Pointer(1), Maximum: int32Pointer(100), Step: int32Pointer(1), AllowedValues: []*int32{int32Pointer(1), nil, int32Pointer(10)}}}, {Name: stringPointer("Standard")}}},
				{Name: stringPointer("unknown-model")},
				nil,
			},
		},
		modelErrors: map[string]error{},
	}
	p := testProvider(t, []cliSubscription{{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"}}, map[string]subscriptionClient{"sub-a": client})

	result, err := p.FetchModels(context.Background(), accountID)
	if err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if len(result.Models) != 2 || len(result.Failures) != 0 || result.FetchedAt == "" {
		t.Fatalf("model result = %+v, want two candidates and no failures", result)
	}
	if result.Models[0].Name != "gpt-4o" || result.Models[0].Format != "OpenAI" || result.Models[0].Version != "2024-11-20" || result.Models[0].Lifecycle != "GenerallyAvailable" || !result.Models[0].IsDefaultVersion {
		t.Fatalf("mapped model = %+v", result.Models[0])
	}
	if len(result.Models[0].SKUs) != 2 || result.Models[0].SKUs[1].Name != "Standard" {
		t.Fatalf("mapped SKUs = %v", result.Models[0].SKUs)
	}
	capacity := result.Models[0].SKUs[0].Capacity
	if capacity == nil || *capacity.Default != 10 || *capacity.Minimum != 1 || *capacity.Maximum != 100 || *capacity.Step != 1 {
		t.Fatalf("mapped capacity = %+v, want every reported constraint", capacity)
	}
	if len(capacity.AllowedValues) != 2 || capacity.AllowedValues[1] != 10 {
		t.Fatalf("mapped allowed values = %v, want the two reported values", capacity.AllowedValues)
	}
	if result.Models[0].SKUs[1].Capacity != nil {
		t.Fatalf("Standard capacity = %+v, want no contract when Azure omits it", result.Models[0].SKUs[1].Capacity)
	}
	if result.Models[1].Format != "" || result.Models[1].Version != "" || result.Models[1].Lifecycle != "" || result.Models[1].SKUs == nil {
		t.Fatalf("mapped unknown fields = %+v", result.Models[1])
	}
}

func TestFetchPlacementsKeepsGroupsWithoutFoundriesAndReportsSubscriptionFailures(t *testing.T) {
	kind := "AIServices"
	other := "SpeechServices"
	location := "japaneast"
	accountID := "/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/account-a"
	otherID := "/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/speech-a"
	good := &fakeSubscriptionClient{
		groups: []resourceGroupInfo{{name: "rg-a", location: "japaneast"}, {name: "rg-empty", location: "eastus"}},
		accounts: []*armcognitiveservices.Account{
			{ID: &accountID, Kind: &kind, Location: &location},
			{ID: &otherID, Kind: &other, Location: &location},
			nil,
		},
	}
	bad := &fakeSubscriptionClient{groupsErr: errors.New("resource groups unavailable")}
	p := testProvider(t, []cliSubscription{
		{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"},
		{ID: "sub-b", Name: "Subscription B", TenantID: "tenant-b", TenantDisplayName: "Tenant B", CloudName: azureCloudName, State: "Enabled"},
	}, map[string]subscriptionClient{"sub-a": good, "sub-b": bad})

	result, err := p.FetchPlacements(context.Background())
	if err != nil {
		t.Fatalf("FetchPlacements returned error: %v", err)
	}
	if len(result.Subscriptions) != 1 || len(result.Failures) != 1 || result.FetchedAt == "" {
		t.Fatalf("placement result = %+v, want one subscription and one reported failure", result)
	}
	placement := result.Subscriptions[0]
	if placement.ID != "sub-a" || len(placement.Groups) != 2 || placement.Groups[1].Name != "rg-empty" {
		t.Fatalf("groups = %+v, want the group without a Foundry kept", placement)
	}
	if len(placement.Foundries) != 1 || placement.Foundries[0].Name != "account-a" || placement.Foundries[0].ResourceGroup != "rg-a" {
		t.Fatalf("foundries = %+v, want only the Foundry-kind account", placement.Foundries)
	}
}

func TestFetchDeploymentNamesReportsFailureInsteadOfAFreeName(t *testing.T) {
	client := &fakeSubscriptionClient{
		deployments: map[string][]*armcognitiveservices.Deployment{
			"account-a": {{Name: stringPointer("chat-production")}, nil, {Name: stringPointer("")}},
		},
		deploymentErrors: map[string]error{"account-b": errors.New("deployments unavailable")},
	}
	p := testProvider(t, []cliSubscription{{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"}}, map[string]subscriptionClient{"sub-a": client})

	result, err := p.FetchDeploymentNames(context.Background(), "/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/account-a")
	if err != nil {
		t.Fatalf("FetchDeploymentNames returned error: %v", err)
	}
	if len(result.Names) != 1 || result.Names[0] != "chat-production" || len(result.Failures) != 0 {
		t.Fatalf("names = %+v, want only the named deployment", result)
	}

	failed, err := p.FetchDeploymentNames(context.Background(), "/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.CognitiveServices/accounts/account-b")
	if err != nil {
		t.Fatalf("FetchDeploymentNames returned error: %v", err)
	}
	if len(failed.Names) != 0 || len(failed.Failures) != 1 {
		t.Fatalf("failed read = %+v, want no names and one failure", failed)
	}

	invalid, err := p.FetchDeploymentNames(context.Background(), "/subscriptions/sub-a/not-an-account")
	if err != nil || len(invalid.Failures) != 1 || invalid.Failures[0].Code != "invalid-account-id" {
		t.Fatalf("invalid target result = %+v, err = %v", invalid, err)
	}
}

func TestFetchRegionModelsKeepsFoundryKindsAndRejectsMissingTarget(t *testing.T) {
	lifecycle := armcognitiveservices.ModelLifecycleStatusGenerallyAvailable
	aiServices := "AIServices"
	speech := "SpeechServices"
	accountSKU := "S0"
	client := &fakeSubscriptionClient{
		regionModels: map[string][]*armcognitiveservices.Model{
			"japaneast": {
				{Kind: &aiServices, SKUName: &accountSKU, Model: &armcognitiveservices.AccountModel{
					Name: stringPointer("gpt-4o"), Format: stringPointer("OpenAI"), Version: stringPointer("2024-11-20"),
					LifecycleStatus: &lifecycle,
					SKUs: []*armcognitiveservices.ModelSKU{{Name: stringPointer("GlobalStandard"),
						Capacity: &armcognitiveservices.CapacityConfig{Default: int32Pointer(10), Minimum: int32Pointer(1), Maximum: int32Pointer(100), Step: int32Pointer(1)}}},
				}},
				{Kind: &speech, Model: &armcognitiveservices.AccountModel{Name: stringPointer("speech-model")}},
				{Kind: &aiServices},
				nil,
			},
		},
	}
	p := testProvider(t, []cliSubscription{{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"}}, map[string]subscriptionClient{"sub-a": client})

	result, err := p.FetchRegionModels(context.Background(), "sub-a", "japaneast")
	if err != nil {
		t.Fatalf("FetchRegionModels returned error: %v", err)
	}
	if len(result.Models) != 1 || len(result.Failures) != 0 || result.FetchedAt == "" {
		t.Fatalf("region result = %+v, want only the Foundry-kind candidate", result)
	}
	candidate := result.Models[0]
	if candidate.Name != "gpt-4o" || len(candidate.SKUs) != 1 || candidate.SKUs[0].Capacity == nil || *candidate.SKUs[0].Capacity.Default != 10 {
		t.Fatalf("mapped region candidate = %+v", candidate)
	}

	missing, err := p.FetchRegionModels(context.Background(), "sub-a", "")
	if err != nil || len(missing.Failures) != 1 || missing.Failures[0].Code != "invalid-region-target" {
		t.Fatalf("missing region result = %+v, err = %v", missing, err)
	}
}

func TestFetchModelsRejectsInvalidOrUnavailableAccountTarget(t *testing.T) {
	p := testProvider(t, []cliSubscription{{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"}}, map[string]subscriptionClient{"sub-a": &fakeSubscriptionClient{}})
	for _, test := range []struct {
		name      string
		accountID string
		code      string
	}{
		{name: "invalid", accountID: "/subscriptions/sub-a/not-an-account", code: "invalid-account-id"},
		{name: "unavailable-subscription", accountID: "/subscriptions/sub-b/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account", code: "subscription-not-available"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := p.FetchModels(context.Background(), test.accountID)
			if err != nil || len(result.Failures) != 1 || result.Failures[0].Code != test.code {
				t.Fatalf("result = %+v, err=%v; want %s failure", result, err, test.code)
			}
		})
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

func TestFetchExcludesForbiddenSubscriptionDiscoveryFromProgress(t *testing.T) {
	clients := map[string]subscriptionClient{
		"sub-a": &fakeSubscriptionClient{accountsErr: &azcore.ResponseError{ErrorCode: "AuthorizationFailed", StatusCode: 403}},
		"sub-b": &fakeSubscriptionClient{accountsErr: &azcore.ResponseError{ErrorCode: "AuthorizationFailed", StatusCode: 403}},
		"sub-c": &fakeSubscriptionClient{accountsErr: &azcore.ResponseError{ErrorCode: "AuthorizationFailed", StatusCode: 403}},
	}
	p := testProvider(t, []cliSubscription{
		{ID: "sub-a", Name: "Subscription A", TenantID: "tenant-a", TenantDisplayName: "Tenant A", CloudName: azureCloudName, State: "Enabled"},
		{ID: "sub-b", Name: "Subscription B", TenantID: "tenant-b", TenantDisplayName: "Tenant B", CloudName: azureCloudName, State: "Enabled"},
		{ID: "sub-c", Name: "Subscription C", TenantID: "tenant-c", TenantDisplayName: "Tenant C", CloudName: azureCloudName, State: "Enabled"},
	}, clients)

	progress := make([]service.DeploymentProgress, 0)
	result, err := p.Fetch(context.Background(), func(snapshot service.DeploymentProgress) {
		progress = append(progress, snapshot)
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Failures) != 0 || result.SuccessfulAccounts != 0 || result.TotalAccounts == nil || *result.TotalAccounts != 0 {
		t.Fatalf("all-forbidden result = %#v, want clean zero-account result", result)
	}
	if len(progress) != 2 {
		t.Fatalf("all-forbidden progress count = %d, want initial discovery and fetch-start only; progress = %#v", len(progress), progress)
	}
	if progress[0].Stage != service.DeploymentProgressStageDiscovering || progress[0].CompletedSubscriptions != 0 || progress[0].TotalSubscriptions != nil {
		t.Fatalf("initial progress = %#v", progress[0])
	}
	if progress[1].Stage != service.DeploymentProgressStageFetching || progress[1].CompletedSubscriptions != 0 || progress[1].TotalSubscriptions == nil || *progress[1].TotalSubscriptions != 0 {
		t.Fatalf("fetch-start progress = %#v, want 0/0 after excluding all forbidden subscriptions", progress[1])
	}
}

func TestFetchSeparatesForbiddenSubscriptionFromMixedDiscoveryProgress(t *testing.T) {
	account := &armcognitiveservices.Account{
		ID:       stringPointer("/subscriptions/sub-success/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/account"),
		Name:     stringPointer("account"),
		Kind:     stringPointer(accountKindAIServices),
		Location: stringPointer("japaneast"),
	}

	run := func(t *testing.T, forbiddenLast bool) {
		t.Helper()
		forbiddenID := "sub-forbidden"
		forbiddenTenant := "Tenant A"
		successID := "sub-success"
		successTenant := "Tenant B"
		failureID := "sub-auth-failure"
		failureTenant := "Tenant C"
		if forbiddenLast {
			forbiddenTenant = "Tenant C"
			successTenant = "Tenant A"
			failureTenant = "Tenant B"
		}
		clients := map[string]subscriptionClient{
			forbiddenID: &fakeSubscriptionClient{accountsErr: &azcore.ResponseError{ErrorCode: "AuthorizationFailed", StatusCode: 403}},
			successID: &fakeSubscriptionClient{
				accounts:         []*armcognitiveservices.Account{account},
				deployments:      map[string][]*armcognitiveservices.Deployment{"account": {{Name: stringPointer("deployment")}}},
				deploymentErrors: map[string]error{},
			},
			failureID: &fakeSubscriptionClient{accountsErr: &azcore.ResponseError{ErrorCode: "Unauthorized", StatusCode: 401}},
		}
		p := testProvider(t, []cliSubscription{
			{ID: forbiddenID, Name: forbiddenID, TenantID: forbiddenTenant, TenantDisplayName: forbiddenTenant, CloudName: azureCloudName, State: "Enabled"},
			{ID: successID, Name: successID, TenantID: successTenant, TenantDisplayName: successTenant, CloudName: azureCloudName, State: "Enabled"},
			{ID: failureID, Name: failureID, TenantID: failureTenant, TenantDisplayName: failureTenant, CloudName: azureCloudName, State: "Enabled"},
		}, clients)

		progress := make([]service.DeploymentProgress, 0)
		result, err := p.Fetch(context.Background(), func(snapshot service.DeploymentProgress) {
			progress = append(progress, snapshot)
		})
		if err != nil {
			t.Fatalf("Fetch returned error: %v", err)
		}
		if len(result.Deployments) != 1 || result.Deployments[0].Name != "deployment" {
			t.Fatalf("deployments = %#v, want successful deployment only", result.Deployments)
		}
		if len(result.Failures) != 1 || result.Failures[0].Scope != "subscription" || result.Failures[0].Code != "Unauthorized" {
			t.Fatalf("failures = %#v, want one authentication failure", result.Failures)
		}
		if result.SuccessfulAccounts != 1 || result.TotalAccounts != nil {
			t.Fatalf("account totals = successful %d, total %v; want 1/unknown after discovery failure", result.SuccessfulAccounts, result.TotalAccounts)
		}

		if len(progress) != 5 {
			t.Fatalf("mixed progress count = %d, want initial + 2 eligible discoveries + fetch start + account; progress = %#v", len(progress), progress)
		}
		if progress[0].Stage != service.DeploymentProgressStageDiscovering || progress[0].TotalSubscriptions != nil {
			t.Fatalf("initial progress = %#v", progress[0])
		}
		if progress[1].CompletedSubscriptions != 1 || progress[1].TotalSubscriptions != nil {
			t.Fatalf("first eligible discovery progress = %#v, want 1/unknown", progress[1])
		}
		if forbiddenLast {
			if progress[2].CompletedSubscriptions != 2 || progress[2].TotalSubscriptions != nil {
				t.Fatalf("discovery before final forbidden response = %#v, want 2/unknown", progress[2])
			}
		} else if progress[2].CompletedSubscriptions != 2 || progress[2].TotalSubscriptions == nil || *progress[2].TotalSubscriptions != 2 {
			t.Fatalf("final eligible discovery progress = %#v, want 2/2", progress[2])
		}
		fetchStart := progress[3]
		if fetchStart.Stage != service.DeploymentProgressStageFetching || fetchStart.CompletedSubscriptions != 2 || fetchStart.TotalSubscriptions == nil || *fetchStart.TotalSubscriptions != 2 {
			t.Fatalf("fetch-start progress = %#v, want 2/2 after excluding forbidden subscription", fetchStart)
		}
		if fetchStart.Result.TotalAccounts != nil || len(fetchStart.Result.Failures) != 1 {
			t.Fatalf("fetch-start result = %#v, want authentication failure and unknown account total", fetchStart.Result)
		}
		accountProgress := progress[4]
		if accountProgress.CompletedAccounts != 1 || accountProgress.Result.SuccessfulAccounts != 1 || accountProgress.Result.TotalAccounts != nil || len(accountProgress.Result.Deployments) != 1 || len(accountProgress.Result.Failures) != 1 {
			t.Fatalf("account progress = %#v, want successful row, unknown total, and retained authentication failure", accountProgress)
		}
	}

	t.Run("forbidden-first", func(t *testing.T) { run(t, false) })
	t.Run("forbidden-last", func(t *testing.T) { run(t, true) })
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

func int32Pointer(value int32) *int32 { return &value }
