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
func NewSilentCredential(record LoginRecord) (azcore.TokenCredential, error) {
	if _, err := record.account(); err != nil {
		return nil, err
	}
	return credential(record.Record, record.SelectedTenantID, true)
}

// credential uses the persistent token cache. With a record and silent set, it
// only reads or refreshes cached tokens and fails instead of opening a browser.
func credential(record azidentity.AuthenticationRecord, tenantID string, silent bool) (*azidentity.InteractiveBrowserCredential, error) {
	tokenCache, err := cache.New(&cache.Options{Name: tokenCacheName})
	if err != nil {
		return nil, err
	}
	return azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
		TenantID:                       tenantID,
		AuthenticationRecord:           record,
		Cache:                          tokenCache,
		DisableAutomaticAuthentication: silent,
	})
}

// signIn signs in with the default browser, acquires an ARM token into the
// persistent token cache. Tenant discovery is a separate ARM request.
// A failure to write the token cache fails the token acquisition itself.
func signIn(ctx context.Context) (azidentity.AuthenticationRecord, error) {
	var record azidentity.AuthenticationRecord
	cred, err := credential(record, "", false)
	if err != nil {
		return record, err
	}
	// ARM clients request CAE tokens, which use a separate cache from non-CAE tokens.
	return cred.Authenticate(ctx, &policy.TokenRequestOptions{Scopes: []string{armScope}, EnableCAE: true})
}

// restoreToken acquires an ARM token for the selected tenant without user
// interaction. It does not request the tenant list.
func restoreToken(ctx context.Context, record LoginRecord) error {
	return acquireTenantToken(ctx, record)
}

func acquireTenantToken(ctx context.Context, record LoginRecord) error {
	cred, err := NewSilentCredential(record)
	if err != nil {
		return err
	}
	if _, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{armScope}, EnableCAE: true}); err != nil {
		return err
	}
	return nil
}

func listTenants(ctx context.Context, record azidentity.AuthenticationRecord) ([]Tenant, error) {
	cred, err := credential(record, "", true)
	if err != nil {
		return nil, err
	}
	tenants, err := armsubscriptions.NewTenantsClient(cred, nil)
	if err != nil {
		return nil, err
	}
	result := []Tenant{}
	pager := tenants.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range page.Value {
			if t == nil || t.TenantID == nil || *t.TenantID == "" || t.DisplayName == nil || *t.DisplayName == "" {
				return nil, fmt.Errorf("ARM のテナント一覧に ID または表示名がありません。")
			}
			result = append(result, Tenant{ID: *t.TenantID, DisplayName: *t.DisplayName})
		}
	}
	return result, nil
}
