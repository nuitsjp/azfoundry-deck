package foundry

import (
	"context"
	"fmt"
	"slices"

	"azfoundrydeck/internal/fault"
)

type DeploymentDeleteSource interface {
	DeleteDeployment(context.Context, Foundry, Deployment) error
}

// DeleteDeployment deletes one deployment of the selected Foundry in Azure, then
// re-fetches that Foundry's models and replaces the in-memory deployments and the view.
func (s *Service) DeleteDeployment(ctx context.Context, deploymentID string) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.deleteDeployment(ctx, file, deploymentID)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.DeleteDeployment", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("DEPLOYMENT_DELETE_FAILED", "Could not delete the deployment or refresh the deployed models.")
	}
	return view, nil
}

func (s *Service) deleteDeployment(ctx context.Context, file, deploymentID string) (InitialFoundryView, error) {
	view, err := s.readView(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	foundryIndex := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if foundryIndex < 0 {
		return InitialFoundryView{}, fmt.Errorf("selected Foundry is not in the saved list")
	}
	deploymentIndex := slices.IndexFunc(view.Deployments, func(deployment Deployment) bool { return deployment.ID == deploymentID })
	if deploymentIndex < 0 {
		return InitialFoundryView{}, fmt.Errorf("selected deployment is not in the selected Foundry's saved list")
	}
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	deleteSource, ok := source.(DeploymentDeleteSource)
	if !ok {
		return InitialFoundryView{}, fmt.Errorf("deployment deletion is not connected to Azure yet")
	}
	selected := view.Foundries[foundryIndex]
	if err := deleteSource.DeleteDeployment(ctx, selected, view.Deployments[deploymentIndex]); err != nil {
		return InitialFoundryView{}, err
	}
	if s.limits.file == file && s.limits.foundryID == selected.ID {
		s.startQuotaRefresh(ctx)
	}
	return s.acquireModels(ctx, file, view, selected)
}
