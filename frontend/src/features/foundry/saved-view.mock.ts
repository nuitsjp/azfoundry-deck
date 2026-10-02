import type { InitialFoundryView } from './models';

// Fixed saved-state response for UI agreement; removed when connected to file storage.
export const savedView: InitialFoundryView = {
  foundries: [
    {
      id: 'production',
      name: 'contoso-foundry-production-japaneast',
      subscriptionName: 'Contoso AI Production Subscription',
      resourceGroupName: 'rg-ai-production-japaneast',
    },
    {
      id: 'development',
      name: 'contoso-foundry-development',
      subscriptionName: 'Contoso Development',
      resourceGroupName: 'rg-ai-development',
    },
    {
      id: 'research',
      name: 'contoso-foundry-research',
      subscriptionName: 'Contoso Research',
      resourceGroupName: 'rg-ai-research',
    },
  ],
  selectedFoundryId: 'development',
  deployments: [
    { id: 'chat', deploymentName: 'development-chat', modelName: 'gpt-4.1', version: '2025-04-14' },
    {
      id: 'mini',
      deploymentName: 'development-mini',
      modelName: 'gpt-4.1-mini',
      version: '2025-04-14',
    },
    {
      id: 'embedding',
      deploymentName: 'development-embedding',
      modelName: 'text-embedding-3-small',
      version: '1',
    },
  ],
};
