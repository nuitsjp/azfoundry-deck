import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { MantineProvider } from '@mantine/core';
import { AddDeploymentModal } from '../../src/usecases/initial-deployments/AddDeploymentModal';
import { getModelCatalog, type ModelCatalogView } from '../../src/features/foundry/model-catalog';

vi.mock('@wailsio/runtime', () => ({ Events: { On: vi.fn(() => vi.fn()) } }));
vi.mock('../../src/features/foundry/model-catalog', () => ({ getModelCatalog: vi.fn() }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.resetAllMocks();
});

const catalog = (foundryId: string, name: string): ModelCatalogView => ({
  foundryId,
  quotaStatus: 'ready',
  quotaError: null,
  models: [
    {
      name,
      publisher: 'Test publisher',
      option: 'Standard',
      tasks: ['Chat'],
      sub: 'Test model',
      maxCapacity: 80000,
      versions: ['1'],
      skus: [{ name: 'GlobalStandard', maxCapacity: 80000 }],
      inputRate: null,
      outputRate: null,
    },
  ],
});

it.each(['success', 'failure'] as const)(
  'ignores a previous Foundry request that completes with %s after the current catalog',
  async (outcome) => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    let resolvePrevious!: (value: ModelCatalogView) => void;
    let rejectPrevious!: (reason: Error) => void;
    const previousRequest = new Promise<ModelCatalogView>((resolve, reject) => {
      resolvePrevious = resolve;
      rejectPrevious = reject;
    });
    vi.mocked(getModelCatalog)
      .mockReturnValueOnce(previousRequest)
      .mockResolvedValueOnce(catalog('current-foundry', 'current-model'));
    const onClose = vi.fn();
    const onDeploy = vi.fn();
    const view = (foundryID: string) => (
      <MantineProvider env="test">
        <AddDeploymentModal foundryID={foundryID} opened onClose={onClose} onDeploy={onDeploy} />
      </MantineProvider>
    );
    const { rerender } = render(view('previous-foundry'));
    expect(screen.getByText('Loading model catalog...')).toBeVisible();

    await act(async () => {
      rerender(view('current-foundry'));
    });
    expect(screen.getByRole('textbox', { name: /Deployment name/ })).toHaveValue('current-model');

    await act(async () => {
      if (outcome === 'success') {
        resolvePrevious(catalog('previous-foundry', 'previous-model'));
      } else {
        rejectPrevious(new Error('Previous catalog failed'));
      }
    });
    expect(screen.getByRole('textbox', { name: /Deployment name/ })).toHaveValue('current-model');
    expect(screen.queryByText('previous-model')).not.toBeInTheDocument();
    expect(screen.queryByText('Previous catalog failed')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deploy' })).toBeEnabled();
  },
);
