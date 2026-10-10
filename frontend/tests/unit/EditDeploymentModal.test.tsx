import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MantineProvider } from '@mantine/core';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EditDeploymentModal } from '../../src/usecases/initial-deployments/EditDeploymentModal';
import { getDeploymentSettings } from '../../src/features/foundry/deployment-settings';

vi.mock('../../src/features/foundry/deployment-settings', () => ({
  getDeploymentSettings: vi.fn(),
}));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.resetAllMocks();
});

it('moves an RPM capacity below 1,000 by the step the settings return', async () => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  vi.mocked(getDeploymentSettings).mockResolvedValue({
    deploymentId: 'decision',
    deploymentName: 'msft-decision-1',
    modelName: 'Microsoft-Decision-1',
    skuName: 'GlobalStandard',
    option: 'Standard',
    version: '1',
    versions: ['1'],
    capacity: 112,
    capacityMaximum: 150,
    capacityMinimum: 1,
    capacityStep: 1,
    capacityUnit: 'RPM',
    upgradePolicy: 'OnceNewDefaultVersionAvailable',
  });
  const onUpdate = vi.fn();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MantineProvider env="test">
        <EditDeploymentModal
          deploymentID="decision"
          busy={false}
          error={null}
          onClose={vi.fn()}
          onClearError={vi.fn()}
          onUpdate={onUpdate}
        />
      </MantineProvider>
    </QueryClientProvider>,
  );

  const slider = await screen.findByRole('slider');
  expect(slider).toHaveAttribute('aria-valuenow', '112');
  fireEvent.keyDown(slider, { key: 'ArrowRight' });
  expect(slider).toHaveAttribute('aria-valuenow', '113');
  expect(screen.getByRole('textbox', { name: 'Capacity' })).toHaveValue('113');

  fireEvent.change(screen.getByRole('textbox', { name: 'Capacity' }), { target: { value: '40' } });
  expect(slider).toHaveAttribute('aria-valuenow', '40');
  fireEvent.click(screen.getByRole('button', { name: 'Update' }));
  expect(onUpdate).toHaveBeenCalledWith(expect.objectContaining({ capacity: 40 }));
});
