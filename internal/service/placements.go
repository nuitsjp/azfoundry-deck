package service

import "context"

// PlacementFoundry is one existing Foundry account a model can be added to.
// A Foundry with no deployment is included, so it carries no deployment rows.
type PlacementFoundry struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	Region        string `json:"region"`
}

// PlacementGroup is one resource group that can hold a new or existing Foundry.
type PlacementGroup struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

// PlacementSubscription is one signed-in subscription with the placements it
// offers. Groups and foundries are listed independently so a group without a
// Foundry, and a Foundry without a deployment, both remain selectable.
type PlacementSubscription struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	TenantName string             `json:"tenantName"`
	Groups     []PlacementGroup   `json:"groups"`
	Foundries  []PlacementFoundry `json:"foundries"`
}

// PlacementResult is one complete read of the places a model can be added to.
type PlacementResult struct {
	Subscriptions []PlacementSubscription `json:"subscriptions"`
	Failures      []FetchFailure          `json:"failures"`
	FetchedAt     string                  `json:"fetchedAt"`
}

// DeploymentNameResult is the existing deployment names of one Foundry. The
// screen rejects a duplicate name only when the read succeeded; a failure must
// not be treated as "the name is free".
type DeploymentNameResult struct {
	Names     []string       `json:"names"`
	Failures  []FetchFailure `json:"failures"`
	FetchedAt string         `json:"fetchedAt"`
}

// PlacementService is the screen-facing service for choosing where a model is
// added. It only reads; creating a resource group or Foundry is not part of it.
type PlacementService struct {
	fetch           func(context.Context) (PlacementResult, error)
	fetchDeployment func(context.Context, string) (DeploymentNameResult, error)
	mock            bool
}

// NewPlacementService creates the common service used by both the mock and the
// Azure implementation.
func NewPlacementService(
	fetch func(context.Context) (PlacementResult, error),
	fetchDeployment func(context.Context, string) (DeploymentNameResult, error),
	mock bool,
) *PlacementService {
	return &PlacementService{fetch: fetch, fetchDeployment: fetchDeployment, mock: mock}
}

// GetPlacements reads the subscriptions, resource groups and Foundries the
// signed-in account can add a model to.
func (s *PlacementService) GetPlacements(ctx context.Context) (PlacementResult, error) {
	result, err := s.fetch(ctx)
	return normalizePlacementResult(result), err
}

// GetDeploymentNames reads the deployment names of one Foundry so the screen can
// reject a name that already exists in that exact Foundry.
func (s *PlacementService) GetDeploymentNames(ctx context.Context, accountID string) (DeploymentNameResult, error) {
	result, err := s.fetchDeployment(ctx, accountID)
	if result.Names == nil {
		result.Names = make([]string, 0)
	}
	if result.Failures == nil {
		result.Failures = make([]FetchFailure, 0)
	}
	return result, err
}

func normalizePlacementResult(result PlacementResult) PlacementResult {
	if result.Subscriptions == nil {
		result.Subscriptions = make([]PlacementSubscription, 0)
	}
	if result.Failures == nil {
		result.Failures = make([]FetchFailure, 0)
	}
	for index := range result.Subscriptions {
		if result.Subscriptions[index].Groups == nil {
			result.Subscriptions[index].Groups = make([]PlacementGroup, 0)
		}
		if result.Subscriptions[index].Foundries == nil {
			result.Subscriptions[index].Foundries = make([]PlacementFoundry, 0)
		}
	}
	return result
}
