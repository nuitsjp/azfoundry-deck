import type {
  Foundry,
  Deployment,
  InitialFoundryView as BoundInitialView,
} from '@bindings/azfoundrydeck/internal/foundry/models';

export type { Foundry, Deployment };

// A successful Go response always initializes both slices, including empty deployments.
export type InitialFoundryView = Omit<BoundInitialView, 'foundries' | 'deployments'> & {
  foundries: Foundry[];
  deployments: Deployment[];
};
