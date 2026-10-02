import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

const production = {
  id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
  name: 'contoso-foundry-production-japaneast',
  subscriptionName: 'Contoso AI Production Subscription',
  resourceGroupName: 'rg-ai-production-japaneast',
};
const development = {
  id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development',
  name: 'contoso-foundry-development',
  subscriptionName: 'Contoso Development',
  resourceGroupName: 'rg-ai-development',
};
const research = {
  id: '/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research',
  name: 'contoso-foundry-research',
  subscriptionName: 'Contoso Research',
  resourceGroupName: 'rg-ai-research',
};
// Saved only: the e2e fixed source no longer returns it.
const legacy = {
  id: '/subscriptions/review-legacy/resourceGroups/rg-ai-legacy/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-legacy',
  name: 'contoso-foundry-legacy',
  subscriptionName: 'Contoso Legacy',
  resourceGroupName: 'rg-ai-legacy',
};
const savedFoundries = [production, development, legacy];
const refreshedFoundries = [production, development, research];
const savedModels = [
  ['saved-production-chat', 'saved-production-model', 'saved-version-1'],
  ['saved-production-embedding', 'saved-embedding-model', 'saved-version-2'],
];
const legacyModels = [['legacy-chat', 'legacy-model', 'legacy-version']];
const fetchedModels = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const deployments = (foundry: { id: string }, models: string[][]) =>
  models.map(([deploymentName, modelName, version]) => ({
    id: `${foundry.id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  }));
const savedAt = '2001-02-03T13:05:06+09:00';
const original: InitialFoundryView = {
  foundries: savedFoundries,
  selectedFoundryId: production.id,
  deployments: deployments(production, savedModels),
  foundriesFetchedAt: savedAt,
  deploymentsFetchedAt: savedAt,
};
const label = (foundry: typeof production) =>
  `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`;
// The browser runs on the same machine, so local time matches.
const displayed = (value: string) => {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
};

// Every source stage waits for its release file, so each progress state is observable.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('Foundry一覧を更新する', async ({ page, app }) => {
  const stateFile = join(app.dataDir, 'foundry-state.json');
  const modelFile = (foundry: { id: string }) =>
    join(
      app.dataDir,
      'foundry-models',
      `${createHash('sha256').update(foundry.id).digest('hex')}.json`,
    );
  const savedModelFile = (models: string[][], foundry: { id: string }) =>
    JSON.stringify({ fetchedAt: savedAt, deployments: deployments(foundry, models) }, null, 2) +
    '\n';
  const productionText = savedModelFile(savedModels, production);
  const release = (stage: string) =>
    writeFileSync(join(app.dataDir, `e2e-foundry-${stage}-release`), '');
  const prepare = (view: InitialFoundryView) => {
    writeFileSync(stateFile, JSON.stringify(view));
    mkdirSync(join(app.dataDir, 'foundry-models'), { recursive: true });
    writeFileSync(modelFile(production), productionText);
    writeFileSync(modelFile(legacy), savedModelFile(legacyModels, legacy));
  };
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const refresh = page.getByRole('button', { name: 'Foundry一覧を更新' });
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'Foundry一覧を更新しています' });
  const subscriptionRows = dialog.locator(
    'table[aria-label="サブスクリプションの取得状況"] tbody tr',
  );
  const saveStatus = dialog.getByText('ファイルへの保存', { exact: true }).locator('..');
  const foundriesFetched = page.getByText(/^Foundry一覧の最終取得 /);
  const modelsFetched = page.getByText(/件・最終取得 /);
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
  };
  const readState = () => JSON.parse(readFileSync(stateFile, 'utf8')) as InitialFoundryView;

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
      }),
    );
    prepare(original);
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
    await expect(foundriesFetched).toHaveText(`Foundry一覧の最終取得 ${displayed(savedAt)}`);
    await expect(modelsFetched).toHaveText(`2 件・最終取得 ${displayed(savedAt)}`);
    await refresh.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Foundry一覧を更新');
    expect(snapshots).toEqual([]);
  });

  await test.step('手順1', async () => {
    await refresh.click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('サブスクリプションを検索中', { exact: true })).toBeVisible();
    await expect(dialog.getByText('発見 0 件', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    await expect(selected).toBeDisabled();
    await expect(refresh).toBeDisabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
    expect(readState()).toEqual(original);
  });

  await test.step('手順2', async () => {
    release('discovery');
    await expect(subscriptionRows).toHaveCount(3);
    await expect(subscriptionRows.nth(0)).toContainText(production.subscriptionName);
    await expect(subscriptionRows.nth(0)).toContainText('待機中');
    release('start');
    await expect(dialog.getByText('サブスクリプションの検索完了', { exact: true })).toBeVisible();
    await expect(subscriptionRows.filter({ hasText: 'Foundry取得中' })).toHaveCount(3);
    release('second');
    await expect(subscriptionRows).toHaveCount(2);
    await expect(subscriptionRows.filter({ hasText: development.subscriptionName })).toHaveCount(0);
    await expect(dialog.getByText('完了 1 / 発見 3 件', { exact: true })).toBeVisible();
    await expect(dialog.getByText('デプロイモデルの取得', { exact: true })).toHaveCount(0);
    await expect(saveStatus).toContainText('待機中');
    expect(readState()).toEqual(original);
  });

  await test.step('手順3', async () => {
    release('remaining');
    await expect(dialog).toHaveCount(0);
    const saving = snapshots.findIndex((snapshot) => snapshot.savePhase === 'running');
    const saved = snapshots.findIndex((snapshot) => snapshot.savePhase === 'completed');
    expect(saving).toBeGreaterThan(0);
    expect(saved).toBeGreaterThan(saving);
    const state = readState();
    expect(state).toEqual({
      ...original,
      foundries: refreshedFoundries,
      foundriesFetchedAt: expect.any(String),
    });
    expect(state.foundriesFetchedAt).not.toBe(savedAt);
    await expect(foundriesFetched).toHaveText(
      `Foundry一覧の最終取得 ${displayed(state.foundriesFetchedAt)}`,
    );
    await expect(modelsFetched).toHaveText(`2 件・最終取得 ${displayed(savedAt)}`);
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
  });

  await test.step('手順4', async () => {
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(refreshedFoundries.map(label));
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) await expect(option).toBeVisible();
    await page.keyboard.press('Escape');
  });

  await test.step('受け入れ条件', async () => {
    // Kept selection: no model acquisition, its model file untouched, removed Foundry's file deleted.
    for (const snapshot of snapshots) {
      expect(snapshot.selectedFoundryName).toBe('');
      expect(snapshot.modelPhase).toBe('waiting');
    }
    expect(readFileSync(modelFile(production), 'utf8')).toBe(productionText);
    expect(existsSync(modelFile(legacy))).toBe(false);
    const refreshed = readState();
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
    await expect(foundriesFetched).toHaveText(
      `Foundry一覧の最終取得 ${displayed(refreshed.foundriesFetchedAt)}`,
    );

    // Removed selection: select the first Foundry and fetch its models.
    prepare({
      ...original,
      selectedFoundryId: legacy.id,
      deployments: deployments(legacy, legacyModels),
    });
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(legacy));
    snapshots.length = 0;
    await refresh.click();
    await expect(dialog.getByText('デプロイモデルの取得', { exact: true })).toBeVisible();
    await expect(dialog.getByText(production.name, { exact: true })).toBeVisible();
    await expect(dialog.getByText('取得したモデル 0 件', { exact: true })).toBeVisible();
    await expect(selected).toHaveText(label(legacy));
    await assertModels(legacyModels);
    release('models');
    await expect(dialog).toHaveCount(0);
    expect(snapshots.at(-1)).toMatchObject({
      selectedFoundryName: production.name,
      modelPhase: 'completed',
      modelCount: 3,
      savePhase: 'completed',
    });
    const state = readState();
    expect(state).toEqual({
      foundries: refreshedFoundries,
      selectedFoundryId: production.id,
      deployments: deployments(production, fetchedModels),
      foundriesFetchedAt: expect.any(String),
      deploymentsFetchedAt: expect.any(String),
    });
    expect(state.deploymentsFetchedAt).not.toBe(savedAt);
    expect(JSON.parse(readFileSync(modelFile(production), 'utf8'))).toEqual({
      fetchedAt: state.deploymentsFetchedAt,
      deployments: state.deployments,
    });
    expect(existsSync(modelFile(legacy))).toBe(false);
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(
      `3 件・最終取得 ${displayed(state.deploymentsFetchedAt)}`,
    );
  });
});
