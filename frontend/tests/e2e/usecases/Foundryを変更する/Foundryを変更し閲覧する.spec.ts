import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

const foundries = [
  {
    id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    name: 'contoso-foundry-production-japaneast',
    subscriptionName: 'Contoso AI Production Subscription',
    resourceGroupName: 'rg-ai-production-japaneast',
  },
  {
    id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development',
    name: 'contoso-foundry-development',
    subscriptionName: 'Contoso Development',
    resourceGroupName: 'rg-ai-development',
  },
  {
    id: '/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research',
    name: 'contoso-foundry-research',
    subscriptionName: 'Contoso Research',
    resourceGroupName: 'rg-ai-research',
  },
];
const oldModels = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const newModels = [
  ['development-chat', 'gpt-4.1', '2025-04-14'],
  ['development-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['development-embedding', 'text-embedding-3-large', '1'],
];
const original = {
  foundries,
  selectedFoundryId: foundries[0].id,
  foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
};
const expected = { ...original, selectedFoundryId: foundries[1].id };
const labels = foundries.map(
  (foundry) => `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`,
);

// Discovery stays unreleased. The model acquisition finishes only while its release file exists.
test.use({
  serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1', AZFOUNDRYDECK_E2E_HOLD_RESTORE: '1' },
});

test('Foundryを変更し閲覧する', async ({ page, app }) => {
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const stateFile = join(viewDir, 'foundry-state.json');
  const modelsRelease = join(app.dataDir, 'e2e-foundry-models-release');
  const restoreRelease = join(app.dataDir, 'e2e-restore-release');
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'Loading deployments' });
  const modelStatus = dialog.getByText('Deployments', { exact: true }).locator('..');
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td:nth-child(-n + 3)')).toHaveText(model);
    }
  };

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
    // The start fetches the saved selection's deployments from Azure; hold them again afterwards.
    writeFileSync(modelsRelease, '');
    writeFileSync(restoreRelease, '');
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    rmSync(modelsRelease);
    snapshots.length = 0;
  });

  await test.step('手順1', async () => {
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) await expect(option).toBeVisible();
  });

  await test.step('手順2', async () => {
    await page.getByRole('option', { name: labels[1], exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(foundries[1].name, { exact: true })).toBeVisible();
    await expect(modelStatus).toContainText('Loading');
    await expect(dialog.getByText('0 models', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('table')).toHaveCount(0);
    await expect(dialog.getByText(/Foundries|ファイルへの保存|サブスクリプション/)).toHaveCount(0);
    await expect(selected).toBeDisabled();
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(original);
  });

  await test.step('手順3', async () => {
    writeFileSync(modelsRelease, '');
    await expect(dialog).toHaveCount(0);
    const completedModels = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'completed' && snapshot.modelCount === 3,
    );
    expect(completedModels).toBeGreaterThanOrEqual(0);
    expect(
      snapshots.some((snapshot) => snapshot.modelPhase === 'running' && snapshot.modelCount === 3),
    ).toBe(true);
  });

  await test.step('手順4', async () => {
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    // Only the selection changes; the Foundry list and its fetch time stay as saved.
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(expected);
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
  });

  await test.step('手順5', async () => {
    await page.setViewportSize({ width: 480, height: 720 });
    const label = selected.locator('.mantine-InputPlaceholder-placeholder');
    expect(
      await label.evaluate((element) => ({
        overflow: getComputedStyle(element).overflow,
        textOverflow: getComputedStyle(element).textOverflow,
        whiteSpace: getComputedStyle(element).whiteSpace,
        truncated: element.scrollWidth > element.clientWidth,
      })),
    ).toEqual({
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      truncated: true,
    });
    await label.hover();
    await expect(page.getByRole('tooltip')).toHaveText(labels[1]);
  });

  await test.step('受け入れ条件', async () => {
    for (const snapshot of snapshots) {
      expect(snapshot.foundryPhase).toBe('completed');
      expect(snapshot.selectedFoundryName).toBe(foundries[1].name);
    }
    expect(snapshots.at(-1)).toEqual({
      foundryPhase: 'completed',
      foundryCount: original.foundries.length,
      selectedFoundryName: foundries[1].name,
      modelPhase: 'completed',
      modelCount: 3,
    });
    // The next start keeps the changed selection and fetches its deployments from Azure again.
    // The startup restore is held until the page has opened its WebSocket, so no event is lost.
    rmSync(modelsRelease);
    rmSync(restoreRelease);
    await app.restart();
    const socket = page.waitForEvent('websocket');
    await page.goto(app.url);
    await socket;
    writeFileSync(restoreRelease, '');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(foundries[1].name, { exact: true })).toBeVisible();
    await expect(modelStatus).toContainText('Loading');
    writeFileSync(modelsRelease, '');
    await expect(dialog).toHaveCount(0);
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    // Selecting the current Foundry only closes the dropdown: no fetch, no progress, no save.
    const unchanged = {
      text: readFileSync(stateFile, 'utf8'),
      mtimeMs: statSync(stateFile).mtimeMs,
    };
    const eventCount = snapshots.length;
    await selected.click();
    await page.getByRole('option', { name: labels[1], exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await expect(dialog).toHaveCount(0);
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    expect(snapshots).toHaveLength(eventCount);
    expect({
      text: readFileSync(stateFile, 'utf8'),
      mtimeMs: statSync(stateFile).mtimeMs,
    }).toEqual(unchanged);
  });
});
