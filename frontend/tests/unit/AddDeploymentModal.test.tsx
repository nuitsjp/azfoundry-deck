import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
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
      skus: [
        {
          name: 'GlobalStandard',
          maxCapacity: 80000,
          minCapacity: 1000,
          capacityStep: 1000,
          capacityPerUnit: 1000,
          capacityUnit: 'TPM',
        },
      ],
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

it('offers an RPM capacity by the SKU step and deploys it in SKU capacity units', async () => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  const view = catalog('foundry', 'Microsoft-Decision-1');
  view.models![0].skus = [
    {
      name: 'GlobalStandard',
      maxCapacity: 38,
      minCapacity: 1,
      capacityStep: 1,
      capacityPerUnit: 1,
      capacityUnit: 'RPM',
    },
  ];
  vi.mocked(getModelCatalog).mockResolvedValue(view);
  const onDeploy = vi.fn();
  await act(async () => {
    render(
      <MantineProvider env="test">
        <AddDeploymentModal foundryID="foundry" opened onClose={vi.fn()} onDeploy={onDeploy} />
      </MantineProvider>,
    );
  });

  const slider = screen.getByRole('slider');
  expect(slider).toHaveAttribute('aria-valuemin', '1');
  expect(slider).toHaveAttribute('aria-valuemax', '38');
  expect(slider).toHaveAttribute('aria-valuenow', '19');
  expect(screen.getByText(/RPM \(Available\)/)).toBeVisible();
  fireEvent.keyDown(slider, { key: 'ArrowRight' });
  expect(screen.getByRole('textbox', { name: 'Capacity' })).toHaveValue('20');
  fireEvent.click(screen.getByRole('button', { name: 'Deploy' }));
  expect(onDeploy).toHaveBeenCalledWith(expect.objectContaining({ skuCapacity: 20 }));
});
