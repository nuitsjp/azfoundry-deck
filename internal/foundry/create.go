package foundry

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"azfoundrydeck/internal/fault"
)

type ModelSKUItem struct {
	Name        string `json:"name"`
	MaxCapacity *int64 `json:"maxCapacity"`
}

type ModelCatalogItem struct {
	Name        string         `json:"name"`
	Publisher   string         `json:"publisher"`
	Option      string         `json:"option"`
	Tasks       []string       `json:"tasks"`
	Sub         string         `json:"sub"`
	MaxCapacity *int64         `json:"maxCapacity"`
	Versions    []string       `json:"versions"`
	SKUs        []ModelSKUItem `json:"skus"`
	InputRate   *string        `json:"inputRate"`
	OutputRate  *string        `json:"outputRate"`
}

type DeploymentCreateSpec struct {
	DeploymentName string `json:"deploymentName"`
	ModelName      string `json:"modelName"`
	Version        string `json:"version"`
	SKU            string `json:"sku"`
	Capacity       *int64 `json:"capacity"`
	UpgradePolicy  string `json:"upgradePolicy"`
}

type ModelCatalogSource interface {
	ListModels(context.Context, Foundry) ([]ModelCatalogItem, error)
}

type DeploymentCreateSource interface {
	CreateDeployment(context.Context, Foundry, DeploymentCreateSpec) error
}

// GetModelCatalog returns the catalog of available models for the selected Foundry.
func (s *Service) GetModelCatalog(ctx context.Context) ([]ModelCatalogItem, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return nil, err
	}
	file, err := s.file()
	if err != nil {
		return nil, err
	}
	view, err := read(file)
	if err != nil {
		return nil, err
	}
	foundryIndex := slices.IndexFunc(view.Foundries, func(f Foundry) bool { return f.ID == view.SelectedFoundryID })
	if foundryIndex < 0 {
		return nil, fmt.Errorf("selected Foundry is not in the saved list")
	}
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	catalogSource, ok := source.(ModelCatalogSource)
	if !ok {
		return nil, fmt.Errorf("model catalog is not connected to Azure yet")
	}
	items, err := catalogSource.ListModels(ctx, view.Foundries[foundryIndex])
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetModelCatalog", "cause", err)
		if ctx.Err() != nil {
			return nil, fault.Public(ctx.Err())
		}
		return nil, fault.New("MODEL_CATALOG_FAILED", "Could not load the available model catalog.")
	}
	return items, nil
}

// CreateDeployment deploys a new model to the selected Foundry in Azure, then
// re-fetches that Foundry's models and replaces the saved files and the view.
func (s *Service) CreateDeployment(ctx context.Context, spec DeploymentCreateSpec) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.createDeployment(ctx, file, spec)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.CreateDeployment", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		msg := "Could not create the deployment or refresh the deployed models."
		if strings.Contains(err.Error(), "InsufficientQuota") {
			msg = "Insufficient quota: The requested capacity exceeds the available quota for this model in your subscription."
		} else if strings.Contains(err.Error(), "already exists") {
			msg = err.Error()
		}
		return InitialFoundryView{}, fault.New("DEPLOYMENT_CREATE_FAILED", msg)
	}
	return view, nil
}

func (s *Service) createDeployment(ctx context.Context, file string, spec DeploymentCreateSpec) (InitialFoundryView, error) {
	if strings.TrimSpace(spec.DeploymentName) == "" {
		return InitialFoundryView{}, fmt.Errorf("deployment name cannot be empty")
	}
	if strings.TrimSpace(spec.ModelName) == "" {
		return InitialFoundryView{}, fmt.Errorf("model name cannot be empty")
	}
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	foundryIndex := slices.IndexFunc(view.Foundries, func(f Foundry) bool { return f.ID == view.SelectedFoundryID })
	if foundryIndex < 0 {
		return InitialFoundryView{}, fmt.Errorf("selected Foundry is not in the saved list")
	}

	// Reject if deployment name already exists in this Foundry to prevent accidental overwrite
	trimmedName := strings.TrimSpace(spec.DeploymentName)
	for _, d := range view.Deployments {
		if strings.EqualFold(d.DeploymentName, trimmedName) {
			return InitialFoundryView{}, fmt.Errorf("The deployment name '%s' already exists in this Foundry.", trimmedName)
		}
	}
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	createSource, ok := source.(DeploymentCreateSource)
	if !ok {
		return InitialFoundryView{}, fmt.Errorf("deployment creation is not connected to Azure yet")
	}
	selected := view.Foundries[foundryIndex]
	if err := createSource.CreateDeployment(ctx, selected, spec); err != nil {
		return InitialFoundryView{}, err
	}
	return s.acquireModels(ctx, file, view, selected)
}
