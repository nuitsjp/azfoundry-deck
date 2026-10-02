import { GetInitialView } from '@bindings/azfoundrydeck/internal/foundry/service';
import { Events } from '@wailsio/runtime';
import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export async function loadInitialView(
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  if (import.meta.env.MODE === 'foundry-review') {
    const { savedView } = await import('./saved-view.mock');
    return savedView;
  }
  const unsubscribe = Events.On('foundry:progress', (event) =>
    report(event.data as FoundryProgress),
  );
  try {
    return (await GetInitialView()) as InitialFoundryView;
  } finally {
    unsubscribe();
  }
}
