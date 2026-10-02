package azauth

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

const armScope = "https://management.azure.com/.default"

// Browser signs in with the default browser, acquires an ARM token and
// resolves the signed-in tenant's display name from ARM.
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
