//go:build !e2e

package azauth

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

const armScope = "https://management.azure.com/.default"

// NewSilentCredential uses the signed-in account without opening a browser.
func NewSilentCredential(record azidentity.AuthenticationRecord) (azcore.TokenCredential, error) {
	return credential(record, true)
}

// credential uses the persistent token cache. With a record and silent set, it
// only reads or refreshes cached tokens and fails instead of opening a browser.
func credential(record azidentity.AuthenticationRecord, silent bool) (*azidentity.InteractiveBrowserCredential, error) {
	tokenCache, err := cache.New(&cache.Options{Name: tokenCacheName})
	if err != nil {
		return nil, err
	}
	return azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
		AuthenticationRecord:           record,
		Cache:                          tokenCache,
		DisableAutomaticAuthentication: silent,
	})
}

// signIn signs in with the default browser, acquires an ARM token into the
// persistent token cache and resolves the signed-in tenant's display name.
// A failure to write the token cache fails the token acquisition itself.
func signIn(ctx context.Context) (Account, azidentity.AuthenticationRecord, error) {
	var record azidentity.AuthenticationRecord
	cred, err := credential(record, false)
	if err != nil {
		return Account{}, record, err
	}
	record, err = cred.Authenticate(ctx, &policy.TokenRequestOptions{Scopes: []string{armScope}})
	if err != nil {
		return Account{}, record, err
	}
	account, err := tenantAccount(ctx, cred, record)
	return account, record, err
}

// restoreAccount acquires an ARM token from the persistent cache for the saved record
// without user interaction, then resolves the tenant's display name.
func restoreAccount(ctx context.Context, record azidentity.AuthenticationRecord) (Account, error) {
	cred, err := credential(record, true)
	if err != nil {
		return Account{}, err
	}
	if _, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{armScope}}); err != nil {
		return Account{}, err
	}
	return tenantAccount(ctx, cred, record)
}

func tenantAccount(ctx context.Context, cred *azidentity.InteractiveBrowserCredential, record azidentity.AuthenticationRecord) (Account, error) {
	tenants, err := armsubscriptions.NewTenantsClient(cred, nil)
	if err != nil {
		return Account{}, err
	}
	pager := tenants.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return Account{}, err
		}
		for _, t := range page.Value {
			if t != nil && t.TenantID != nil && *t.TenantID == record.TenantID {
				if t.DisplayName == nil || *t.DisplayName == "" {
					return Account{}, fmt.Errorf("tenant %s has no display name", record.TenantID)
				}
				return Account{Username: record.Username, TenantName: *t.DisplayName}, nil
			}
		}
	}
	return Account{}, fmt.Errorf("tenant %s is not in the ARM tenant list", record.TenantID)
}
