import type { InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

// The refreshed models add one deployment to the current ones.
export async function refreshDeploymentsReview(
  view: InitialFoundryView,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  const foundry = view.foundries.find((item) => item.id === view.selectedFoundryId)!;
  const deployments = [
    ...view.deployments,
    {
      id: `${foundry.id}/deployments/added-chat`,
      deploymentName: 'added-chat',
      modelName: 'gpt-4.1-nano',
      version: '2025-04-14',
    },
  ];
  let progress: FoundryProgress = {
    subscriptionSearch: 'completed',
    subscriptions: [],
    selectedFoundryName: foundry.name,
    modelPhase: 'running',
    modelCount: 0,
    savePhase: 'waiting',
  };
  const publish = (update: Partial<FoundryProgress>) => {
    progress = { ...progress, ...update };
    report(progress);
  };
  const next = () => new Promise<void>((resolve) => setTimeout(resolve, 1200));
  publish({});
  await next();
  publish({ modelCount: Math.min(2, deployments.length) });
  await next();
  publish({ modelPhase: 'completed', modelCount: deployments.length });
  await next();
  publish({ savePhase: 'running' });
  await next();
  publish({ savePhase: 'completed' });
  await next();
  return { ...view, deployments, deploymentsFetchedAt: new Date().toISOString() };
}
