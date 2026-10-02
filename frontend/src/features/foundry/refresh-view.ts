import { RefreshFoundries } from '@bindings/azfoundrydeck/internal/foundry/service';
import { Events } from '@wailsio/runtime';
import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export async function refreshFoundries(
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  const unsubscribe = Events.On('foundry:progress', (event) =>
    report(event.data as FoundryProgress),
  );
  try {
    return (await RefreshFoundries()) as InitialFoundryView;
  } finally {
    unsubscribe();
  }
}
