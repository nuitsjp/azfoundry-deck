package foundry

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// These fixtures control the external definitions/quota boundary independently.
// Catalog deliberately returns a fresh projection, as the boundary contract requires.
type catalogSnapshot struct {
	name    string
	maximum int64
	refresh func(context.Context) (CapacityLimits, error)
}

func (l catalogSnapshot) Maximum(Deployment) *float64  { maximum := float64(l.maximum); return &maximum }
func (l catalogSnapshot) Versions(Deployment) []string { return []string{"1", "2"} }
func (l catalogSnapshot) Catalog() []ModelCatalogItem {
	maximum := l.maximum
	return []ModelCatalogItem{{Name: l.name, Versions: []string{"1", "2"}, MaxCapacity: &maximum,
		SKUs: []ModelSKUItem{{Name: "Standard", MaxCapacity: &maximum}}}}
}
func (l catalogSnapshot) RefreshQuota(ctx context.Context) (CapacityLimits, error) {
	return l.refresh(ctx)
}

type catalogCacheSource struct {
	controlledSource
	limits   func(context.Context, Foundry, func(CapacityLimits)) (CapacityLimits, error)
	mutation func() error
}

func (s catalogCacheSource) CapacityLimits(ctx context.Context, f Foundry, publish func(CapacityLimits)) (CapacityLimits, error) {
	return s.limits(ctx, f, publish)
}
func (s catalogCacheSource) CreateDeployment(context.Context, Foundry, DeploymentCreateSpec) error {
	return s.mutation()
}
func (s catalogCacheSource) DeleteDeployment(context.Context, Foundry, Deployment) error {
	return s.mutation()
}
func (s catalogCacheSource) UpdateDeployment(context.Context, Foundry, Deployment, DeploymentUpdateSpec) error {
	return s.mutation()
}

func newCatalogCacheService(t *testing.T, source catalogCacheSource) *Service {
	t.Helper()
	file := filepath.Join(t.TempDir(), "foundry-state.json")
	if err := save(file, InitialFoundryView{Foundries: []Foundry{{ID: "first"}, {ID: "second"}}, SelectedFoundryID: "first"}); err != nil {
		t.Fatal(err)
	}
	capacity, unit := float64(50000), "TPM"
	source.deployments = func(_ context.Context, f Foundry, _ func(int)) ([]Deployment, error) {
		return []Deployment{{ID: f.ID + "-chat", DeploymentName: "chat", Version: "1", Capacity: &capacity, CapacityUnit: &unit}}, nil
	}
	return newFoundryService(file, source, func(string, any) {})
}

func awaitCatalogQuota(t *testing.T, ctx context.Context, service *Service) {
	t.Helper()
	service.operations.Lock()
	done := service.limits.fetch.done
	service.operations.Unlock()
	awaitCapacitySignal(t, ctx, done)
}

func requireCatalogState(t *testing.T, ctx context.Context, service *Service, status string, maximum *int64) {
	t.Helper()
	catalog, err := service.GetModelCatalog(ctx)
	if err != nil || catalog.FoundryID != "first" || catalog.QuotaStatus != status || len(catalog.Models) != 1 || catalog.Models[0].Name != "model-first" || len(catalog.Models[0].SKUs) != 1 {
		t.Fatalf("catalog = %#v, %v", catalog, err)
	}
	for _, got := range []*int64{catalog.Models[0].MaxCapacity, catalog.Models[0].SKUs[0].MaxCapacity} {
		if (got == nil) != (maximum == nil) || (got != nil && *got != *maximum) {
			t.Fatalf("catalog maximum = %v, want %v, status %s", got, maximum, status)
		}
	}
	if (catalog.QuotaError != nil) != (status == "error") {
		t.Fatalf("quota error = %v for status %s", catalog.QuotaError, status)
	}
}

