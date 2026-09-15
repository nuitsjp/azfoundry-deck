// Package azurego contains the Azure boundary used by the F1 and F2 screens.
//
// Azure CLI is used only for the already signed-in account list and for token
// acquisition through AzureCLICredential. Resource reads are sent directly to
// Azure Resource Manager by the official Azure SDK for Go.
package azurego

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

const (
	defaultSubscriptionConcurrency = 4
	defaultAccountConcurrency      = 8
	defaultOperationTimeout        = 2 * time.Minute
	defaultCLICommandTimeout       = 30 * time.Second

	azureCloudName = "AzureCloud"

	accountKindAIServices = "AIServices"
	accountKindOpenAI     = "OpenAI"

	resourcesAPIVersion   = "2021-04-01"
	deploymentsAPIVersion = "2025-09-01" // Matches armcognitiveservices/v3 v3.0.0.
	modelsAPIVersion      = "2025-09-01" // Matches armcognitiveservices/v3 v3.0.0.
)

// Provider is the real Azure boundary for F1/F2. It discovers subscriptions from
// the signed-in Azure CLI account list, then reads accounts and deployments
// with the ARM SDK. Each request carries its own tenant and subscription.
type Provider struct {
	subscriptionConcurrency int
	accountConcurrency      int
	operationTimeout        time.Duration
	cliCommandTimeout       time.Duration
	runCommand              commandRunner
	newClient               subscriptionClientFactory
	clientsMu               sync.Mutex
	clients                 map[subscriptionClientKey]subscriptionClient
}

type providerOptions struct {
	subscriptionConcurrency int
	accountConcurrency      int
	operationTimeout        time.Duration
	cliCommandTimeout       time.Duration
	runCommand              commandRunner
	newClient               subscriptionClientFactory
}

// NewProvider creates the real Azure provider without making a network call.
// Authentication and resource reads start only when Fetch is called.
func NewProvider() *Provider {
	return newProvider(providerOptions{})
}

func newProvider(options providerOptions) *Provider {
	if options.subscriptionConcurrency < 1 {
		options.subscriptionConcurrency = defaultSubscriptionConcurrency
	}
	if options.accountConcurrency < 1 {
		options.accountConcurrency = defaultAccountConcurrency
	}
	if options.operationTimeout <= 0 {
		options.operationTimeout = defaultOperationTimeout
	}
	if options.cliCommandTimeout <= 0 {
		options.cliCommandTimeout = defaultCLICommandTimeout
	}
	if options.runCommand == nil {
		options.runCommand = runAzureCLICommand
	}
	if options.newClient == nil {
		options.newClient = newSDKSubscriptionClient
	}
	return &Provider{
		subscriptionConcurrency: options.subscriptionConcurrency,
		accountConcurrency:      options.accountConcurrency,
		operationTimeout:        options.operationTimeout,
		cliCommandTimeout:       options.cliCommandTimeout,
		runCommand:              options.runCommand,
		newClient:               options.newClient,
		clients:                 make(map[subscriptionClientKey]subscriptionClient),
	}
}

type commandRunner func(context.Context, ...string) ([]byte, []byte, error)

type subscriptionClientFactory func(subscriptionInfo) (subscriptionClient, error)

// subscriptionClient is deliberately smaller than the SDK client. It keeps
// the screen-facing service independent of SDK model types while allowing the
// Azure boundary to test aggregation and error handling without live Azure.
type subscriptionClient interface {
	listAccounts(context.Context) ([]*armcognitiveservices.Account, error)
	listDeployments(context.Context, string, string) ([]*armcognitiveservices.Deployment, error)
	listModels(context.Context, string, string) ([]*armcognitiveservices.AccountModel, error)
	listRegionModels(context.Context, string) ([]*armcognitiveservices.Model, error)
	listResourceGroups(context.Context) ([]resourceGroupInfo, error)
}

// resourceGroupInfo is the subset of a resource group the placement read needs.
type resourceGroupInfo struct {
	name     string
	location string
}

type subscriptionInfo struct {
	id         string
	name       string
	tenantID   string
	tenantName string
	cloudName  string
	state      string
	user       cliUser
}

type cliUser struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type cliSubscription struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	TenantID          string  `json:"tenantId"`
	TenantDisplayName string  `json:"tenantDisplayName"`
	CloudName         string  `json:"cloudName"`
	State             string  `json:"state"`
	User              cliUser `json:"user"`
}

type subscriptionClientKey struct {
	tenantID       string
	subscriptionID string
	user           cliUser
}

func clientKey(subscription subscriptionInfo) subscriptionClientKey {
	return subscriptionClientKey{
		tenantID:       strings.ToLower(subscription.tenantID),
		subscriptionID: strings.ToLower(subscription.id),
		user:           subscription.user,
	}
}

// Only SDK clients (and their in-memory token caches) survive a Fetch. The CLI
// account list and all resource data are read again on every update.
func (p *Provider) retainClients(subscriptions []subscriptionInfo) {
	active := make(map[subscriptionClientKey]bool, len(subscriptions))
	for _, subscription := range subscriptions {
		active[clientKey(subscription)] = true
	}
	p.clientsMu.Lock()
	defer p.clientsMu.Unlock()
	for key := range p.clients {
		if !active[key] {
			delete(p.clients, key)
		}
	}
}

