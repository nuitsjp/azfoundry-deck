import type { Foundry, InitialFoundryView } from './models';
import type { FoundryProgress } from './progress';

// After a refresh, Research has disappeared and Staging has been added.
const foundries: Foundry[] = [
  {
    id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    name: 'contoso-foundry-production-japaneast',
    subscriptionName: 'Contoso AI Production Subscription',
    resourceGroupName: 'rg-ai-production-japaneast',
  },
  {
    id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development',
    name: 'contoso-foundry-development',
    subscriptionName: 'Contoso Development',
    resourceGroupName: 'rg-ai-development',
  },
  {
    id: '/subscriptions/review-staging/resourceGroups/rg-ai-staging/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-staging',
    name: 'contoso-foundry-staging',
    subscriptionName: 'Contoso Staging',
    resourceGroupName: 'rg-ai-staging',
  },
];

const firstDeployments = [
  { deploymentName: 'chat-production', modelName: 'gpt-4.1', version: '2025-04-14' },
  { deploymentName: 'chat-mini', modelName: 'gpt-4.1-mini', version: '2025-04-14' },
  { deploymentName: 'embeddings', modelName: 'text-embedding-3-large', version: '1' },
].map((deployment) => ({
  ...deployment,
  id: `${foundries[0].id}/deployments/${deployment.deploymentName}`,
}));

export async function refreshFoundriesReview(
  view: InitialFoundryView,
  report: (progress: FoundryProgress) => void,
): Promise<InitialFoundryView> {
  const subscriptions = foundries.map((foundry) => ({
    id: foundry.id.split('/')[2],
    name: foundry.subscriptionName,
    phase: 'waiting',
    foundryCount: 0,
  }));
  let progress: FoundryProgress = {
    subscriptionSearch: 'searching',
    subscriptions: [],
    selectedFoundryName: '',
    modelPhase: 'waiting',
    modelCount: 0,
    savePhase: 'waiting',
  };
  const publish = (update: Partial<FoundryProgress>) => {
    progress = { ...progress, ...update };
    report(progress);
  };
  const step = (index: number, phase: string, foundryCount = 0) => {
    subscriptions[index] = { ...subscriptions[index], phase, foundryCount };
    publish({ subscriptions: subscriptions.slice() });
  };
  const next = () => new Promise<void>((resolve) => setTimeout(resolve, 1200));
  publish({});
  await next();
  publish({ subscriptions: subscriptions.slice(0, 2) });
  await next();
  publish({ subscriptionSearch: 'completed', subscriptions: subscriptions.slice() });
  step(0, 'running');
  step(1, 'running');
  await next();
  step(0, 'completed', 1);
  step(2, 'running');
  await next();
  step(1, 'completed', 1);
  await next();
  step(2, 'completed', 1);
  const now = new Date().toISOString();
  const kept = foundries.some((foundry) => foundry.id === view.selectedFoundryId);
  if (!kept) {
    await next();
    publish({ selectedFoundryName: foundries[0].name, modelPhase: 'running' });
    await next();
    publish({ modelPhase: 'completed', modelCount: firstDeployments.length });
  }
  await next();
  publish({ savePhase: 'running' });
  await next();
  publish({ savePhase: 'completed' });
  await next();
  return {
    foundries,
    selectedFoundryId: kept ? view.selectedFoundryId : foundries[0].id,
    deployments: kept ? view.deployments : firstDeployments,
    foundriesFetchedAt: now,
    deploymentsFetchedAt: kept ? view.deploymentsFetchedAt : now,
  };
}
