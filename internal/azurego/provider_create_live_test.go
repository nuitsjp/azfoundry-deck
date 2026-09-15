package azurego

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

// TestLiveAzureCreateStopsOnExistingDeployment checks the stop that protects an
// existing deployment. It targets a deployment that already exists, so the
// request is never sent and nothing is created, changed or deleted.
//
// Run it only against a subscription and Foundry you are allowed to write to:
//
//	AZFOUNDRY_LIVE_WRITE=1 AZFOUNDRY_LIVE_SUBSCRIPTION=... AZFOUNDRY_LIVE_GROUP=...
//	AZFOUNDRY_LIVE_FOUNDRY=... AZFOUNDRY_LIVE_DEPLOYMENT=... go test ./internal/azurego -run TestLiveAzureCreate
func TestLiveAzureCreateStopsOnExistingDeployment(t *testing.T) {
	if os.Getenv("AZFOUNDRY_LIVE_WRITE") != "1" {
		t.Skip("set AZFOUNDRY_LIVE_WRITE=1 with the live target variables to run this verification")
	}
	request := service.CreateRequest{
		SubscriptionID: os.Getenv("AZFOUNDRY_LIVE_SUBSCRIPTION"),
		ResourceGroup:  os.Getenv("AZFOUNDRY_LIVE_GROUP"),
		FoundryName:    os.Getenv("AZFOUNDRY_LIVE_FOUNDRY"),
		DeploymentName: os.Getenv("AZFOUNDRY_LIVE_DEPLOYMENT"),
		Format:         os.Getenv("AZFOUNDRY_LIVE_FORMAT"),
		Model:          os.Getenv("AZFOUNDRY_LIVE_MODEL"),
		Version:        os.Getenv("AZFOUNDRY_LIVE_VERSION"),
		SKU:            os.Getenv("AZFOUNDRY_LIVE_SKU"),
		Capacity:       10,
	}
	if request.SubscriptionID == "" || request.ResourceGroup == "" || request.FoundryName == "" || request.DeploymentName == "" {
		t.Fatal("the live target variables are incomplete")
	}

	result, err := NewProvider().CreateModelDeployment(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateModelDeployment returned error: %v", err)
	}
	t.Logf("outcome=%s detail=%s", result.Outcome, result.Detail)
	if result.Outcome != service.OutcomeConflict {
		t.Fatalf("outcome = %q, want %q so the existing deployment is never updated", result.Outcome, service.OutcomeConflict)
	}
	if result.CreatedGroup || result.CreatedFoundry {
		t.Fatalf("result = %+v, want no resource created for an existing name", result)
	}
	if !strings.HasPrefix(result.DeploymentID, "/subscriptions/") {
		t.Fatalf("deployment ID = %q, want the full resource ID", result.DeploymentID)
	}
}