func (p *Provider) subscriptionClient(subscription subscriptionInfo) (subscriptionClient, error) {
	p.clientsMu.Lock()
	defer p.clientsMu.Unlock()
	key := clientKey(subscription)
	if client, ok := p.clients[key]; ok {
		return client, nil
	}
	// Construction does not perform I/O. Keep it inside the lock so concurrent
	// refreshes share one SDK pipeline, without serializing network requests.
	client, err := p.newClient(subscription)
	if err != nil {
		return nil, err
	}
	p.clients[key] = client
	return client, nil
}

type subscriptionDiscovery struct {
	subscription subscriptionInfo
	client       subscriptionClient
	accounts     []discoveredAccount
	err          error
}

type discoveredAccount struct {
	subscription  subscriptionInfo
	client        subscriptionClient
	id            string
	name          string
	resourceGroup string
	region        string
	invalidErr    error
}

type accountFetch struct {
	account     discoveredAccount
	deployments []*armcognitiveservices.Deployment
	err         error
}

type commandError struct {
	stderr string
	err    error
}

func (e *commandError) Error() string {
	if strings.TrimSpace(e.stderr) == "" {
		if e.err == nil {
			return "Azure CLI command failed"
		}
		return e.err.Error()
	}
	if e.err == nil {
		return strings.TrimSpace(e.stderr)
	}
	return fmt.Sprintf("%v: %s", e.err, strings.TrimSpace(e.stderr))
}

func (e *commandError) Unwrap() error { return e.err }

