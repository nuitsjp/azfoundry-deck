package foundry

import (
	"context"
	"fmt"

	"azfoundrydeck/internal/fault"
)

// CapacityLimits holds one Foundry's model definitions and its regional shared
// quota, from which the allocatable maximum of each deployment is derived.
type CapacityLimits interface {
	// Maximum is nil when the model definition or the quota cannot give a value.
	Maximum(Deployment) *float64
	// Versions lists the model versions offered for the deployment, including its current one.
	Versions(Deployment) []string
	// RefreshQuota returns the same definitions with the shared quota fetched again.
	RefreshQuota(context.Context) (CapacityLimits, error)
}

// CapacitySource fetches the capacity limits of one Foundry at the external boundary.
type CapacitySource interface {
	CapacityLimits(context.Context, Foundry) (CapacityLimits, error)
}

// GetCapacityMaximum returns the allocatable capacity maximum of one deployment of
// the selected Foundry. The limits are fetched the first time and then kept in
// memory until the view is loaded again or the Foundry changes.
func (s *Service) GetCapacityMaximum(ctx context.Context, deploymentID string) (*float64, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return nil, err
	}
	file, err := s.file()
	var maximum *float64
	if err == nil {
		maximum, err = s.capacityMaximum(ctx, file, deploymentID)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetCapacityMaximum", "cause", err)
		if ctx.Err() != nil {
			return nil, fault.Public(ctx.Err())
		}
		return nil, fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve the capacity maximum from the source or read the current Foundry selection.")
	}
	return maximum, nil
}

func (s *Service) capacityMaximum(ctx context.Context, file, deploymentID string) (*float64, error) {
	foundry, deployment, source, err := s.selectedDeployment(file, deploymentID)
	if err != nil {
		return nil, err
	}
	limits, _, err := s.capacityLimits(ctx, file, foundry, source)
	if err != nil {
		return nil, err
	}
	return limits.Maximum(deployment), nil
}

// capacityLimits returns the kept limits of the Foundry, fetching them when absent.
// fetched reports whether this call fetched them.
func (s *Service) capacityLimits(ctx context.Context, file string, foundry Foundry, source Source) (CapacityLimits, bool, error) {
	if s.limits.value != nil && s.limits.file == file && s.limits.foundryID == foundry.ID {
		return s.limits.value, false, nil
	}
	capacitySource, ok := source.(CapacitySource)
	if !ok {
		return nil, false, fmt.Errorf("capacity limits are not connected to Azure yet")
	}
	limits, err := capacitySource.CapacityLimits(ctx, foundry)
	if err != nil {
		return nil, false, err
	}
	s.limits = limitsCache{file: file, foundryID: foundry.ID, value: limits}
	return limits, true, nil
}
