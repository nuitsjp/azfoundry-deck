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
	"strings"
	"sync"
	"testing"
	"time"
)

type controlledSource struct {
	foundries   func(context.Context) ([]Foundry, error)
	deployments func(context.Context, Foundry, func(int)) ([]Deployment, error)
}

func (s controlledSource) Foundries(ctx context.Context) ([]Foundry, error) {
	return s.foundries(ctx)
}

func (s controlledSource) Deployments(ctx context.Context, foundry Foundry, progress func(int)) ([]Deployment, error) {
	return s.deployments(ctx, foundry, progress)
}

func TestInitialViewSelectsFirstSortedFoundryAfterListAndSavesOnlyTheFoundryList(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// The source returns them out of order; the service orders by subscription name, then Foundry name.
	first := Foundry{ID: "first", Name: "a-foundry", SubscriptionName: "Alpha"}
	second := Foundry{ID: "second", Name: "b-foundry", SubscriptionName: "Alpha"}
	third := Foundry{ID: "third", Name: "a-foundry", SubscriptionName: "Beta"}
	models := []Deployment{{ID: "model-1", DeploymentName: "chat", ModelName: "gpt", Version: "1"}}
	listed := false
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) {
			listed = true
			return []Foundry{third, second, first}, nil
		},
		deployments: func(_ context.Context, selected Foundry, report func(int)) ([]Deployment, error) {
			if !listed {
				t.Error("models were requested before the Foundry list was complete")
			}
			if selected != first {
				t.Errorf("selected %v, want first sorted %v", selected, first)
			}
			report(len(models))
			return models, nil
		},
	}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	var events []Progress
	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(name string, data any) {
			if name != ProgressEvent {
				t.Errorf("unexpected event %q", name)
			}
			events = append(events, data.(Progress))
		})
	got, err := service.GetInitialView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := InitialFoundryView{Foundries: []Foundry{first, second, third}, SelectedFoundryID: first.ID, Deployments: models, FoundriesFetchedAt: got.FoundriesFetchedAt, DeploymentsFetchedAt: got.DeploymentsFetchedAt}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("view = %#v, want %#v", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved savedState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if wantSaved := (savedState{Foundries: want.Foundries, SelectedFoundryID: first.ID, FoundriesFetchedAt: want.FoundriesFetchedAt}); !reflect.DeepEqual(saved, wantSaved) {
		t.Fatalf("saved = %#v, want %#v", saved, wantSaved)
	}
	if strings.Contains(string(data), "deployments") {
		t.Fatalf("deployments were saved: %s", data)
	}
	if events[0] != (Progress{FoundryPhase: "running", ModelPhase: "waiting"}) {
		t.Fatalf("unexpected initial progress: %#v", events[0])
	}
	if last := events[len(events)-1]; last != (Progress{FoundryPhase: "completed", FoundryCount: 3, SelectedFoundryName: first.Name, ModelPhase: "completed", ModelCount: 1}) {
		t.Fatalf("incomplete final progress: %#v", last)
	}
	for _, event := range events {
		if event.ModelPhase != "waiting" && event.FoundryPhase != "completed" {
			t.Fatalf("model acquisition began before the Foundry list completed: %#v", event)
		}
	}
}

func TestFoundryListFailureSkipsModelsAndKeepsSavedState(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) { return nil, errors.New("resource graph failed") },
		deployments: func(context.Context, Foundry, func(int)) ([]Deployment, error) {
			t.Error("models were requested after the Foundry list failed")
			return nil, errors.New("unexpected models")
		},
	}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	previous := []byte("previous successful state")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ string, data any) {
			if data.(Progress).ModelPhase != "waiting" {
				t.Error("failed list entered model acquisition")
			}
		})
	view, err := service.acquire(ctx, path)
	if err == nil || !reflect.DeepEqual(view, InitialFoundryView{}) {
		t.Fatalf("returned partial success: %#v, %v", view, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, previous) {
		t.Fatalf("previous state overwritten: %q", data)
	}
}

