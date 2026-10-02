import { RefreshDeployments } from '@bindings/azfoundrydeck/internal/foundry/service';
import { Events } from '@wailsio/runtime';
import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export async function refreshDeployments(
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  const unsubscribe = Events.On('foundry:progress', (event) =>
    report(event.data as FoundryProgress),
  );
  try {
    return (await RefreshDeployments()) as InitialFoundryView;
  } finally {
    unsubscribe();
  }
}
