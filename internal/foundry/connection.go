package foundry

import (
	"context"

	"azfoundrydeck/internal/fault"
)

// Connection is the Azure OpenAI endpoint and key 1 of one Foundry. It is held in
// memory only and the key is never written to files or logs.
type Connection struct {
	Endpoint string `json:"endpoint"`
	Key      string `json:"key"`
}

// ConnectionSource fetches the connection of one Foundry at the external boundary.
type ConnectionSource interface {
	Connection(context.Context, Foundry) (Connection, error)
}

const ConnectionReadyEvent = "foundry:connection-ready"

type ConnectionState struct {
	FoundryID  string      `json:"foundryId"`
	Loading    bool        `json:"loading"`
	Connection *Connection `json:"connection"`
}

type connectionFetch struct {
	done   chan struct{}
	cancel context.CancelFunc
	value  Connection
	err    error
}

type connectionCache struct {
	file, foundryID string
	fetch           *connectionFetch
}

// startConnection follows the capacity limits: it starts with the deployment list,
// outlives that request, and a discarded fetch never reaches the current cache.
func (s *Service) startConnection(ctx context.Context, file string, foundry Foundry, source Source) {
	if s.connection.file == file && s.connection.foundryID == foundry.ID && s.connection.fetch != nil {
		return
	}
	connectionSource, ok := source.(ConnectionSource)
	if !ok {
		return
	}
	fetchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	fetch := &connectionFetch{done: make(chan struct{}), cancel: cancel}
	s.connection = connectionCache{file: file, foundryID: foundry.ID, fetch: fetch}
	go func() {
		defer cancel()
		fetch.value, fetch.err = connectionSource.Connection(fetchCtx, foundry)
		if fetch.err != nil && fetchCtx.Err() == nil {
			s.logger.Error("operation_failed", "operation", "foundry.Connection", "cause", fetch.err)
		}
		close(fetch.done)
		if fetchCtx.Err() == nil {
			s.emit(ConnectionReadyEvent, foundry.ID)
		}
	}()
}

// GetConnectionState reads the connection of the selected Foundry without waiting for it.
func (s *Service) GetConnectionState(ctx context.Context) (ConnectionState, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	state := ConnectionState{FoundryID: s.current.foundryID}
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
	fetch := s.connection.fetch
	if state.FoundryID == "" || fetch == nil || s.connection.foundryID != state.FoundryID {
		return state, nil
	}
	select {
	case <-fetch.done:
		if fetch.err == nil {
			state.Connection = &fetch.value
		}
	default:
		state.Loading = true
	}
	return state, nil
}
