package foundry

import (
	"context"
	"fmt"
	"slices"

	"azfoundrydeck/internal/fault"
)

// DeploymentSettings is the current deployment and the choices offered when changing it.
type DeploymentSettings struct {
	DeploymentID    string   `json:"deploymentId"`
	DeploymentName  string   `json:"deploymentName"`
	ModelName       string   `json:"modelName"`
	SKUName         string   `json:"skuName"`
	Option          string   `json:"option"`
	Version         string   `json:"version"`
	Versions        []string `json:"versions"`
	Capacity        *float64 `json:"capacity"`
	CapacityMaximum *float64 `json:"capacityMaximum"`
	CapacityUnit    *string  `json:"capacityUnit"`
	UpgradePolicy   string   `json:"upgradePolicy"`
}

// DeploymentUpdateSpec is the subset of deployment settings the operator can change.
type DeploymentUpdateSpec struct {
	DeploymentID  string `json:"deploymentId"`
	Version       string `json:"version"`
	Capacity      *int64 `json:"capacity"`
	UpgradePolicy string `json:"upgradePolicy"`
}

// DeploymentUpdateSource loads and changes one deployment at the external boundary.
type DeploymentUpdateSource interface {
	DeploymentSettings(context.Context, Foundry, Deployment) (DeploymentSettings, error)
	UpdateDeployment(context.Context, Foundry, Deployment, DeploymentUpdateSpec) error
}

// GetDeploymentSettings returns the current settings and the choices for one deployment.
func (s *Service) GetDeploymentSettings(ctx context.Context, deploymentID string) (DeploymentSettings, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return DeploymentSettings{}, err
	}
	file, err := s.file()
	var settings DeploymentSettings
	if err == nil {
		settings, err = s.deploymentSettings(ctx, file, deploymentID)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetDeploymentSettings", "cause", err)
		if ctx.Err() != nil {
			return DeploymentSettings{}, fault.Public(ctx.Err())
		}
		return DeploymentSettings{}, fault.New("DEPLOYMENT_UPDATE_FAILED", "Could not load the deployment settings.")
	}
	return settings, nil
}

func (s *Service) deploymentSettings(ctx context.Context, file, deploymentID string) (DeploymentSettings, error) {
	foundry, deployment, source, err := s.selectedDeployment(file, deploymentID)
	if err != nil {
		return DeploymentSettings{}, err
	}
	updateSource, ok := source.(DeploymentUpdateSource)
	if !ok {
		return DeploymentSettings{}, fmt.Errorf("deployment update is not connected to Azure yet")
	}
	return updateSource.DeploymentSettings(ctx, foundry, deployment)
}

// UpdateDeployment changes one deployment of the selected Foundry, then re-fetches
// that Foundry's models and replaces the saved files and the view.
func (s *Service) UpdateDeployment(ctx context.Context, spec DeploymentUpdateSpec) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.updateDeployment(ctx, file, spec)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.UpdateDeployment", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("DEPLOYMENT_UPDATE_FAILED", "Could not update the deployment or refresh the deployed models.")
	}
	return view, nil
}

func (s *Service) updateDeployment(ctx context.Context, file string, spec DeploymentUpdateSpec) (InitialFoundryView, error) {
	foundry, deployment, source, err := s.selectedDeployment(file, spec.DeploymentID)
	if err != nil {
		return InitialFoundryView{}, err
	}
	updateSource, ok := source.(DeploymentUpdateSource)
	if !ok {
		return InitialFoundryView{}, fmt.Errorf("deployment update is not connected to Azure yet")
	}
	if err := updateSource.UpdateDeployment(ctx, foundry, deployment, spec); err != nil {
		return InitialFoundryView{}, err
	}
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	return s.acquireModels(ctx, file, view, foundry)
}

func (s *Service) selectedDeployment(file, deploymentID string) (Foundry, Deployment, Source, error) {
	view, err := read(file)
	if err != nil {
		return Foundry{}, Deployment{}, nil, err
	}
	foundryIndex := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool {
		return foundry.ID == view.SelectedFoundryID
	})
	if foundryIndex < 0 {
		return Foundry{}, Deployment{}, nil, fmt.Errorf("selected Foundry is not in the saved list")
	}
	deploymentIndex := slices.IndexFunc(view.Deployments, func(deployment Deployment) bool {
		return deployment.ID == deploymentID
	})
	if deploymentIndex < 0 {
		return Foundry{}, Deployment{}, nil, fmt.Errorf("selected deployment is not in the selected Foundry's saved list")
	}
	source, err := s.source()
	if err != nil {
		return Foundry{}, Deployment{}, nil, err
	}
	return view.Foundries[foundryIndex], view.Deployments[deploymentIndex], source, nil
}
