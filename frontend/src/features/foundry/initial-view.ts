import { GetInitialView } from '@bindings/azfoundrydeck/internal/foundry/service';
import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export async function loadInitialView(
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  if (import.meta.env.VITE_FOUNDRY_PROGRESS_REVIEW === '1') {
    const review = await import('./progress-review');
    await review.showReviewProgress(report);
    const view = (await GetInitialView()) as InitialFoundryView;
    await review.showReviewSaved(report);
    return view;
  }
  return GetInitialView() as Promise<InitialFoundryView>;
}