// Fetch obtains the cross-subscription F1 result and reports immutable
// snapshots as each subscription and account completes.
func (p *Provider) Fetch(ctx context.Context, report func(service.DeploymentProgress)) (service.DeploymentResult, error) {
	if err := ctx.Err(); err != nil {
		return emptyResult(), err
	}

	reportSnapshot(report, service.DeploymentProgress{
		Stage:  service.DeploymentProgressStageDiscovering,
		Result: emptyResult(),
	})

	commandCtx, cancel := context.WithTimeout(ctx, p.cliCommandTimeout)
	subscriptions, err := p.listSubscriptions(commandCtx)
	cancel()
	if err != nil {
		p.retainClients(nil)
		if ctx.Err() != nil {
			return emptyResult(), ctx.Err()
		}
		result := emptyResult()
		result.Failures = []service.FetchFailure{failureForError("azure-cli", subscriptionInfo{}, discoveredAccount{}, err)}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	p.retainClients(subscriptions)

	failures := make([]service.FetchFailure, 0)
	accounts := make([]discoveredAccount, 0)
	discoveryComplete := true
	totalSubscriptionResponses := len(subscriptions)
	completedSubscriptionResponses := 0
	completedSubscriptions := 0
	var totalSub *int
	processDiscovery := func(discovery subscriptionDiscovery) {
		completedSubscriptionResponses++
		if discovery.err != nil && isExcludedTenantPermission(discovery.err) {
			// A tenant not authorized for subscription discovery is outside
			// F1's target set and is intentionally absent from the result and
			// progress counts.
			if completedSubscriptionResponses == totalSubscriptionResponses {
				total := completedSubscriptions
				totalSub = &total
			}
			return
		}

		completedSubscriptions++
		if discovery.err != nil {
			discoveryComplete = false
			failures = append(failures, failureForError("subscription", discovery.subscription, discoveredAccount{}, discovery.err))
		} else {
			accounts = append(accounts, discovery.accounts...)
		}

		if completedSubscriptionResponses == totalSubscriptionResponses {
			total := completedSubscriptions
			totalSub = &total
		}
		progressResult := emptyResult()
		progressResult.Failures = cloneFailures(failures)
		if completedSubscriptionResponses == totalSubscriptionResponses && discoveryComplete {
			deduplicated := deduplicateAccounts(accounts)
			total := len(deduplicated)
			progressResult.TotalAccounts = &total
		}
		reportSnapshot(report, service.DeploymentProgress{
			Stage:                  service.DeploymentProgressStageDiscovering,
			CompletedSubscriptions: completedSubscriptions,
			TotalSubscriptions:     cloneIntPointer(totalSub),
			Result:                 progressResult,
		})
	}
	p.discoverSubscriptions(ctx, subscriptions, processDiscovery)
	if err := ctx.Err(); err != nil {
		return emptyResult(), err
	}
	if totalSubscriptionResponses == 0 {
		total := 0
		totalSub = &total
		progressResult := emptyResult()
		progressResult.TotalAccounts = &total
		reportSnapshot(report, service.DeploymentProgress{
			Stage:              service.DeploymentProgressStageDiscovering,
			TotalSubscriptions: cloneIntPointer(totalSub),
			Result:             progressResult,
		})
	}

	accounts = deduplicateAccounts(accounts)
	var totalAccounts *int
	if discoveryComplete {
		total := len(accounts)
		totalAccounts = &total
	}

	fetchStart := emptyResult()
	fetchStart.Failures = cloneFailures(failures)
	fetchStart.TotalAccounts = cloneIntPointer(totalAccounts)
	reportSnapshot(report, service.DeploymentProgress{
		Stage:                  service.DeploymentProgressStageFetching,
		CompletedSubscriptions: completedSubscriptions,
		TotalSubscriptions:     cloneIntPointer(totalSub),
		Result:                 fetchStart,
	})

	deployments := make([]service.Deployment, 0)
	successfulAccounts := 0
	completedAccounts := 0
	processAccount := func(fetched accountFetch) {
		completedAccounts++
		if fetched.err != nil {
			if errors.Is(fetched.err, context.Canceled) && ctx.Err() != nil {
				return
			}
			failures = append(failures, failureForError("account", fetched.account.subscription, fetched.account, fetched.err))
		} else {
			successfulAccounts++
			for _, deployment := range fetched.deployments {
				if deployment != nil {
					deployments = append(deployments, deploymentRow(fetched.account, deployment))
				}
			}
		}

		snapshot := emptyResult()
		snapshot.Deployments = cloneDeployments(deployments)
		snapshot.Failures = cloneFailures(failures)
		snapshot.SuccessfulAccounts = successfulAccounts
		snapshot.TotalAccounts = cloneIntPointer(totalAccounts)
		reportSnapshot(report, service.DeploymentProgress{
			Stage:                  service.DeploymentProgressStageFetching,
			CompletedSubscriptions: completedSubscriptions,
			CompletedAccounts:      completedAccounts,
			TotalSubscriptions:     cloneIntPointer(totalSub),
			Result:                 snapshot,
		})
	}
	p.fetchAccounts(ctx, accounts, processAccount)
	if err := ctx.Err(); err != nil {
		return emptyResult(), err
	}

	result := emptyResult()
	result.Deployments = deployments
	result.Failures = failures
	result.SuccessfulAccounts = successfulAccounts
	result.TotalAccounts = totalAccounts
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// FetchModels reads the model candidates for the exact account selected from
// the F1 result. The account resource ID is parsed and checked before any ARM
// request is made; the subscription is then resolved from the same signed-in
// Azure CLI account list used by F1.
func (p *Provider) FetchModels(ctx context.Context, accountID string) (service.ModelResult, error) {
	result := emptyModelResult()
	if err := ctx.Err(); err != nil {
		return result, err
	}

	subscriptionID, resourceGroup, accountName := resourceIDTargetParts(accountID)
	account := discoveredAccount{id: accountID, name: accountName, resourceGroup: resourceGroup}
	if subscriptionID == "" || resourceGroup == "" || accountName == "" {
		result.Failures = []service.FetchFailure{{
			Scope:       "account",
			AccountName: accountName,
			Code:        "invalid-account-id",
			Message:     "選択したアカウントの ARM リソース ID が不正です。",
			Action:      "デプロイ一覧に戻って対象アカウントを選び直してください。",
		}}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	subscription, client, failure, err := p.resolveSubscriptionClient(ctx, subscriptionID, account)
	if err != nil {
		return result, err
	}
	if failure != nil {
		result.Failures = []service.FetchFailure{*failure}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	account.subscription = subscription
	account.client = client
	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	models, err := client.listModels(operationCtx, resourceGroup, accountName)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Failures = []service.FetchFailure{failureForError("account", subscription, account, err)}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	result.Models = modelCandidates(models)
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// FetchRegionModels reads the candidates a subscription can deploy in one
// region. It is used before a new Foundry exists, so the result is provisional:
// the same account-scoped read is repeated after the Foundry is created.
func (p *Provider) FetchRegionModels(ctx context.Context, subscriptionID, region string) (service.ModelResult, error) {
	result := emptyModelResult()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if subscriptionID == "" || region == "" {
		result.Failures = []service.FetchFailure{{
			Scope:   "region",
			Code:    "invalid-region-target",
			Message: "モデル候補を取得するサブスクリプションとリージョンが指定されていません。",
			Action:  "配置先を選び直してください。",
		}}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	subscription, client, failure, err := p.resolveSubscriptionClient(ctx, subscriptionID, discoveredAccount{})
	if err != nil {
		return result, err
	}
	if failure != nil {
		result.Failures = []service.FetchFailure{*failure}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	models, err := client.listRegionModels(operationCtx, region)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Failures = []service.FetchFailure{failureForError("region", subscription, discoveredAccount{}, err)}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	result.Models = regionModelCandidates(models)
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// resolveSubscriptionClient turns a subscription ID into the signed-in
// subscription and its ARM client. A returned failure is a screen-facing reason
// for stopping; a returned error is the caller's cancelled context.
func (p *Provider) resolveSubscriptionClient(ctx context.Context, subscriptionID string, account discoveredAccount) (subscriptionInfo, subscriptionClient, *service.FetchFailure, error) {
	commandCtx, cancel := context.WithTimeout(ctx, p.cliCommandTimeout)
	subscriptions, err := p.listSubscriptions(commandCtx)
	cancel()
	if err != nil {
		p.retainClients(nil)
		if ctx.Err() != nil {
			return subscriptionInfo{}, nil, nil, ctx.Err()
		}
		failure := failureForError("azure-cli", subscriptionInfo{}, account, err)
		return subscriptionInfo{}, nil, &failure, nil
	}
	p.retainClients(subscriptions)

	var subscription subscriptionInfo
	for _, candidate := range subscriptions {
		if strings.EqualFold(candidate.id, subscriptionID) {
			subscription = candidate
			break
		}
	}
	if subscription.id == "" {
		failure := service.FetchFailure{
			Scope:            "account",
			SubscriptionName: subscriptionID,
			AccountName:      account.name,
			Code:             "subscription-not-available",
			Message:          "対象アカウントのサブスクリプションを現在の Azure CLI セッションから確認できません。",
			Action:           "Azure CLI のサインイン先とサブスクリプションへのアクセス権を確認してから再試行してください。",
		}
		return subscriptionInfo{}, nil, &failure, nil
	}

	client, err := p.subscriptionClient(subscription)
	if err != nil {
		failure := failureForError("account", subscription, account, err)
		return subscriptionInfo{}, nil, &failure, nil
	}
	return subscription, client, nil, nil
}

// FetchPlacements reads every place a model can be added to: the signed-in
// subscriptions, their resource groups, and the Foundry accounts in them. A
// Foundry with no deployment is included, so this read does not go through the
// deployment list.
func (p *Provider) FetchPlacements(ctx context.Context) (service.PlacementResult, error) {
	result := emptyPlacementResult()
	if err := ctx.Err(); err != nil {
		return result, err
	}

	commandCtx, cancel := context.WithTimeout(ctx, p.cliCommandTimeout)
	subscriptions, err := p.listSubscriptions(commandCtx)
	cancel()
	if err != nil {
		p.retainClients(nil)
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Failures = []service.FetchFailure{failureForError("azure-cli", subscriptionInfo{}, discoveredAccount{}, err)}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	p.retainClients(subscriptions)

	reads := make([]placementRead, len(subscriptions))
	var wait sync.WaitGroup
	workers := workerCount(p.subscriptionConcurrency, len(subscriptions))
	jobs := make(chan int)
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wait.Done()
			for index := range jobs {
				reads[index] = p.readPlacement(ctx, subscriptions[index])
			}
		}()
	}
	for index := range subscriptions {
		select {
		case jobs <- index:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return emptyPlacementResult(), err
	}

	for _, read := range reads {
		if read.err != nil {
			result.Failures = append(result.Failures, failureForError("subscription", read.subscription, discoveredAccount{}, read.err))
			continue
		}
		result.Subscriptions = append(result.Subscriptions, read.placement)
	}
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

type placementRead struct {
	subscription subscriptionInfo
	placement    service.PlacementSubscription
	err          error
}

func (p *Provider) readPlacement(ctx context.Context, subscription subscriptionInfo) placementRead {
	read := placementRead{subscription: subscription}
	client, err := p.subscriptionClient(subscription)
	if err != nil {
		read.err = err
		return read
	}
	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	groups, err := client.listResourceGroups(operationCtx)
	cancel()
	if err != nil {
		read.err = err
		return read
	}
	operationCtx, cancel = context.WithTimeout(ctx, p.operationTimeout)
	accounts, err := client.listAccounts(operationCtx)
	cancel()
	if err != nil {
		read.err = err
		return read
	}

	placement := service.PlacementSubscription{
		ID:         subscription.id,
		Name:       subscription.name,
		TenantName: subscription.tenantName,
		Groups:     make([]service.PlacementGroup, 0, len(groups)),
		Foundries:  make([]service.PlacementFoundry, 0, len(accounts)),
	}
	for _, group := range groups {
		placement.Groups = append(placement.Groups, service.PlacementGroup{Name: group.name, Location: group.location})
	}
	for _, account := range accounts {
		if account == nil || !isFoundryAccount(account.Kind) {
			continue
		}
		id := value(account.ID)
		resourceGroup, name := resourceIDAccountParts(id)
		if id == "" || resourceGroup == "" || name == "" {
			continue
		}
		placement.Foundries = append(placement.Foundries, service.PlacementFoundry{
			ID:            id,
			Name:          name,
			ResourceGroup: resourceGroup,
			Region:        value(account.Location),
		})
	}
	read.placement = placement
	return read
}

// FetchDeploymentNames reads the deployment names of one Foundry. The screen
// uses it to reject an existing name; a failure is reported instead of being
// treated as "the name is free".
func (p *Provider) FetchDeploymentNames(ctx context.Context, accountID string) (service.DeploymentNameResult, error) {
	result := service.DeploymentNameResult{Names: make([]string, 0), Failures: make([]service.FetchFailure, 0)}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	subscriptionID, resourceGroup, accountName := resourceIDTargetParts(accountID)
	account := discoveredAccount{id: accountID, name: accountName, resourceGroup: resourceGroup}
	if subscriptionID == "" || resourceGroup == "" || accountName == "" {
		result.Failures = []service.FetchFailure{{
			Scope:       "account",
			AccountName: accountName,
			Code:        "invalid-account-id",
			Message:     "選択したFoundryの ARM リソース ID が不正です。",
			Action:      "配置先を選び直してください。",
		}}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	subscription, client, failure, err := p.resolveSubscriptionClient(ctx, subscriptionID, account)
	if err != nil {
		return result, err
	}
	if failure != nil {
		result.Failures = []service.FetchFailure{*failure}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	deployments, err := client.listDeployments(operationCtx, resourceGroup, accountName)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Failures = []service.FetchFailure{failureForError("account", subscription, account, err)}
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}
	for _, deployment := range deployments {
		if deployment == nil {
			continue
		}
		if name := value(deployment.Name); name != "" {
			result.Names = append(result.Names, name)
		}
	}
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func (p *Provider) listSubscriptions(ctx context.Context) ([]subscriptionInfo, error) {
	stdout, stderr, err := p.runCommand(ctx, "account", "list", "--all", "--only-show-errors", "--output", "json")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &commandError{stderr: string(stderr), err: err}
	}
	var values []cliSubscription
	if err := json.Unmarshal(stdout, &values); err != nil {
		return nil, fmt.Errorf("Azure CLI のサブスクリプション一覧を解析できません: %w", err)
	}

	byKey := make(map[string]subscriptionInfo, len(values))
	for _, value := range values {
		if !strings.EqualFold(strings.TrimSpace(value.CloudName), azureCloudName) {
			continue
		}
		if !strings.EqualFold(value.State, "Enabled") {
			continue
		}
		id := strings.TrimSpace(value.ID)
		tenantID := strings.TrimSpace(value.TenantID)
		if id == "" || tenantID == "" {
			continue
		}
		// Azure CLI includes one tenant-level pseudo-account per tenant when
		// --all is used. It has the tenant ID in both fields and is not an
		// ARM subscription that can contain target accounts.
		if strings.EqualFold(id, tenantID) {
			continue
		}
		name := strings.TrimSpace(value.Name)
		if name == "" {
			name = id
		}
		tenantName := strings.TrimSpace(value.TenantDisplayName)
		if tenantName == "" {
			tenantName = tenantID
		}
		item := subscriptionInfo{
			id:         id,
			name:       name,
			tenantID:   tenantID,
			tenantName: tenantName,
			cloudName:  value.CloudName,
			state:      value.State,
			user:       value.User,
		}
		key := strings.ToLower(tenantID + "/" + id)
		byKey[key] = item
	}

	result := make([]subscriptionInfo, 0, len(byKey))
	for _, value := range byKey {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].tenantName != result[j].tenantName {
			return result[i].tenantName < result[j].tenantName
		}
		if result[i].name != result[j].name {
			return result[i].name < result[j].name
		}
		return result[i].id < result[j].id
	})
	return result, nil
}

func (p *Provider) discoverSubscriptions(ctx context.Context, subscriptions []subscriptionInfo, report func(subscriptionDiscovery)) {
	if len(subscriptions) == 0 {
		return
	}

	jobs := make(chan subscriptionInfo)
	results := make(chan subscriptionDiscovery, len(subscriptions))
	workers := workerCount(p.subscriptionConcurrency, len(subscriptions))
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wait.Done()
			for subscription := range jobs {
				results <- p.discoverSubscription(ctx, subscription)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, subscription := range subscriptions {
			select {
			case jobs <- subscription:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wait.Wait()
		close(results)
	}()

	for result := range results {
		if report != nil {
			report(result)
		}
	}
}

func (p *Provider) discoverSubscription(ctx context.Context, subscription subscriptionInfo) subscriptionDiscovery {
	result := subscriptionDiscovery{subscription: subscription}
	if err := ctx.Err(); err != nil {
		result.err = err
		return result
	}
	client, err := p.subscriptionClient(subscription)
	if err != nil {
		result.err = err
		return result
	}
	result.client = client
	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	accounts, err := client.listAccounts(operationCtx)
	cancel()
	if err != nil {
		result.err = err
		return result
	}

	result.accounts = make([]discoveredAccount, 0, len(accounts))
	for _, account := range accounts {
		if account == nil || !isFoundryAccount(account.Kind) {
			continue
		}
		found, err := discoveredAccountFromSDK(subscription, client, account)
		if err != nil {
			found.invalidErr = err
		}
		result.accounts = append(result.accounts, found)
	}
	return result
}

func (p *Provider) fetchAccounts(ctx context.Context, accounts []discoveredAccount, report func(accountFetch)) {
	if len(accounts) == 0 {
		return
	}

	jobs := make(chan discoveredAccount)
	results := make(chan accountFetch, len(accounts))
	workers := workerCount(p.accountConcurrency, len(accounts))
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wait.Done()
			for account := range jobs {
				results <- p.fetchAccount(ctx, account)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, account := range accounts {
			select {
			case jobs <- account:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wait.Wait()
		close(results)
	}()

	for result := range results {
		if report != nil {
			report(result)
		}
	}
}

func (p *Provider) fetchAccount(ctx context.Context, account discoveredAccount) accountFetch {
	result := accountFetch{account: account}
	if account.invalidErr != nil {
		result.err = account.invalidErr
		return result
	}
	if err := ctx.Err(); err != nil {
		result.err = err
		return result
	}
	operationCtx, cancel := context.WithTimeout(ctx, p.operationTimeout)
	result.deployments, result.err = account.client.listDeployments(operationCtx, account.resourceGroup, account.name)
	cancel()
	return result
}

type sdkSubscriptionClient struct {
	client         *arm.Client
	subscriptionID string
}

func newSDKSubscriptionClient(subscription subscriptionInfo) (subscriptionClient, error) {
	// Azure CLI rejects a token request carrying both --tenant and
	// --subscription. A subscription ID is globally unique and selects the
	// corresponding tenant without changing the CLI's default account.
	credential, err := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{
		Subscription: subscription.id,
	})
	if err != nil {
		return nil, fmt.Errorf("Azure CLI 資格情報を初期化できません: %w", err)
	}
	clientOptions := &arm.ClientOptions{DisableRPRegistration: true}
	client, err := arm.NewClient("github.com/nuitsjp/azfoundry-deck", "v0.0.0", credential, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("Azure ARM SDK を初期化できません: %w", err)
	}
	return &sdkSubscriptionClient{
		client:         client,
		subscriptionID: subscription.id,
	}, nil
}

func (c *sdkSubscriptionClient) listAccounts(ctx context.Context) ([]*armcognitiveservices.Account, error) {
	// Resources List avoids the extra provider-specific account pages. Only
	// resource metadata is needed here; kind filtering remains in discovery.
	query := url.Values{
		"api-version": {resourcesAPIVersion},
		"$filter":     {"resourceType eq 'Microsoft.CognitiveServices/accounts'"},
	}
	next := c.client.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) + "/resources?" + query.Encode()
	result := make([]*armcognitiveservices.Account, 0)
	for next != "" {
		var page armcognitiveservices.AccountListResult
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, fmt.Errorf("Cognitive Services アカウント一覧の取得に失敗しました: %w", err)
		}
		result = append(result, page.Value...)
		next = value(page.NextLink)
	}
	return result, nil
}

func (c *sdkSubscriptionClient) listDeployments(ctx context.Context, resourceGroup, accountName string) ([]*armcognitiveservices.Deployment, error) {
	// Both lists use the same standard ARM pipeline, including retry, token
	// caching and cancellation. The deployment API and SDK model stay unchanged.
	next := c.client.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) +
		"/resourceGroups/" + url.PathEscape(resourceGroup) + "/providers/Microsoft.CognitiveServices/accounts/" +
		url.PathEscape(accountName) + "/deployments?api-version=" + deploymentsAPIVersion
	result := make([]*armcognitiveservices.Deployment, 0)
	for next != "" {
		var page armcognitiveservices.DeploymentListResult
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, fmt.Errorf("Cognitive Services デプロイ一覧の取得に失敗しました: %w", err)
		}
		result = append(result, page.Value...)
		next = value(page.NextLink)
	}
	return result, nil
}

func (c *sdkSubscriptionClient) listModels(ctx context.Context, resourceGroup, accountName string) ([]*armcognitiveservices.AccountModel, error) {
	next := c.client.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) +
		"/resourceGroups/" + url.PathEscape(resourceGroup) + "/providers/Microsoft.CognitiveServices/accounts/" +
		url.PathEscape(accountName) + "/models?api-version=" + modelsAPIVersion
	result := make([]*armcognitiveservices.AccountModel, 0)
	for next != "" {
		var page armcognitiveservices.AccountModelListResult
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, fmt.Errorf("Cognitive Services モデル一覧の取得に失敗しました: %w", err)
		}
		result = append(result, page.Value...)
		next = value(page.NextLink)
	}
	return result, nil
}

// listResourceGroups reads the resource groups of one subscription. A group is
// selectable even when it holds no Foundry, so this list is read separately from
// the account list.
func (c *sdkSubscriptionClient) listResourceGroups(ctx context.Context) ([]resourceGroupInfo, error) {
	next := c.client.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) +
		"/resourcegroups?api-version=" + resourcesAPIVersion
	result := make([]resourceGroupInfo, 0)
	for next != "" {
		var page struct {
			Value []struct {
				Name     *string `json:"name"`
				Location *string `json:"location"`
			} `json:"value"`
			NextLink *string `json:"nextLink"`
		}
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, fmt.Errorf("リソースグループ一覧の取得に失敗しました: %w", err)
		}
		for _, group := range page.Value {
			name := value(group.Name)
			if name == "" {
				continue
			}
			result = append(result, resourceGroupInfo{name: name, location: value(group.Location)})
		}
		next = value(page.NextLink)
	}
	return result, nil
}

// listRegionModels reads the candidates a subscription can deploy in one region.
// Unlike the account-scoped list it does not require an existing Foundry, so a
// Foundry that has not been created yet can still be configured.
func (c *sdkSubscriptionClient) listRegionModels(ctx context.Context, location string) ([]*armcognitiveservices.Model, error) {
	next := c.client.Endpoint() + "/subscriptions/" + url.PathEscape(c.subscriptionID) +
		"/providers/Microsoft.CognitiveServices/locations/" + url.PathEscape(location) +
		"/models?api-version=" + modelsAPIVersion
	result := make([]*armcognitiveservices.Model, 0)
	for next != "" {
		var page armcognitiveservices.ModelListResult
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, fmt.Errorf("リージョンのモデル一覧の取得に失敗しました: %w", err)
		}
		result = append(result, page.Value...)
		next = value(page.NextLink)
	}
	return result, nil
}

func (c *sdkSubscriptionClient) getJSON(ctx context.Context, requestURL string, target any) error {
	req, err := runtime.NewRequest(ctx, http.MethodGet, requestURL)
	if err != nil {
		return err
	}
	// nextLink is external input. Do not send this subscription's token to
	// another host or let a page silently switch the requested subscription.
	base, err := url.Parse(c.client.Endpoint())
	if err != nil {
		return err
	}
	targetURL := req.Raw().URL
	if targetURL.Scheme != base.Scheme || !strings.EqualFold(targetURL.Host, base.Host) || targetURL.User != nil ||
		!strings.HasPrefix(strings.ToLower(path.Clean(targetURL.Path)), strings.ToLower("/subscriptions/"+c.subscriptionID+"/")) {
		return errors.New("Azure のページ URL が要求先のサブスクリプションと一致しません")
	}
	response, err := c.client.Pipeline().Do(req)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return runtime.NewResponseError(response)
	}
	return runtime.UnmarshalAsJSON(response, target)
}

func isFoundryAccount(kind *string) bool {
	return kind != nil && (strings.EqualFold(strings.TrimSpace(*kind), accountKindAIServices) || strings.EqualFold(strings.TrimSpace(*kind), accountKindOpenAI))
}

func discoveredAccountFromSDK(subscription subscriptionInfo, client subscriptionClient, account *armcognitiveservices.Account) (discoveredAccount, error) {
	result := discoveredAccount{
		subscription: subscription,
		client:       client,
		id:           value(account.ID),
		name:         value(account.Name),
		region:       value(account.Location),
	}
	resourceGroup, nameFromID := resourceIDAccountParts(result.id)
	result.resourceGroup = resourceGroup
	if result.name == "" {
		result.name = nameFromID
	}
	if result.id == "" {
		return result, errors.New("Azure のアカウント応答にリソース ID がありません")
	}
	if result.resourceGroup == "" || result.name == "" {
		return result, errors.New("Azure のアカウントリソース ID からリソース グループまたはアカウント名を取得できません")
	}
	return result, nil
}

func resourceIDAccountParts(id string) (resourceGroup, accountName string) {
	_, resourceGroup, accountName = resourceIDTargetParts(id)
	return resourceGroup, accountName
}

func resourceIDTargetParts(id string) (subscriptionID, resourceGroup, accountName string) {
	segments := strings.Split(strings.Trim(id, "/"), "/")
	providerName := ""
	for index := 0; index+1 < len(segments); index++ {
		key := segments[index]
		value, err := url.PathUnescape(segments[index+1])
		if err != nil {
			continue
		}
		switch {
		case strings.EqualFold(key, "subscriptions"):
			subscriptionID = value
		case strings.EqualFold(key, "resourceGroups"):
			resourceGroup = value
		case strings.EqualFold(key, "providers"):
			providerName = value
		case strings.EqualFold(key, "accounts"):
			accountName = value
		}
	}
	if !strings.EqualFold(providerName, "Microsoft.CognitiveServices") {
		return "", "", ""
	}
	return subscriptionID, resourceGroup, accountName
}

func modelCandidates(models []*armcognitiveservices.AccountModel) []service.ModelCandidate {
	result := make([]service.ModelCandidate, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		candidate := service.ModelCandidate{
			Name:    value(model.Name),
			Format:  value(model.Format),
			Version: value(model.Version),
			SKUs:    make([]service.ModelSKU, 0, len(model.SKUs)),
		}
		if model.LifecycleStatus != nil {
			candidate.Lifecycle = string(*model.LifecycleStatus)
		}
		if model.IsDefaultVersion != nil {
			candidate.IsDefaultVersion = *model.IsDefaultVersion
		}
		seenSKUs := make(map[string]struct{}, len(model.SKUs))
		for _, sku := range model.SKUs {
			name := ""
			if sku != nil {
				name = value(sku.Name)
			}
			if name == "" {
				continue
			}
			if _, seen := seenSKUs[name]; seen {
				continue
			}
			seenSKUs[name] = struct{}{}
			candidate.SKUs = append(candidate.SKUs, service.ModelSKU{Name: name, Capacity: capacityContract(sku.Capacity)})
		}
		result = append(result, candidate)
	}
	return result
}

// regionModelCandidates keeps only the candidates that belong to the account
// kind this feature creates. The region list also carries other kinds, and the
// outer skuName describes the account SKU rather than a deployment SKU.
func regionModelCandidates(models []*armcognitiveservices.Model) []service.ModelCandidate {
	accountModels := make([]*armcognitiveservices.AccountModel, 0, len(models))
	for _, model := range models {
		if model == nil || model.Model == nil || !isFoundryAccount(model.Kind) {
			continue
		}
		accountModels = append(accountModels, model.Model)
	}
	return modelCandidates(accountModels)
}

// capacityContract keeps every constraint Azure reports for one SKU. A missing
// constraint stays missing so the screen can refuse the combination instead of
// inventing a capacity value.
func capacityContract(config *armcognitiveservices.CapacityConfig) *service.CapacityContract {
	if config == nil {
		return nil
	}
	contract := service.CapacityContract{
		Default:       cloneInt32Pointer(config.Default),
		Minimum:       cloneInt32Pointer(config.Minimum),
		Maximum:       cloneInt32Pointer(config.Maximum),
		Step:          cloneInt32Pointer(config.Step),
		AllowedValues: make([]int32, 0, len(config.AllowedValues)),
	}
	for _, allowed := range config.AllowedValues {
		if allowed == nil {
			continue
		}
		contract.AllowedValues = append(contract.AllowedValues, *allowed)
	}
	return &contract
}

func cloneInt32Pointer(source *int32) *int32 {
	if source == nil {
		return nil
	}
	copied := *source
	return &copied
}

func deploymentRow(account discoveredAccount, deployment *armcognitiveservices.Deployment) service.Deployment {
	row := service.Deployment{
		ID:               value(deployment.ID),
		Name:             value(deployment.Name),
		TenantID:         account.subscription.tenantID,
		TenantName:       account.subscription.tenantName,
		SubscriptionID:   account.subscription.id,
		SubscriptionName: account.subscription.name,
		AccountID:        account.id,
		AccountName:      account.name,
		Region:           account.region,
		TPM:              nil,
		RPM:              nil,
	}
	if row.ID == "" && row.Name != "" && account.id != "" {
		row.ID = strings.TrimRight(account.id, "/") + "/deployments/" + url.PathEscape(row.Name)
	}
	if properties := deployment.Properties; properties != nil {
		if model := properties.Model; model != nil {
			row.Model = value(model.Name)
			row.ModelVersion = value(model.Version)
		}
		row.TPM, row.RPM = quantityFromRateLimits(properties.RateLimits)
	}
	if deployment.SKU != nil {
		row.SKU = value(deployment.SKU.Name)
	}
	return row
}

func quantityFromRateLimits(rules []*armcognitiveservices.ThrottlingRule) (tpm, rpm *int64) {
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(value(rule.Key)))
		quantity := perMinuteQuantity(rule.Count, rule.RenewalPeriod)
		if quantity == nil {
			continue
		}
		switch key {
		case "token":
			if tpm == nil {
				tpm = quantity
			}
		case "request":
			if rpm == nil {
				rpm = quantity
			}
		}
	}
	return tpm, rpm
}

