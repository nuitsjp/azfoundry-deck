import { cleanup } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { confirmQuit } from '../../src/features/application/queries';

const calls = vi.hoisted(() => ({
  confirm: vi.fn(),
  quit: vi.fn(),
}));
vi.mock('@wailsio/runtime', () => ({
  Application: { Quit: calls.quit },
  Events: { On: () => () => {} },
}));
vi.mock('@bindings/azfoundrydeck/internal/desktop/service', () => ({
  ConfirmQuit: calls.confirm,
}));

beforeEach(() => {
  vi.resetAllMocks();
  calls.quit.mockResolvedValue(undefined);
});
afterEach(cleanup);

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

it('waits for Go quit approval before requesting native quit', async () => {
  const approved = deferred();
  calls.confirm.mockReturnValue(approved.promise);
  const closing = confirmQuit();
  expect(calls.confirm).toHaveBeenCalledOnce();
  expect(calls.quit).not.toHaveBeenCalled();
  approved.resolve();
  await closing;
  expect(calls.quit).toHaveBeenCalledOnce();
});

it('keeps the application open when Go rejects quit', async () => {
  const failure = new Error('BUSY');
  calls.confirm.mockRejectedValue(failure);
  await expect(confirmQuit()).rejects.toBe(failure);
  expect(calls.quit).not.toHaveBeenCalled();
});
