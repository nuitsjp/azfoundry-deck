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

func (s controlledCreateSource) ListModels(ctx context.Context, f Foundry) ([]ModelCatalogItem, error) {
	if s.listModels != nil {
		return s.listModels(ctx, f)
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
	initial := InitialFoundryView{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
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
	if !reflect.DeepEqual(items, expectedItems) {
		t.Fatalf("items = %#v, want %#v", items, expectedItems)
	}
}

func TestGetModelCatalogFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := InitialFoundryView{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
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
	initial := InitialFoundryView{
		Foundries:         []Foundry{f},
		SelectedFoundryID: f.ID,
		Deployments:       []Deployment{{ID: "old-deploy", DeploymentName: "old"}},
	}
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

	view, err := service.CreateDeployment(ctx, deployedSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(view.Deployments) != 2 || view.Deployments[1].DeploymentName != "new-deploy" {
		t.Fatalf("unexpected returned deployments: %#v", view.Deployments)
	}

	// Verify state file was updated
	savedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var savedView InitialFoundryView
	if err := json.Unmarshal(savedData, &savedView); err != nil {
		t.Fatal(err)
	}
	if len(savedView.Deployments) != 2 {
		t.Fatalf("saved deployments length = %d, want 2", len(savedView.Deployments))
	}
}

func TestCreateDeploymentValidationAndFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	f := Foundry{ID: "f1", Name: "foundry-1"}
	initial := InitialFoundryView{Foundries: []Foundry{f}, SelectedFoundryID: f.ID}
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
	initialWithDeploy := InitialFoundryView{
		Foundries:         []Foundry{f},
		SelectedFoundryID: f.ID,
		Deployments:       []Deployment{{ID: "d1", DeploymentName: "existing-chat"}},
	}
	data2, _ := json.Marshal(initialWithDeploy)
	_ = os.WriteFile(path, data2, 0600)

	_, err = service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "existing-chat", ModelName: "gpt-4o"})
	if err == nil {
		t.Fatal("expected error for duplicate deployment name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}
}
