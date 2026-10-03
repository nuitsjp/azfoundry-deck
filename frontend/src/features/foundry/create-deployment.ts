import { CreateDeployment } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { DeploymentCreateSpec, InitialFoundryView } from './models';

export async function createDeployment(spec: DeploymentCreateSpec): Promise<InitialFoundryView> {
  return (await CreateDeployment(spec)) as InitialFoundryView;
}
