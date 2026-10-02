import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

export const deploymentRefreshEnabled = import.meta.env.MODE === 'deployment-refresh-review';

export async function refreshDeployments(
  view: InitialFoundryView,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  if (deploymentRefreshEnabled) {
    const { refreshDeploymentsReview } = await import('./refresh-deployments-review');
    return refreshDeploymentsReview(view, report);
  }
  throw new Error('デプロイモデルの更新は実処理に未接続です。');
}
