package azurego

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// TestLiveAzureModels is opt-in because it reads the caller's signed-in Azure
// CLI session and performs read-only ARM model-list requests. It deliberately
// uses the same Provider.FetchModels method as the application screen.
func TestLiveAzureModels(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LIVE") != "1" {
		t.Skip("set AZFOUNDRY_LIVE=1 to run the live Azure model verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	provider := NewProvider()
	subscriptions, err := provider.listSubscriptions(ctx)
	if err != nil {
		t.Fatal("could not read CLI subscriptions")
	}

	successfulAccounts := 0
	permissionFailures := 0
	for _, subscription := range subscriptions {
		client, err := newSDKSubscriptionClient(subscription)
		if err != nil {
			t.Logf("subscription=%q SDK unavailable", subscription.name)
			continue
		}
		accounts, err := client.listAccounts(ctx)
		if err != nil {
			var responseError *azcore.ResponseError
			if errors.As(err, &responseError) && responseError.StatusCode == 403 {
				permissionFailures++
				t.Logf("subscription=%q account discovery forbidden", subscription.name)
				continue
			}
			t.Logf("subscription=%q account discovery unavailable: %s", subscription.name, liveModelErrorCode(err))
			continue
		}

		for _, account := range accounts {
			if account == nil || !isFoundryAccount(account.Kind) || value(account.ID) == "" {
				continue
			}
			result, err := provider.FetchModels(ctx, value(account.ID))
			if err != nil {
				t.Logf("account=%q model request canceled or failed: %s", value(account.Name), liveModelErrorCode(err))
				continue
			}
			if len(result.Failures) > 0 {
				if strings.Contains(strings.ToLower(result.Failures[0].Code), "forbidden") || strings.Contains(strings.ToLower(result.Failures[0].Code), "authorization") {
					permissionFailures++
				}
				t.Logf("account=%q model request unavailable: code=%s", value(account.Name), result.Failures[0].Code)
				continue
			}
			if len(result.Models) == 0 {
				t.Logf("account=%q returned zero model candidates", value(account.Name))
				continue
			}
			if result.Models[0].Name == "" || result.Models[0].Version == "" {
				t.Fatalf("account=%q first model lacks required name/version: %+v", value(account.Name), result.Models[0])
			}
			successfulAccounts++
			t.Logf("account=%q models=%d first=%s/%s lifecycle=%s skus=%d", value(account.Name), len(result.Models), result.Models[0].Name, result.Models[0].Version, result.Models[0].Lifecycle, len(result.Models[0].SKUs))
			break
		}
	}

	if successfulAccounts == 0 {
		t.Fatalf("no live model candidate response was verified; permission failures=%d", permissionFailures)
	}
}

func liveModelErrorCode(err error) string {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		return fmt.Sprintf("http-%d/%s", responseError.StatusCode, responseError.ErrorCode)
	}
	return err.Error()
}
