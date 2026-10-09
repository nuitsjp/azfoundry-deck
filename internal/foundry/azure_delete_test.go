//go:build !e2e

package foundry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type failingDeleteCredential struct {
	err   error
	calls int
}

func (c *failingDeleteCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.calls++
	return azcore.AccessToken{}, c.err
}

func TestFoundryDeletionStopsWhenListingFails(t *testing.T) {
	foundry := Foundry{
		ID:                "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-rg/providers/Microsoft.CognitiveServices/accounts/test-foundry",
		Name:              "test-foundry",
		ResourceGroupName: "test-rg",
	}
	for _, tc := range []struct {
		name, operation string
		delete          func(*azureSource, context.Context, Foundry) error
	}{
		{"delete", "list projects in Foundry", (*azureSource).DeleteFoundry},
		{"purge", "list deleted Foundry accounts", (*azureSource).PurgeFoundry},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := errors.New("token unavailable")
			credential := &failingDeleteCredential{err: cause}
			source := &azureSource{credential: credential}
			err := tc.delete(source, t.Context(), foundry)
			if !errors.Is(err, cause) || !strings.Contains(err.Error(), tc.operation) {
				t.Fatalf("got %v, want %s failure wrapping %v", err, tc.operation, cause)
			}
			if credential.calls != 1 {
				t.Fatalf("made %d token requests after listing failed, want 1", credential.calls)
			}
		})
	}
}
