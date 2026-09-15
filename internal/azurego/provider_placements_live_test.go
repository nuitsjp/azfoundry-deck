package azurego

import (
	"context"
	"os"
	"testing"
)

// TestLiveAzurePlacements reads the real placements with the signed-in Azure
// CLI. It only reads; no resource is created, changed or deleted.
func TestLiveAzurePlacements(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LIVE") != "1" {
		t.Skip("set AZFOUNDRY_LIVE=1 to run the live Azure verification")
	}
	result, err := NewProvider().FetchPlacements(context.Background())
	if err != nil {
		t.Fatalf("FetchPlacements returned error: %v", err)
	}
	for _, failure := range result.Failures {
		t.Logf("failure scope=%s subscription=%s code=%s message=%s", failure.Scope, failure.SubscriptionName, failure.Code, failure.Message)
	}
	for _, subscription := range result.Subscriptions {
		t.Logf("subscription=%s groups=%d foundries=%d", subscription.Name, len(subscription.Groups), len(subscription.Foundries))
	}
	if len(result.Subscriptions) == 0 && len(result.Failures) == 0 {
		t.Fatal("placement read returned neither a subscription nor a failure")
	}
}
