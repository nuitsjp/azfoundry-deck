import { createHash } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

const subscriptions = [
  'Contoso AI Production Subscription',
  'Contoso Development',
  'Contoso Research',
];
const foundries = [
  {
    id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    name: 'contoso-foundry-production-japaneast',
    subscriptionName: subscriptions[0],
    resourceGroupName: 'rg-ai-production-japaneast',
  },
  {
    id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development',
    name: 'contoso-foundry-development',
    subscriptionName: subscriptions[1],
    resourceGroupName: 'rg-ai-development',
  },
  {
    id: '/subscriptions/review-research/resourceGroups/rg-ai-research/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-research',
    name: 'contoso-foundry-research',
    subscriptionName: subscriptions[2],
    resourceGroupName: 'rg-ai-research',
  },
];
const models = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const labels = foundries.map(
  (foundry) => `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`,
);

test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('デプロイモデルを初回閲覧する', async ({ page, app }) => {
  const release = (stage: string) =>
    writeFileSync(join(app.dataDir, `e2e-foundry-${stage}-release`), '');
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const savedFile = join(viewDir, 'foundry-state.json');
  const dialog = page.getByRole('dialog', { name: 'デプロイモデルを取得しています' });
  const rows = dialog
    .getByRole('table', { name: 'サブスクリプションの取得状況' })
    .locator('tbody tr');
  const modelStatus = dialog.getByText('デプロイモデルの取得', { exact: true }).locator('..');
  const saveStatus = dialog.getByText('ファイルへの保存', { exact: true }).locator('..');
  const frames: string[] = [];
  // Observe the real server event boundary, including save phases that may share a React render.
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => frames.push(payload.toString()));
  });

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
    await app.restart();
    expect(existsSync(savedFile)).toBe(false);
  });

  await test.step('手順1', async () => {
    await page.goto(app.url);
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('サブスクリプションを検索中', { exact: true })).toBeVisible();
    await expect(dialog.getByRole('status')).toHaveText(
      '完了 0 / 発見 0 件（検索中のため総数は未確定）',
    );
    await expect(rows).toHaveCount(0);
    await expect(modelStatus).toContainText('待機中');
    await expect(saveStatus).toContainText('待機中');
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    expect(existsSync(join(app.dataDir, 'e2e-signin-called'))).toBe(false);
  });

  await test.step('手順2', async () => {
    release('discovery');
    await expect(rows).toHaveCount(3);
    await expect(rows.locator('td:first-child')).toHaveText(subscriptions);
    await expect(rows.locator('td:nth-child(2)')).toHaveText(['待機中', '待機中', '待機中']);
    await expect(rows.locator('td:nth-child(3)')).toHaveText(['0 件', '0 件', '0 件']);
    await expect(dialog.getByRole('status')).toHaveText(
      '完了 0 / 発見 3 件（検索中のため総数は未確定）',
    );

    release('start');
    await expect(dialog.getByText('サブスクリプションの検索完了', { exact: true })).toBeVisible();
    await expect(rows.locator('td:nth-child(2)')).toHaveText([
      'Foundry取得中',
      'Foundry取得中',
      'Foundry取得中',
    ]);
    await expect(rows.first().locator('td:nth-child(3)')).toHaveText('1 件');
    await expect(
      dialog.getByText('待機中 0 件・Foundry取得中 3 件', { exact: true }),
    ).toBeVisible();
    await expect(dialog.getByText(foundries[0].name, { exact: true })).toBeVisible();
    await expect(modelStatus).toContainText('取得中');
    await expect(dialog.getByRole('status')).toHaveText('完了 0 / 発見 3 件');

    release('second');
    await expect(rows.locator('td:first-child')).toHaveText([subscriptions[0], subscriptions[2]]);
    await expect(dialog.getByRole('status')).toHaveText('完了 1 / 発見 3 件');
    await expect(dialog.getByText(subscriptions[1], { exact: true })).toHaveCount(0);
    release('models');
    await expect(modelStatus).toContainText('完了');
    await expect(dialog.getByText('取得したモデル 3 件', { exact: true })).toBeVisible();
    await expect(rows).toHaveCount(2);
    await expect(saveStatus).toContainText('待機中');
    expect(existsSync(savedFile)).toBe(false);
  });

  await test.step('手順3', async () => {
    release('remaining');
    await expect(dialog).toHaveCount(0);
    const savedView = JSON.parse(readFileSync(savedFile, 'utf8'));
    expect(savedView).toEqual({
      foundries,
      selectedFoundryId: foundries[0].id,
      deployments: models.map(([deploymentName, modelName, version]) => ({
        id: `${foundries[0].id}/deployments/${deploymentName}`,
        deploymentName,
        modelName,
        version,
      })),
    });
    const modelRows = page.getByRole('table', { name: 'デプロイ済みモデル' }).locator('tbody tr');
    await expect(modelRows).toHaveCount(3);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveText(labels[0]);
  });

  await test.step('手順4', async () => {
    await page.getByRole('button', { name: 'Foundry', exact: true }).click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) {
      await expect(option).toBeVisible();
    }
  });

  await test.step('手順5', async () => {
    await page.keyboard.press('Escape');
    await expect(page.getByRole('option')).toHaveCount(0);
    await page.setViewportSize({ width: 640, height: 720 });
    const selected = page.getByRole('button', { name: 'Foundry', exact: true });
    const label = selected.locator('.mantine-InputPlaceholder-placeholder');
    const layout = await label.evaluate((element) => ({
      overflow: getComputedStyle(element).overflow,
      textOverflow: getComputedStyle(element).textOverflow,
      whiteSpace: getComputedStyle(element).whiteSpace,
      truncated: element.scrollWidth > element.clientWidth,
    }));
    expect(layout).toEqual({
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      truncated: true,
    });
    await label.hover();
    await expect(page.getByRole('tooltip')).toHaveText(labels[0]);
  });
  await test.step('受け入れ条件', async () => {
    const snapshots = frames
      .filter((frame) => frame.includes('foundry:progress'))
      .map((frame) => JSON.parse(frame).data as FoundryProgress);
    const saving = snapshots.findIndex((snapshot) => snapshot.savePhase === 'running');
    const saved = snapshots.findIndex((snapshot) => snapshot.savePhase === 'completed');
    expect(saving).toBeGreaterThanOrEqual(0);
    expect(saved).toBeGreaterThan(saving);
    expect(snapshots.at(-1)).toEqual({
      subscriptionSearch: 'completed',
      subscriptions: foundries.map((foundry) => ({
        id: foundry.id.split('/')[2],
        name: foundry.subscriptionName,
        phase: 'completed',
        foundryCount: 1,
      })),
      selectedFoundryName: foundries[0].name,
      modelPhase: 'completed',
      modelCount: models.length,
      savePhase: 'completed',
    });
    // The fixed source has three subscriptions; the eight-worker limit is checked with real Azure.
  });
});
