import { GetDeploymentDetail } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { DeploymentDetail } from './models';

export async function getDeploymentDetail(deploymentID: string): Promise<DeploymentDetail> {
  return GetDeploymentDetail(deploymentID);
}
