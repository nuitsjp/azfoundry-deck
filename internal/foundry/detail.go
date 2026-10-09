package foundry

import (
	"context"
	"fmt"

	"azfoundrydeck/internal/fault"
)

// CapacityLimits is an immutable snapshot of one Foundry's definitions and quota.
type CapacityLimits interface {
	Maximum(Deployment) *float64
	Versions(Deployment) []string
	Catalog() []ModelCatalogItem
	RefreshQuota(context.Context) (CapacityLimits, error)
}

type CapacitySource interface {
	// publishModels exposes definitions while the independent quota fetch is pending.
	CapacityLimits(context.Context, Foundry, func(CapacityLimits)) (CapacityLimits, error)
}

const CapacityReadyEvent = "foundry:capacity-ready"

type CapacityState struct {
	FoundryID string              `json:"foundryId"`
	Loading   bool                `json:"loading"`
	Maximums  map[string]*float64 `json:"maximums"`
	Error     *fault.Error        `json:"error"`
}

// Closing each channel publishes the corresponding immutable result. Only
// readers holding operations apply a result to the currently selected view.
type limitsFetch struct {
	done       chan struct{}
	modelsDone chan struct{}
	cancel     context.CancelFunc
	models     CapacityLimits
	value      CapacityLimits
	err        error
}

func (s *Service) startCapacityLimits(ctx context.Context, file string, foundry Foundry, source Source) {
	if s.limits.file == file && s.limits.foundryID == foundry.ID && (s.limits.value != nil || s.limits.fetch != nil) {
		return
	}
	capacitySource, ok := source.(CapacitySource)
	if !ok {
		return
	}
	fetchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	fetch := &limitsFetch{done: make(chan struct{}), modelsDone: make(chan struct{}), cancel: cancel}
	s.limits = limitsCache{file: file, foundryID: foundry.ID, fetch: fetch}
	go func() {
		defer cancel()
		published := false
		fetch.value, fetch.err = capacitySource.CapacityLimits(fetchCtx, foundry, func(models CapacityLimits) {
			fetch.models = models
			published = true
			close(fetch.modelsDone)
			if fetchCtx.Err() == nil {
				s.emit(CapacityReadyEvent, foundry.ID)
			}
		})
		if !published {
			fetch.models = fetch.value
		}
		if fetch.err != nil && fetchCtx.Err() == nil {
			s.logger.Error("operation_failed", "operation", "foundry.CapacityLimits", "cause", fetch.err)
		}
		close(fetch.done)
		if !published {
			close(fetch.modelsDone)
		}
		if fetchCtx.Err() == nil {
			s.emit(CapacityReadyEvent, foundry.ID)
		}
	}()
}

// readLimits is called with operations held. Pending or failed quota snapshots
// retain definitions, but callers must not use their old capacity maximums.
func (s *Service) readLimits() (CapacityLimits, bool, error) {
	fetch := s.limits.fetch
	if fetch == nil {
		return s.limits.value, s.limits.value == nil, nil
	}
	select {
	case <-fetch.done:
		if fetch.value != nil {
			s.limits.value = fetch.value
		}
		return s.limits.value, false, fetch.err
	default:
		select {
		case <-fetch.modelsDone:
			if s.limits.value == nil {
				s.limits.value = fetch.models
			}
		default:
		}
		return s.limits.value, true, nil
	}
}

