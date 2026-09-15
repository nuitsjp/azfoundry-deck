package mock

import (
	"context"
	"errors"
	"testing"
)

func TestModelFetchReportsAnUnknownFoundry(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait

	result, err := provider.FetchModels(context.Background(), "/subscriptions/unknown")
	if err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if len(result.Models) != 0 || len(result.Failures) != 1 || result.Failures[0].Code != "account-not-found" {
		t.Fatalf("unknown Foundry result = %+v, want a reported failure", result)
	}
}

func TestModelFetchHonorsCancellation(t *testing.T) {
	provider := NewProvider()
	provider.wait = waitContext
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := provider.FetchModels(ctx, addFoundryID(addFoundries[0]))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchModels error = %v, want context.Canceled", err)
	}
	if result.Models == nil || result.Failures == nil {
		t.Fatal("cancelled result arrays must be empty arrays, not nil")
	}
}
