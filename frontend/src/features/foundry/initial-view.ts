import type { InitialFoundryView } from './models';

// UI composition point. Fixed data is enabled only in the screen review build.
export async function loadInitialView(): Promise<InitialFoundryView> {
  return {
    foundries: [
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
        id: '/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research',
        name: 'contoso-foundry-research',
        subscriptionName: 'Contoso Research',
        resourceGroupName: 'rg-ai-research',
      },
    ],
    selectedFoundryId:
      '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    deployments: [
      {
        id: 'review-chat',
        deploymentName: 'chat-production',
        modelName: 'gpt-4.1',
        version: '2025-04-14',
      },
      {
        id: 'review-mini',
        deploymentName: 'chat-mini',
        modelName: 'gpt-4.1-mini',
        version: '2025-04-14',
      },
      {
        id: 'review-embedding',
        deploymentName: 'embeddings',
        modelName: 'text-embedding-3-large',
        version: '1',
      },
    ],
  };
}
