package mock

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

func discardProgress(service.DeploymentProgress) {}

func noWait(context.Context, time.Duration) error { return nil }

func TestSuccessFixtureHasExactAccountAndDeploymentShape(t *testing.T) {
	provider := NewProvider()
	provider.wait = func(context.Context, time.Duration) error { return nil }
	result, err := provider.Fetch(context.Background(), discardProgress)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if result.TotalAccounts == nil || *result.TotalAccounts != 10 {
		t.Fatalf("TotalAccounts = %v, want 10", result.TotalAccounts)
	}
	if result.SuccessfulAccounts != 10 {
		t.Fatalf("SuccessfulAccounts = %d, want 10", result.SuccessfulAccounts)
	}
	if len(result.Deployments) != 24 {
		t.Fatalf("deployment count = %d, want 24", len(result.Deployments))
	}
	if len(result.Failures) != 0 {
		t.Fatalf("failure count = %d, want 0", len(result.Failures))
	}

	byName := make(map[string][]string)
	for _, deployment := range result.Deployments {
		byName[deployment.Name] = append(byName[deployment.Name], deployment.ID)
	}
	if len(byName["chat-prod"]) < 2 || byName["chat-prod"][0] == byName["chat-prod"][1] {
		t.Fatal("same-name deployments must retain distinct resource IDs")
	}
	var missingTPM, missingRPM, missingBoth bool
	for _, deployment := range result.Deployments {
		switch {
		case deployment.TPM == nil && deployment.RPM == nil:
			missingBoth = true
		case deployment.TPM == nil:
			missingTPM = true
		case deployment.RPM == nil:
			missingRPM = true
		}
	}
	if !missingTPM || !missingRPM || !missingBoth {
		t.Fatalf("missing quantity combinations = tpm:%t rpm:%t both:%t", missingTPM, missingRPM, missingBoth)
	}
	if result.Deployments[0].TPM == nil || *result.Deployments[0].TPM != 120000 {
		t.Fatalf("normal first deployment TPM = %v, want 120000", result.Deployments[0].TPM)
	}
	if result.Deployments[0].AccountID != "/subscriptions/subscription-001/resourceGroups/rg-account-001/providers/Microsoft.CognitiveServices/accounts/contoso-chat-prod" {
		t.Fatalf("account resource ID = %q, want complete ARM resource ID", result.Deployments[0].AccountID)
	}
}

func TestPartialAndAllAccountsFailedRemainDistinct(t *testing.T) {
	provider := NewProvider()
	provider.wait = func(context.Context, time.Duration) error { return nil }

	if err := provider.SetScenario(ScenarioPartial); err != nil {
		t.Fatal(err)
	}
	partial, err := provider.Fetch(context.Background(), discardProgress)
	if err != nil {
		t.Fatalf("partial Fetch returned error: %v", err)
	}
	if partial.SuccessfulAccounts != 6 || partial.TotalAccounts == nil || *partial.TotalAccounts != 10 {
		t.Fatalf("partial account counts = successful:%d total:%v, want 6/10", partial.SuccessfulAccounts, partial.TotalAccounts)
	}
	if len(partial.Failures) != 4 || len(partial.Deployments) == 0 {
		t.Fatalf("partial result = %d deployments, %d failures; want deployments and 4 failures", len(partial.Deployments), len(partial.Failures))
	}

	if err := provider.SetScenario(ScenarioAllAccountsFailed); err != nil {
		t.Fatal(err)
	}
	failed, err := provider.Fetch(context.Background(), discardProgress)
	if err != nil {
		t.Fatalf("all-accounts-failed Fetch returned error: %v", err)
	}
	if failed.SuccessfulAccounts != 0 || failed.TotalAccounts == nil || *failed.TotalAccounts != 10 {
		t.Fatalf("all-failed account counts = successful:%d total:%v, want 0/10", failed.SuccessfulAccounts, failed.TotalAccounts)
	}
	if len(failed.Deployments) != 0 || len(failed.Failures) != 10 {
		t.Fatalf("all-failed result = %d deployments, %d failures; want 0/10", len(failed.Deployments), len(failed.Failures))
	}
}