// startQuotaRefresh invalidates the old quota immediately. A refresh triggered
// after a mutation succeeds always follows any older fetch of that same view.
func (s *Service) startQuotaRefresh(ctx context.Context) *limitsFetch {
	base, _, _ := s.readLimits() //nolint:errcheck // A previous fetch failure must not prevent the new quota refresh.
	previous := s.limits.fetch
	foundryID := s.limits.foundryID
	fetchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	fetch := &limitsFetch{done: make(chan struct{}), modelsDone: make(chan struct{})}
	fetch.cancel = func() {
		cancel()
		if previous != nil {
			previous.cancel()
		}
	}
	s.limits.fetch = fetch
	go func() {
		published := false
		defer func() {
			if !published {
				close(fetch.modelsDone)
			}
			if previous != nil && fetchCtx.Err() != nil {
				previous.cancel()
				<-previous.done
			}
			close(fetch.done)
			if fetchCtx.Err() == nil {
				s.emit(CapacityReadyEvent, foundryID)
			}
			cancel()
		}()
		if base == nil && previous != nil {
			select {
			case <-fetchCtx.Done():
				fetch.err = fetchCtx.Err()
				return
			case <-previous.modelsDone:
				base = previous.models
			}
		}
		fetch.models = base
		published = true
		close(fetch.modelsDone)
		if previous != nil {
			select {
			case <-fetchCtx.Done():
				fetch.err = fetchCtx.Err()
				return
			case <-previous.done:
				if previous.value != nil {
					base = previous.value
				}
			}
		}
		fetch.value = base
		if base == nil {
			fetch.err = fmt.Errorf("model definitions are unavailable")
			return
		}
		value, err := base.RefreshQuota(fetchCtx)
		if value != nil {
			fetch.value = value
		}
		fetch.err = err
		if err != nil && fetchCtx.Err() == nil {
			s.logger.Error("operation_failed", "operation", "foundry.RefreshQuota", "cause", err)
		}
	}()
	s.emit(CapacityReadyEvent, foundryID)
	return fetch
}

func waitLimits(ctx context.Context, fetch *limitsFetch, modelsOnly bool) error {
	if fetch == nil {
		return fmt.Errorf("capacity limits are not connected to Azure")
	}
	done := fetch.done
	if modelsOnly {
		done = fetch.modelsDone
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *Service) sameLimits(file, foundryID string, fetch *limitsFetch) bool {
	return s.limits.file == file && s.limits.foundryID == foundryID && s.limits.fetch == fetch
}

func (s *Service) GetCapacityState(ctx context.Context, retry bool) (CapacityState, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	state := CapacityState{FoundryID: s.current.foundryID, Maximums: map[string]*float64{}}
	if err := s.signedIn(ctx); err != nil {
		return state, err
	}
	file, err := s.file()
	if err != nil {
		return state, fault.Public(err)
	}
	if file != s.current.file {
		s.clearDeployments()
		state.FoundryID = ""
		return state, nil
	}
	if state.FoundryID == "" {
		return state, nil
	}
	limits, loading, err := s.readLimits()
	if retry && err != nil {
		if limits != nil {
			s.startQuotaRefresh(ctx)
		} else {
			view, readErr := s.readView(file)
			if readErr != nil {
				return state, fault.Public(readErr)
			}
			for _, foundry := range view.Foundries {
				if foundry.ID == state.FoundryID {
					source, sourceErr := s.source()
					if sourceErr != nil {
						return state, fault.Public(sourceErr)
					}
					s.limits = limitsCache{}
					s.startCapacityLimits(ctx, file, foundry, source)
					break
				}
			}
		}
		limits, loading, err = s.readLimits()
	}
	state.Loading = loading
	if err != nil {
		state.Error = fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve the capacity maximum.")
		return state, nil
	}
	if !loading && limits != nil {
		for _, deployment := range s.current.deployments {
			state.Maximums[deployment.ID] = limits.Maximum(deployment)
		}
	}
	return state, nil
}

func (s *Service) GetCapacityMaximum(ctx context.Context, deploymentID string) (*float64, error) {
	s.operations.Lock()
	if err := s.signedIn(ctx); err != nil {
		s.operations.Unlock()
		return nil, err
	}
	file, err := s.file()
	var foundry Foundry
	var deployment Deployment
	var source Source
	if err == nil {
		foundry, deployment, source, err = s.selectedDeployment(file, deploymentID)
	}
	if err != nil {
		s.operations.Unlock()
		return nil, fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not read the selected deployment.")
	}
	s.startCapacityLimits(ctx, file, foundry, source)
	fetch := s.limits.fetch
	s.operations.Unlock()
	if err := waitLimits(ctx, fetch, false); err != nil {
		return nil, fault.Public(err)
	}
	s.operations.Lock()
	defer s.operations.Unlock()
	if !s.sameLimits(file, foundry.ID, fetch) {
		return nil, fault.New("DEPLOYMENT_DETAIL_FAILED", "The selected Foundry changed.")
	}
	limits, _, err := s.readLimits()
	if err != nil || limits == nil {
		return nil, fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve the capacity maximum.")
	}
	return limits.Maximum(deployment), nil
}