func perMinuteQuantity(count, renewalPeriod *float32) *int64 {
	if count == nil || renewalPeriod == nil {
		return nil
	}
	value := float64(*count)
	period := float64(*renewalPeriod)
	if math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(period) || math.IsInf(period, 0) || value < 0 || period <= 0 {
		return nil
	}
	perMinute := value * 60 / period
	if math.IsNaN(perMinute) || math.IsInf(perMinute, 0) || perMinute > 9_223_372_036_854_775_807 {
		return nil
	}
	rounded := math.Round(perMinute)
	if math.Abs(perMinute-rounded) > 0.000001 {
		return nil
	}
	quantity := int64(rounded)
	return &quantity
}

func deduplicateAccounts(accounts []discoveredAccount) []discoveredAccount {
	seen := make(map[string]struct{}, len(accounts))
	result := make([]discoveredAccount, 0, len(accounts))
	for _, account := range accounts {
		key := strings.ToLower(account.subscription.tenantID + "/" + account.subscription.id + "/" + account.id)
		if account.id == "" {
			key += "/" + strings.ToLower(account.name)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, account)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].id) < strings.ToLower(result[j].id) })
	return result
}

func isExcludedTenantPermission(err error) bool {
	var responseError *azcore.ResponseError
	return errors.As(err, &responseError) && responseError.StatusCode == 403
}

