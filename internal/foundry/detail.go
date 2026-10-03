package foundry

import (
	"context"
	"fmt"
	"slices"

	"azfoundrydeck/internal/fault"
)

type DeploymentDetail struct {
	ID                   string  `json:"id"`
	DeploymentName       string  `json:"deploymentName"`
	ModelName            string  `json:"modelName"`
	Version              string  `json:"version"`
	SKUName              *string `json:"skuName"`
	Capacity             *int32  `json:"capacity"`
	CapacityMaximum      *int32  `json:"capacityMaximum"`
	CapacityUnit         *string `json:"capacityUnit"`
	ProvisioningState    *string `json:"provisioningState"`
	VersionUpgradePolicy *string `json:"versionUpgradePolicy"`
	FetchedAt            string  `json:"fetchedAt"`
}

type DeploymentDetailSource interface {
	DeploymentDetail(context.Context, Foundry, Deployment) (DeploymentDetail, error)
}

func (s *Service) GetDeploymentDetail(ctx context.Context, deploymentID string) (DeploymentDetail, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return DeploymentDetail{}, err
	}
	file, err := s.file()
	var detail DeploymentDetail
	if err == nil {
		detail, err = s.deploymentDetail(ctx, file, deploymentID)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetDeploymentDetail", "cause", err)
		if ctx.Err() != nil {
			return DeploymentDetail{}, fault.Public(ctx.Err())
		}
		return DeploymentDetail{}, fault.New("DEPLOYMENT_DETAIL_FAILED", "Could not retrieve deployment details from the source or read the current Foundry selection.")
	}
	detail.FetchedAt = fetchedNow()
	return detail, nil
}

func (s *Service) deploymentDetail(ctx context.Context, file, deploymentID string) (DeploymentDetail, error) {
	view, err := read(file)
	if err != nil {
		return DeploymentDetail{}, err
	}
	foundryIndex := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool {
		return foundry.ID == view.SelectedFoundryID
	})
	if foundryIndex < 0 {
		return DeploymentDetail{}, fmt.Errorf("selected Foundry is not in the saved list")
	}
	deploymentIndex := slices.IndexFunc(view.Deployments, func(deployment Deployment) bool {
		return deployment.ID == deploymentID
	})
	if deploymentIndex < 0 {
		return DeploymentDetail{}, fmt.Errorf("selected deployment is not in the selected Foundry's saved list")
	}
	source, err := s.source()
	if err != nil {
		return DeploymentDetail{}, err
	}
	detailSource, ok := source.(DeploymentDetailSource)
	if !ok {
		return DeploymentDetail{}, fmt.Errorf("deployment detail retrieval is not connected to Azure yet")
	}
	return detailSource.DeploymentDetail(ctx, view.Foundries[foundryIndex], view.Deployments[deploymentIndex])
}
