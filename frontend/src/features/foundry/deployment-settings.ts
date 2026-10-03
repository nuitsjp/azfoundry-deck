import { GetDeploymentSettings } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { DeploymentSettings } from './models';

export async function getDeploymentSettings(deploymentID: string): Promise<DeploymentSettings> {
  return GetDeploymentSettings(deploymentID);
}
