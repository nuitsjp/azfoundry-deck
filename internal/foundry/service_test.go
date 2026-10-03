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
	foundries   func(context.Context) ([]Foundry, error)
	deployments func(context.Context, Foundry, func(int)) ([]Deployment, error)
}

func (s controlledSource) Foundries(ctx context.Context) ([]Foundry, error) {
	return s.foundries(ctx)
}

func (s controlledSource) Deployments(ctx context.Context, foundry Foundry, progress func(int)) ([]Deployment, error) {
	return s.deployments(ctx, foundry, progress)
}

func TestInitialViewSelectsFirstSortedFoundryAfterListAndSavesAllResults(t *testing.T) {
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
	var saved InitialFoundryView
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved = %#v, want %#v", saved, want)
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

func TestChangeFoundryAcquiresOnlySelectedModelsAndCommitsAfterSaving(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	oldFoundry := Foundry{ID: "old", Name: "old-foundry"}
	target := Foundry{ID: "target", Name: "target-foundry"}
	oldModels := []Deployment{{ID: "old-model", DeploymentName: "old-chat"}}
	models := []Deployment{{ID: "target-model", DeploymentName: "target-chat"}}
	previous := InitialFoundryView{Foundries: []Foundry{oldFoundry, target}, SelectedFoundryID: oldFoundry.ID, Deployments: oldModels}
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
	var saved InitialFoundryView
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, got.view) {
		t.Fatalf("saved state = %#v, returned %#v", saved, got.view)
	}
	for id, wantModels := range map[string][]Deployment{oldFoundry.ID: oldModels, target.ID: models} {
		data, err := os.ReadFile(modelsPath(path, id))
		if err != nil {
			t.Fatal(err)
		}
		var saved savedModels
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved.Deployments, wantModels) {
			t.Fatalf("archived %s models = %#v, want %#v", id, saved.Deployments, wantModels)
		}
	}
	first, last := events[0], events[len(events)-1]
	if first.FoundryPhase != "completed" || first.ModelPhase != "running" || first.ModelCount != 0 || first.SelectedFoundryName != target.Name {
		t.Fatalf("unexpected initial progress: %#v", first)
	}
	if last.ModelPhase != "completed" || last.ModelCount != len(models) || last.SelectedFoundryName != target.Name {
		t.Fatalf("incomplete final progress: %#v", last)
	}
}

func TestChangeFoundryReusesSavedModelsAndDoesNotSaveSameSelection(t *testing.T) {
	for _, sameSelection := range []bool{false, true} {
		name := "saved-target"
		if sameSelection {
			name = "same-selection"
		}
		t.Run(name, func(t *testing.T) {
			old := Foundry{ID: "old"}
			target := Foundry{ID: "target"}
			previous := InitialFoundryView{Foundries: []Foundry{old, target}, SelectedFoundryID: old.ID, Deployments: []Deployment{{ID: "old-model"}}}
			models := []Deployment{{ID: "saved-target-model", Version: "saved-version"}}
			path := filepath.Join(t.TempDir(), "foundry-state.json")
			previousData, err := json.Marshal(previous)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, previousData, 0600); err != nil {
				t.Fatal(err)
			}
			selectedID := target.ID
			cachePath := modelsPath(path, target.ID)
			cacheData, err := json.Marshal(savedModels{FetchedAt: "2024-01-01T09:00:00+09:00", Deployments: models})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cachePath, cacheData, 0600); err != nil {
				t.Fatal(err)
			}
			if sameSelection {
				selectedID, cachePath, cacheData = old.ID, path, previousData
			}
			stamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			if err := os.Chtimes(cachePath, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			service := New(new(sync.Mutex), func() (Source, error) {
				t.Error("saved models called source factory")
				return nil, errors.New("unexpected source call")
			}, func(context.Context) error { return nil }, func() (string, error) { return path, nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), func(string, any) {
				t.Error("saved models emitted acquisition progress")
			})
			view, err := service.ChangeFoundry(t.Context(), selectedID)
			if err != nil {
				t.Fatal(err)
			}
			want := previous
			if !sameSelection {
				want.SelectedFoundryID, want.Deployments, want.DeploymentsFetchedAt = target.ID, models, "2024-01-01T09:00:00+09:00"
			}
			if !reflect.DeepEqual(view, want) {
				t.Fatalf("view = %#v, want %#v", view, want)
			}
			data, err := os.ReadFile(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(data, cacheData) || !after.ModTime().Equal(before.ModTime()) {
				t.Fatal("existing cache or same selected state was rewritten")
			}
			if sameSelection {
				if _, err := os.Stat(modelsPath(path, old.ID)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("same selection saved a model archive: %v", err)
				}
			}
			stateData, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var saved InitialFoundryView
			if err := json.Unmarshal(stateData, &saved); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved, view) {
				t.Fatalf("saved selection = %#v, returned %#v", saved, view)
			}
		})
	}
}

func TestChangeFoundryArchiveSaveFailureKeepsPreviousSelectionAndModels(t *testing.T) {
	old := Foundry{ID: "old"}
	target := Foundry{ID: "target"}
	previous := InitialFoundryView{Foundries: []Foundry{old, target}, SelectedFoundryID: old.ID, Deployments: []Deployment{{ID: "old-model"}}}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	previousData, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, previousData, 0600); err != nil {
		t.Fatal(err)
	}
	acquired := false
	source := controlledSource{
		foundries: func(context.Context) ([]Foundry, error) {
			t.Error("switching listed Foundries")
			return nil, errors.New("unexpected Foundry list")
		},
		deployments: func(_ context.Context, selected Foundry, report func(int)) ([]Deployment, error) {
			if selected.ID != target.ID {
				t.Errorf("selected %s, want %s", selected.ID, target.ID)
			}
			acquired = true
			// The target was absent when read; this collision forces the final rename to fail.
			if err := os.Mkdir(modelsPath(path, target.ID), 0700); err != nil {
				return nil, err
			}
			report(1)
			return []Deployment{{ID: "new-model"}}, nil
		},
	}
	var events []Progress
	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ string, data any) { events = append(events, data.(Progress)) })
	view, err := service.ChangeFoundry(t.Context(), target.ID)
	if err == nil || !reflect.DeepEqual(view, InitialFoundryView{}) {
		t.Fatalf("returned success after archive failure: %#v, %v", view, err)
	}
	if !acquired || events[len(events)-1].ModelPhase != "completed" {
		t.Fatal("did not reach the archive save boundary")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, previousData) {
		t.Fatalf("previous state overwritten: %s", data)
	}
	archive, err := os.ReadFile(modelsPath(path, old.ID))
	if err != nil {
		t.Fatal(err)
	}
	var oldModels savedModels
	if err := json.Unmarshal(archive, &oldModels); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldModels.Deployments, previous.Deployments) {
		t.Fatalf("old model archive = %#v, want %#v", oldModels.Deployments, previous.Deployments)
	}
}
