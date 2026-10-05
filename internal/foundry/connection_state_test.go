package foundry

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type connectionStateSource struct {
	controlledSource
	connection func(context.Context, Foundry) (Connection, error)
}

func (s connectionStateSource) Connection(ctx context.Context, foundry Foundry) (Connection, error) {
	return s.connection(ctx, foundry)
}

func TestConnectionStateIgnoresOldFoundryCompletionAfterSwitch(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	file := filepath.Join(t.TempDir(), "foundry-state.json")
	if err := save(file, InitialFoundryView{
		Foundries:         []Foundry{{ID: "first", Name: "a"}, {ID: "second", Name: "b"}},
		SelectedFoundryID: "first",
	}); err != nil {
		t.Fatal(err)
	}
	releaseOld := make(chan struct{})
	service := newFoundryService(file, connectionStateSource{
		controlledSource: controlledSource{
			deployments: func(_ context.Context, foundry Foundry, _ func(int)) ([]Deployment, error) {
				return []Deployment{{ID: foundry.ID + "-chat"}}, nil
			},
		},
		connection: func(_ context.Context, foundry Foundry) (Connection, error) {
			if foundry.ID == "first" {
				// Complete after cancellation, like a response already in flight when the selection changed.
				select {
				case <-releaseOld:
				case <-ctx.Done():
					return Connection{}, ctx.Err()
				}
			}
			return Connection{Endpoint: foundry.Name + "/openai/v1", Key: foundry.ID + "-key"}, nil
		},
	}, func(string, any) {})
	if _, err := service.GetInitialView(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := service.GetConnectionState(ctx)
	if err != nil || state.FoundryID != "first" || !state.Loading || state.Connection != nil {
		t.Fatalf("pending state = %#v, %v", state, err)
	}
	oldDone := service.connection.fetch.done
	if _, err := service.ChangeFoundry(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	awaitCapacitySignal(t, ctx, service.connection.fetch.done)
	close(releaseOld)
	awaitCapacitySignal(t, ctx, oldDone)
	state, err = service.GetConnectionState(ctx)
	if err != nil || state.FoundryID != "second" || state.Loading || state.Connection == nil || *state.Connection != (Connection{Endpoint: "b/openai/v1", Key: "second-key"}) {
		t.Fatalf("state after old completion = %#v, %v", state, err)
	}
}
