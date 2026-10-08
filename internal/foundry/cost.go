package foundry

import (
	"context"
	"fmt"
	"slices"

	"azfoundrydeck/internal/fault"
)

// Cost is the month-to-date actual cost of one subscription in its billing currency.
// It is held in memory only.
type Cost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// CostSource fetches the month-to-date cost of the Foundry's subscription at the external boundary.
type CostSource interface {
	Cost(context.Context, Foundry) (Cost, error)
}

const CostReadyEvent = "foundry:cost-ready"

type CostState struct {
	FoundryID string       `json:"foundryId"`
	Loading   bool         `json:"loading"`
	Cost      *Cost        `json:"cost"`
	Error     *fault.Error `json:"error"`
}

type costFetch struct {
	done   chan struct{}
	cancel context.CancelFunc
	value  Cost
	err    error
}

type costCache struct {
	file, foundryID string
	fetch           *costFetch
}

// startCost fetches the cost when a Foundry is selected, even for one in the
// subscription already shown, but not again for the same Foundry unless forced by
// Refresh cost. A discarded fetch never reaches the current cache.
func (s *Service) startCost(ctx context.Context, file string, foundry Foundry, source Source, force bool) {
	if !force && s.cost.file == file && s.cost.foundryID == foundry.ID && s.cost.fetch != nil {
		return
	}
	costSource, ok := source.(CostSource)
	if !ok {
		return
	}
	if s.cost.fetch != nil {
		s.cost.fetch.cancel()
	}
	fetchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	fetch := &costFetch{done: make(chan struct{}), cancel: cancel}
	s.cost = costCache{file: file, foundryID: foundry.ID, fetch: fetch}
	// The screen is told to read the new Loading state instead of the previous cost.
	s.emit(CostReadyEvent, foundry.ID)
	go func() {
		defer cancel()
		fetch.value, fetch.err = costSource.Cost(fetchCtx, foundry)
		if fetch.err != nil && fetchCtx.Err() == nil {
			s.logger.Error("operation_failed", "operation", "foundry.Cost", "cause", fetch.err)
		}
		close(fetch.done)
		if fetchCtx.Err() == nil {
			s.emit(CostReadyEvent, foundry.ID)
		}
	}()
}

// GetCostState reads the cost of the selected Foundry's subscription without waiting for it.
func (s *Service) GetCostState(ctx context.Context) (CostState, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	state := CostState{FoundryID: s.current.foundryID}
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
	fetch := s.cost.fetch
	if state.FoundryID == "" || fetch == nil || s.cost.foundryID != state.FoundryID {
		return state, nil
	}
	select {
	case <-fetch.done:
		if fetch.err == nil {
			state.Cost = &fetch.value
		} else {
			state.Error = fault.New("COST_LOAD_FAILED", "Could not retrieve the month-to-date cost.")
		}
	default:
		state.Loading = true
	}
	return state, nil
}

// RefreshCost fetches the selected Foundry's subscription cost again without
// touching the deployments, limits, connection or state file.
func (s *Service) RefreshCost(ctx context.Context) error {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return err
	}
	err := s.refreshCost(ctx)
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.RefreshCost", "cause", err)
		return fault.New("COST_LOAD_FAILED", "Could not retrieve the month-to-date cost.")
	}
	return nil
}

func (s *Service) refreshCost(ctx context.Context) error {
	file, err := s.file()
	if err != nil {
		return err
	}
	view, err := read(file)
	if err != nil {
		return err
	}
	index := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if index < 0 {
		return fmt.Errorf("selected Foundry is not in the saved list")
	}
	source, err := s.source()
	if err != nil {
		return err
	}
	s.startCost(ctx, file, view.Foundries[index], source, true)
	return nil
}