func failureForError(scope string, subscription subscriptionInfo, account discoveredAccount, err error) service.FetchFailure {
	code, message, action := describeError(err)
	failure := service.FetchFailure{
		Scope:            scope,
		TenantName:       subscription.tenantName,
		SubscriptionName: subscription.name,
		AccountName:      account.name,
		Code:             code,
		Message:          message,
		Action:           action,
	}
	if scope == "azure-cli" {
		failure.TenantName = ""
		failure.SubscriptionName = ""
	}
	return failure
}

func describeError(err error) (code, message, action string) {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		code = responseError.ErrorCode
		if code == "" {
			code = fmt.Sprintf("http-%d", responseError.StatusCode)
		}
		switch responseError.StatusCode {
		case 401:
			return code, "Azure Resource Manager が認証を拒否しました。", "Azure CLI でサインイン状態を確認してから更新してください。"
		case 403:
			return code, "対象スコープへのアクセス権がありません。", "対象サブスクリプションまたはアカウントの Azure RBAC を確認してください。"
		case 404:
			return code, "Azure の対象リソースが見つかりません。", "対象リソースを確認してから再取得してください。"
		case 429:
			return code, "Azure の要求制限に達しました。", "時間をおいて再取得してください。"
		default:
			return code, fmt.Sprintf("Azure Resource Manager が HTTP %d を返しました。", responseError.StatusCode), "Azure の状態を確認してから再取得してください。"
		}
	}

	var commandErr *commandError
	if errors.As(err, &commandErr) {
		lower := strings.ToLower(commandErr.Error())
		if errors.Is(commandErr, exec.ErrNotFound) || strings.Contains(lower, "executable file not found") {
			return "azure-cli-not-found", "Azure CLI が見つかりません。", "Azure CLI をインストールして PATH を確認してください。"
		}
		if strings.Contains(lower, "az login") || strings.Contains(lower, "not logged") || strings.Contains(lower, "login required") {
			return "azure-cli-not-logged-in", "Azure CLI にサインインしていません。", "Azure CLI で az login を実行してから再試行してください。"
		}
		return "azure-cli-error", "Azure CLI からサブスクリプション一覧を取得できませんでした。", "Azure CLI の状態を確認してから再試行してください。"
	}
	var authenticationErr *azidentity.AuthenticationFailedError
	if errors.As(err, &authenticationErr) {
		return "azure-cli-not-logged-in", "Azure CLI のサインイン状態を確認できません。", "Azure CLI で対象テナントに az login を実行してから再試行してください。"
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "azureclicredential") && (strings.Contains(lower, "az login") || strings.Contains(lower, "aadsts") || strings.Contains(lower, "refresh token")) {
		return "azure-cli-not-logged-in", "Azure CLI のサインイン状態を確認できません。", "Azure CLI で対象テナントに az login を実行してから再試行してください。"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "Azure への要求がタイムアウトしました。", "通信状態を確認してから再取得してください。"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", "Azure への要求がキャンセルされました。", "再取得してください。"
	}
	return "azure-error", "Azure の応答を処理できませんでした。", "Azure の状態を確認してから再取得してください。"
}

