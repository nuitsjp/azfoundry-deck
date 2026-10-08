import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
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
// The e2e fixed source returns these for Production and Development.
const fetchedModels = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const developmentModels = [
  ['development-chat', 'gpt-4.1', '2025-04-14'],
  ['development-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['development-embedding', 'text-embedding-3-large', '1'],
];
const savedAt = '2001-02-03T13:05:06+09:00';
// The state file holds only the Foundry list, the selection and the list's fetch time.
const original = {
  foundries,
  selectedFoundryId: production.id,
  foundriesFetchedAt: savedAt,
};
const label = (foundry: typeof production) =>
  `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`;
// The browser runs on the same machine, so local time matches.
const displayed = (value: string) => {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
};

// Model acquisition waits for its release file, including the one at startup; discovery is never released.
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
  const release = (stage: string) =>
    writeFileSync(join(app.dataDir, `e2e-foundry-${stage}-release`), '');
  const hold = (stage: string) => rmSync(join(app.dataDir, `e2e-foundry-${stage}-release`));
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
      await expect(modelRows.nth(index).locator('td:nth-child(-n + 3)')).toHaveText(model);
    }
  };
  const readState = () => JSON.parse(readFileSync(stateFile, 'utf8'));

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
    mkdirSync(viewDir, { recursive: true });
    writeFileSync(stateFile, JSON.stringify(original));
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    // Startup fetches the selected Foundry's deployments from Azure; release that held stage,
    // then hold it again so the refresh is observable.
    release('models');
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(/^3 · Last fetched /);
    hold('models');
    await refresh.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Refresh models');
    snapshots.length = 0;
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
    await assertModels(fetchedModels);
    expect(readState()).toEqual(original);
  });

  await test.step('手順2', async () => {
    release('models');
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
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(/^3 · Last fetched \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
    await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
  });

  await test.step('受け入れ条件', async () => {
    for (const snapshot of snapshots) {
      expect(snapshot.foundryPhase).toBe('completed');
      expect(snapshot.selectedFoundryName).toBe(production.name);
    }
    // Nothing but the Foundry list, the selection and the list's fetch time is saved.
    expect(readState()).toEqual(original);
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);

    // Refreshing the models leaves the subscription's cost as it is.
    const costValue = page.getByLabel('This month', { exact: true });
    const costLoading = page.getByRole('status', { name: 'This month loading' });
    release('cost');
    await expect(costValue).toHaveText('¥12,346');
    hold('cost');
    await refresh.click();
    await expect(dialog).toHaveCount(0);
    await expect(costLoading).toHaveCount(0);
    await expect(costValue).toHaveText('¥12,346');
    release('cost');

    // A restart does not keep the refreshed list: it is fetched from Azure again.
    release('models');
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(modelsFetched).toHaveText(/^3 · Last fetched /);

    // Switching away and back fetches each Foundry's deployments from Azure again.
    await selected.click();
    await page.getByRole('option', { name: label(development), exact: true }).click();
    await expect(selected).toHaveText(label(development));
    await assertModels(developmentModels);
    await selected.click();
    await page.getByRole('option', { name: label(production), exact: true }).click();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
  });
});
