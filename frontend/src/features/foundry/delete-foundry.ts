import {
  DeleteFoundry,
  InspectFoundryDeletion,
} from '@bindings/azfoundrydeck/internal/foundry/service';
import type {
  FoundryDeleteProgress,
  FoundryDeletionPlan,
} from '@bindings/azfoundrydeck/internal/foundry/models';
import { Events } from '@wailsio/runtime';
import type { InitialFoundryView } from './models';

export type { FoundryDeleteProgress, FoundryDeletionPlan };

export async function inspectFoundryDeletion(): Promise<FoundryDeletionPlan> {
  return (await InspectFoundryDeletion()) as FoundryDeletionPlan;
}

export async function deleteFoundry(
  report: (progress: FoundryDeleteProgress) => void,
): Promise<InitialFoundryView> {
  const unsubscribe = Events.On('foundry:delete-progress', (event) =>
    report(event.data as FoundryDeleteProgress),
  );
  try {
    return (await DeleteFoundry()) as InitialFoundryView;
  } finally {
    unsubscribe();
  }
}
