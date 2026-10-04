package foundry

import (
	"context"
	"fmt"
	"strings"

	"azfoundrydeck/internal/fault"
)

type FoundrySubscription struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type FoundryRegion struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type FoundryCreateSpec struct {
	SubscriptionID    string `json:"subscriptionId"`
	ResourceGroupName string `json:"resourceGroupName"`
	FoundryName       string `json:"foundryName"`
	Region            string `json:"region"`
}

const FoundryCreateProgressEvent = "foundry:create-progress"

type FoundryCreateProgress struct {
	ResourceGroupName  string `json:"resourceGroupName"`
	FoundryName        string `json:"foundryName"`
	ResourceGroupPhase string `json:"resourceGroupPhase"`
	FoundryPhase       string `json:"foundryPhase"`
	ViewPhase          string `json:"viewPhase"`
}

// FoundryCreateSource is the ARM boundary shared by the creation UI and service.
type FoundryCreateSource interface {
	FoundrySubscriptions(context.Context) ([]FoundrySubscription, error)
	FoundryRegions(context.Context, string) ([]FoundryRegion, error)
	CreateResourceGroup(context.Context, FoundryCreateSpec) error
	CreateFoundry(context.Context, FoundryCreateSpec) (Foundry, error)
}

func (s *Service) creationSource() (FoundryCreateSource, error) {
	source, err := s.source()
	if err != nil {
		return nil, err
	}
	creation, ok := source.(FoundryCreateSource)
	if !ok {
		return nil, fmt.Errorf("Foundry creation is not connected to Azure yet")
	}
	return creation, nil
}

func (s *Service) GetFoundrySubscriptions(ctx context.Context) ([]FoundrySubscription, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return nil, err
	}
	source, err := s.creationSource()
	if err == nil {
		var subscriptions []FoundrySubscription
		subscriptions, err = source.FoundrySubscriptions(ctx)
		if err == nil {
			return subscriptions, nil
		}
	}
	s.logger.Error("operation_failed", "operation", "foundry.GetFoundrySubscriptions", "cause", err)
	return nil, fault.New("FOUNDRY_SETTINGS_FAILED", "Could not load subscriptions for Foundry creation.")
}

func (s *Service) GetFoundryRegions(ctx context.Context, subscriptionID string) ([]FoundryRegion, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return nil, err
	}
	source, err := s.creationSource()
	if err == nil {
		var regions []FoundryRegion
		regions, err = source.FoundryRegions(ctx, subscriptionID)
		if err == nil {
			return regions, nil
		}
	}
	s.logger.Error("operation_failed", "operation", "foundry.GetFoundryRegions", "cause", err)
	return nil, fault.New("FOUNDRY_SETTINGS_FAILED", "Could not load regions for Foundry creation.")
}

func (s *Service) CreateFoundry(ctx context.Context, spec FoundryCreateSpec) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.createFoundry(ctx, file, spec)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.CreateFoundry", "cause", err)
		return InitialFoundryView{}, fault.New("FOUNDRY_CREATE_FAILED", "Could not create the resource group and Foundry or update Home. Resources already created are not automatically deleted.")
	}
	return view, nil
}

func (s *Service) createFoundry(ctx context.Context, file string, spec FoundryCreateSpec) (InitialFoundryView, error) {
	if strings.TrimSpace(spec.SubscriptionID) == "" || strings.TrimSpace(spec.ResourceGroupName) == "" || strings.TrimSpace(spec.FoundryName) == "" || strings.TrimSpace(spec.Region) == "" {
		return InitialFoundryView{}, fmt.Errorf("subscription, resource group name, Foundry name and region are required")
	}
	view, err := s.readView(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	source, err := s.creationSource()
	if err != nil {
		return InitialFoundryView{}, err
	}
	progress := FoundryCreateProgress{
		ResourceGroupName: spec.ResourceGroupName, FoundryName: spec.FoundryName,
		ResourceGroupPhase: "running", FoundryPhase: "waiting", ViewPhase: "waiting",
	}
	s.emit(FoundryCreateProgressEvent, progress)
	if err := source.CreateResourceGroup(ctx, spec); err != nil {
		return InitialFoundryView{}, err
	}
	progress.ResourceGroupPhase, progress.FoundryPhase = "completed", "running"
	s.emit(FoundryCreateProgressEvent, progress)
	created, err := source.CreateFoundry(ctx, spec)
	if err != nil {
		return InitialFoundryView{}, err
	}
	progress.FoundryPhase, progress.ViewPhase = "completed", "running"
	s.emit(FoundryCreateProgressEvent, progress)
	// The ARM creation result is authoritative; do not wait for Resource Graph indexing.
	view.Foundries = append(view.Foundries, created)
	sortFoundries(view.Foundries)
	s.clearDeployments()
	view, err = s.acquireModels(ctx, file, view, created)
	if err != nil {
		return InitialFoundryView{}, err
	}
	progress.ViewPhase = "completed"
	s.emit(FoundryCreateProgressEvent, progress)
	return view, nil
}
