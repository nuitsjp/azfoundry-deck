package foundry

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type capacityStateSource struct {
	controlledSource
	limits func(context.Context, Foundry) (CapacityLimits, error)
}

func (s capacityStateSource) CapacityLimits(ctx context.Context, foundry Foundry, _ func(CapacityLimits)) (CapacityLimits, error) {
	return s.limits(ctx, foundry)
}

func capacityStateService(t *testing.T, limits func(context.Context, Foundry) (CapacityLimits, error), emit func(string, any)) *Service {
	t.Helper()
	file := filepath.Join(t.TempDir(), "foundry-state.json")
	if err := save(file, InitialFoundryView{
		Foundries:         []Foundry{{ID: "first", Name: "a"}, {ID: "second", Name: "b"}},
		SelectedFoundryID: "first",
	}); err != nil {
		t.Fatal(err)
	}
	source := capacityStateSource{
		controlledSource: controlledSource{
			deployments: func(_ context.Context, foundry Foundry, _ func(int)) ([]Deployment, error) {
				return []Deployment{{ID: foundry.ID + "-chat", Version: "1"}}, nil
			},
		},
		limits: limits,
	}
	return newFoundryService(file, source, emit)
}

func awaitCapacitySignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestCapacityStateReturnsWhileLimitsRunAndOutlivesInitialRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	var fetches atomic.Int32
	service := capacityStateService(t, func(fetchCtx context.Context, _ Foundry) (CapacityLimits, error) {
		fetches.Add(1)
		started <- fetchCtx
		select {
		case <-release:
			return fakeLimits{quota: 100}, nil
		case <-fetchCtx.Done():
			return nil, fetchCtx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, func(string, any) {})
	source, err := service.source()
	if err != nil {
		t.Fatal(err)
	}
	controlled := source.(capacityStateSource)
	var fetchCtx context.Context
	controlled.deployments = func(requestCtx context.Context, _ Foundry, _ func(int)) ([]Deployment, error) {
		// The list is still running when the capacity request starts.
		select {
		case fetchCtx = <-started:
			return []Deployment{{ID: "first-chat", Version: "1"}}, nil
		case <-requestCtx.Done():
			return nil, requestCtx.Err()
		}
	}
	service.source = func() (Source, error) { return controlled, nil }
	requestCtx, cancelRequest := context.WithCancel(ctx)
	defer cancelRequest()
	view, err := service.GetInitialView(requestCtx)
	if err != nil || len(view.Deployments) != 1 {
		t.Fatalf("initial view = %#v, %v", view, err)
	}
	for range 2 {
		state, err := service.GetCapacityState(ctx, false)
		if err != nil || !state.Loading || state.Error != nil || len(state.Maximums) != 0 {
			t.Fatalf("pending state = %#v, %v", state, err)
		}
	}
	cancelRequest()
	select {
	case <-fetchCtx.Done():
		t.Fatal("initial request cancellation cancelled the capacity fetch")
	default:
	}
	close(release)
	awaitCapacitySignal(t, ctx, service.limits.fetch.done)
	state, err := service.GetCapacityState(ctx, false)
	if err != nil || state.Loading || state.Error != nil || state.Maximums["first-chat"] == nil || *state.Maximums["first-chat"] != 100 || fetches.Load() != 1 {
		t.Fatalf("ready state = %#v, %v, fetches = %d", state, err, fetches.Load())
	}
}

func TestCapacityStateIgnoresOldFoundryCompletionAfterSwitch(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	releaseOld := make(chan struct{})
	service := capacityStateService(t, func(_ context.Context, foundry Foundry) (CapacityLimits, error) {
		if foundry.ID == "first" {
			// Deliberately complete after cancellation to model an external response
			// that was already in flight when the selection changed.
			select {
			case <-releaseOld:
				return fakeLimits{quota: 100}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return fakeLimits{quota: 200}, nil
	}, func(string, any) {})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	oldDone := service.limits.fetch.done
	if _, err := service.ChangeFoundry(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, service.limits.fetch.done)
	close(releaseOld)
	awaitCapacitySignal(t, ctx, oldDone)
	state, err := service.GetCapacityState(ctx, false)
	if err != nil || state.FoundryID != "second" || state.Loading || state.Error != nil || state.Maximums["second-chat"] == nil || *state.Maximums["second-chat"] != 200 || len(state.Maximums) != 1 {
		t.Fatalf("state after old completion = %#v, %v", state, err)
	}
}

func TestStopViewWaitsForCancellationAndKeepsOrDiscardsTheView(t *testing.T) {
	for _, discard := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep", true: "discard"}[discard], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cancelled := make(chan struct{})
			finish := make(chan struct{})
			events := make(chan string, 2)
			service := capacityStateService(t, func(fetchCtx context.Context, _ Foundry) (CapacityLimits, error) {
				select {
				case <-fetchCtx.Done():
					close(cancelled)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				select {
				case <-finish:
					return nil, fetchCtx.Err()
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}, func(name string, data any) {
				if name == CapacityReadyEvent {
					events <- data.(string)
				}
			})
			if _, err := service.GetInitialView(ctx); err != nil {
				t.Fatal(err)
			}
			stopped := make(chan struct{})
			go func() {
				service.operations.Lock()
				StopView(service, discard)
				service.operations.Unlock()
				close(stopped)
			}()
			awaitCapacitySignal(t, ctx, cancelled)
			select {
			case <-stopped:
				t.Fatal("StopView returned before the cancelled source exited")
			default:
			}
			close(finish)
			awaitCapacitySignal(t, ctx, stopped)
			state, err := service.GetCapacityState(ctx, false)
			if err != nil || state.Loading {
				t.Fatalf("stopped state = %#v, %v", state, err)
			}
			if discard {
				if state.FoundryID != "" || len(service.current.deployments) != 0 || service.limits.fetch != nil {
					t.Fatalf("discarded view retained data: %#v, %#v", service.current, service.limits)
				}
				select {
				case event := <-events:
					t.Fatalf("discarded view emitted capacity notification: %s", event)
				default:
				}
			} else {
				if state.FoundryID != "first" || state.Error == nil || len(service.current.deployments) != 1 {
					t.Fatalf("kept view = %#v, %#v", state, service.current)
				}
				select {
				case event := <-events:
					if event != "first" {
						t.Fatalf("notified Foundry = %q", event)
					}
				case <-ctx.Done():
					t.Fatal("kept view was not notified of its stopped capacity fetch")
				}
			}
		})
	}
}

func TestCapacityStateRetriesOnlyExplicitlyAndRecovers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var fetches atomic.Int32
	retryStarted := make(chan struct{})
	releaseRetry := make(chan struct{})
	service := capacityStateService(t, func(fetchCtx context.Context, _ Foundry) (CapacityLimits, error) {
		if fetches.Add(1) == 1 {
			return nil, errors.New("capacity request failed")
		}
		close(retryStarted)
		select {
		case <-releaseRetry:
			return fakeLimits{quota: 100}, nil
		case <-fetchCtx.Done():
			return nil, fetchCtx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, func(string, any) {})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, service.limits.fetch.done)
	for range 2 {
		state, err := service.GetCapacityState(ctx, false)
		if err != nil || state.Loading || state.Error == nil || state.Error.Code != "DEPLOYMENT_DETAIL_FAILED" || fetches.Load() != 1 {
			t.Fatalf("failed state = %#v, %v, fetches = %d", state, err, fetches.Load())
		}
	}
	state, err := service.GetCapacityState(ctx, true)
	if err != nil || !state.Loading || state.Error != nil {
		t.Fatalf("retry state = %#v, %v", state, err)
	}
	awaitCapacitySignal(t, ctx, retryStarted)
	if _, err := service.GetCapacityState(ctx, true); err != nil || fetches.Load() != 2 {
		t.Fatalf("pending retry restarted the source: %v, fetches = %d", err, fetches.Load())
	}
	close(releaseRetry)
	awaitCapacitySignal(t, ctx, service.limits.fetch.done)
	state, err = service.GetCapacityState(ctx, false)
	if err != nil || state.Loading || state.Error != nil || state.Maximums["first-chat"] == nil || *state.Maximums["first-chat"] != 100 || fetches.Load() != 2 {
		t.Fatalf("recovered state = %#v, %v, fetches = %d", state, err, fetches.Load())
	}
}
