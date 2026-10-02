//go:build !e2e

package azauth

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

const (
	armScope = "https://management.azure.com/.default"
	// tokenCacheName isolates this app's persistent token cache. On Windows the
	// SDK stores it DPAPI-encrypted at %LOCALAPPDATA%\.IdentityService\<name>.
	tokenCacheName = "azfoundrydeck"
)

// signIn signs in with the default browser, acquires an ARM token into the
// persistent token cache and resolves the signed-in tenant's display name.
// A failure to write the token cache fails the token acquisition itself.
func signIn(ctx context.Context) (Account, azidentity.AuthenticationRecord, error) {
	var record azidentity.AuthenticationRecord
	tokenCache, err := cache.New(&cache.Options{Name: tokenCacheName})
	if err != nil {
		return Account{}, record, err
	}
	cred, err := azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{Cache: tokenCache})
	if err != nil {
		return Account{}, record, err
	}
	record, err = cred.Authenticate(ctx, &policy.TokenRequestOptions{Scopes: []string{armScope}})
	if err != nil {
		return Account{}, record, err
	}
	tenants, err := armsubscriptions.NewTenantsClient(cred, nil)
	if err != nil {
		return Account{}, record, err
	}
	pager := tenants.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return Account{}, record, err
		}
		for _, t := range page.Value {
			if t != nil && t.TenantID != nil && *t.TenantID == record.TenantID {
				if t.DisplayName == nil || *t.DisplayName == "" {
					return Account{}, record, fmt.Errorf("tenant %s has no display name", record.TenantID)
				}
				return Account{Username: record.Username, TenantName: *t.DisplayName}, record, nil
			}
		}
	}
	return Account{}, record, fmt.Errorf("tenant %s is not in the ARM tenant list", record.TenantID)
}
