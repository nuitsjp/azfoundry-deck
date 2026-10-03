import { createHash } from 'node:crypto';
import { existsSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

// The fixed source finds no Foundry; the list is held until released.
// The first progress event is lost if it is sent before the page's WebSocket is registered, so the
// startup restore is held until the page has opened it, and discovery then starts after that.
test.use({
  serverEnv: {
    AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1',
    AZFOUNDRYDECK_E2E_HOLD_RESTORE: '1',
    AZFOUNDRYDECK_E2E_FOUNDRIES: 'none',
  },
});

test('Foundryが存在しない状態で初回閲覧する', async ({ page, app }) => {
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const savedFile = join(viewDir, 'foundry-state.json');
  const dialog = page.getByRole('dialog', { name: 'デプロイモデルを取得しています' });
  const foundryStep = dialog.getByText('Foundry一覧の取得', { exact: true }).locator('..');
  const modelStep = dialog.getByText('デプロイモデルの取得', { exact: true }).locator('..');
  const foundry = page.locator('button[aria-label="Foundry"]');
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const updateFoundries = page.getByRole('button', { name: 'Foundry一覧を更新' });
  const updateModels = page.getByRole('button', { name: 'モデルを更新' });
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const assertEmptyHome = async () => {
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(foundry).toBeVisible();
    await expect(foundry).toHaveText('');
    await foundry.click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await expect(modelRows).toHaveCount(0);
    await expect(page.getByText('0 件', { exact: true })).toBeVisible();
    await expect(updateFoundries).toBeEnabled();
    await expect(updateModels).toBeDisabled();
    await expect(
      page.getByText(/Foundry一覧の最終取得 \d{4}-\d{2}-\d{2} \d{2}:\d{2}/),
    ).toBeVisible();
  };

  await test.step('分岐条件', async () => {
    writeFileSync(
      join(app.dataDir, 'e2e-authentication-record.json'),
      JSON.stringify({
        authority: 'login.microsoftonline.com',
        clientId: 'e2e-client',
        homeAccountId: 'e2e-object.e2e-tenant',
        tenantId: 'e2e-tenant',
        username: 'operator@contoso.onmicrosoft.com',
        version: '1.0',
        tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
        selectedTenantId: 'e2e-azure-tenant',
      }),
    );
    await app.restart();
    expect(existsSync(savedFile)).toBe(false);
  });

  await test.step('手順1', async () => {
    const socket = page.waitForEvent('websocket');
    await page.goto(app.url);
    await socket;
    writeFileSync(join(app.dataDir, 'e2e-restore-release'), '');
    await expect(dialog).toBeVisible();
    await expect(foundryStep).toContainText('取得中');
    await expect(modelStep).toContainText('待機中');
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    expect(existsSync(savedFile)).toBe(false);
  });

  await test.step('手順2', async () => {
    writeFileSync(join(app.dataDir, 'e2e-foundry-discovery-release'), '');
    await expect.poll(() => snapshots.some((s) => s.foundryPhase === 'completed')).toBe(true);
    expect(snapshots.at(-1)).toMatchObject({ foundryCount: 0, modelPhase: 'waiting' });
    // 選択先の Foundry がなく、モデルの取得は行わない。
    expect(snapshots.every((s) => s.selectedFoundryName === '' && s.modelCount === 0)).toBe(true);
    expect(snapshots.some((s) => s.modelPhase === 'running')).toBe(false);
  });

  await test.step('手順3', async () => {
    await expect(dialog).toHaveCount(0);
    await assertEmptyHome();
  });

  await test.step('受け入れ条件', async () => {
    // 0件の結果を、空の一覧と取得日時だけで保存する。モデルファイルは作らない。
    const saved = JSON.parse(readFileSync(savedFile, 'utf8'));
    expect(saved).toEqual({
      foundries: [],
      selectedFoundryId: '',
      deployments: [],
      foundriesFetchedAt: expect.any(String),
      deploymentsFetchedAt: '',
    });
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
    // 再読み込み・再起動後は、保存済みの状態から同じ表示にし、取得も再保存もしない。
    const text = readFileSync(savedFile, 'utf8');
    const modified = statSync(savedFile).mtimeMs;
    const count = snapshots.length;
    await page.reload();
    await assertEmptyHome();
    await app.restart();
    await page.goto(app.url);
    await assertEmptyHome();
    expect(snapshots.length).toBe(count);
    expect(readFileSync(savedFile, 'utf8')).toBe(text);
    expect(statSync(savedFile).mtimeMs).toBe(modified);
  });
});
