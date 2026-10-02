import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export const foundryChangeEnabled = import.meta.env.MODE === 'foundry-change-review';

export async function changeFoundry(
  view: InitialFoundryView,
  id: string,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  if (foundryChangeEnabled) {
    const { changeFoundryReview } = await import('./change-view-review');
    return changeFoundryReview(view, id, report);
  }
  throw new Error('Foundry変更は実処理に未接続です。');
}