func reportSnapshot(report func(service.DeploymentProgress), progress service.DeploymentProgress) {
	if report != nil {
		report(progress)
	}
}

func emptyResult() service.DeploymentResult {
	return service.DeploymentResult{
		Deployments: make([]service.Deployment, 0),
		Failures:    make([]service.FetchFailure, 0),
	}
}

func emptyPlacementResult() service.PlacementResult {
	return service.PlacementResult{
		Subscriptions: make([]service.PlacementSubscription, 0),
		Failures:      make([]service.FetchFailure, 0),
	}
}

func emptyModelResult() service.ModelResult {
	return service.ModelResult{
		Models:   make([]service.ModelCandidate, 0),
		Failures: make([]service.FetchFailure, 0),
	}
}

func cloneFailures(failures []service.FetchFailure) []service.FetchFailure {
	result := make([]service.FetchFailure, len(failures))
	copy(result, failures)
	return result
}

func cloneDeployments(deployments []service.Deployment) []service.Deployment {
	result := make([]service.Deployment, len(deployments))
	for index, deployment := range deployments {
		result[index] = deployment
		if deployment.TPM != nil {
			value := *deployment.TPM
			result[index].TPM = &value
		}
		if deployment.RPM != nil {
			value := *deployment.RPM
			result[index].RPM = &value
		}
	}
	return result
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func workerCount(limit, jobs int) int {
	if jobs < limit {
		return jobs
	}
	return limit
}

func value[T any](pointer *T) (result T) {
	if pointer != nil {
		return *pointer
	}
	return result
}
