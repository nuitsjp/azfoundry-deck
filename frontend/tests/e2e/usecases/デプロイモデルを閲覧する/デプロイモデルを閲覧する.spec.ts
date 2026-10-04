import { createHash } from 'node:crypto';
import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
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
  (foundry) => `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`,
);

// The first progress event is lost if it is sent before the page's WebSocket is registered, so the
// startup restore is held until the page has opened it, and discovery then starts after that.
test.use({
  serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1', AZFOUNDRYDECK_E2E_HOLD_RESTORE: '1' },
});

test('デプロイモデルを閲覧する', async ({ page, app }) => {
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
  const dialog = page.getByRole('dialog', { name: 'Loading deployments' });
  const foundryStep = dialog.getByText('Foundries', { exact: true }).locator('..');
  const modelStep = dialog.getByText('Deployments', { exact: true }).locator('..');
  const table = page.getByRole('table', { name: 'Deployments', exact: true });
  const capacityCells = table.locator('tbody td:nth-child(4)');
  const details = page.getByRole('region', { name: 'Details', exact: true });
  const capacities = ['50,000 / 160,000 TPM', '100,000 / 250,000 TPM', '20,000 / 80,000 TPM'];
  const frames: string[] = [];
  let dialogSize: { width: number; height: number } | null = null;
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
    const socket = page.waitForEvent('websocket');
    await page.goto(app.url);
    await socket;
    writeFileSync(join(app.dataDir, 'e2e-restore-release'), '');
    await expect(page.getByRole('banner').getByText('Azure Foundry Deck')).toBeVisible();
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(dialog).toBeVisible();
    await expect(foundryStep).toContainText('Loading');
    await expect(modelStep).toContainText('Waiting');
    await expect(dialog.getByText('ファイルへの保存')).toHaveCount(0);
    await expect(dialog.getByText('サブスクリプション')).toHaveCount(0);
    const box = await dialog.boundingBox();
    dialogSize = box && { width: box.width, height: box.height };
    expect(dialogSize).not.toBeNull();
    await expect(dialog.getByRole('button')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(dialog).toBeVisible();
    expect(existsSync(join(app.dataDir, 'e2e-signin-called'))).toBe(false);
  });

  await test.step('手順2', async () => {
    release('discovery');
    await expect(foundryStep).toContainText('Completed');
    await expect(foundryStep).toContainText('3 Foundries');
    await expect(modelStep).toContainText('Loading');
    await expect(dialog.getByText(foundries[0].name, { exact: true })).toBeVisible();
    await expect(dialog.getByText('0 models', { exact: true })).toBeVisible();
    const box = await dialog.boundingBox();
    expect(box && { width: box.width, height: box.height }).toEqual(dialogSize);
    expect(existsSync(savedFile)).toBe(false);
  });

  await test.step('手順3', async () => {
    release('models');
    await expect(dialog).toHaveCount(0);
    const savedView = JSON.parse(readFileSync(savedFile, 'utf8'));
    // The state file holds the Foundry list and selection only; deployments are never saved.
    expect(savedView).toEqual({
      foundries,
      selectedFoundryId: foundries[0].id,
      foundriesFetchedAt: expect.any(String),
    });
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
    const modelRows = page.getByRole('table', { name: 'Deployments' }).locator('tbody tr');
    await expect(modelRows).toHaveCount(3);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td:nth-child(-n + 3)')).toHaveText(model);
    }
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveText(labels[0]);
    await expect(table.locator('thead th')).toHaveText([
      'Deployment',
      'Model',
      'Version',
      'Capacity',
    ]);
    await expect(capacityCells).toHaveText([
      '50,000 / Loading... TPM',
      '100,000 / Loading... TPM',
      '20,000 / Loading... TPM',
    ]);
    await expect(table.getByRole('status')).toHaveCount(3);
    const loadingGeometry = await capacityCells
      .locator('.deployment-capacity')
      .evaluateAll((values) =>
        values.map((value) => {
          const boxes = [...value.children].map((element) => element.getBoundingClientRect());
          const status = value.querySelector('[role="status"]')!;
          const centers = [
            ...boxes.map((box) => box.y + box.height / 2),
            ...[...status.children].map((element) => {
              const box = element.getBoundingClientRect();
              return box.y + box.height / 2;
            }),
          ];
          return {
            slash: boxes[1].x,
            denominator: boxes[2].x,
            unit: boxes[3].x,
            centerDifference: Math.max(...centers) - Math.min(...centers),
          };
        }),
      );
    for (const slot of ['slash', 'denominator', 'unit'] as const)
      expect(
        Math.max(...loadingGeometry.map((row) => row[slot])) -
          Math.min(...loadingGeometry.map((row) => row[slot])),
      ).toBeLessThan(0.1);
    for (const row of loadingGeometry) expect(row.centerDifference).toBeLessThan(0.1);
    await expect(details).toHaveText('Details');
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
    // Completing the external fetch updates every row without selecting a deployment.
    release('detail');
    await expect(capacityCells).toHaveText(capacities);
    await expect(table.getByRole('status')).toHaveCount(0);
    await expect(details).toHaveText('Details');
  });
  await test.step('受け入れ条件', async () => {
    await capacityCells.first().click();
    for (const width of [1100, 1280, 1440]) {
      await page.setViewportSize({ width, height: 720 });
      const geometry = await capacityCells.locator('.deployment-capacity').evaluateAll((values) =>
        values.map((value) => {
          const boxes = [...value.children].map((element) => element.getBoundingClientRect());
          const box = value.getBoundingClientRect();
          const cell = value.closest('td')!.getBoundingClientRect();
          const columns = getComputedStyle(value).gridTemplateColumns.split(' ');
          return {
            numeratorRight: boxes[0].right,
            slash: boxes[1].x,
            denominatorRight: boxes[2].right,
            unit: boxes[3].x,
            numeratorWidth: columns[0],
            denominatorWidth: columns[2],
            height: box.height,
            right: box.right,
            cellRight: cell.right,
          };
        }),
      );
      for (const slot of ['numeratorRight', 'slash', 'denominatorRight', 'unit'] as const)
        expect(
          Math.max(...geometry.map((row) => row[slot])) -
            Math.min(...geometry.map((row) => row[slot])),
        ).toBeLessThan(0.1);
      expect(new Set(geometry.map((row) => row.numeratorWidth)).size).toBe(1);
      expect(new Set(geometry.map((row) => row.denominatorWidth)).size).toBe(1);
      for (const row of geometry) {
        expect(row.right).toBeLessThanOrEqual(row.cellRight);
        expect(row.height).toBeLessThan(21);
      }
      const listBox = await page
        .getByRole('region', { name: 'Deployments', exact: true })
        .boundingBox();
      const detailBox = await details.boundingBox();
      expect(listBox!.width / (listBox!.width + detailBox!.width)).toBeCloseTo(0.6, 3);
      const tableBox = await table.boundingBox();
      expect(tableBox!.x + tableBox!.width).toBeLessThanOrEqual(listBox!.x + listBox!.width);
      const detailCapacity = await details.locator('.deployment-capacity').boundingBox();
      expect(detailCapacity!.x + detailCapacity!.width).toBeLessThanOrEqual(
        detailBox!.x + detailBox!.width,
      );
      for (const button of await details.getByRole('button').all()) {
        const buttonBox = await button.boundingBox();
        expect(buttonBox!.x).toBeGreaterThanOrEqual(detailBox!.x);
        expect(buttonBox!.x + buttonBox!.width).toBeLessThanOrEqual(
          detailBox!.x + detailBox!.width,
        );
      }
    }
    await page.setViewportSize({ width: 1280, height: 720 });
    for (const [index, capacity] of capacities.entries()) {
      await capacityCells.nth(index).click();
      await expect(
        details.getByRole('heading', { name: models[index][0], exact: true }),
      ).toBeVisible();
      await expect(details.locator('dd').nth(3)).toHaveText(capacity);
      await expect(details.getByRole('status')).toHaveCount(0);
    }
    const snapshots = frames
      .filter((frame) => frame.includes('foundry:progress'))
      .map((frame) => JSON.parse(frame).data as FoundryProgress);
    // The model step never starts before the Foundry list completes, and no save phase exists.
    for (const snapshot of snapshots) {
      if (snapshot.modelPhase !== 'waiting') expect(snapshot.foundryPhase).toBe('completed');
    }
    expect(snapshots.at(-1)).toEqual({
      foundryPhase: 'completed',
      foundryCount: foundries.length,
      selectedFoundryName: foundries[0].name,
      modelPhase: 'completed',
      modelCount: models.length,
    });

    // With a saved Foundry list and a selection other than the first, the next start keeps that
    // selection, does not fetch the Foundry list, and fetches only the deployments from Azure.
    const savedFoundries = [
      {
        id: '/subscriptions/saved-a/resourceGroups/rg-saved-a/providers/Microsoft.CognitiveServices/accounts/saved-foundry-a',
        name: 'saved-foundry-a',
        subscriptionName: 'Saved A Subscription',
        resourceGroupName: 'rg-saved-a',
      },
      {
        id: '/subscriptions/saved-b/resourceGroups/rg-saved-b/providers/Microsoft.CognitiveServices/accounts/saved-foundry-b',
        name: 'saved-foundry-b',
        subscriptionName: 'Saved B Subscription',
        resourceGroupName: 'rg-saved-b',
      },
    ];
    const savedState = {
      foundries: savedFoundries,
      selectedFoundryId: savedFoundries[1].id,
      foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
    };
    const savedText = JSON.stringify(savedState, null, 2) + '\n';
    writeFileSync(savedFile, savedText);
    for (const stage of ['restore', 'foundry-models']) {
      rmSync(join(app.dataDir, `e2e-${stage}-release`));
    }
    const before = frames.length;
    await app.restart();
    const socket = page.waitForEvent('websocket');
    await page.goto(app.url);
    await socket;
    writeFileSync(join(app.dataDir, 'e2e-restore-release'), '');
    await expect(dialog).toBeVisible();
    await expect(modelStep).toContainText('Loading');
    await expect(dialog.getByText(savedFoundries[1].name, { exact: true })).toBeVisible();
    await expect(page.getByRole('table', { name: 'Deployments' })).toHaveCount(0);
    release('models');
    await expect(dialog).toHaveCount(0);
    const restored = page.getByRole('table', { name: 'Deployments' }).locator('tbody tr');
    await expect(restored).toHaveCount(models.length);
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveText(
      `${savedFoundries[1].name} (${savedFoundries[1].subscriptionName} - ${savedFoundries[1].resourceGroupName})`,
    );
    const restartSnapshots = frames
      .slice(before)
      .filter((frame) => frame.includes('foundry:progress'))
      .map((frame) => JSON.parse(frame).data as FoundryProgress);
    expect(restartSnapshots.length).toBeGreaterThan(0);
    expect(restartSnapshots.every((snapshot) => snapshot.foundryPhase === 'completed')).toBe(true);
    expect(restartSnapshots.at(-1)).toEqual({
      foundryPhase: 'completed',
      foundryCount: savedFoundries.length,
      selectedFoundryName: savedFoundries[1].name,
      modelPhase: 'completed',
      modelCount: models.length,
    });
    expect(JSON.parse(readFileSync(savedFile, 'utf8'))).toEqual(savedState);
    expect(existsSync(join(viewDir, 'foundry-models'))).toBe(false);
    await expect(
      page.getByText(/^3 · Last fetched (?!2001)\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/),
    ).toBeVisible();
    await expect(page.getByText(/^Last fetched 2001-02-0\d \d{2}:\d{2}$/)).toBeVisible();
  });
});
