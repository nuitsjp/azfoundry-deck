import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, statSync, utimesSync, writeFileSync } from 'node:fs';
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
const foundries = [production, development];
const savedModels = [
  ['saved-production-chat', 'saved-production-model', 'saved-version-1'],
  ['saved-production-embedding', 'saved-embedding-model', 'saved-version-2'],
];
const developmentModels = [
  ['saved-development-chat', 'saved-development-model', 'saved-version-3'],
];
// The e2e fixed source returns these for Production.
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
  foundries,
  selectedFoundryId: production.id,
  deployments: deployments(production, savedModels),
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

// Model acquisition waits for its release file; discovery is never released.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('デプロイモデルを更新する', async ({ page, app }) => {
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
  const savedModelFile = (foundry: { id: string }, models: string[][]) =>
    JSON.stringify({ fetchedAt: savedAt, deployments: deployments(foundry, models) }, null, 2) +
    '\n';
  const fileIdentity = (path: string) => ({
    text: readFileSync(path, 'utf8'),
    mtimeMs: statSync(path).mtimeMs,
  });
  let developmentFile: ReturnType<typeof fileIdentity>;
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const refreshFoundries = page.getByRole('button', { name: 'Refresh Foundries' });
  const refresh = page.getByRole('button', { name: 'Refresh models' });
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'Refreshing models' });
  const modelStatus = dialog.getByText('Deployments', { exact: true }).locator('..');
  const foundriesFetched = page.getByText(/^Last fetched /);
  const modelsFetched = page.getByText(/ · Last fetched /);
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
  };
  const readState = () => JSON.parse(readFileSync(stateFile, 'utf8')) as InitialFoundryView;

  await test.step('開始条件', async () => {
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
    writeFileSync(modelFile(production), savedModelFile(production, savedModels));
    writeFileSync(modelFile(development), savedModelFile(development, developmentModels));
    const oldTime = new Date('2001-02-03T04:05:06Z');
    utimesSync(modelFile(development), oldTime, oldTime);
    developmentFile = fileIdentity(modelFile(development));
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
    await expect(modelsFetched).toHaveText(`2 · Last fetched ${displayed(savedAt)}`);
    await refresh.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Refresh models');
    expect(snapshots).toEqual([]);
  });

  await test.step('手順1', async () => {
    await refresh.click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(production.name, { exact: true })).toBeVisible();
    await expect(modelStatus).toContainText('Loading');
    await expect(dialog.getByText('0 models', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('table')).toHaveCount(0);
    await expect(dialog.getByText(/Foundries|ファイルへの保存|サブスクリプション/)).toHaveCount(0);
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    await expect(selected).toBeDisabled();
    await expect(refreshFoundries).toBeDisabled();
    await expect(refresh).toBeDisabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(savedModels);
    expect(readState()).toEqual(original);
  });

  await test.step('手順2', async () => {
    writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
    await expect(dialog).toHaveCount(0);
    const counted = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'running' && snapshot.modelCount === 3,
    );
    const completed = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'completed' && snapshot.modelCount === 3,
    );
    expect(counted).toBeGreaterThan(0);
    expect(completed).toBeGreaterThan(counted);
  });

  await test.step('手順3', async () => {
    const state = readState();
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(
      `3 · Last fetched ${displayed(state.deploymentsFetchedAt)}`,
    );
    await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
  });

  await test.step('受け入れ条件', async () => {
    for (const snapshot of snapshots) {
      expect(snapshot.foundryPhase).toBe('completed');
      expect(snapshot.selectedFoundryName).toBe(production.name);
    }
    const state = readState();
    expect(state).toEqual({
      ...original,
      deployments: deployments(production, fetchedModels),
      deploymentsFetchedAt: expect.any(String),
    });
    expect(state.deploymentsFetchedAt).not.toBe(savedAt);
    expect(JSON.parse(readFileSync(modelFile(production), 'utf8'))).toEqual({
      fetchedAt: state.deploymentsFetchedAt,
      deployments: state.deployments,
    });
    expect(fileIdentity(modelFile(development))).toEqual(developmentFile);

    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(
      `3 · Last fetched ${displayed(state.deploymentsFetchedAt)}`,
    );

    // Switching away and back restores the refreshed models from their file.
    await selected.click();
    await page.getByRole('option', { name: label(development), exact: true }).click();
    await expect(selected).toHaveText(label(development));
    await assertModels(developmentModels);
    await selected.click();
    await page.getByRole('option', { name: label(production), exact: true }).click();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(
      `3 · Last fetched ${displayed(state.deploymentsFetchedAt)}`,
    );
  });
});
