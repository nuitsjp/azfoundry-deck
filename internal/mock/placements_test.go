package mock

import (
	"context"
	"testing"
)

func TestPlacementsIncludeEmptyGroupsAndFoundriesWithoutDeployments(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait

	result, err := provider.FetchPlacements(context.Background())
	if err != nil {
		t.Fatalf("FetchPlacements returned error: %v", err)
	}
	if len(result.Subscriptions) != 2 || len(result.Failures) != 0 || result.FetchedAt == "" {
		t.Fatalf("placement result = %+v, want the two addition subscriptions", result)
	}

	first := result.Subscriptions[0]
	if first.ID != "subscription-001" || first.Name != "Contoso Production" || first.TenantName != "Contoso Engineering" {
		t.Fatalf("first subscription = %+v", first)
	}
	if len(first.Groups) != 2 || first.Groups[1].Name != "rg-empty" {
		t.Fatalf("groups = %+v, want the group without a Foundry", first.Groups)
	}
	if len(first.Foundries) != 2 || first.Foundries[1].Name != "contoso-first-model" || first.Foundries[1].Region != "japaneast" {
		t.Fatalf("foundries = %+v, want the Foundry without deployments", first.Foundries)
	}
	second := result.Subscriptions[1]
	if len(second.Groups) != 1 || len(second.Foundries) != 0 {
		t.Fatalf("second subscription = %+v, want a group without any Foundry", second)
	}
}

func TestDeploymentNamesSeparateEmptyFoundryFromUnknownTarget(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait

	existing, err := provider.FetchDeploymentNames(context.Background(), addFoundryID(addFoundries[0]))
	if err != nil {
		t.Fatalf("FetchDeploymentNames returned error: %v", err)
	}
	if len(existing.Names) != 1 || existing.Names[0] != "chat-production" || len(existing.Failures) != 0 {
		t.Fatalf("existing names = %+v, want the fixture deployment", existing)
	}

	empty, err := provider.FetchDeploymentNames(context.Background(), addFoundryID(addFoundries[1]))
	if err != nil {
		t.Fatalf("FetchDeploymentNames returned error: %v", err)
	}
	if len(empty.Names) != 0 || len(empty.Failures) != 0 {
		t.Fatalf("empty Foundry names = %+v, want an empty list without failures", empty)
	}

	unknown, err := provider.FetchDeploymentNames(context.Background(), "/subscriptions/none/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/missing")
	if err != nil {
		t.Fatalf("FetchDeploymentNames returned error: %v", err)
	}
	if len(unknown.Failures) != 1 || unknown.Failures[0].Code != "account-not-found" {
		t.Fatalf("unknown Foundry result = %+v, want a reported failure", unknown)
	}
}

func TestAddModelCandidatesFollowTheirOwnScenarioControl(t *testing.T) {
	provider := NewProvider()
	provider.wait = noWait

	result, err := provider.FetchModels(context.Background(), addFoundryID(addFoundries[0]))
	if err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if len(result.Models) != 4 || result.Models[0].Version != "2024-08-06" || !result.Models[0].IsDefaultVersion {
		t.Fatalf("addition candidates = %+v, want the fixture rows", result.Models)
	}
	if *result.Models[0].SKUs[1].Capacity.Default != 10 {
		t.Fatalf("GlobalStandard default = %+v, want 10", result.Models[0].SKUs[1].Capacity)
	}

	if err := provider.SetAddModelScenario(AddScenarioMissingCapacity); err != nil {
		t.Fatalf("SetAddModelScenario returned error: %v", err)
	}
	missing, err := provider.FetchRegionModels(context.Background(), "subscription-001", "japaneast")
	if err != nil {
		t.Fatalf("FetchRegionModels returned error: %v", err)
	}
	if len(missing.Models) != 1 || missing.Models[0].SKUs[0].Capacity != nil {
		t.Fatalf("missing capacity candidates = %+v", missing.Models)
	}

	if err := provider.SetAddModelScenario(AddScenarioFailure); err != nil {
		t.Fatalf("SetAddModelScenario returned error: %v", err)
	}
	failed, err := provider.FetchModels(context.Background(), addFoundryID(addFoundries[0]))
	if err != nil {
		t.Fatalf("FetchModels returned error: %v", err)
	}
	if len(failed.Models) != 0 || len(failed.Failures) != 1 || failed.Failures[0].Code != "model-catalog-unavailable" {
		t.Fatalf("failure scenario = %+v", failed)
	}

	if provider.SetAddModelScenario("unknown-scenario") == nil {
		t.Fatal("SetAddModelScenario accepted an unknown scenario")
	}
}
