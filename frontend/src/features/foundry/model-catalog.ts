import { GetModelCatalog } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { ModelCatalogItem } from './models';

export interface ModelCatalogView {
  models: ModelCatalogItem[];
  quotaStatus: 'loading' | 'ready' | 'error';
  quotaError?: string;
}

export async function getModelCatalog(foundryID: string): Promise<ModelCatalogView> {
  if (import.meta.env.MODE === 'catalog-review') {
    const review = await import('./model-catalog.review');
    return review.getModelCatalog(foundryID);
  }
  const result = await GetModelCatalog();
  return { models: result ?? [], quotaStatus: 'ready' };
}
