//go:build e2e

package foundry

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

// deletedFoundries remembers Foundries removed through the fixed source, so later
// list fetches omit them.
var deletedFoundries = struct {
	sync.Mutex
	ids map[string]bool
}{ids: map[string]bool{}}

func withoutDeletedFoundries(foundries []Foundry) []Foundry {
	deletedFoundries.Lock()
	defer deletedFoundries.Unlock()
	return slices.DeleteFunc(foundries, func(foundry Foundry) bool { return deletedFoundries.ids[foundry.ID] })
}

// fixedDeletionWait makes the slow Azure deletion visible during screen review only.
func fixedDeletionWait(ctx context.Context, duration time.Duration) error {
	if os.Getenv("AZFOUNDRYDECK_E2E_FOUNDRY_DELETE_REVIEW") != "1" {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration):
		return nil
	}
}

// The fixed resource groups hold only Foundry resources.
func (fixedSource) ResourceGroupHoldsOnlyFoundry(ctx context.Context, foundry Foundry) (bool, error) {
	return true, ctx.Err()
}

func (fixedSource) DeleteFoundry(ctx context.Context, foundry Foundry) error {
	if err := fixedDeletionWait(ctx, 3*time.Second); err != nil {
		return err
	}
	// AZFOUNDRYDECK_E2E_FAIL=foundry-delete: Azure rejects the deletion.
	if os.Getenv("AZFOUNDRYDECK_E2E_FAIL") == "foundry-delete" {
		return fmt.Errorf("simulated Foundry deletion failure")
	}
	deletedFoundries.Lock()
	deletedFoundries.ids[foundry.ID] = true
	deletedFoundries.Unlock()
	return ctx.Err()
}

func (fixedSource) PurgeFoundry(ctx context.Context, foundry Foundry) error {
	return fixedDeletionWait(ctx, 2*time.Second)
}

func (fixedSource) DeleteResourceGroup(ctx context.Context, foundry Foundry) error {
	return fixedDeletionWait(ctx, 3*time.Second)
}
