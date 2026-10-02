export type AcquisitionPhase = 'waiting' | 'running' | 'completed';

export interface SubscriptionProgress {
  id: string;
  name: string;
  phase: AcquisitionPhase;
  foundryCount: number;
}

export interface FoundryProgress {
  subscriptionSearch: 'searching' | 'completed';
  subscriptions: SubscriptionProgress[];
  selectedFoundryName: string;
  modelPhase: AcquisitionPhase;
  modelCount: number;
  savePhase: AcquisitionPhase;
}

export const initialProgress: FoundryProgress = {
  subscriptionSearch: 'searching',
  subscriptions: [],
  selectedFoundryName: '',
  modelPhase: 'waiting',
  modelCount: 0,
  savePhase: 'waiting',
};