func TestEmptyAndDiscoveryFailureScenariosKeepTheirMeaning(t *testing.T) {
	provider := NewProvider()
	provider.wait = func(context.Context, time.Duration) error { return nil }

	if err := provider.SetScenario(ScenarioEmpty); err != nil {
		t.Fatal(err)
	}
	empty, err := provider.Fetch(context.Background(), discardProgress)
	if err != nil {
		t.Fatalf("empty Fetch returned error: %v", err)
	}
	if empty.TotalAccounts == nil || *empty.TotalAccounts != 10 || empty.SuccessfulAccounts != 10 || len(empty.Deployments) != 0 || len(empty.Failures) != 0 {
		t.Fatalf("empty result has wrong shape: %+v", empty)
	}

	for _, scenario := range []string{ScenarioFailure, ScenarioCLIMissing} {
		if err := provider.SetScenario(scenario); err != nil {
			t.Fatal(err)
		}
		result, err := provider.Fetch(context.Background(), discardProgress)
		if err != nil {
			t.Fatalf("%s Fetch returned error: %v", scenario, err)
		}
		if result.TotalAccounts != nil || len(result.Deployments) != 0 || len(result.Failures) != 1 {
			t.Fatalf("%s result should be an unresolved discovery failure: %+v", scenario, result)
		}
		failure := result.Failures[0]
		if failure.Scope == "" || failure.Code == "" || failure.Message == "" || failure.Action == "" {
			t.Fatalf("%s failure lacks scope or remediation: %+v", scenario, failure)
		}
	}
}

func TestSetScenarioRejectsUnknownName(t *testing.T) {
	provider := NewProvider()
	for _, scenario := range []string{"not-a-scenario", "forbidden"} {
		if err := provider.SetScenario(scenario); err == nil {
			t.Fatalf("SetScenario accepted an unknown scenario %q", scenario)
		}
	}
}

