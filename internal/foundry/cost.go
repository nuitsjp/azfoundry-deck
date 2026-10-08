package foundry

import (
	"context"

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

// startCost fetches the cost whenever a Foundry is selected and on Refresh models,
// even for a Foundry in the subscription already shown. A discarded fetch never
// reaches the current cache.
func (s *Service) startCost(ctx context.Context, file string, foundry Foundry, source Source) {
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
