import { DeleteDeployment } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { InitialFoundryView } from './models';

export async function deleteDeployment(id: string): Promise<InitialFoundryView> {
  return (await DeleteDeployment(id)) as InitialFoundryView;
}
