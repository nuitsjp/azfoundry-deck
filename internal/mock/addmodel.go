package mock

import (
	"strings"

	"github.com/nuitsjp/azfoundry-deck/internal/service"
)

// The model addition screen has its own fixture. It is deliberately separate
// from the F1 deployment fixture: the placement read reaches resource groups
// without a Foundry and Foundries without a deployment, which never appear in
// the deployment list.

// Model load scenarios of the model addition screen.
const (
	AddScenarioSuccess         = "success"
	AddScenarioSlow            = "slow"
	AddScenarioEmpty           = "empty"
	AddScenarioFailure         = "failure"
	AddScenarioNoDefault       = "no-default"
	AddScenarioMissingCapacity = "missing-capacity"
)

var validAddScenarios = map[string]struct{}{
	AddScenarioSuccess:         {},
	AddScenarioSlow:            {},
	AddScenarioEmpty:           {},
	AddScenarioFailure:         {},
	AddScenarioNoDefault:       {},
	AddScenarioMissingCapacity: {},
}

type addFoundry struct {
	subscription string
	group        string
	name         string
	region       string
	deployments  []string
}

type addSubscription struct {
	id     string
	name   string
	tenant string
}

var addSubscriptions = []addSubscription{
	{id: "subscription-001", name: "Contoso Production", tenant: "Contoso Engineering"},
	{id: "subscription-002", name: "Contoso Sandbox", tenant: "Contoso Engineering"},
}

var addGroups = []struct {
	subscription string
	name         string
}{
	{subscription: "subscription-001", name: "rg-production"},
	{subscription: "subscription-001", name: "rg-empty"},
	{subscription: "subscription-002", name: "rg-sandbox"},
}

var addFoundries = []addFoundry{
	{subscription: "subscription-001", group: "rg-production", name: "contoso-chat-prod", region: "eastus", deployments: []string{"chat-production"}},
	{subscription: "subscription-001", group: "rg-production", name: "contoso-first-model", region: "japaneast", deployments: []string{}},
}

func addFoundryID(foundry addFoundry) string {
	return "/subscriptions/" + foundry.subscription + "/resourceGroups/" + foundry.group +
		"/providers/Microsoft.CognitiveServices/accounts/" + foundry.name
}

func addFoundryByID(accountID string) (addFoundry, bool) {
	for _, foundry := range addFoundries {
		if strings.EqualFold(addFoundryID(foundry), accountID) {
			return foundry, true
		}
	}
	return addFoundry{}, false
}

// placementResult lists every place the mock offers for a new model.
func placementResult() service.PlacementResult {
	result := service.PlacementResult{
		Subscriptions: make([]service.PlacementSubscription, 0, len(addSubscriptions)),
		Failures:      make([]service.FetchFailure, 0),
	}
	for _, subscription := range addSubscriptions {
		placement := service.PlacementSubscription{
			ID:         subscription.id,
			Name:       subscription.name,
			TenantName: subscription.tenant,
			Groups:     make([]service.PlacementGroup, 0, 2),
			Foundries:  make([]service.PlacementFoundry, 0, 2),
		}
		for _, group := range addGroups {
			if group.subscription != subscription.id {
				continue
			}
			placement.Groups = append(placement.Groups, service.PlacementGroup{Name: group.name, Location: "japaneast"})
		}
		for _, foundry := range addFoundries {
			if foundry.subscription != subscription.id {
				continue
			}
			placement.Foundries = append(placement.Foundries, service.PlacementFoundry{
				ID:            addFoundryID(foundry),
				Name:          foundry.name,
				ResourceGroup: foundry.group,
				Region:        foundry.region,
			})
		}
		result.Subscriptions = append(result.Subscriptions, placement)
	}
	return result
}

// deploymentNamesForAccount returns the fixed deployment names of one Foundry.
// A Foundry without a deployment returns an empty list, not a failure.
func deploymentNamesForAccount(accountID string) ([]string, bool) {
	foundry, known := addFoundryByID(accountID)
	if !known {
		return nil, false
	}
	names := make([]string, 0, len(foundry.deployments))
	names = append(names, foundry.deployments...)
	return names, true
}

func int32Pointer(value int32) *int32 { return &value }

