import type { FoundryProgress, AcquisitionPhase } from './progress';
import { initialProgress } from './progress';

const names = [
  'Contoso AI Production Subscription',
  'Contoso Development',
  'Contoso Research',
  ...Array.from(
    { length: 9 },
    (_, index) => `Contoso Sandbox ${String(index + 1).padStart(2, '0')}`,
  ),
];
const selectedFoundryName = 'contoso-foundry-production-japaneast';

// Fixed snapshots for UI agreement; these do not implement a work queue.
const snapshots: { delay: number; progress: FoundryProgress }[] = [
  {
    delay: 1200,
    progress: {
      ...initialProgress,
      subscriptions: rows(['running', 'running', 'running', 'running']),
    },
  },
  {
    delay: 1200,
    progress: {
      ...initialProgress,
      subscriptions: rows(Array<AcquisitionPhase>(8).fill('running')),
      selectedFoundryName,
      modelPhase: 'running',
    },
  },
  {
    delay: 1200,
    progress: {
      ...initialProgress,
      subscriptionSearch: 'completed',
      subscriptions: rows([
        ...Array<AcquisitionPhase>(8).fill('running'),
        ...Array<AcquisitionPhase>(4).fill('waiting'),
      ]),
      selectedFoundryName,
      modelPhase: 'completed',
      modelCount: 3,
    },
  },
  {
    delay: 1600,
    progress: {
      ...initialProgress,
      subscriptionSearch: 'completed',
      subscriptions: rows([
        'running',
        'running',
        'completed',
        'running',
        'running',
        'completed',
        'running',
        'running',
        'running',
        'running',
        'waiting',
        'waiting',
      ]),
      selectedFoundryName,
      modelPhase: 'completed',
      modelCount: 3,
    },
  },
  {
    delay: 1600,
    progress: {
      ...initialProgress,
      subscriptionSearch: 'completed',
      subscriptions: rows([
        'completed',
        'completed',
        'completed',
        'completed',
        'running',
        'completed',
        'running',
        'running',
        'running',
        'running',
        'running',
        'running',
      ]),
      selectedFoundryName,
      modelPhase: 'completed',
      modelCount: 3,
    },
  },
  {
    delay: 1600,
    progress: {
      ...initialProgress,
      subscriptionSearch: 'completed',
      subscriptions: rows([
        ...Array<AcquisitionPhase>(8).fill('completed'),
        ...Array<AcquisitionPhase>(4).fill('running'),
      ]),
      selectedFoundryName,
      modelPhase: 'completed',
      modelCount: 3,
    },
  },
  {
    delay: 1600,
    progress: {
      ...initialProgress,
      subscriptionSearch: 'completed',
      subscriptions: rows(Array<AcquisitionPhase>(12).fill('completed')),
      selectedFoundryName,
      modelPhase: 'completed',
      modelCount: 3,
      savePhase: 'running',
    },
  },
];

function rows(phases: AcquisitionPhase[]) {
  return phases.map((phase, index) => ({
    id: `review-subscription-${index + 1}`,
    name: names[index],
    phase,
    foundryCount: index < 3 && phase === 'completed' ? 1 : 0,
  }));
}

export async function showReviewProgress(report: (progress: FoundryProgress) => void) {
  report(initialProgress);
  for (const snapshot of snapshots) {
    await new Promise((resolve) => setTimeout(resolve, snapshot.delay));
    report({
      ...snapshot.progress,
      subscriptions: snapshot.progress.subscriptions.map((subscription, index) =>
        index === 0 && snapshot.progress.selectedFoundryName
          ? { ...subscription, foundryCount: 1 }
          : subscription,
      ),
    });
  }
  await new Promise((resolve) => setTimeout(resolve, 1200));
}

export async function showReviewSaved(report: (progress: FoundryProgress) => void) {
  report({ ...snapshots[snapshots.length - 1].progress, savePhase: 'completed' });
  await new Promise((resolve) => setTimeout(resolve, 700));
}
