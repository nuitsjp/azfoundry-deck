package azurego

import (
	"testing"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

func TestCapacityMismatchAcceptsAMaximumOnlyContract(t *testing.T) {
	maximum := int32(1000000)
	minimum := int32(15)
	step := int32(5)
	for _, test := range []struct {
		name     string
		capacity int32
		contract *service.CapacityContract
		wantStop bool
	}{
		{name: "missing contract", capacity: 10, contract: nil, wantStop: true},
		// Every pay-as-you-go SKU reports only default and maximum.
		{name: "maximum only", capacity: 10, contract: &service.CapacityContract{Maximum: &maximum}},
		{name: "maximum only above", capacity: 2000000, contract: &service.CapacityContract{Maximum: &maximum}, wantStop: true},
		{name: "maximum only zero", capacity: 0, contract: &service.CapacityContract{Maximum: &maximum}, wantStop: true},
		// Provisioned SKUs report minimum, maximum and step but no default.
		{name: "range", capacity: 20, contract: &service.CapacityContract{Minimum: &minimum, Maximum: &maximum, Step: &step}},
		{name: "below minimum", capacity: 10, contract: &service.CapacityContract{Minimum: &minimum, Maximum: &maximum, Step: &step}, wantStop: true},
		{name: "allowed value", capacity: 5, contract: &service.CapacityContract{AllowedValues: []int32{1, 5, 10}}},
		{name: "outside allowed values", capacity: 7, contract: &service.CapacityContract{AllowedValues: []int32{1, 5, 10}}, wantStop: true},
		{name: "no usable constraint", capacity: 10, contract: &service.CapacityContract{}, wantStop: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := capacityMismatch(test.capacity, test.contract)
			if (message != "") != test.wantStop {
				t.Fatalf("capacityMismatch = %q, wantStop = %v", message, test.wantStop)
			}
		})
	}
}

func TestCreateResourceIDsUseTheConfirmedTarget(t *testing.T) {
	request := service.CreateRequest{
		SubscriptionID: "sub",
		ResourceGroup:  "rg",
		FoundryName:    "foundry",
		DeploymentName: "deployment",
	}
	want := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/foundry/deployments/deployment"
	if got := deploymentResourceID(request); got != want {
		t.Fatalf("deployment ID = %q, want %q", got, want)
	}
	if got := foundryResourceID(request); got != "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/foundry" {
		t.Fatalf("foundry ID = %q", got)
	}
	if got := groupResourceID(request); got != "/subscriptions/sub/resourceGroups/rg" {
		t.Fatalf("group ID = %q", got)
	}
}

// The SDK treats zero as "use the default of three attempts", so only a negative
// value stops a write from being resent.
func TestWriteRetryIsDisabledByANegativeMaxRetries(t *testing.T) {
	if noWriteRetry.MaxRetries >= 0 {
		t.Fatalf("MaxRetries = %d, want a negative value so writes are never resent", noWriteRetry.MaxRetries)
	}
}
