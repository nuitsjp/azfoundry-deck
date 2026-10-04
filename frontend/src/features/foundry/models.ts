import type {
  Foundry,
  Deployment,
  ModelCatalogItem,
  ModelSKUItem,
  DeploymentCreateSpec,
  DeploymentSettings,
  DeploymentUpdateSpec,
  InitialFoundryView as BoundInitialView,
} from '@bindings/azfoundrydeck/internal/foundry/models';

export type {
  Foundry,
  Deployment,
  ModelCatalogItem,
  ModelSKUItem,
  DeploymentCreateSpec,
  DeploymentSettings,
  DeploymentUpdateSpec,
};

// A successful Go response always initializes both slices, including empty deployments.
export type InitialFoundryView = Omit<BoundInitialView, 'foundries' | 'deployments'> & {
  foundries: Foundry[];
  deployments: Deployment[];
};
