import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';
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
  ['saved-production-chat', 'saved-production-model', 'saved-version-1'],
  ['saved-production-mini', 'saved-mini-model', 'saved-version-2'],
  ['saved-production-embedding', 'saved-embedding-model', 'saved-version-3'],
];
const newModels = [
  ['development-chat', 'gpt-4.1', '2025-04-14'],
  ['development-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['development-embedding', 'text-embedding-3-large', '1'],
];
const deployments = (index: number, models: string[][]) =>
  models.map(([deploymentName, modelName, version]) => ({
    id: `${foundries[index].id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  }));
const original: InitialFoundryView = {
  foundries,
  selectedFoundryId: foundries[0].id,
  deployments: deployments(0, oldModels),
};
const expected: InitialFoundryView = {
  foundries,
  selectedFoundryId: foundries[1].id,
  deployments: deployments(1, newModels),
};
const labels = foundries.map(
  (foundry) => `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`,
);

// Discovery stays unreleased. Only the requested model acquisition can finish.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('Foundryを変更し初回閲覧する', async ({ page, app }) => {
  const stateFile = join(app.dataDir, 'foundry-state.json');
  const cacheFile = (id: string) =>
    join(app.dataDir, 'foundry-models', `${createHash('sha256').update(id).digest('hex')}.json`);
  const oldFile = cacheFile(foundries[0].id);
  const targetFile = cacheFile(foundries[1].id);
  const oldText = JSON.stringify(original.deployments, null, 2) + '\n';
  const snapshots: FoundryProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const dialog = page.getByRole('dialog', { name: 'デプロイモデルを取得しています' });
  const modelStatus = dialog.getByText('デプロイモデルの取得', { exact: true }).locator('..');
  const saveStatus = dialog.getByText('ファイルへの保存', { exact: true }).locator('..');
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
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
      }),
    );
    writeFileSync(stateFile, JSON.stringify(original));
    mkdirSync(join(app.dataDir, 'foundry-models'));
    writeFileSync(oldFile, oldText);
    expect(existsSync(targetFile)).toBe(false);
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    expect(snapshots).toEqual([]);
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
    await expect(modelStatus).toContainText('取得中');
    await expect(saveStatus).toContainText('待機中');
    await expect(dialog.getByText('取得したモデル 0 件', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('table')).toHaveCount(0);
    await expect(dialog.getByText(/サブスクリプション|Foundry取得中/)).toHaveCount(0);
    await expect(selected).toBeDisabled();
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(original);
    expect(existsSync(targetFile)).toBe(false);
    expect(readFileSync(oldFile, 'utf8')).toBe(oldText);
  });

  await test.step('手順3', async () => {
    writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
    await expect(dialog).toHaveCount(0);
    const completedModels = snapshots.findIndex(
      (snapshot) => snapshot.modelPhase === 'completed' && snapshot.modelCount === 3,
    );
    const saving = snapshots.findIndex((snapshot) => snapshot.savePhase === 'running');
    const saved = snapshots.findIndex((snapshot) => snapshot.savePhase === 'completed');
    expect(completedModels).toBeGreaterThanOrEqual(0);
    expect(saving).toBeGreaterThan(completedModels);
    expect(saved).toBeGreaterThan(saving);
    expect(
      snapshots.some((snapshot) => snapshot.modelPhase === 'running' && snapshot.modelCount === 3),
    ).toBe(true);
  });

  await test.step('手順4', async () => {
    await expect(selected).toBeEnabled();
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(expected);
    expect(JSON.parse(readFileSync(targetFile, 'utf8'))).toEqual(expected.deployments);
    expect(readFileSync(oldFile, 'utf8')).toBe(oldText);
  });

  await test.step('手順5', async () => {
    await page.setViewportSize({ width: 640, height: 720 });
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
      expect(snapshot.subscriptionSearch).toBe('completed');
      expect(snapshot.subscriptions).toEqual([]);
      expect(snapshot.selectedFoundryName).toBe(foundries[1].name);
    }
    expect(snapshots.at(-1)).toMatchObject({
      modelPhase: 'completed',
      modelCount: 3,
      savePhase: 'completed',
    });
    const paths = [stateFile, oldFile, targetFile];
    const unchanged = paths.map((path) => ({
      text: readFileSync(path, 'utf8'),
      mtimeMs: statSync(path).mtimeMs,
    }));
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    const eventCount = snapshots.length;
    await selected.click();
    await page.getByRole('option', { name: labels[1], exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await expect(dialog).toHaveCount(0);
    await expect(selected).toHaveText(labels[1]);
    await assertModels(newModels);
    expect(snapshots).toHaveLength(eventCount);
    expect(
      paths.map((path) => ({ text: readFileSync(path, 'utf8'), mtimeMs: statSync(path).mtimeMs })),
    ).toEqual(unchanged);
    expect(JSON.parse(readFileSync(oldFile, 'utf8'))).toEqual(original.deployments);
  });
});
