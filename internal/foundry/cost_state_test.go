package foundry

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type costStateSource struct {
	controlledSource
	cost func(context.Context, Foundry) (Cost, error)
}

func (s costStateSource) Cost(ctx context.Context, foundry Foundry) (Cost, error) {
	return s.cost(ctx, foundry)
}

func TestCostStateRefetchesOnRefreshAndIgnoresOldCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	file := filepath.Join(t.TempDir(), "foundry-state.json")
	if err := save(file, InitialFoundryView{
		Foundries:         []Foundry{{ID: "first", Name: "a"}},
		SelectedFoundryID: "first",
	}); err != nil {
		t.Fatal(err)
	}
	releaseOld := make(chan struct{})
	var calls atomic.Int32
	service := newFoundryService(file, costStateSource{
		controlledSource: controlledSource{
			deployments: func(_ context.Context, foundry Foundry, _ func(int)) ([]Deployment, error) {
				return []Deployment{{ID: foundry.ID + "-chat"}}, nil
			},
		},
		cost: func(_ context.Context, _ Foundry) (Cost, error) {
			if calls.Add(1) == 1 {
				// Fail after the refresh, like a response already in flight when it started.
				select {
				case <-releaseOld:
				case <-ctx.Done():
					return Cost{}, ctx.Err()
				}
				return Cost{}, errors.New("throttled")
			}
			return Cost{Amount: 12.5, Currency: "JPY"}, nil
		},
	}, func(string, any) {})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := service.GetCostState(ctx)
	if err != nil || state.FoundryID != "first" || !state.Loading || state.Cost != nil || state.Error != nil {
		t.Fatalf("pending state = %#v, %v", state, err)
	}
	oldDone := service.cost.fetch.done
	// Refreshing the same Foundry fetches the cost again.
	if _, err := service.RefreshDeployments(ctx); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, service.cost.fetch.done)
	close(releaseOld)
	awaitCapacitySignal(t, ctx, oldDone)
	state, err = service.GetCostState(ctx)
	if err != nil || state.Loading || state.Error != nil || state.Cost == nil || *state.Cost != (Cost{Amount: 12.5, Currency: "JPY"}) {
		t.Fatalf("state after old completion = %#v, %v", state, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("cost calls = %d, want 2", calls.Load())
	}
}

func TestCostStateReportsFailureWithPublicCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	file := filepath.Join(t.TempDir(), "foundry-state.json")
	if err := save(file, InitialFoundryView{
		Foundries:         []Foundry{{ID: "first", Name: "a"}},
		SelectedFoundryID: "first",
	}); err != nil {
		t.Fatal(err)
	}
	service := newFoundryService(file, costStateSource{
		controlledSource: controlledSource{
			deployments: func(_ context.Context, foundry Foundry, _ func(int)) ([]Deployment, error) {
				return []Deployment{{ID: foundry.ID + "-chat"}}, nil
			},
		},
		cost: func(context.Context, Foundry) (Cost, error) {
			return Cost{}, errors.New("AuthorizationFailed: secret detail")
		},
	}, func(string, any) {})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, service.cost.fetch.done)
	state, err := service.GetCostState(ctx)
	if err != nil || state.Loading || state.Cost != nil || state.Error == nil || state.Error.Code != "COST_LOAD_FAILED" || state.Error.Message != "Could not retrieve the month-to-date cost." {
		t.Fatalf("failed state = %#v, %v", state, err)
	}
}
