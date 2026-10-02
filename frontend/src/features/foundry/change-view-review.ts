import type { Deployment, InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

const deploymentsByFoundry: Record<string, Omit<Deployment, 'id'>[]> = {
  '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development':
    [
      { deploymentName: 'development-chat', modelName: 'gpt-4.1', version: '2025-04-14' },
      { deploymentName: 'development-mini', modelName: 'gpt-4.1-mini', version: '2025-04-14' },
      {
        deploymentName: 'development-embedding',
        modelName: 'text-embedding-3-large',
        version: '1',
      },
    ],
  '/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research':
    [{ deploymentName: 'research-chat', modelName: 'gpt-4.1', version: '2025-04-14' }],
};

export async function changeFoundryReview(
  view: InitialFoundryView,
  id: string,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  const foundry = view.foundries.find((item) => item.id === id)!;
  const deployments = deploymentsByFoundry[id].map((deployment) => ({
    ...deployment,
    id: `${id}/deployments/${deployment.deploymentName}`,
  }));
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
  publish({ modelCount: 1 });
  if (deployments.length > 1) {
    await next();
    publish({ modelCount: deployments.length });
  }
  await next();
  publish({ modelPhase: 'completed', savePhase: 'running' });
  await next();
  publish({ savePhase: 'completed' });
  await next();
  return { ...view, selectedFoundryId: id, deployments };
}
