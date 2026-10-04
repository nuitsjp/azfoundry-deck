import { GetCapacityMaximum } from '@bindings/azfoundrydeck/internal/foundry/service';

export async function getCapacityMaximum(deploymentID: string): Promise<number | null> {
  return GetCapacityMaximum(deploymentID);
}