func TestCatalogSharesHomeFetchAndPublishesModelsBeforeQuota(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	models, quota := make(chan struct{}), make(chan struct{})
	var definitions, refreshes atomic.Int32
	limits := catalogSnapshot{name: "model-first", maximum: 160000, refresh: func(context.Context) (CapacityLimits, error) {
		refreshes.Add(1)
		return nil, errors.New("unexpected quota refresh")
	}}
	service := newCatalogCacheService(t, catalogCacheSource{limits: func(fetchCtx context.Context, _ Foundry, publish func(CapacityLimits)) (CapacityLimits, error) {
		definitions.Add(1)
		select {
		case <-models:
		case <-fetchCtx.Done():
			return nil, fetchCtx.Err()
		}
		publish(limits)
		select {
		case <-quota:
			return limits, nil
		case <-fetchCtx.Done():
			return limits, fetchCtx.Err()
		}
	}})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	// Several open requests wait for the same models; a Home state read remains usable.
	type response struct {
		catalog ModelCatalogView
		err     error
	}
	results := make(chan response, 3)
	for range 3 {
		go func() { result, err := service.GetModelCatalog(ctx); results <- response{result, err} }()
	}
	state, err := service.GetCapacityState(ctx, false)
	if err != nil || !state.Loading {
		t.Fatalf("pending Home state = %#v, %v", state, err)
	}
	close(models)
	for range 3 {
		select {
		case result := <-results:
			catalog := result.catalog
			if result.err != nil || catalog.QuotaStatus != "loading" || len(catalog.Models) != 1 || len(catalog.Models[0].SKUs) != 1 || catalog.Models[0].MaxCapacity != nil || catalog.Models[0].SKUs[0].MaxCapacity != nil {
				t.Fatalf("models before quota = %#v, %v", catalog, result.err)
			}
		case <-ctx.Done():
			t.Fatal("catalog waited for quota instead of returning models")
		}
	}
	close(quota)
	awaitCatalogQuota(t, ctx, service)
	maximum := int64(160000)
	for range 3 {
		requireCatalogState(t, ctx, service, "ready", &maximum)
	}
	if definitions.Load() != 1 || refreshes.Load() != 0 {
		t.Fatalf("definitions = %d, refreshes = %d", definitions.Load(), refreshes.Load())
	}
}

func TestSettingsQuotaWaitReleasesOperationLockAndRejectsOldFoundryResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	var definitions atomic.Int32
	var limits catalogSnapshot
	limits = catalogSnapshot{name: "model-first", maximum: 160000, refresh: func(context.Context) (CapacityLimits, error) {
		close(started)
		// Simulate an already in-flight external response arriving after cancellation.
		select {
		case <-release:
			return limits, nil
		case <-ctx.Done():
			return limits, ctx.Err()
		}
	}}
	service := newCatalogCacheService(t, catalogCacheSource{limits: func(_ context.Context, f Foundry, _ func(CapacityLimits)) (CapacityLimits, error) {
		definitions.Add(1)
		if f.ID == "second" {
			return catalogSnapshot{name: "model-second", maximum: 80000}, nil
		}
		return limits, nil
	}})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	awaitCatalogQuota(t, ctx, service)
	result := make(chan error, 1)
	go func() { _, err := service.GetDeploymentSettings(ctx, "first-chat"); result <- err }()
	awaitCapacitySignal(t, ctx, started)
	if _, err := service.ChangeFoundry(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("settings from old quota were returned after Foundry changed")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	awaitCatalogQuota(t, ctx, service)
	state, err := service.GetCapacityState(ctx, false)
	if err != nil || state.FoundryID != "second" || state.Maximums["second-chat"] == nil || *state.Maximums["second-chat"] != 80000 || definitions.Load() != 2 {
		t.Fatalf("state after old quota completion = %#v, %v; definitions = %d", state, err, definitions.Load())
	}
}

