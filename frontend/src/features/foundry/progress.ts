import type {
  Progress,
  SubscriptionProgress,
} from '@bindings/azfoundrydeck/internal/foundry/models';

export type FoundryProgress = Omit<Progress, 'subscriptions'> & {
  subscriptions: SubscriptionProgress[];
};
export type AcquisitionPhase = Progress['modelPhase'];

export const initialProgress: FoundryProgress = {
  subscriptionSearch: 'searching',
  subscriptions: [],
  selectedFoundryName: '',
  modelPhase: 'waiting',
  modelCount: 0,
  savePhase: 'waiting',
};
