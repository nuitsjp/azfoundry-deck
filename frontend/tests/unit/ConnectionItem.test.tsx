import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MantineProvider } from '@mantine/core';
import { ConnectionItem } from '../../src/usecases/initial-deployments/FoundryConnection';
import { reportFrontendError } from '../../src/features/application/queries';

vi.mock('@wailsio/runtime', () => ({ Events: { On: vi.fn() } }));
vi.mock('../../src/features/foundry/connection', () => ({ getConnectionState: vi.fn() }));
vi.mock('../../src/features/application/queries', () => ({ reportFrontendError: vi.fn() }));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('copies the full value, keeps the displayed key masked, and restores the button after two seconds', async () => {
  vi.useFakeTimers();
  const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue();
  vi.stubGlobal('navigator', { clipboard: { writeText } });
  render(
    <MantineProvider env="test">
      <ConnectionItem name="API key" shown="••••••••••••test" value="secret-test" loading={false} />
    </MantineProvider>,
  );

  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: 'Copy API key' }));
  });
  expect(writeText).toHaveBeenCalledWith('secret-test');
  expect(screen.getByRole('button', { name: 'Copied' })).toBeEnabled();
  expect(screen.getByText('••••••••••••test')).toBeVisible();
  expect(screen.queryByText('secret-test')).not.toBeInTheDocument();

  act(() => {
    vi.advanceTimersByTime(1999);
  });
  expect(screen.getByRole('button', { name: 'Copied' })).toBeEnabled();
  act(() => {
    vi.advanceTimersByTime(1);
  });
  expect(screen.getByRole('button', { name: 'Copy API key' })).toBeEnabled();
  expect(reportFrontendError).not.toHaveBeenCalled();
});

it('reports clipboard rejection without showing a successful copy', async () => {
  const failure = new Error('Clipboard access denied');
  const writeText = vi.fn<(value: string) => Promise<void>>().mockRejectedValue(failure);
  vi.stubGlobal('navigator', { clipboard: { writeText } });
  render(
    <MantineProvider env="test">
      <ConnectionItem name="API key" shown="••••••••••••test" value="secret-test" loading={false} />
    </MantineProvider>,
  );

  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: 'Copy API key' }));
  });
  expect(reportFrontendError).toHaveBeenCalledExactlyOnceWith(failure);
  expect(screen.getByRole('button', { name: 'Copy API key' })).toBeEnabled();
  expect(screen.queryByRole('button', { name: 'Copied' })).not.toBeInTheDocument();
  expect(screen.queryByText('secret-test')).not.toBeInTheDocument();
});
