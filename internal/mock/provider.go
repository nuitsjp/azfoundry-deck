package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

const (
	ScenarioSuccess           = "success"
	ScenarioEmpty             = "empty"
	ScenarioPartial           = "partial"
	ScenarioFailure           = "failure"
	ScenarioCLIMissing        = "cli-missing"
	ScenarioAllAccountsFailed = "all-accounts-failed"
	ScenarioLoading           = "loading"
	ScenarioDelayed           = "delayed"
)

var validScenarios = map[string]struct{}{
	ScenarioSuccess:           {},
	ScenarioEmpty:             {},
	ScenarioPartial:           {},
	ScenarioFailure:           {},
	ScenarioCLIMissing:        {},
	ScenarioAllAccountsFailed: {},
	ScenarioLoading:           {},
	ScenarioDelayed:           {},
}

type sleeper func(context.Context, time.Duration) error

const (
	discoveryStepCount = 5
	accountStepCount   = 10
)

var (
	// The order models responses arriving independently of fixture order. The
	// snapshots themselves are rebuilt in fixture order before each report.
	discoveryOrder = [...]int{2, 0, 4, 1, 3}
	accountOrder   = [...]int{6, 0, 8, 2, 9, 4, 1, 7, 3, 5}
)

// Provider is the Azure boundary for the mock. It snapshots the selected
// scenario at the beginning of Fetch, so changing the scenario while a read
// is in flight cannot change that read's response.
type Provider struct {
	mu             sync.Mutex
	scenario       string
	addScenario    string
	createScenario string
	wait           sleeper
}

// NewProvider creates a provider in the normal successful state.
func NewProvider() *Provider {
	return &Provider{
		scenario:       ScenarioSuccess,
		addScenario:    AddScenarioSuccess,
		createScenario: service.OutcomeSuccess,
		wait:           waitContext,
	}
}

// SetScenario selects the response used by the next Fetch call.
func (p *Provider) SetScenario(name string) error {
	if _, ok := validScenarios[name]; !ok {
		return fmt.Errorf("unknown mock scenario %q", name)
	}
	p.mu.Lock()
	p.scenario = name
	p.mu.Unlock()
	return nil
}

// SetAddModelScenario selects the response used by the next model addition read.
func (p *Provider) SetAddModelScenario(name string) error {
	if _, ok := validAddScenarios[name]; !ok {
		return fmt.Errorf("unknown mock add-model scenario %q", name)
	}
	p.mu.Lock()
	p.addScenario = name
	p.mu.Unlock()
	return nil
}

// SetCreateScenario selects the creation result reproduced by the next add.
func (p *Provider) SetCreateScenario(name string) error {
	if _, ok := validCreateScenarios[name]; !ok {
		return fmt.Errorf("unknown mock create scenario %q", name)
	}
	p.mu.Lock()
	p.createScenario = name
	p.mu.Unlock()
	return nil
}

// CreateModelDeployment reproduces one creation result. It never contacts Azure
// and never stores an operation record.
func (p *Provider) CreateModelDeployment(ctx context.Context, request service.CreateRequest) (service.CreateResult, error) {
	p.mu.Lock()
	scenario := p.createScenario
	wait := p.wait
	p.mu.Unlock()

	if err := wait(ctx, 600*time.Millisecond); err != nil {
		return service.CreateResult{}, err
	}
	result := service.CreateResult{
		Outcome:      scenario,
		OperationID:  "mock-operation",
		FoundryID:    request.FoundryName,
		DeploymentID: request.DeploymentName,
	}
	// A stage that succeeded before the failure stays created.
	result.CreatedGroup = request.GroupIsNew
	result.CreatedFoundry = request.FoundryIsNew && scenario != service.OutcomeGroupFailure
	return result, nil
}

// CheckCreateOperation reproduces a read-only status check.
func (p *Provider) CheckCreateOperation(ctx context.Context, operationID string) (service.CreateResult, error) {
	p.mu.Lock()
	scenario := p.createScenario
	wait := p.wait
	p.mu.Unlock()
	if err := wait(ctx, 400*time.Millisecond); err != nil {
		return service.CreateResult{}, err
	}
	return service.CreateResult{Outcome: scenario, OperationID: operationID}, nil
}

// PendingCreateOperations reproduces what an earlier run left unresolved. The
// mock stores no record, so it reports one only while the unknown result is
// selected: that is the state a process death leaves behind.
func (p *Provider) PendingCreateOperations(context.Context) ([]service.PendingOperation, error) {
	p.mu.Lock()
	scenario := p.createScenario
	p.mu.Unlock()
	if scenario != service.OutcomeUnknown {
		return []service.PendingOperation{}, nil
	}
	return []service.PendingOperation{{
		ID:             "mock-operation",
		StartedAt:      "2026-09-15T02:14:00Z",
		SubscriptionID: "00000000-0000-0000-0000-000000000001",
		DeploymentID:   "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/rg-production/providers/Microsoft.CognitiveServices/accounts/contoso-chat-prod/deployments/chat-production-2",
		Stage:          "deployment",
		State:          "sent",
	}}, nil
}

