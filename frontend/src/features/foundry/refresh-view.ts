import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export const foundryRefreshEnabled = import.meta.env.MODE === 'foundry-refresh-review';

export async function refreshFoundries(
  view: InitialFoundryView,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  if (foundryRefreshEnabled) {
    const { refreshFoundriesReview } = await import('./refresh-view-review');
    return refreshFoundriesReview(view, report);
  }
  throw new Error('Foundry一覧の更新は実処理に未接続です。');
}