func TestChangeFoundryAcquiresOnlySelectedModelsAndSavesOnlyTheSelection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	oldFoundry := Foundry{ID: "old", Name: "old-foundry"}
	target := Foundry{ID: "target", Name: "target-foundry"}
	models := []Deployment{{ID: "target-model", DeploymentName: "target-chat"}}
	previous := savedState{Foundries: []Foundry{oldFoundry, target}, SelectedFoundryID: oldFoundry.ID}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	previousData, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, previousData, 0600); err != nil {
		t.Fatal(err)
	}
	started := make(chan Foundry, 1)
	release := make(chan struct{})
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) {
			t.Error("switching listed Foundries")
			return nil, errors.New("unexpected Foundry list")
		},
		deployments: func(ctx context.Context, selected Foundry, report func(int)) ([]Deployment, error) {
			started <- selected
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			report(len(models))
			return models, nil
		},
	}
	var events []Progress
	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(name string, data any) {
			if name != ProgressEvent {
				t.Errorf("unexpected event %q", name)
			}
			events = append(events, data.(Progress))
		})
	type result struct {
		view InitialFoundryView
		err  error
	}
	finished := make(chan result, 1)
	go func() { view, err := service.ChangeFoundry(ctx, target.ID); finished <- result{view, err} }()
	select {
	case selected := <-started:
		if selected != target {
			t.Fatalf("selected = %#v, want %#v", selected, target)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, previousData) {
		t.Fatalf("saved selection changed during acquisition: %s", data)
	}
	close(release)
	var got result
	select {
	case got = <-finished:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	want := InitialFoundryView{Foundries: previous.Foundries, SelectedFoundryID: target.ID, Deployments: models, DeploymentsFetchedAt: got.view.DeploymentsFetchedAt}
	if !reflect.DeepEqual(got.view, want) {
		t.Fatalf("view = %#v, want %#v", got.view, want)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved savedState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if wantSaved := (savedState{Foundries: previous.Foundries, SelectedFoundryID: target.ID}); !reflect.DeepEqual(saved, wantSaved) {
		t.Fatalf("saved state = %#v, want %#v", saved, wantSaved)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "foundry-models")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("model files were saved: %v", err)
	}
	first, last := events[0], events[len(events)-1]
	if first.FoundryPhase != "completed" || first.ModelPhase != "running" || first.ModelCount != 0 || first.SelectedFoundryName != target.Name {
		t.Fatalf("unexpected initial progress: %#v", first)
	}
	if last.ModelPhase != "completed" || last.ModelCount != len(models) || last.SelectedFoundryName != target.Name {
		t.Fatalf("incomplete final progress: %#v", last)
	}
}

func newFoundryService(path string, source Source, emit func(string, any)) *Service {
	return New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), emit)
}

func TestInitialViewWithSavedStateFetchesDeploymentsAndKeepsSelection(t *testing.T) {
	old := Foundry{ID: "old"}
	target := Foundry{ID: "target"}
	saved := savedState{Foundries: []Foundry{old, target}, SelectedFoundryID: target.ID, FoundriesFetchedAt: "2024-01-01T09:00:00+09:00"}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	models := []Deployment{{ID: "target-model"}}
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) {
			t.Error("saved Foundries were fetched again")
			return nil, errors.New("unexpected Foundry list")
		},
		deployments: func(_ context.Context, selected Foundry, report func(int)) ([]Deployment, error) {
			if selected != target {
				t.Errorf("fetched %v, want the saved selection %v", selected, target)
			}
			report(len(models))
			return models, nil
		},
	}
	service := newFoundryService(path, source, func(string, any) {})
	view, err := service.GetInitialView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := InitialFoundryView{Foundries: saved.Foundries, SelectedFoundryID: target.ID, Deployments: models, FoundriesFetchedAt: saved.FoundriesFetchedAt, DeploymentsFetchedAt: view.DeploymentsFetchedAt}
	if !reflect.DeepEqual(view, want) || view.DeploymentsFetchedAt == "" {
		t.Fatalf("view = %#v, want %#v", view, want)
	}
}

func TestChangeToTheSameFoundryKeepsDeploymentsWithoutFetching(t *testing.T) {
	old := Foundry{ID: "old"}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, err := json.Marshal(savedState{Foundries: []Foundry{old}, SelectedFoundryID: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	fetches := 0
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) { return nil, errors.New("unexpected Foundry list") },
		deployments: func(context.Context, Foundry, func(int)) ([]Deployment, error) {
			fetches++
			return []Deployment{{ID: "model"}}, nil
		},
	}
	service := newFoundryService(path, source, func(string, any) {})
	loaded, err := service.GetInitialView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	same, err := service.ChangeFoundry(t.Context(), old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetches != 1 || !reflect.DeepEqual(same, loaded) {
		t.Fatalf("fetches = %d, view = %#v, want the loaded view %#v", fetches, same, loaded)
	}
}

func TestChangeFoundryFetchFailureKeepsSavedSelection(t *testing.T) {
	old := Foundry{ID: "old"}
	target := Foundry{ID: "target"}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	previousData, err := json.Marshal(savedState{Foundries: []Foundry{old, target}, SelectedFoundryID: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, previousData, 0600); err != nil {
		t.Fatal(err)
	}
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) { return nil, errors.New("unexpected Foundry list") },
		deployments: func(context.Context, Foundry, func(int)) ([]Deployment, error) {
			return nil, errors.New("deployments failed")
		},
	}
	service := newFoundryService(path, source, func(string, any) {})
	view, err := service.ChangeFoundry(t.Context(), target.ID)
	if err == nil || !reflect.DeepEqual(view, InitialFoundryView{}) {
		t.Fatalf("returned success after fetch failure: %#v, %v", view, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, previousData) {
		t.Fatalf("saved state changed: %s", data)
	}
}