func TestCatalogDiscardsBlockedResponseAfterFoundryChange(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldModels := make(chan struct{})
	service := newCatalogCacheService(t, catalogCacheSource{limits: func(_ context.Context, f Foundry, publish func(CapacityLimits)) (CapacityLimits, error) {
		if f.ID == "first" {
			select {
			case <-oldModels:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		limits := catalogSnapshot{name: "model-" + f.ID, maximum: 160000}
		publish(limits)
		return limits, nil
	}})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	var requests atomic.Int32
	service.signedIn = func(context.Context) error {
		if requests.Add(1) == 1 {
			close(entered)
		}
		return nil
	}
	result := make(chan error, 1)
	go func() { _, err := service.GetModelCatalog(ctx); result <- err }()
	awaitCapacitySignal(t, ctx, entered)
	// Changing selection must acquire the shared operation lock while the old catalog is waiting.
	if _, err := service.ChangeFoundry(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	close(oldModels)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("old Foundry catalog was returned after selection changed")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	awaitCatalogQuota(t, ctx, service)
	catalog, err := service.GetModelCatalog(ctx)
	if err != nil || catalog.FoundryID != "second" || len(catalog.Models) != 1 || catalog.Models[0].Name != "model-second" {
		t.Fatalf("new Foundry catalog = %#v, %v", catalog, err)
	}
}

func TestCatalogQuotaRefreshAfterMutationAndRetryRetainsDefinitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	quotaStarted, releaseQuota, retryStarted, releaseRetry := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var definitions, refreshes atomic.Int32
	var limits catalogSnapshot
	limits = catalogSnapshot{name: "model-first", maximum: 160000, refresh: func(fetchCtx context.Context) (CapacityLimits, error) {
		if refreshes.Add(1) == 1 {
			close(quotaStarted)
			select {
			case <-releaseQuota:
				return limits, errors.New("quota unavailable")
			case <-fetchCtx.Done():
				return limits, fetchCtx.Err()
			}
		}
		close(retryStarted)
		select {
		case <-releaseRetry:
			refreshed := limits
			refreshed.maximum = 80000
			return refreshed, nil
		case <-fetchCtx.Done():
			return limits, fetchCtx.Err()
		}
	}}
	service := newCatalogCacheService(t, catalogCacheSource{limits: func(context.Context, Foundry, func(CapacityLimits)) (CapacityLimits, error) {
		definitions.Add(1)
		return limits, nil
	}, mutation: func() error { return nil }})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	awaitCatalogQuota(t, ctx, service)
	if _, err := service.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "new-chat", ModelName: "model-first"}); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, quotaStarted)
	requireCatalogState(t, ctx, service, "loading", nil)
	state, err := service.GetCapacityState(ctx, false)
	if err != nil || !state.Loading || len(state.Maximums) != 0 {
		t.Fatalf("mutation pending state = %#v, %v", state, err)
	}
	close(releaseQuota)
	awaitCatalogQuota(t, ctx, service)
	for range 2 {
		requireCatalogState(t, ctx, service, "error", nil)
	}
	state, err = service.GetCapacityState(ctx, false)
	if err != nil || state.Error == nil || len(state.Maximums) != 0 || refreshes.Load() != 1 {
		t.Fatalf("failed quota state = %#v, %v; refreshes = %d", state, err, refreshes.Load())
	}
	if _, err := service.GetCapacityState(ctx, true); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, retryStarted)
	if _, err := service.GetCapacityState(ctx, true); err != nil {
		t.Fatal(err)
	}
	requireCatalogState(t, ctx, service, "loading", nil)
	close(releaseRetry)
	awaitCatalogQuota(t, ctx, service)
	maximum := int64(80000)
	requireCatalogState(t, ctx, service, "ready", &maximum)
	if definitions.Load() != 1 || refreshes.Load() != 2 {
		t.Fatalf("definitions = %d, refreshes = %d", definitions.Load(), refreshes.Load())
	}
}

func TestOnlyQuotaChangingSuccessfulOperationsRefreshQuota(t *testing.T) {
	capacity, sameCapacity := int64(60000), int64(50000)
	cases := []struct {
		name      string
		operation func(context.Context, *Service) error
		refresh   bool
	}{
		{"add", func(ctx context.Context, s *Service) error {
			_, err := s.CreateDeployment(ctx, DeploymentCreateSpec{DeploymentName: "new", ModelName: "model-first"})
			return err
		}, true},
		{"delete", func(ctx context.Context, s *Service) error {
			_, err := s.DeleteDeployment(ctx, "first-chat")
			return err
		}, true},
		{"capacity", func(ctx context.Context, s *Service) error {
			_, err := s.UpdateDeployment(ctx, DeploymentUpdateSpec{DeploymentID: "first-chat", Capacity: &capacity})
			return err
		}, true},
		{"version and upgrade policy", func(ctx context.Context, s *Service) error {
			_, err := s.UpdateDeployment(ctx, DeploymentUpdateSpec{DeploymentID: "first-chat", Version: "2", UpgradePolicy: "NoAutoUpgrade", Capacity: &sameCapacity})
			return err
		}, false},
		{"refresh deployments", func(ctx context.Context, s *Service) error { _, err := s.RefreshDeployments(ctx); return err }, false},
	}
	for _, tc := range cases {
		for _, failed := range []bool{false, true} {
			if failed && tc.name == "refresh deployments" {
				continue
			}
			t.Run(tc.name+map[bool]string{false: "/success", true: "/failure"}[failed], func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				var definitions, refreshes atomic.Int32
				var limits catalogSnapshot
				limits = catalogSnapshot{name: "model-first", maximum: 160000, refresh: func(context.Context) (CapacityLimits, error) { refreshes.Add(1); return limits, nil }}
				service := newCatalogCacheService(t, catalogCacheSource{limits: func(context.Context, Foundry, func(CapacityLimits)) (CapacityLimits, error) {
					definitions.Add(1)
					return limits, nil
				}, mutation: func() error {
					if failed {
						return errors.New("Azure mutation failed")
					}
					return nil
				}})
				if _, err := service.GetInitialView(ctx); err != nil {
					t.Fatal(err)
				}
				awaitCatalogQuota(t, ctx, service)
				if err := tc.operation(ctx, service); (err != nil) != failed {
					t.Fatalf("operation error = %v, failed = %v", err, failed)
				}
				awaitCatalogQuota(t, ctx, service)
				want := int32(0)
				if tc.refresh && !failed {
					want = 1
				}
				if definitions.Load() != 1 || refreshes.Load() != want {
					t.Fatalf("definitions = %d, refreshes = %d, want 1 / %d", definitions.Load(), refreshes.Load(), want)
				}
			})
		}
	}
}
