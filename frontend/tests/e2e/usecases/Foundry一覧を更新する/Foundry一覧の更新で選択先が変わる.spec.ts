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
  selectedFoundryId: legacy.id,
  deployments: deployments(legacy, legacyModels),
  foundriesFetchedAt: savedAt,
  deploymentsFetchedAt: savedAt,
};
const label = (foundry: typeof production) =>
  `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`;
// The browser runs on the same machine, so local time matches.
const displayed = (value: string) => {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
};

// Every source stage waits for its release file, so each progress state is observable.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('Foundry一覧の更新で選択先が変わる', async ({ page, app }) => {
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const stateFile = join(viewDir, 'foundry-state.json');
  const modelFile = (foundry: { id: string }) =>
    join(
      viewDir,
      'foundry-models',
      `${createHash('sha256').update(foundry.id).digest('hex')}.json`,
    );
  const savedModelFile = (models: string[][], foundry: { id: string }) =>
    JSON.stringify({ fetchedAt: savedAt, deployments: deployments(foundry, models) }, null, 2) +
    '\n';
  const productionText = savedModelFile(savedModels, production);
  const release = (stage: string) =>
    writeFileSync(join(app.dataDir, `e2e-foundry-${stage}-release`), '');
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const refresh = page.getByRole('button', { name: 'Refresh Foundries' });
  const refreshModels = page.getByRole('button', { name: 'Refresh models' });
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'Refreshing Foundries' });
  const foundryStep = dialog.getByText('Foundries', { exact: true }).locator('..');
  const modelStep = dialog.getByText('Deployments', { exact: true }).locator('..');
  let dialogSize: { width: number; height: number } | null = null;
  const foundriesFetched = page.getByText(/^Last fetched /);
  const modelsFetched = page.getByText(/ · Last fetched /);
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
        tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
        selectedTenantId: 'e2e-azure-tenant',
      }),
    );
    mkdirSync(join(viewDir, 'foundry-models'), { recursive: true });
    writeFileSync(stateFile, JSON.stringify(original));
    writeFileSync(modelFile(production), productionText);
    writeFileSync(modelFile(legacy), savedModelFile(legacyModels, legacy));
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(label(legacy));
    await assertModels(legacyModels);
    await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
    await expect(modelsFetched).toHaveText(`1 · Last fetched ${displayed(savedAt)}`);
    await refresh.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Refresh Foundries');
    expect(snapshots).toEqual([]);
  });

  await test.step('手順1', async () => {
    await refresh.click();
    await expect(dialog).toBeVisible();
    await expect(foundryStep).toContainText('Loading');
    await expect(modelStep).toContainText('Waiting');
    await expect(dialog.getByText('ファイルへの保存')).toHaveCount(0);
    const box = await dialog.boundingBox();
    dialogSize = box && { width: box.width, height: box.height };
    expect(dialogSize).not.toBeNull();
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    await expect(selected).toBeDisabled();
    await expect(refresh).toBeDisabled();
    await expect(refreshModels).toBeDisabled();
    await expect(selected).toHaveText(label(legacy));
    await assertModels(legacyModels);
    expect(readState()).toEqual(original);
  });

  await test.step('手順2', async () => {
    release('discovery');
    await expect(foundryStep).toContainText('Completed');
    await expect(foundryStep).toContainText('3 Foundries');
    expect(readState()).toEqual(original);
  });

  await test.step('手順3', async () => {
    await expect(modelStep).toContainText('Loading');
    await expect(modelStep).not.toContainText('Waiting');
    await expect(dialog.getByText(production.name, { exact: true })).toBeVisible();
    await expect(dialog.getByText('0 models', { exact: true })).toBeVisible();
    const box = await dialog.boundingBox();
    expect(box && { width: box.width, height: box.height }).toEqual(dialogSize);
    await expect(selected).toHaveText(label(legacy));
    await assertModels(legacyModels);
    expect(readState()).toEqual(original);
    expect(readFileSync(modelFile(production), 'utf8')).toBe(productionText);
    expect(existsSync(modelFile(legacy))).toBe(true);
    release('models');
    await expect
      .poll(() => snapshots.some((snapshot) => snapshot.modelPhase === 'completed'))
      .toBe(true);
    const counted = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'running' && snapshot.modelCount === 3,
    );
    const completed = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'completed' && snapshot.modelCount === 3,
    );
    expect(counted).toBeGreaterThan(0);
    expect(completed).toBeGreaterThan(counted);
  });

  await test.step('手順4', async () => {
    await expect(dialog).toHaveCount(0);
    expect(snapshots.at(-1)).toEqual({
      foundryPhase: 'completed',
      foundryCount: refreshedFoundries.length,
      selectedFoundryName: production.name,
      modelPhase: 'completed',
      modelCount: 3,
    });
    const state = readState();
    expect(state).toEqual({
      foundries: refreshedFoundries,
      selectedFoundryId: production.id,
      deployments: deployments(production, fetchedModels),
      foundriesFetchedAt: expect.any(String),
      deploymentsFetchedAt: expect.any(String),
    });
    expect(state.foundriesFetchedAt).not.toBe(savedAt);
    expect(state.deploymentsFetchedAt).not.toBe(savedAt);
    expect(Date.parse(state.foundriesFetchedAt)).toBeLessThanOrEqual(
      Date.parse(state.deploymentsFetchedAt),
    );
    expect(JSON.parse(readFileSync(modelFile(production), 'utf8'))).toEqual({
      fetchedAt: state.deploymentsFetchedAt,
      deployments: state.deployments,
    });
    expect(existsSync(modelFile(legacy))).toBe(false);
    await expect(selected).toBeEnabled();
    await expect(refresh).toBeEnabled();
    await expect(refreshModels).toBeEnabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(foundriesFetched).toHaveText(
      `Last fetched ${displayed(state.foundriesFetchedAt)}`,
    );
    await expect(modelsFetched).toHaveText(
      `3 · Last fetched ${displayed(state.deploymentsFetchedAt)}`,
    );
  });

  await test.step('手順5', async () => {
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(refreshedFoundries.map(label));
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) await expect(option).toBeVisible();
    await page.keyboard.press('Escape');
  });

  await test.step('受け入れ条件', async () => {
    const modelProgress = snapshots.filter((snapshot) => snapshot.selectedFoundryName !== '');
    expect(modelProgress[0]).toMatchObject({
      selectedFoundryName: production.name,
      modelPhase: 'running',
      modelCount: 0,
    });
    for (const snapshot of modelProgress) {
      expect(snapshot.selectedFoundryName).toBe(production.name);
      expect(snapshot.modelPhase).not.toBe('waiting');
    }
    const refreshed = readState();
    snapshots.length = 0;
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(foundriesFetched).toHaveText(
      `Last fetched ${displayed(refreshed.foundriesFetchedAt)}`,
    );
    await expect(modelsFetched).toHaveText(
      `3 · Last fetched ${displayed(refreshed.deploymentsFetchedAt)}`,
    );
    expect(readState()).toEqual(refreshed);
    expect(snapshots).toEqual([]);
  });
});
