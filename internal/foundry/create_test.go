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

	"azfoundrydeck/internal/fault"
)

type controlledCreateSource struct {
	controlledSource
	listModels       func(context.Context, Foundry) ([]ModelCatalogItem, error)
	createDeployment func(context.Context, Foundry, DeploymentCreateSpec) error
}

type controlledCatalogLimits struct {
	fakeLimits
	items []ModelCatalogItem
}

func (l controlledCatalogLimits) Catalog() []ModelCatalogItem { return l.items }

func (s controlledCreateSource) CapacityLimits(ctx context.Context, f Foundry, _ func(CapacityLimits)) (CapacityLimits, error) {
	if s.listModels != nil {
		items, err := s.listModels(ctx, f)
		if err != nil {
			return nil, err
		}
		return controlledCatalogLimits{items: items}, nil
	}
	return nil, errors.New("listModels not implemented")
}

func (s controlledCreateSource) CreateDeployment(ctx context.Context, f Foundry, spec DeploymentCreateSpec) error {
	if s.createDeployment != nil {
		return s.createDeployment(ctx, f, spec)
	}
	return errors.New("createDeployment not implemented")
}

func TestGetModelCatalogSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := savedState{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(path, data, 0600)

	expectedItems := []ModelCatalogItem{
		{Name: "gpt-4o", Publisher: "OpenAI", Option: "Pay-as-you-go", Tasks: []string{"chat"}},
	}

	source := controlledCreateSource{
		listModels: func(_ context.Context, gotF Foundry) ([]ModelCatalogItem, error) {
			if gotF.ID != f.ID {
				t.Fatalf("expected foundry %v, got %v", f.ID, gotF.ID)
			}
			return expectedItems, nil
		},
	}

	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(string, any) {})

	items, err := service.GetModelCatalog(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(items.Models, expectedItems) {
		t.Fatalf("items = %#v, want %#v", items.Models, expectedItems)
	}
}

func TestGetModelCatalogFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := savedState{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(path, data, 0600)

	source := controlledCreateSource{
		listModels: func(context.Context, Foundry) ([]ModelCatalogItem, error) {
			return nil, errors.New("azure error")
		},
	}

	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(string, any) {})

	_, err := service.GetModelCatalog(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var fErr *fault.Error
	if !errors.As(err, &fErr) || fErr.Code != "MODEL_CATALOG_FAILED" {
		t.Fatalf("expected MODEL_CATALOG_FAILED fault, got %v", err)
	}
}

func TestCreateDeploymentSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := savedState{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(path, data, 0600)

	deployedSpec := DeploymentCreateSpec{
		DeploymentName: "new-deploy",
		ModelName:      "gpt-4o",
		Version:        "2024-05-13",
		SKU:            "Standard",
		UpgradePolicy:  "OnceNewDefaultVersionAvailable",
	}

	created := false
	source := controlledCreateSource{
		controlledSource: controlledSource{
			deployments: func(_ context.Context, gotF Foundry, report func(int)) ([]Deployment, error) {
				if !created {
					t.Fatal("deployments fetched before CreateDeployment completed")
				}
				report(2)
				return []Deployment{
					{ID: "old-deploy", DeploymentName: "old"},
					{ID: "new-deploy", DeploymentName: "new-deploy", ModelName: "gpt-4o"},
				}, nil
			},
		},
		createDeployment: func(_ context.Context, gotF Foundry, spec DeploymentCreateSpec) error {
			if gotF.ID != f.ID {
				t.Fatalf("expected foundry %v, got %v", f.ID, gotF.ID)
			}
			if !reflect.DeepEqual(spec, deployedSpec) {
				t.Fatalf("spec = %#v, want %#v", spec, deployedSpec)
			}
			created = true
			return nil
		},
	}

	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(string, any) {})

	service.setDeployments(path, f.ID, []Deployment{{ID: "old-deploy", DeploymentName: "old"}}, "")
	view, err := service.CreateDeployment(ctx, deployedSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(view.Deployments) != 2 || view.Deployments[1].DeploymentName != "new-deploy" {
		t.Fatalf("unexpected returned deployments: %#v", view.Deployments)
	}

	// The new list is kept in memory, not in the state file.
	if len(service.current.deployments) != 2 {
		t.Fatalf("kept deployments length = %d, want 2", len(service.current.deployments))
	}
	savedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(savedData), "deployments") {
		t.Fatalf("deployments were saved: %s", savedData)
	}
}

func TestCreateDeploymentValidationAndFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := savedState{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(path, data, 0600)

	source := controlledCreateSource{
		createDeployment: func(context.Context, Foundry, DeploymentCreateSpec) error {
			return errors.New("azure deployment failed")
		},
	}

	service := New(new(sync.Mutex), func() (Source, error) { return source, nil }, func(context.Context) error { return nil }, func() (string, error) { return path, nil },
		slog.New(slog.NewTextHandler(io.Discard, nil)), func(string, any) {})

	// Empty deployment name
	_, err := service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "", ModelName: "gpt-4o"})
	if err == nil {
		t.Fatal("expected error for empty deployment name")
	}
	var fErr *fault.Error
	if !errors.As(err, &fErr) || fErr.Code != "DEPLOYMENT_CREATE_FAILED" {
		t.Fatalf("expected DEPLOYMENT_CREATE_FAILED, got %v", err)
	}

	// Empty model name
	_, err = service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "deploy-1", ModelName: ""})
	if err == nil {
		t.Fatal("expected error for empty model name")
	}

	// Azure deployment failure
	_, err = service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "deploy-1", ModelName: "gpt-4o"})
	if err == nil {
		t.Fatal("expected error for azure failure")
	}

	// Duplicate deployment name
	service.setDeployments(path, f.ID, []Deployment{{ID: "d1", DeploymentName: "existing-chat"}}, "")

	_, err = service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "existing-chat", ModelName: "gpt-4o"})
	if err == nil {
		t.Fatal("expected error for duplicate deployment name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}
}
