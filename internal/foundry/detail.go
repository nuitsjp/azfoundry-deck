package foundry

import (
	"context"
	"fmt"

	"azfoundrydeck/internal/fault"
)

// CapacityLimits holds one Foundry's model definitions and its regional shared
// quota, from which the allocatable maximum of each deployment is derived.
type CapacityLimits interface {
	// Maximum is nil when the model definition or the quota cannot give a value.
	Maximum(Deployment) *float64
	// Versions lists the model versions offered for the deployment, including its current one.
	Versions(Deployment) []string
	// RefreshQuota returns the same definitions with the shared quota fetched again.
	RefreshQuota(context.Context) (CapacityLimits, error)
}

// CapacitySource fetches the capacity limits of one Foundry at the external boundary.
type CapacitySource interface {
	CapacityLimits(context.Context, Foundry) (CapacityLimits, error)
}

const CapacityReadyEvent = "foundry:capacity-ready"

type CapacityState struct {
	FoundryID string              `json:"foundryId"`
	Loading   bool                `json:"loading"`
	Maximums  map[string]*float64 `json:"maximums"`
	Error     *fault.Error        `json:"error"`
}

// Results are published by closing done. A discarded fetch never writes to the
// current cache, so completion after a Foundry change cannot replace its limits.
type limitsFetch struct {
	done   chan struct{}
	cancel context.CancelFunc
	value  CapacityLimits
	err    error
}

func (s *Service) startCapacityLimits(ctx context.Context, file string, foundry Foundry, source Source) {
	if s.limits.file == file && s.limits.foundryID == foundry.ID && (s.limits.value != nil || s.limits.fetch != nil) {
		return
	}
	capacitySource, ok := source.(CapacitySource)
	if !ok {
		return
	}
	// The fetch outlives the initial-view request and is cancelled when that view
	// is discarded. It does not hold the operation lock while calling Azure.
	fetchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	fetch := &limitsFetch{done: make(chan struct{}), cancel: cancel}
	s.limits = limitsCache{file: file, foundryID: foundry.ID, fetch: fetch}
	go func() {
		defer cancel()
		fetch.value, fetch.err = capacitySource.CapacityLimits(fetchCtx, foundry)
		if fetch.err != nil && fetchCtx.Err() == nil {
			s.logger.Error("operation_failed", "operation", "foundry.CapacityLimits", "cause", fetch.err)
		}
		close(fetch.done)
		if fetchCtx.Err() == nil {
			s.emit(CapacityReadyEvent, foundry.ID)
		}
	}()
}

// GetCapacityState reads the current fetch without waiting for it. Only an
// explicit retry restarts a failed fetch; selecting a deployment never does.
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
	if retry && s.limits.fetch != nil {
		select {
		case <-s.limits.fetch.done:
			if s.limits.fetch.err != nil {
				view, err := s.readView(s.current.file)
				if err != nil {
					return state, fault.Public(err)
				}
				for _, foundry := range view.Foundries {
					if foundry.ID == state.FoundryID {
						source, err := s.source()
						if err != nil {
							return state, fault.Public(err)
						}
						s.limits = limitsCache{}
						s.startCapacityLimits(ctx, s.current.file, foundry, source)
						break
					}
				}
			}
		default:
		}
	}
	if s.limits.value == nil && s.limits.fetch != nil {
		select {
		case <-s.limits.fetch.done:
			if s.limits.fetch.err != nil {
				state.Error = fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve the capacity maximum.")
				return state, nil
			}
			s.limits.value = s.limits.fetch.value
		default:
			state.Loading = true
			return state, nil
		}
	}
	if s.limits.value != nil {
		for _, deployment := range s.current.deployments {
			state.Maximums[deployment.ID] = s.limits.value.Maximum(deployment)
		}
	}
	return state, nil
}

// GetCapacityMaximum returns the allocatable capacity maximum of one deployment of
// the selected Foundry. It reuses the fetch started with the deployment list,
// waiting for it when needed. Limits remain in memory until the view changes.
func (s *Service) GetCapacityMaximum(ctx context.Context, deploymentID string) (*float64, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return nil, err
	}
	file, err := s.file()
	var maximum *float64
	if err == nil {
		maximum, err = s.capacityMaximum(ctx, file, deploymentID)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetCapacityMaximum", "cause", err)
		if ctx.Err() != nil {
			return nil, fault.Public(ctx.Err())
		}
		return nil, fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve the capacity maximum from the source or read the current Foundry selection.")
	}
	return maximum, nil
}

func (s *Service) capacityMaximum(ctx context.Context, file, deploymentID string) (*float64, error) {
	foundry, deployment, source, err := s.selectedDeployment(file, deploymentID)
	if err != nil {
		return nil, err
	}
	limits, _, err := s.capacityLimits(ctx, file, foundry, source)
	if err != nil {
		return nil, err
	}
	return limits.Maximum(deployment), nil
}

// capacityLimits returns the kept limits of the Foundry, fetching them when absent.
// fetched reports whether this call fetched them.
func (s *Service) capacityLimits(ctx context.Context, file string, foundry Foundry, source Source) (CapacityLimits, bool, error) {
	if s.limits.value != nil && s.limits.file == file && s.limits.foundryID == foundry.ID {
		return s.limits.value, false, nil
	}
	started := s.limits.fetch == nil
	s.startCapacityLimits(ctx, file, foundry, source)
	if s.limits.fetch == nil {
		return nil, false, fmt.Errorf("capacity limits are not connected to Azure yet")
	}
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-s.limits.fetch.done:
	}
	if s.limits.fetch.err != nil {
		return nil, false, s.limits.fetch.err
	}
	s.limits.value = s.limits.fetch.value
	return s.limits.value, started, nil
}