// Fetch returns one deterministic F1 read result and reports each response
// arrival in a fixed order.
func (p *Provider) Fetch(ctx context.Context, report func(service.DeploymentProgress)) (service.DeploymentResult, error) {
	p.mu.Lock()
	scenario := p.scenario
	wait := p.wait
	p.mu.Unlock()

	delay := 350 * time.Millisecond
	switch scenario {
	case ScenarioLoading:
		delay = 30 * time.Second
	case ScenarioDelayed:
		delay = 8 * time.Second
	}

	reportSnapshot(report, service.DeploymentProgress{
		Stage:  service.DeploymentProgressStageDiscovering,
		Result: emptyResult(),
	})

	if scenario == ScenarioFailure || scenario == ScenarioCLIMissing {
		if err := wait(ctx, delay); err != nil {
			return emptyResult(), err
		}
		if err := ctx.Err(); err != nil {
			return emptyResult(), err
		}
		result := resultForScenario(scenario)
		result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	result := resultForScenario(scenario)
	if scenario == ScenarioDelayed {
		setDelayedQuantity(result.Deployments)
	}
	discoveryTotal := delay / 5
	discoveryDelay := discoveryTotal / discoveryStepCount
	accountTotal := delay - discoveryTotal
	accountDelay := accountTotal / accountStepCount
	discoveredSubscriptions := make([]bool, discoveryStepCount)
	for completed := 1; completed <= discoveryStepCount; completed++ {
		if err := wait(ctx, discoveryDelay); err != nil {
			return emptyResult(), err
		}
		if err := ctx.Err(); err != nil {
			return emptyResult(), err
		}
		discoveredSubscriptions[discoveryOrder[completed-1]] = true
		completedSubscriptions := 0
		for _, discovered := range discoveredSubscriptions {
			if discovered {
				completedSubscriptions++
			}
		}
		var totalSubscriptions = discoveryStepCount
		discoveryResult := emptyResult()
		if completed == discoveryStepCount {
			discoveryResult.TotalAccounts = intPointer(accountStepCount)
		}
		reportSnapshot(report, service.DeploymentProgress{
			Stage:                  service.DeploymentProgressStageDiscovering,
			CompletedSubscriptions: completedSubscriptions,
			TotalSubscriptions:     &totalSubscriptions,
			Result:                 discoveryResult,
		})
	}

	totalAccounts := accountStepCount
	reportSnapshot(report, service.DeploymentProgress{
		Stage:                  service.DeploymentProgressStageFetching,
		CompletedSubscriptions: discoveryStepCount,
		TotalSubscriptions:     intPointer(discoveryStepCount),
		Result: service.DeploymentResult{
			Deployments:   make([]service.Deployment, 0),
			Failures:      make([]service.FetchFailure, 0),
			TotalAccounts: intPointer(totalAccounts),
		},
	})

	completedAccounts := make([]bool, accountStepCount)
	for completed := 1; completed <= accountStepCount; completed++ {
		if err := wait(ctx, accountDelay); err != nil {
			return emptyResult(), err
		}
		if err := ctx.Err(); err != nil {
			return emptyResult(), err
		}
		accountIndex := accountOrder[completed-1]
		completedAccounts[accountIndex] = true
		snapshot := cumulativeAccountResult(result, completedAccounts)
		snapshot.TotalAccounts = intPointer(totalAccounts)
		reportSnapshot(report, service.DeploymentProgress{
			Stage:                  service.DeploymentProgressStageFetching,
			CompletedSubscriptions: discoveryStepCount,
			CompletedAccounts:      completed,
			TotalSubscriptions:     intPointer(discoveryStepCount),
			Result:                 snapshot,
		})
	}

	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// FetchModels returns deterministic candidates for an existing Foundry. An
// unknown Foundry is reported as a failure so the screen does not read the
// missing answer as an empty candidate list.
func (p *Provider) FetchModels(ctx context.Context, accountID string) (service.ModelResult, error) {
	if err := ctx.Err(); err != nil {
		return emptyModelResult(), err
	}
	foundry, known := addFoundryByID(accountID)
	if !known {
		return service.ModelResult{
			Models: make([]service.ModelCandidate, 0),
			Failures: []service.FetchFailure{{
				Scope:       "account",
				AccountName: accountID,
				Code:        "account-not-found",
				Message:     "選択したFoundryが見つかりません。",
				Action:      "配置先を選び直してください。",
			}},
			FetchedAt: nowUTC(),
		}, nil
	}
	return p.fetchAddModelCandidates(ctx, foundry.name)
}

// FetchRegionModels returns deterministic candidates for a region, which is the
// read a Foundry that does not exist yet needs.
func (p *Provider) FetchRegionModels(ctx context.Context, subscriptionID, region string) (service.ModelResult, error) {
	if err := ctx.Err(); err != nil {
		return emptyModelResult(), err
	}
	if subscriptionID == "" || region == "" {
		return service.ModelResult{
			Models: make([]service.ModelCandidate, 0),
			Failures: []service.FetchFailure{{
				Scope:   "region",
				Code:    "invalid-region-target",
				Message: "モデル候補を取得するサブスクリプションとリージョンが指定されていません。",
				Action:  "配置先を選び直してください。",
			}},
			FetchedAt: nowUTC(),
		}, nil
	}

	return p.fetchAddModelCandidates(ctx, region)
}

// fetchAddModelCandidates serves the model addition screen.
func (p *Provider) fetchAddModelCandidates(ctx context.Context, target string) (service.ModelResult, error) {
	p.mu.Lock()
	scenario := p.addScenario
	wait := p.wait
	p.mu.Unlock()

	delay := 800 * time.Millisecond
	if scenario == AddScenarioSlow {
		delay = 8 * time.Second
	}
	if err := wait(ctx, delay); err != nil {
		return emptyModelResult(), err
	}
	if err := ctx.Err(); err != nil {
		return emptyModelResult(), err
	}
	result := addModelResultForScenario(target, scenario)
	result.FetchedAt = nowUTC()
	return result, nil
}

// FetchPlacements returns the deterministic subscriptions, resource groups and
// Foundries a model can be added to.
func (p *Provider) FetchPlacements(ctx context.Context) (service.PlacementResult, error) {
	p.mu.Lock()
	wait := p.wait
	p.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return service.PlacementResult{}, err
	}
	if err := wait(ctx, 250*time.Millisecond); err != nil {
		return service.PlacementResult{}, err
	}
	result := placementResult()
	result.FetchedAt = nowUTC()
	return result, nil
}

// FetchDeploymentNames returns the deployment names of one mock Foundry. An
// unknown Foundry is reported as a failure so the screen does not treat the
// missing answer as a free name.
func (p *Provider) FetchDeploymentNames(ctx context.Context, accountID string) (service.DeploymentNameResult, error) {
	p.mu.Lock()
	wait := p.wait
	p.mu.Unlock()

	result := service.DeploymentNameResult{Names: make([]string, 0), Failures: make([]service.FetchFailure, 0)}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := wait(ctx, 250*time.Millisecond); err != nil {
		return result, err
	}
	names, known := deploymentNamesForAccount(accountID)
	if !known {
		result.Failures = []service.FetchFailure{{
			Scope:       "account",
			AccountName: accountID,
			Code:        "account-not-found",
			Message:     "選択したFoundryが見つかりません。",
			Action:      "配置先を選び直してください。",
		}}
		result.FetchedAt = nowUTC()
		return result, nil
	}
	result.Names = names
	result.FetchedAt = nowUTC()
	return result, nil
}

func reportSnapshot(report func(service.DeploymentProgress), progress service.DeploymentProgress) {
	if report != nil {
		report(progress)
	}
}

func intPointer(value int) *int {
	return &value
}

func setDelayedQuantity(deployments []service.Deployment) {
	if len(deployments) == 0 || deployments[0].TPM == nil {
		return
	}
	oldValue := int64(111000)
	deployments[0].TPM = &oldValue
}

func cumulativeAccountResult(final service.DeploymentResult, completed []bool) service.DeploymentResult {
	snapshot := emptyResult()
	for _, deployment := range final.Deployments {
		if accountCompleted(deployment.AccountID, completed) {
			snapshot.Deployments = append(snapshot.Deployments, cloneDeployment(deployment))
		}
	}
	for _, failure := range final.Failures {
		if accountCompletedByName(failure.AccountName, completed) {
			snapshot.Failures = append(snapshot.Failures, failure)
		}
	}
	for accountIndex, done := range completed {
		if done && !hasAccountFailure(final.Failures, mockAccounts[accountIndex].accountName) {
			snapshot.SuccessfulAccounts++
		}
	}
	return snapshot
}

func cloneDeployment(deployment service.Deployment) service.Deployment {
	if deployment.TPM != nil {
		tpm := *deployment.TPM
		deployment.TPM = &tpm
	}
	if deployment.RPM != nil {
		rpm := *deployment.RPM
		deployment.RPM = &rpm
	}
	return deployment
}

func accountCompleted(accountID string, completed []bool) bool {
	for index, account := range mockAccounts {
		if accountResourceID(account) == accountID {
			return completed[index]
		}
	}
	return false
}

func accountCompletedByName(accountName string, completed []bool) bool {
	if accountName == "" {
		return false
	}
	for index, account := range mockAccounts {
		if account.accountName == accountName {
			return completed[index]
		}
	}
	return false
}

func hasAccountFailure(failures []service.FetchFailure, accountName string) bool {
	for _, failure := range failures {
		if failure.AccountName == accountName {
			return true
		}
	}
	return false
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func emptyResult() service.DeploymentResult {
	return service.DeploymentResult{
		Deployments: make([]service.Deployment, 0),
		Failures:    make([]service.FetchFailure, 0),
	}
}

func emptyModelResult() service.ModelResult {
	return service.ModelResult{
		Models:   make([]service.ModelCandidate, 0),
		Failures: make([]service.FetchFailure, 0),
	}
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
