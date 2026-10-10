package foundry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeLimits struct{ quota float64 }

func (l fakeLimits) Maximum(d Deployment) *float64              { value := l.quota; return &value }
func (l fakeLimits) CapacityStep(Deployment) (float64, float64) { return 1, 1 }
func (l fakeLimits) Versions(d Deployment) []string             { return []string{d.Version} }
func (l fakeLimits) Catalog() []ModelCatalogItem                { return []ModelCatalogItem{} }
func (l fakeLimits) RefreshQuota(context.Context) (CapacityLimits, error) {
	return fakeLimits{quota: l.quota + 1}, nil
}

type limitsSource struct {
	controlledSource
	fetches *int
}

func (s limitsSource) CapacityLimits(context.Context, Foundry, func(CapacityLimits)) (CapacityLimits, error) {
	*s.fetches++
	return fakeLimits{quota: 100}, nil
}

func newLimitsService(t *testing.T) (*Service, *int) {
	t.Helper()
	first, second := Foundry{ID: "first", Name: "a"}, Foundry{ID: "second", Name: "b"}
	path := filepath.Join(t.TempDir(), "foundry-state.json")
	data, err := json.Marshal(savedState{Foundries: []Foundry{first, second}, SelectedFoundryID: first.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	fetches := 0
	source := limitsSource{fetches: &fetches, controlledSource: controlledSource{
		foundries: func(context.Context) ([]Foundry, error) { return nil, errors.New("unexpected Foundry list") },
		deployments: func(_ context.Context, foundry Foundry, _ func(int)) ([]Deployment, error) {
			return []Deployment{{ID: foundry.ID + "-chat", Version: "1"}, {ID: foundry.ID + "-mini", Version: "1"}}, nil
		},
	}}
	return newFoundryService(path, source, func(string, any) {}), &fetches
}

func TestCapacityMaximumIsFetchedOncePerFoundryAndDiscardedOnChange(t *testing.T) {
	service, fetches := newLimitsService(t)
	ctx := t.Context()
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first-chat", "first-mini", "first-chat"} {
		maximum, err := service.GetCapacityMaximum(ctx, id)
		if err != nil || maximum == nil || *maximum != 100 {
			t.Fatalf("maximum = %v, %v", maximum, err)
		}
	}
	if *fetches != 1 {
		t.Fatalf("fetched %d times for one Foundry, want 1", *fetches)
	}
	// Refreshing the deployments of the same Foundry does not refetch the limits.
	if _, err := service.RefreshDeployments(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetCapacityMaximum(ctx, "first-mini"); err != nil || *fetches != 1 {
		t.Fatalf("refreshing deployments refetched the limits: %d, %v", *fetches, err)
	}
	if _, err := service.ChangeFoundry(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetCapacityMaximum(ctx, "second-chat"); err != nil || *fetches != 2 {
		t.Fatalf("changing the Foundry did not discard the limits: %d, %v", *fetches, err)
	}
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetCapacityMaximum(ctx, "second-chat"); err != nil || *fetches != 3 {
		t.Fatalf("loading the view did not discard the limits: %d, %v", *fetches, err)
	}
}

func TestDeploymentSettingsRefreshesOnlyTheQuota(t *testing.T) {
	service, fetches := newLimitsService(t)
	ctx := t.Context()
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetCapacityMaximum(ctx, "first-chat"); err != nil {
		t.Fatal(err)
	}
	unit, capacity := "TPM", float64(10)
	service.current.deployments[0].CapacityUnit, service.current.deployments[0].Capacity = &unit, &capacity
	settings, err := service.GetDeploymentSettings(ctx, "first-chat")
	if err != nil {
		t.Fatal(err)
	}
	if *fetches != 1 || settings.CapacityMaximum == nil || *settings.CapacityMaximum != 101 || settings.Option != "Standard" {
		t.Fatalf("fetches = %d, settings = %#v, want one fetch and a refreshed maximum", *fetches, settings)
	}
}
