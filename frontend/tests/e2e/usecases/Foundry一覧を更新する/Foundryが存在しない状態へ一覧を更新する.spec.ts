import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

const foundry = (key: string, name: string, subscriptionName: string, group: string) => ({
  id: `/subscriptions/review-${key}/resourceGroups/${group}/providers/Microsoft.CognitiveServices/accounts/${name}`,
  name,
  subscriptionName,
  resourceGroupName: group,
});
const production = foundry(
  'production',
  'contoso-foundry-production-japaneast',
  'Contoso AI Production Subscription',
  'rg-ai-production-japaneast',
);
const legacy = foundry('legacy', 'contoso-foundry-legacy', 'Contoso Legacy', 'rg-ai-legacy');
const savedAt = '2001-02-03T13:05:06+09:00';
const deployment = (owner: { id: string }, name: string) => ({
  id: `${owner.id}/deployments/${name}`,
  deploymentName: name,
  modelName: `${name}-model`,
  version: 'saved-version',
});
const original: InitialFoundryView = {
  foundries: [production, legacy],
  selectedFoundryId: production.id,
  deployments: [deployment(production, 'saved-production-chat')],
  foundriesFetchedAt: savedAt,
  deploymentsFetchedAt: savedAt,
};

// The fixed source finds one subscription without any Foundry; discovery waits for its release.
test.use({
  serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1', AZFOUNDRYDECK_E2E_FOUNDRIES: 'none' },
});

test('Foundryが存在しない状態へ一覧を更新する', async ({ page, app }) => {
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const stateFile = join(viewDir, 'foundry-state.json');
  const modelsDir = join(viewDir, 'foundry-models');
  const modelFile = (owner: { id: string }) =>
    join(modelsDir, `${createHash('sha256').update(owner.id).digest('hex')}.json`);
  const selected = page.locator('button[aria-label="Foundry"]');
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const updateFoundries = page.getByRole('button', { name: 'Foundry一覧を更新' });
  const updateModels = page.getByRole('button', { name: 'モデルを更新' });
  const dialog = page.getByRole('dialog', { name: 'Foundry一覧を更新しています' });
  const saveStatus = dialog.getByText('ファイルへの保存', { exact: true }).locator('..');
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
    await expect(selected).toHaveText('');
    await selected.click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await expect(modelRows).toHaveCount(0);
    await expect(page.getByText('0 件', { exact: true })).toBeVisible();
    await expect(updateFoundries).toBeEnabled();
    await expect(updateModels).toBeDisabled();
  };
  let before = '';

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
    mkdirSync(modelsDir, { recursive: true });
    writeFileSync(stateFile, JSON.stringify(original));
    for (const owner of [production, legacy]) {
      writeFileSync(
        modelFile(owner),
        JSON.stringify({ fetchedAt: savedAt, deployments: [deployment(owner, 'saved')] }, null, 2),
      );
    }
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toContainText(production.name);
    await expect(modelRows).toHaveCount(1);
    expect(snapshots).toEqual([]);
    before = readFileSync(stateFile, 'utf8');
  });

  await test.step('手順1', async () => {
    await updateFoundries.click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('サブスクリプションを検索中', { exact: true })).toBeVisible();
    await expect(saveStatus).toContainText('待機中');
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    // 保存が成功するまで、更新前の一覧・選択・モデルを維持する。
    await expect(selected).toContainText(production.name);
    expect(readFileSync(stateFile, 'utf8')).toBe(before);
    expect(existsSync(modelFile(production))).toBe(true);
    expect(existsSync(modelFile(legacy))).toBe(true);
  });

  await test.step('手順2', async () => {
    writeFileSync(join(app.dataDir, 'e2e-foundry-discovery-release'), '');
    await expect.poll(() => snapshots.some((s) => s.subscriptionSearch === 'completed')).toBe(true);
    // 選択先がなく、モデルの取得は行わない。
    expect(snapshots.every((s) => s.selectedFoundryName === '' && s.modelCount === 0)).toBe(true);
    expect(snapshots.some((s) => s.modelPhase === 'running')).toBe(false);
    await expect(dialog.getByText('デプロイモデルの取得', { exact: true })).toHaveCount(0);
  });

  await test.step('手順3', async () => {
    await expect(dialog).toHaveCount(0);
    expect(snapshots.some((s) => s.savePhase === 'running')).toBe(true);
    expect(snapshots.some((s) => s.savePhase === 'completed')).toBe(true);
    await assertEmptyHome();
  });

  await test.step('受け入れ条件', async () => {
    // 空の一覧・選択なし・空のモデル・一覧の取得日時だけを保存し、モデルファイルはすべて削除する。
    const saved = JSON.parse(readFileSync(stateFile, 'utf8'));
    expect(saved).toEqual({
      foundries: [],
      selectedFoundryId: '',
      deployments: [],
      foundriesFetchedAt: expect.any(String),
      deploymentsFetchedAt: '',
    });
    expect(saved.foundriesFetchedAt).not.toBe(savedAt);
    expect(readdirSync(modelsDir)).toEqual([]);
    // 再読み込み・再起動後は、同じ表示にし、取得も再保存もしない。
    const text = readFileSync(stateFile, 'utf8');
    const modified = statSync(stateFile).mtimeMs;
    const count = snapshots.length;
    await page.reload();
    await assertEmptyHome();
    await app.restart();
    await page.goto(app.url);
    await assertEmptyHome();
    expect(snapshots.length).toBe(count);
    expect(readFileSync(stateFile, 'utf8')).toBe(text);
    expect(statSync(stateFile).mtimeMs).toBe(modified);
  });
});
