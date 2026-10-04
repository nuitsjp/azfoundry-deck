import {
  CreateFoundry,
  GetFoundryRegions,
  GetFoundrySubscriptions,
} from '@bindings/azfoundrydeck/internal/foundry/service';
import type {
  FoundryCreateProgress,
  FoundryCreateSpec,
  FoundryRegion,
  FoundrySubscription,
} from '@bindings/azfoundrydeck/internal/foundry/models';
import { Events } from '@wailsio/runtime';
import type { InitialFoundryView } from './models';

export type { FoundryCreateProgress, FoundryCreateSpec };

export async function getFoundrySubscriptions(): Promise<FoundrySubscription[]> {
  return (await GetFoundrySubscriptions()) as FoundrySubscription[];
}

export async function getFoundryRegions(subscriptionID: string): Promise<FoundryRegion[]> {
  return (await GetFoundryRegions(subscriptionID)) as FoundryRegion[];
}

export async function createFoundry(
  spec: FoundryCreateSpec,
  report: (progress: FoundryCreateProgress) => void,
): Promise<InitialFoundryView> {
  const unsubscribe = Events.On('foundry:create-progress', (event) =>
    report(event.data as FoundryCreateProgress),
  );
  try {
    return (await CreateFoundry(spec)) as InitialFoundryView;
  } finally {
    unsubscribe();
  }
}
