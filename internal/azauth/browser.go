package azauth

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const armScope = "https://management.azure.com/.default"

// Browser signs in with the default browser and acquires one ARM token.
type Browser struct{}

func (Browser) Authenticate(ctx context.Context) (Account, error) {
	cred, err := azidentity.NewInteractiveBrowserCredential(nil)
	if err != nil {
		return Account{}, err
	}
	record, err := cred.Authenticate(ctx, &policy.TokenRequestOptions{Scopes: []string{armScope}})
	if err != nil {
		return Account{}, err
	}
	return Account{Username: record.Username, TenantID: record.TenantID}, nil
}
