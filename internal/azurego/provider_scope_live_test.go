package azurego

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armcognitiveservices "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices/v3"
)

// TestLiveAzureScopes exercises real continuation links and compares list
// values with individual deployment GETs. It never creates resources or roles.
// Authentication/permission gaps are reported separately from verified scopes.
func TestLiveAzureScopes(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LIVE") != "1" {
		t.Skip("set AZFOUNDRY_LIVE=1 to run the read-only scope verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	p := NewProvider()
	subscriptions, err := p.listSubscriptions(ctx)
	if err != nil {
		t.Fatal("could not read CLI subscriptions")
	}
	aadsts := regexp.MustCompile(`AADSTS\d+`)
	successful, forbidden, unavailable, continuationPages, deploymentGets := 0, 0, 0, 0, 0
	for index, subscription := range subscriptions {
		t.Run(fmt.Sprintf("scope_%d", index+1), func(t *testing.T) {
			boundary, err := newSDKSubscriptionClient(subscription)
			if err != nil {
				t.Fatal("could not construct ARM client")
			}
			client := boundary.(*sdkSubscriptionClient)
			accounts, err := client.listAccounts(ctx)
			if err != nil {
				var responseError *azcore.ResponseError
				code, _, _ := describeError(err)
				if errors.As(err, &responseError) && responseError.StatusCode == 403 {
					forbidden++
					t.Logf("discovery excluded: HTTP 403 code=%s", responseError.ErrorCode)
					return
				}
				unavailable++
				// Do not print the underlying auth error, token, claims or user ID.
				t.Logf("discovery unavailable: subscription=%q tenant=%s code=%s aadsts=%s", subscription.name, subscription.tenantID, code, aadsts.FindString(err.Error()))
				return
			}
			successful++
			query := url.Values{"api-version": {resourcesAPIVersion}, "$filter": {"resourceType eq 'Microsoft.CognitiveServices/accounts'"}, "$top": {"1"}}
			next := client.client.Endpoint() + "/subscriptions/" + url.PathEscape(subscription.id) + "/resources?" + query.Encode()
			var paged []*armcognitiveservices.Account
			pages := 0
			for next != "" {
				var page armcognitiveservices.AccountListResult
				if err := client.getJSON(ctx, next, &page); err != nil {
					code, _, _ := describeError(err)
					t.Fatalf("paged discovery failed: %s", code)
				}
				pages++
				paged = append(paged, page.Value...)
				next = value(page.NextLink)
			}
			if !reflect.DeepEqual(liveAccountKeys(accounts), liveAccountKeys(paged)) {
				t.Fatal("unpaged and $top=1 resource metadata differ")
			}
			continuationPages += pages - 1
			t.Logf("resources=%d pages_with_top_1=%d metadata_equal=true", len(accounts), pages)
			for _, account := range accounts {
				if account == nil || !isFoundryAccount(account.Kind) {
					continue
				}
				found, err := discoveredAccountFromSDK(subscription, client, account)
				if err != nil {
					t.Fatal("invalid account metadata")
				}
				deployments, err := client.listDeployments(ctx, found.resourceGroup, found.name)
				if err != nil {
					code, _, _ := describeError(err)
					t.Fatalf("account deployments unavailable: %s", code)
				}
				for _, deployment := range deployments {
					if deployment == nil || value(deployment.ID) == "" {
						t.Fatal("invalid deployment ID in live response")
					}
					var detail armcognitiveservices.Deployment
					if err := client.getJSON(ctx, client.client.Endpoint()+value(deployment.ID)+"?api-version="+deploymentsAPIVersion, &detail); err != nil {
						code, _, _ := describeError(err)
						t.Fatalf("deployment detail GET failed: %s", code)
					}
					if !reflect.DeepEqual(deploymentRow(found, deployment), deploymentRow(found, &detail)) {
						t.Fatal("deployment List and GET display fields differ")
					}
					deploymentGets++
					row := deploymentRow(found, &detail)
					t.Logf("deployment=%s model=%s sku=%s TPM=%s RPM=%s list_get_equal=true", row.Name, row.Model, row.SKU, liveQuantity(row.TPM), liveQuantity(row.RPM))
				}
			}
		})
	}
	if successful == 0 || deploymentGets == 0 {
		t.Error("no live deployment values could be verified")
	}
	t.Logf("SUMMARY subscriptions=%d discovery_success=%d forbidden_excluded=%d unavailable=%d real_continuations=%d deployment_get_matches=%d", len(subscriptions), successful, forbidden, unavailable, continuationPages, deploymentGets)
	if continuationPages == 0 {
		t.Log("LIMIT: no continuation link returned; real paging remains unverified")
	}
	if unavailable > 0 {
		t.Log("LIMIT: unavailable scopes remain unverified; this is not complete discovery success")
	}
	// List/GET parity is not a substitute for Portal-setting verification.
}

func liveAccountKeys(accounts []*armcognitiveservices.Account) []string {
	keys := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account != nil {
			keys = append(keys, strings.ToLower(value(account.ID))+"|"+value(account.Name)+"|"+value(account.Kind)+"|"+value(account.Location))
		}
	}
	sort.Strings(keys)
	return keys
}

func liveQuantity(quantity *int64) string {
	if quantity == nil {
		return "unknown"
	}
	return fmt.Sprint(*quantity)
}
