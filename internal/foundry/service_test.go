package foundry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

type controlledSource struct {
	discover    func(context.Context, func(Foundry), func(DiscoveryProgress)) ([]Foundry, error)
	deployments func(context.Context, Foundry, func(int)) ([]Deployment, error)
}

func (s controlledSource) Discover(ctx context.Context, found func(Foundry), progress func(DiscoveryProgress)) ([]Foundry, error) {
	return s.discover(ctx, found, progress)
}

func (s controlledSource) Deployments(ctx context.Context, foundry Foundry, progress func(int)) ([]Deployment, error) {
	return s.deployments(ctx, foundry, progress)
}

func TestInitialViewStartsModelsBeforeDiscoveryEndsAndSavesAllResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	first := Foundry{ID: "first", Name: "first-foundry"}
	second := Foundry{ID: "second", Name: "second-foundry"}
	models := []Deployment{{ID: "model-1", DeploymentName: "chat", ModelName: "gpt", Version: "1"}}
	modelStarted := make(chan Foundry, 1)
	finishDiscovery := make(chan struct{})
	source := controlledSource{
		discover: func(ctx context.Context, discovered func(Foundry), report func(DiscoveryProgress)) ([]Foundry, error) {
			report(DiscoveryProgress{SubscriptionSearch: "completed", Subscriptions: []SubscriptionProgress{
				{ID: "subscription", Name: "subscription", Phase: "running", FoundryCount: 1},
			}})
			discovered(first)
			select {
			case <-finishDiscovery:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			discovered(second)
			report(DiscoveryProgress{SubscriptionSearch: "completed", Subscriptions: []SubscriptionProgress{
				{ID: "subscription", Name: "subscription", Phase: "completed", FoundryCount: 2},
			}})
			return []Foundry{first, second}, nil
		},
		deployments: func(_ context.Context, selected Foundry, report func(int)) ([]Deployment, error) {
			modelStarted <- selected
			report(len(models))
			return models, nil
		},
	}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	var mu sync.Mutex
	var events []Progress
	service := New(func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, path,
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(name string, data any) {
			if name != ProgressEvent {
				t.Errorf("unexpected event %q", name)
			}
			mu.Lock()
			defer mu.Unlock()
			events = append(events, data.(Progress))
		})
	type result struct {
		view InitialFoundryView
		err  error
	}
	finished := make(chan result, 1)
	go func() { view, err := service.GetInitialView(ctx); finished <- result{view, err} }()
	select {
	case selected := <-modelStarted:
		if selected != first {
			t.Fatalf("selected %v, want first discovery %v", selected, first)
		}
	case <-ctx.Done():
		t.Fatal("model acquisition waited for discovery completion")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file saved before discovery finished: %v", err)
	}
	close(finishDiscovery)
	var got result
	select {
	case got = <-finished:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	want := InitialFoundryView{Foundries: []Foundry{first, second}, SelectedFoundryID: first.ID, Deployments: models}
	if !reflect.DeepEqual(got.view, want) {
		t.Fatalf("view = %#v, want %#v", got.view, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved InitialFoundryView
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved = %#v, want %#v", saved, want)
	}
	mu.Lock()
	defer mu.Unlock()
	last := events[len(events)-1]
	if last.SavePhase != "completed" || last.ModelPhase != "completed" || last.ModelCount != 1 || last.Subscriptions[0].Phase != "completed" {
		t.Fatalf("incomplete final progress: %#v", last)
	}
	for _, event := range events {
		if event.SavePhase != "waiting" && (event.ModelPhase != "completed" || event.Subscriptions[0].Phase != "completed") {
			t.Fatalf("save began before all acquisition completed: %#v", event)
		}
	}
}

func TestDiscoveryFailureCancelsModelsAndKeepsSavedState(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	modelStarted := make(chan struct{})
	modelCanceled := make(chan struct{})
	source := controlledSource{
		discover: func(ctx context.Context, discovered func(Foundry), _ func(DiscoveryProgress)) ([]Foundry, error) {
			discovered(Foundry{ID: "first"})
			select {
			case <-modelStarted:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return nil, errors.New("subscription page failed")
		},
		deployments: func(ctx context.Context, _ Foundry, _ func(int)) ([]Deployment, error) {
			close(modelStarted)
			<-ctx.Done()
			close(modelCanceled)
			return nil, ctx.Err()
		},
	}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	previous := []byte("previous successful state")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	service := New(func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, path,
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ string, data any) {
			if data.(Progress).SavePhase != "waiting" {
				t.Error("failed acquisition entered saving")
			}
		})
	view, err := service.GetInitialView(ctx)
	if err == nil || !reflect.DeepEqual(view, InitialFoundryView{}) {
		t.Fatalf("returned partial success: %#v, %v", view, err)
	}
	select {
	case <-modelCanceled:
	default:
		t.Fatal("model acquisition not canceled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, previous) {
		t.Fatalf("previous state overwritten: %q", data)
	}
}