func TestDelayedScenarioIsCapturedAtFetchStart(t *testing.T) {
	provider := NewProvider()
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	provider.wait = func(ctx context.Context, _ time.Duration) error {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := provider.SetScenario(ScenarioDelayed); err != nil {
		t.Fatal(err)
	}

	resultCh := make(chan struct {
		result service.DeploymentResult
		err    error
	}, 1)
	go func() {
		result, err := provider.Fetch(context.Background(), discardProgress)
		resultCh <- struct {
			result service.DeploymentResult
			err    error
		}{result: result, err: err}
	}()
	<-started
	if err := provider.SetScenario(ScenarioSuccess); err != nil {
		t.Fatal(err)
	}
	close(release)
	got := <-resultCh
	if got.err != nil {
		t.Fatalf("Fetch returned error: %v", got.err)
	}
	if got.result.Deployments[0].TPM == nil || *got.result.Deployments[0].TPM != 111000 {
		t.Fatalf("delayed first deployment TPM = %v, want 111000", got.result.Deployments[0].TPM)
	}
}

func TestLoadingFetchHonorsCancellation(t *testing.T) {
	provider := NewProvider()
	provider.wait = waitContext
	if err := provider.SetScenario(ScenarioLoading); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reports := 0
	result, err := provider.Fetch(ctx, func(progress service.DeploymentProgress) {
		reports++
		if progress.Stage != service.DeploymentProgressStageDiscovering {
			t.Errorf("cancelled initial stage = %q, want discovering", progress.Stage)
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch error = %v, want context.Canceled", err)
	}
	if reports != 1 {
		t.Fatalf("cancelled progress count = %d, want initial report only", reports)
	}
	if result.Deployments == nil || result.Failures == nil {
		t.Fatal("cancelled result arrays must be empty arrays, not nil")
	}
}

func TestProgressReportsUnknownThenKnownTotalsAndFixtureOrder(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait
	reports := make([]service.DeploymentProgress, 0, 17)
	result, err := provider.Fetch(context.Background(), func(progress service.DeploymentProgress) {
		reports = append(reports, progress)
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(reports) != 17 {
		t.Fatalf("progress count = %d, want 17", len(reports))
	}
	first := reports[0]
	if first.Stage != service.DeploymentProgressStageDiscovering || first.TotalSubscriptions != nil || first.Result.TotalAccounts != nil {
		t.Fatalf("initial progress = %+v, want discovering with unknown totals", first)
	}
	if first.Result.Deployments == nil || first.Result.Failures == nil || first.Result.FetchedAt != "" {
		t.Fatal("initial progress must have empty arrays and no fetched timestamp")
	}
	for completed := 1; completed <= 5; completed++ {
		progress := reports[completed]
		if progress.Stage != service.DeploymentProgressStageDiscovering || progress.CompletedSubscriptions != completed || progress.TotalSubscriptions == nil || *progress.TotalSubscriptions != 5 {
			t.Fatalf("discovery progress %d = %+v, want %d/5", completed, progress, completed)
		}
		if progress.CompletedAccounts != 0 || progress.Result.FetchedAt != "" {
			t.Fatalf("discovery progress %d has account progress or timestamp: %+v", completed, progress)
		}
		if completed < 5 && progress.Result.TotalAccounts != nil {
			t.Fatalf("discovery progress %d TotalAccounts = %v, want null", completed, progress.Result.TotalAccounts)
		}
		if completed == 5 && (progress.Result.TotalAccounts == nil || *progress.Result.TotalAccounts != 10) {
			t.Fatalf("discovery completion TotalAccounts = %v, want 10", progress.Result.TotalAccounts)
		}
	}
	fetchStart := reports[6]
	if fetchStart.Stage != service.DeploymentProgressStageFetching || fetchStart.CompletedSubscriptions != 5 || fetchStart.CompletedAccounts != 0 || fetchStart.TotalSubscriptions == nil || fetchStart.Result.TotalAccounts == nil {
		t.Fatalf("fetch start progress = %+v, want known subscription/account totals", fetchStart)
	}
	for completed := 1; completed <= 10; completed++ {
		progress := reports[6+completed]
		if progress.Stage != service.DeploymentProgressStageFetching || progress.CompletedSubscriptions != 5 || progress.CompletedAccounts != completed || progress.Result.TotalAccounts == nil || *progress.Result.TotalAccounts != 10 {
			t.Fatalf("account progress %d = %+v, want 5/%d and total 10", completed, progress, completed)
		}
		if progress.Result.FetchedAt != "" {
			t.Fatalf("account progress %d unexpectedly has fetched timestamp", completed)
		}
	}
	if result.FetchedAt == "" || result.TotalAccounts == nil || *result.TotalAccounts != 10 || len(result.Deployments) != 24 || len(result.Failures) != 0 {
		t.Fatalf("final result = %+v, want complete success result", result)
	}
	want := resultForScenario(ScenarioSuccess)
	result.FetchedAt = ""
	if !reflect.DeepEqual(result, want) {
		t.Fatal("final result does not match the success fixture")
	}
	assertFixtureOrder(t, reports, resultForScenario(ScenarioSuccess))

	withDeployments := -1
	for index, progress := range reports {
		if len(progress.Result.Deployments) > 0 {
			withDeployments = index
			break
		}
	}
	if withDeployments < 0 || withDeployments+1 >= len(reports) || len(reports[withDeployments+1].Result.Deployments) == 0 {
		t.Fatal("expected adjacent snapshots with deployments")
	}
	reports[withDeployments].Result.Deployments[0].Name = "mutated"
	if reports[withDeployments+1].Result.Deployments[0].Name == "mutated" {
		t.Fatal("later progress snapshot shares deployment array with an earlier snapshot")
	}
}

func TestProgressAccumulatesPartialSuccessAndFailureInArrivalOrder(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait
	if err := provider.SetScenario(ScenarioPartial); err != nil {
		t.Fatal(err)
	}
	reports := make([]service.DeploymentProgress, 0, 17)
	result, err := provider.Fetch(context.Background(), func(progress service.DeploymentProgress) {
		reports = append(reports, progress)
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	firstAccount := reports[7]
	if firstAccount.CompletedAccounts != 1 || len(firstAccount.Result.Deployments) != 0 || len(firstAccount.Result.Failures) != 1 || firstAccount.Result.SuccessfulAccounts != 0 {
		t.Fatalf("first account snapshot = %+v, want one failed account", firstAccount)
	}
	secondAccount := reports[8]
	if secondAccount.CompletedAccounts != 2 || len(secondAccount.Result.Deployments) == 0 || len(secondAccount.Result.Failures) != 1 || secondAccount.Result.SuccessfulAccounts != 1 {
		t.Fatalf("second account snapshot = %+v, want success accumulated after failure", secondAccount)
	}
	if result.TotalAccounts == nil || *result.TotalAccounts != 10 || result.SuccessfulAccounts != 6 || len(result.Deployments) != 15 || len(result.Failures) != 4 {
		t.Fatalf("partial final result = %+v, want 6 successes/4 failures", result)
	}
	want := resultForScenario(ScenarioPartial)
	result.FetchedAt = ""
	if !reflect.DeepEqual(result, want) {
		t.Fatal("final result does not match the partial fixture")
	}
	assertFixtureOrder(t, reports, want)
}

func TestLoadingAndDelayedProgressUseFixedTotalWait(t *testing.T) {
	provider := NewProvider()
	var durations []time.Duration
	if err := provider.SetScenario(ScenarioLoading); err != nil {
		t.Fatal(err)
	}
	provider.wait = func(_ context.Context, duration time.Duration) error {
		durations = append(durations, duration)
		return nil
	}
	if _, err := provider.Fetch(context.Background(), discardProgress); err != nil {
		t.Fatalf("loading Fetch returned error: %v", err)
	}
	if len(durations) != 15 {
		t.Fatalf("loading wait count = %d, want 15", len(durations))
	}
	for index := 0; index < 5; index++ {
		if durations[index] != 1200*time.Millisecond {
			t.Errorf("loading discovery wait %d = %s, want 1.2s", index, durations[index])
		}
	}
	for index := 5; index < 15; index++ {
		if durations[index] != 2400*time.Millisecond {
			t.Errorf("loading account wait %d = %s, want 2.4s", index-5, durations[index])
		}
	}

	durations = nil
	if err := provider.SetScenario(ScenarioDelayed); err != nil {
		t.Fatal(err)
	}
	reports := make([]service.DeploymentProgress, 0, 17)
	result, err := provider.Fetch(context.Background(), func(progress service.DeploymentProgress) {
		reports = append(reports, progress)
	})
	if err != nil {
		t.Fatalf("delayed Fetch returned error: %v", err)
	}
	if len(durations) != 15 || durations[0] != 320*time.Millisecond || durations[5] != 640*time.Millisecond {
		t.Fatalf("delayed waits = %v, want five 0.32s and ten 0.64s", durations)
	}
	if result.Deployments[0].TPM == nil || *result.Deployments[0].TPM != 111000 {
		t.Fatalf("delayed final TPM = %v, want 111000", result.Deployments[0].TPM)
	}
	foundDelayed := false
	for _, progress := range reports {
		for _, deployment := range progress.Result.Deployments {
			if deployment.ID == result.Deployments[0].ID {
				foundDelayed = true
				if deployment.TPM == nil || *deployment.TPM != 111000 {
					t.Fatal("delayed quantity was not reflected in an intermediate snapshot")
				}
			}
		}
	}
	if !foundDelayed {
		t.Fatal("delayed first deployment never appeared in progress")
	}
}

func TestDiscoveryFailureReportsStartBeforeItsWait(t *testing.T) {
	provider := NewProvider()
	var waited []time.Duration
	provider.wait = func(_ context.Context, duration time.Duration) error {
		waited = append(waited, duration)
		return nil
	}
	for _, scenario := range []string{ScenarioFailure, ScenarioCLIMissing} {
		if err := provider.SetScenario(scenario); err != nil {
			t.Fatal(err)
		}
		reports := make([]service.DeploymentProgress, 0, 1)
		result, err := provider.Fetch(context.Background(), func(progress service.DeploymentProgress) {
			reports = append(reports, progress)
		})
		if err != nil || len(reports) != 1 || reports[0].Stage != service.DeploymentProgressStageDiscovering || reports[0].TotalSubscriptions != nil {
			t.Fatalf("%s start report/result = reports:%v result:%+v err:%v", scenario, reports, result, err)
		}
		if result.TotalAccounts != nil || len(result.Failures) != 1 || result.FetchedAt == "" {
			t.Fatalf("%s final result = %+v, want unresolved discovery failure", scenario, result)
		}
		if waited[len(waited)-1] != 350*time.Millisecond {
			t.Fatalf("%s wait = %s, want 350ms", scenario, waited[len(waited)-1])
		}
	}
}

func assertFixtureOrder(t *testing.T, reports []service.DeploymentProgress, fixture service.DeploymentResult) {
	t.Helper()
	positions := make(map[string]int, len(fixture.Deployments))
	for index, deployment := range fixture.Deployments {
		positions[deployment.ID] = index
	}
	for progressIndex, progress := range reports {
		last := -1
		for _, deployment := range progress.Result.Deployments {
			position, ok := positions[deployment.ID]
			if !ok || position <= last {
				t.Fatalf("progress %d deployment order is not the fixture order", progressIndex)
			}
			last = position
		}
	}
}
