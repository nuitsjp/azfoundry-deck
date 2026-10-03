import { UpdateDeployment } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { DeploymentUpdateSpec, InitialFoundryView } from './models';

export async function updateDeployment(spec: DeploymentUpdateSpec): Promise<InitialFoundryView> {
  return (await UpdateDeployment(spec)) as InitialFoundryView;
}
