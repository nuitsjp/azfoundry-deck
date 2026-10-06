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

// DeploymentUpdateSource changes one deployment at the external boundary.
type DeploymentUpdateSource interface {
	UpdateDeployment(context.Context, Foundry, Deployment, DeploymentUpdateSpec) error
}

// GetDeploymentSettings returns the current settings and the choices for one deployment.
func (s *Service) GetDeploymentSettings(ctx context.Context, deploymentID string) (DeploymentSettings, error) {
	s.operations.Lock()
	if err := s.signedIn(ctx); err != nil {
		s.operations.Unlock()
		return DeploymentSettings{}, err
	}
	file, err := s.file()
	var foundry Foundry
	var deployment Deployment
	var source Source
	if err == nil {
		foundry, deployment, source, err = s.selectedDeployment(file, deploymentID)
	}
	if err != nil {
		s.operations.Unlock()
		return DeploymentSettings{}, fault.New("DEPLOYMENT_UPDATE_FAILED", "Could not load the deployment settings.")
	}
	s.startCapacityLimits(ctx, file, foundry, source)
	limits, loading, _ := s.readLimits()
	if !loading && limits != nil {
		s.startQuotaRefresh(ctx)
	}
	fetch := s.limits.fetch
	s.operations.Unlock()
	if err := waitLimits(ctx, fetch, false); err != nil {
		return DeploymentSettings{}, fault.Public(err)
	}
	s.operations.Lock()
	defer s.operations.Unlock()
	if !s.sameLimits(file, foundry.ID, fetch) {
		return DeploymentSettings{}, fault.New("DEPLOYMENT_UPDATE_FAILED", "The selected Foundry changed.")
	}
	limits, _, err = s.readLimits()
	if err != nil || limits == nil {
		return DeploymentSettings{}, fault.New("DEPLOYMENT_UPDATE_FAILED", "Could not load the deployment settings.")
	}
	return deploymentSettings(deployment, limits), nil
}

// deploymentSettings builds the settings from the listed deployment and the
// capacity limits. The shared quota is fetched again so the maximum is current.
func deploymentSettings(deployment Deployment, limits CapacityLimits) DeploymentSettings {
	settings := DeploymentSettings{
		DeploymentID: deployment.ID, DeploymentName: deployment.DeploymentName, ModelName: deployment.ModelName,
		Option: "Pay-as-you-go", Version: deployment.Version, Versions: limits.Versions(deployment),
	}
	if deployment.SKUName != nil {
		settings.SKUName = *deployment.SKUName
	}
	if deployment.VersionUpgradePolicy != nil {
		settings.UpgradePolicy = *deployment.VersionUpgradePolicy
	}
	if deployment.CapacityUnit != nil && *deployment.CapacityUnit != "" {
		settings.Option = "Standard"
		settings.Capacity, settings.CapacityUnit = deployment.Capacity, deployment.CapacityUnit
		settings.CapacityMaximum = limits.Maximum(deployment)
	}
	return settings
}

// UpdateDeployment changes one deployment of the selected Foundry, then re-fetches
// that Foundry's models and replaces the in-memory deployments and the view.
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
	if spec.Capacity != nil && (deployment.Capacity == nil || float64(*spec.Capacity) != *deployment.Capacity) &&
		s.limits.file == file && s.limits.foundryID == foundry.ID {
		s.startQuotaRefresh(ctx)
	}
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	return s.acquireModels(ctx, file, view, foundry)
}

func (s *Service) selectedDeployment(file, deploymentID string) (Foundry, Deployment, Source, error) {
	view, err := s.readView(file)
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
