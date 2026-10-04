package foundry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"azfoundrydeck/internal/fault"
)

// FoundryDeletionPlan is what Azure says about the selected Foundry's resource
// group before anything is deleted.
type FoundryDeletionPlan struct {
	FoundryName         string `json:"foundryName"`
	ResourceGroupName   string `json:"resourceGroupName"`
	DeleteResourceGroup bool   `json:"deleteResourceGroup"`
}

const FoundryDeleteProgressEvent = "foundry:delete-progress"

type FoundryDeleteProgress struct {
	FoundryName        string `json:"foundryName"`
	ResourceGroupName  string `json:"resourceGroupName"`
	FoundryPhase       string `json:"foundryPhase"`
	PurgePhase         string `json:"purgePhase"`
	ResourceGroupPhase string `json:"resourceGroupPhase"`
	ViewPhase          string `json:"viewPhase"`
}

// FoundryDeleteSource is the ARM boundary shared by the deletion UI and service.
type FoundryDeleteSource interface {
	// ResourceGroupHoldsOnlyFoundry reports whether every resource in the Foundry's
	// resource group is Foundry related (the account, its projects and deployments).
	ResourceGroupHoldsOnlyFoundry(context.Context, Foundry) (bool, error)
	DeleteFoundry(context.Context, Foundry) error
	PurgeFoundry(context.Context, Foundry) error
	DeleteResourceGroup(context.Context, Foundry) error
}

func (s *Service) deletionSource() (FoundryDeleteSource, error) {
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	deletion, ok := source.(FoundryDeleteSource)
	if !ok {
		return nil, fmt.Errorf("Foundry deletion is not connected to Azure yet")
	}
	return deletion, nil
}

func (s *Service) selectedFoundry(file string) (InitialFoundryView, Foundry, error) {
	view, err := s.readView(file)
	if err != nil {
		return InitialFoundryView{}, Foundry{}, err
	}
	index := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if index < 0 {
		return InitialFoundryView{}, Foundry{}, fmt.Errorf("selected Foundry is not in the saved list")
	}
	return view, view.Foundries[index], nil
}

func (s *Service) deleteFailure(ctx context.Context, operation string, err error) error {
	s.logger.Error("operation_failed", "operation", operation, "cause", err)
	if ctx.Err() != nil {
		return fault.Public(ctx.Err())
	}
	var public *fault.Error
	if errors.As(err, &public) {
		return public
	}
	return fault.New("FOUNDRY_DELETE_FAILED", "Could not delete the Foundry or update Home.")
}

// InspectFoundryDeletion decides how the selected Foundry is deleted, before the
// user confirms. Only a resource group that holds nothing but Foundry resources is
// supported so far.
func (s *Service) InspectFoundryDeletion(ctx context.Context) (FoundryDeletionPlan, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return FoundryDeletionPlan{}, err
	}
	plan, err := s.inspectFoundryDeletion(ctx)
	if err != nil {
		return FoundryDeletionPlan{}, s.deleteFailure(ctx, "foundry.InspectFoundryDeletion", err)
	}
	return plan, nil
}

func (s *Service) inspectFoundryDeletion(ctx context.Context) (FoundryDeletionPlan, error) {
	file, err := s.file()
	if err != nil {
		return FoundryDeletionPlan{}, err
	}
	_, selected, err := s.selectedFoundry(file)
	if err != nil {
		return FoundryDeletionPlan{}, err
	}
	source, err := s.deletionSource()
	if err != nil {
		return FoundryDeletionPlan{}, err
	}
	only, err := source.ResourceGroupHoldsOnlyFoundry(ctx, selected)
	if err != nil {
		return FoundryDeletionPlan{}, err
	}
	if !only {
		return FoundryDeletionPlan{}, fault.New("FOUNDRY_DELETE_UNSUPPORTED", "The resource group also contains resources that are not related to Foundry. Deleting only the Foundry resources is not supported yet.")
	}
	return FoundryDeletionPlan{FoundryName: selected.Name, ResourceGroupName: selected.ResourceGroupName, DeleteResourceGroup: true}, nil
}

// DeleteFoundry deletes the selected Foundry's resource group, purges the Foundry
// and replaces the saved list and selection. The plan is checked again because
// the resource group may have changed since the confirmation.
func (s *Service) DeleteFoundry(ctx context.Context) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	view, err := s.deleteFoundry(ctx)
	if err != nil {
		return InitialFoundryView{}, s.deleteFailure(ctx, "foundry.DeleteFoundry", err)
	}
	return view, nil
}

func (s *Service) deleteFoundry(ctx context.Context) (InitialFoundryView, error) {
	file, err := s.file()
	if err != nil {
		return InitialFoundryView{}, err
	}
	plan, err := s.inspectFoundryDeletion(ctx)
	if err != nil {
		return InitialFoundryView{}, err
	}
	view, selected, err := s.selectedFoundry(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	source, err := s.deletionSource()
	if err != nil {
		return InitialFoundryView{}, err
	}
	progress := FoundryDeleteProgress{
		FoundryName: plan.FoundryName, ResourceGroupName: plan.ResourceGroupName,
		FoundryPhase: "running", PurgePhase: "waiting", ResourceGroupPhase: "waiting", ViewPhase: "waiting",
	}
	s.emit(FoundryDeleteProgressEvent, progress)
	if err := source.DeleteFoundry(ctx, selected); err != nil {
		return InitialFoundryView{}, err
	}
	progress.FoundryPhase, progress.PurgePhase = "completed", "running"
	s.emit(FoundryDeleteProgressEvent, progress)
	if err := source.PurgeFoundry(ctx, selected); err != nil {
		return InitialFoundryView{}, err
	}
	progress.PurgePhase = "completed"
	if plan.DeleteResourceGroup {
		progress.ResourceGroupPhase = "running"
		s.emit(FoundryDeleteProgressEvent, progress)
		if err := source.DeleteResourceGroup(ctx, selected); err != nil {
			return InitialFoundryView{}, err
		}
		progress.ResourceGroupPhase = "completed"
	}
	progress.ViewPhase = "running"
	s.emit(FoundryDeleteProgressEvent, progress)
	inGroup := func(foundry Foundry) bool {
		return foundry.ID == selected.ID || (plan.DeleteResourceGroup && foundry.SubscriptionName == selected.SubscriptionName && foundry.ResourceGroupName == selected.ResourceGroupName)
	}
	view.Foundries = slices.DeleteFunc(view.Foundries, inGroup)
	s.clearDeployments()
	if len(view.Foundries) == 0 {
		view.SelectedFoundryID = ""
		view.Deployments, view.DeploymentsFetchedAt = []Deployment{}, ""
		if err := save(file, view); err != nil {
			return InitialFoundryView{}, err
		}
	} else if view, err = s.acquireModels(ctx, file, view, view.Foundries[0]); err != nil {
		return InitialFoundryView{}, err
	}
	s.limits = limitsCache{}
	progress.ViewPhase = "completed"
	s.emit(FoundryDeleteProgressEvent, progress)
	return view, nil
}
