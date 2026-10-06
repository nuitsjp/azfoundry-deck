import { Events } from '@wailsio/runtime';
import type { ModelCatalogItem } from './models';
import type { ModelCatalogView } from './model-catalog';
import type { FoundryProgress } from './progress';

// Specification review only. Production builds do not import this provider.
const standard: ModelCatalogItem = {
  name: 'gpt-4o',
  publisher: 'OpenAI',
  option: 'Standard',
  tasks: ['Chat', 'Multimodal'],
  sub: '128k context · GlobalStandard',
  maxCapacity: 160000,
  versions: ['2024-11-20', '2024-08-06'],
  skus: [
    { name: 'GlobalStandard', maxCapacity: 160000 },
    { name: 'DataZoneStandard', maxCapacity: 66000 },
  ],
  inputRate: null,
  outputRate: null,
};
const payAsYouGo: ModelCatalogItem = {
  name: 'claude-3-5-sonnet',
  publisher: 'Anthropic',
  option: 'Pay-as-you-go',
  tasks: ['Chat', 'Reasoning'],
  sub: '200k context · Serverless API',
  maxCapacity: null,
  versions: ['20241022'],
  skus: [{ name: 'GlobalProvisioned', maxCapacity: null }],
  inputRate: '$3.00 / 1M',
  outputRate: '$15.00 / 1M',
};
const firstCatalog = [standard, payAsYouGo];
const secondCatalog = [
  {
    ...standard,
    name: 'gpt-4o-mini',
    maxCapacity: 100000,
    skus: [{ name: 'GlobalStandard', maxCapacity: 100000 }],
  },
];
const refreshedCatalog = [
  {
    ...standard,
    maxCapacity: 40000,
    skus: [
      { name: 'GlobalStandard', maxCapacity: 40000 },
      { name: 'DataZoneStandard', maxCapacity: 66000 },
    ],
  },
  payAsYouGo,
];
const started = new Map<string, number>();
const refreshStarted = new Map<string, number>();
const modelFetches = new Map<string, Promise<void>>();

Events.On('foundry:progress', (event) => {
  const progress = event.data as FoundryProgress;
  if (
    new URLSearchParams(window.location.search).get('quota') !== 'refresh' ||
    progress.modelPhase !== 'completed' ||
    progress.modelCount !== 4
  )
    return;
  for (const foundryID of started.keys()) {
    if (
      !foundryID.endsWith('/contoso-foundry-production-japaneast') ||
      !foundryID.endsWith(`/${progress.selectedFoundryName}`) ||
      refreshStarted.has(foundryID)
    )
      continue;
    refreshStarted.set(foundryID, Date.now());
    setTimeout(() => {
      void Events.Emit('foundry:capacity-ready', foundryID);
    }, 4000);
  }
});

export async function getModelCatalog(foundryID: string): Promise<ModelCatalogView> {
  const params = new URLSearchParams(window.location.search);
  let start = started.get(foundryID);
  if (start === undefined) {
    start = Date.now();
    started.set(foundryID, start);
    if (params.get('quota') === 'loading') {
      setTimeout(() => {
        void Events.Emit('foundry:capacity-ready', foundryID);
      }, 4000);
    }
    if (params.get('models') === 'loading') {
      modelFetches.set(foundryID, new Promise((resolve) => setTimeout(resolve, 2000)));
    }
  }
  await modelFetches.get(foundryID);
  const elapsed = Date.now() - start;
  const refreshedAt = refreshStarted.get(foundryID);
  const models = foundryID.endsWith('contoso-foundry-development') ? secondCatalog : firstCatalog;
  if (params.get('quota') === 'error') {
    return { models, quotaStatus: 'error', quotaError: 'Could not load the shared quota.' };
  }
  if (
    (params.get('quota') === 'loading' && elapsed < 4000) ||
    (refreshedAt !== undefined && Date.now() - refreshedAt < 4000)
  ) {
    return { models, quotaStatus: 'loading' };
  }
  return { models: refreshedAt !== undefined ? refreshedCatalog : models, quotaStatus: 'ready' };
}