func rangeCapacity(defaultCapacity, minimum, maximum, step int32) *service.CapacityContract {
	contract := service.CapacityContract{
		Minimum:       int32Pointer(minimum),
		Maximum:       int32Pointer(maximum),
		Step:          int32Pointer(step),
		AllowedValues: []int32{},
	}
	if defaultCapacity > 0 {
		contract.Default = int32Pointer(defaultCapacity)
	}
	return &contract
}

// addModelCandidates returns the flat model-version rows of the addition screen
// fixture. Rows keep the combination of format, name, version and SKU so the
// screen never rebuilds a combination Azure did not report.
func addModelCandidates(scenario string) []service.ModelCandidate {
	switch scenario {
	case AddScenarioNoDefault:
		return []service.ModelCandidate{
			{Format: "OpenAI", Name: "gpt-no-default", Version: "2026-01-01", SKUs: []service.ModelSKU{
				{Name: "GlobalStandard", Capacity: rangeCapacity(5, 1, 20, 1)},
			}},
			{Format: "OpenAI", Name: "gpt-no-default", Version: "2026-02-01", SKUs: []service.ModelSKU{
				{Name: "Standard", Capacity: rangeCapacity(0, 1, 20, 1)},
				{Name: "DataZoneStandard", Capacity: rangeCapacity(5, 1, 20, 1)},
			}},
		}
	case AddScenarioMissingCapacity:
		return []service.ModelCandidate{
			{Format: "OpenAI", Name: "gpt-capacity-unknown", Version: "2026-03-01", IsDefaultVersion: true, SKUs: []service.ModelSKU{
				{Name: "GlobalStandard"},
			}},
		}
	default:
		return []service.ModelCandidate{
			{Format: "OpenAI", Name: "gpt-4o", Version: "2024-08-06", IsDefaultVersion: true, SKUs: []service.ModelSKU{
				{Name: "Standard", Capacity: rangeCapacity(0, 1, 100, 1)},
				{Name: "GlobalStandard", Capacity: rangeCapacity(10, 1, 100, 1)},
			}},
			{Format: "OpenAI", Name: "gpt-4o", Version: "2024-11-20", SKUs: []service.ModelSKU{
				{Name: "GlobalStandard", Capacity: rangeCapacity(20, 1, 100, 1)},
				{Name: "Standard", Capacity: rangeCapacity(0, 1, 100, 1)},
			}},
			{Format: "OpenAI", Name: "gpt-5.4-mini", Version: "2026-03-17", IsDefaultVersion: true, SKUs: []service.ModelSKU{
				{Name: "GlobalStandard", Capacity: rangeCapacity(10, 1, 100, 1)},
			}},
			{Format: "OpenAI", Name: "text-embedding-3-large", Version: "1", IsDefaultVersion: true, SKUs: []service.ModelSKU{
				{Name: "Standard", Capacity: &service.CapacityContract{Default: int32Pointer(1), AllowedValues: []int32{1, 5, 10}}},
			}},
		}
	}
}

// addModelResultForScenario builds the candidate read of the addition screen.
func addModelResultForScenario(target, scenario string) service.ModelResult {
	if _, ok := validAddScenarios[scenario]; !ok {
		panic("invalid mock add-model scenario: " + scenario)
	}
	result := service.ModelResult{
		Models:   make([]service.ModelCandidate, 0),
		Failures: make([]service.FetchFailure, 0),
	}
	switch scenario {
	case AddScenarioEmpty:
		return result
	case AddScenarioFailure:
		result.Failures = []service.FetchFailure{{
			Scope:       "account",
			AccountName: target,
			Code:        "model-catalog-unavailable",
			Message:     "モデル候補を取得できませんでした。",
			Action:      "時間をおいて再取得してください。",
		}}
		return result
	default:
		result.Models = addModelCandidates(scenario)
		return result
	}
}

// Creation outcomes reproduced by the mock. No Azure request is made.
var validCreateScenarios = map[string]struct{}{
	service.OutcomeSuccess:           {},
	service.OutcomeGroupFailure:      {},
	service.OutcomeCandidateMismatch: {},
	service.OutcomeDeploymentFailure: {},
	service.OutcomeUnknown:           {},
	service.OutcomeConflict:          {},
	service.OutcomeListFailure:       {},
	service.OutcomeInterrupted:       {},
}
