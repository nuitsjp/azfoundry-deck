import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
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
// The e2e fixed source returns these for Production; they are fetched at startup and held in memory.
const fetchedModels = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const savedAt = '2001-02-03T13:05:06+09:00';
// The state file holds only the Foundry list, the selection and the list's fetch time.
const original = {
  foundries: savedFoundries,
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

// Every source stage waits for its release file, so each progress state is observable.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('Foundry一覧を更新する', async ({ page, app }) => {
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
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'Refreshing Foundries' });
  const foundryStep = dialog.getByText('Foundries', { exact: true }).locator('..');
  const modelStep = dialog.getByText('Deployments', { exact: true }).locator('..');
  const foundriesFetched = page.getByText(/^Last fetched /);
  const modelsFetched = page.getByText(/ · Last fetched /);
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
  };
  const readState = () => JSON.parse(readFileSync(stateFile, 'utf8'));
  let modelsFetchedText = '';

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
    // Startup fetches the selected Foundry's deployments from Azure; release that held stage.
    release('models');
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
    await expect(modelsFetched).toHaveText(/^3 · Last fetched \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
    modelsFetchedText = (await modelsFetched.textContent()) ?? '';
    await refresh.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Refresh Foundries');
    snapshots.length = 0;
  });

  await test.step('手順1', async () => {
    await refresh.click();
    await expect(dialog).toBeVisible();
    await expect(foundryStep).toContainText('Loading');
    await expect(modelStep).toContainText('Waiting');
    await expect(dialog.getByText('ファイルへの保存')).toHaveCount(0);
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    await expect(selected).toBeDisabled();
    await expect(refresh).toBeDisabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
    expect(readState()).toEqual(original);
  });

  await test.step('手順2', async () => {
    release('discovery');
    // The selected Foundry stays listed, so nothing holds the dialog after the list: it closes at once.
    await expect.poll(() => snapshots.some((s) => s.foundryPhase === 'completed')).toBe(true);
    expect(snapshots.at(-1)).toMatchObject({ foundryCount: refreshedFoundries.length });
  });

  await test.step('手順3', async () => {
    await expect(dialog).toHaveCount(0);
    const state = readState();
    expect(state).toEqual({
      ...original,
      foundries: refreshedFoundries,
      foundriesFetchedAt: expect.any(String),
    });
    expect(state.foundriesFetchedAt).not.toBe(savedAt);
    await expect(foundriesFetched).toHaveText(
      `Last fetched ${displayed(state.foundriesFetchedAt)}`,
    );
    await expect(modelsFetched).toHaveText(modelsFetchedText);
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(label(production));
    await assertModels(fetchedModels);
  });

  await test.step('手順4', async () => {
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(refreshedFoundries.map(label));
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) await expect(option).toBeVisible();
    await page.keyboard.press('Escape');
  });

  await test.step('受け入れ条件', async () => {
    // Kept selection: no model acquisition, and no deployments or model files are saved.
    for (const snapshot of snapshots) {
      expect(snapshot.selectedFoundryName).toBe('');
      expect(snapshot.modelPhase).toBe('waiting');
    }
    expect(snapshots.at(-1)).toEqual({
      foundryPhase: 'completed',
      foundryCount: refreshedFoundries.length,
      selectedFoundryName: '',
      modelPhase: 'waiting',
      modelCount: 0,
    });
    expect(Object.keys(readState()).sort()).toEqual([
      'foundries',
      'foundriesFetchedAt',
      'selectedFoundryId',
    ]);
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
    const refreshed = readState();
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    // The deployments are fetched again at startup; the Foundry list is restored.
    await assertModels(fetchedModels);
    await expect(foundriesFetched).toHaveText(
      `Last fetched ${displayed(refreshed.foundriesFetchedAt)}`,
    );
    await expect(modelsFetched).toHaveText(/^3 · Last fetched /);
    expect(readState()).toEqual(refreshed);
  });
});
