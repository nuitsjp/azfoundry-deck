package mock

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestModelSuccessFixturePreservesVersionsSKUsAndUnknowns(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait
	accountID := accountResourceID(mockAccounts[0])

	result, err := provider.FetchModels(context.Background(), accountID)
	if err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if len(result.Models) != 6 || len(result.Failures) != 0 || result.FetchedAt == "" {
		t.Fatalf("model result = %+v, want six rows and no failures", result)
	}
	if result.Models[0].Name != "gpt-4o" || result.Models[1].Version != "2024-11-20" || !result.Models[1].IsDefaultVersion {
		t.Fatalf("gpt-4o versions/default marker = %+v", result.Models[:2])
	}
	if len(result.Models[0].SKUs) != 2 || result.Models[0].SKUs[1].Name != "Standard" {
		t.Fatalf("gpt-4o SKUs = %v, want both SKU names", result.Models[0].SKUs)
	}
	globalStandard := result.Models[0].SKUs[0]
	if globalStandard.Capacity == nil || globalStandard.Capacity.Default == nil || *globalStandard.Capacity.Default != 10 {
		t.Fatalf("GlobalStandard capacity = %+v, want a default of 10", globalStandard.Capacity)
	}
	standard := result.Models[0].SKUs[1]
	if standard.Capacity == nil || standard.Capacity.Default != nil || standard.Capacity.Minimum == nil || *standard.Capacity.Minimum != 1 {
		t.Fatalf("Standard capacity = %+v, want a range without a default", standard.Capacity)
	}
	embedding := result.Models[3]
	if len(embedding.SKUs) != 1 || len(embedding.SKUs[0].Capacity.AllowedValues) != 3 {
		t.Fatalf("embedding capacity = %+v, want three allowed values", embedding.SKUs)
	}
	preview := result.Models[4]
	if len(preview.SKUs) != 1 || preview.SKUs[0].Capacity != nil {
		t.Fatalf("preview SKU capacity = %+v, want a missing capacity contract", preview.SKUs)
	}
	unknown := result.Models[len(result.Models)-1]
	if unknown.Format != "" || unknown.Version != "" || unknown.Lifecycle != "" || unknown.SKUs == nil {
		t.Fatalf("unknown optional fields = %+v, want empty strings and empty SKU array", unknown)
	}
}

func TestModelScenariosAndTargetValidation(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait
	accountID := accountResourceID(mockAccounts[0])

	for _, test := range []struct {
		name         string
		scenario     string
		wantModels   int
		wantFailures int
	}{
		{name: "empty", scenario: ModelScenarioEmpty, wantModels: 0, wantFailures: 0},
		{name: "failure", scenario: ModelScenarioFailure, wantModels: 0, wantFailures: 1},
		{name: "loading", scenario: ModelScenarioLoading, wantModels: 6, wantFailures: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := provider.SetModelScenario(test.scenario); err != nil {
				t.Fatal(err)
			}
			result, err := provider.FetchModels(context.Background(), accountID)
			if err != nil {
				t.Fatalf("FetchModels returned error: %v", err)
			}
			if len(result.Models) != test.wantModels || len(result.Failures) != test.wantFailures {
				t.Fatalf("result = %+v, want %d models/%d failures", result, test.wantModels, test.wantFailures)
			}
		})
	}

	unknown, err := provider.FetchModels(context.Background(), "/subscriptions/unknown")
	if err != nil || len(unknown.Models) != 0 || len(unknown.Failures) != 1 || unknown.Failures[0].Code != "account-not-found" {
		t.Fatalf("unknown account result = %+v, err=%v", unknown, err)
	}
	if err := provider.SetModelScenario("not-a-scenario"); err == nil {
		t.Fatal("SetModelScenario accepted an unknown scenario")
	}
}

func TestModelFetchHonorsCancellation(t *testing.T) {
	provider := NewProvider()
	provider.wait = waitContext
	if err := provider.SetModelScenario(ModelScenarioLoading); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := provider.FetchModels(ctx, accountResourceID(mockAccounts[0]))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchModels error = %v, want context.Canceled", err)
	}
	if result.Models == nil || result.Failures == nil {
		t.Fatal("cancelled result arrays must be empty arrays, not nil")
	}
}

func TestModelScenarioWaitUsesExpectedDelay(t *testing.T) {
	provider := NewProvider()
	var got time.Duration
	provider.wait = func(_ context.Context, duration time.Duration) error {
		got = duration
		return nil
	}
	if err := provider.SetModelScenario(ModelScenarioDelayed); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.FetchModels(context.Background(), accountResourceID(mockAccounts[0])); err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("model delay = %s, want 2s", got)
	}
}
