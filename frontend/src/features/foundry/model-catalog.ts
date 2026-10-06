import { GetModelCatalog } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { ModelCatalogView } from '@bindings/azfoundrydeck/internal/foundry/models';

export type { ModelCatalogView };

export async function getModelCatalog(foundryID: string): Promise<ModelCatalogView> {
  const result = await GetModelCatalog();
  if (result.foundryId !== foundryID) {
    throw new Error('The selected Foundry changed while loading the model catalog.');
  }
  return result;
}
