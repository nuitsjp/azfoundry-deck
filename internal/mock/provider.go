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

// Provider is the Azure boundary for the F1 mock. It snapshots the selected
// scenario at the beginning of Fetch, so changing the scenario while a read
// is in flight cannot change that read's response.
type Provider struct {
	mu       sync.Mutex
	scenario string
	wait     sleeper
}

// NewProvider creates a provider in the normal successful state.
func NewProvider() *Provider {
	return &Provider{
		scenario: ScenarioSuccess,
		wait:     waitContext,
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
