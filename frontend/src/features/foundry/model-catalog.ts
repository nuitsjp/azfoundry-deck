import { GetModelCatalog } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { ModelCatalogItem } from './models';

export async function getModelCatalog(): Promise<ModelCatalogItem[]> {
  const result = await GetModelCatalog();
  return result ?? [];
}
