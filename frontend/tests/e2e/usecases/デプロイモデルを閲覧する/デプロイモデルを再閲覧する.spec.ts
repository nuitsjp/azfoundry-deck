import { readFileSync, statSync, utimesSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';

// Distinct from the external E2E source, including a saved selection other than its first item.
const foundries = [
  {
    id: '/subscriptions/saved-development/resourceGroups/rg-saved-development/providers/Microsoft.CognitiveServices/accounts/saved-foundry-development',
    name: 'saved-foundry-development',
    subscriptionName: 'Saved Development Subscription',
    resourceGroupName: 'rg-saved-development',
  },
  {
    id: '/subscriptions/saved-production/resourceGroups/rg-saved-production-japaneast/providers/Microsoft.CognitiveServices/accounts/saved-foundry-production-japaneast-long-name',
    name: 'saved-foundry-production-japaneast-long-name',
    subscriptionName: 'Saved AI Production Subscription',
    resourceGroupName: 'rg-saved-production-japaneast',
  },
  {
    id: '/subscriptions/saved-research/resourceGroups/rg-saved-research/providers/Microsoft.CognitiveServices/accounts/saved-foundry-research',
    name: 'saved-foundry-research',
    subscriptionName: 'Saved Research Subscription',
    resourceGroupName: 'rg-saved-research',
  },
];
const models = [
  ['saved-chat', 'saved-chat-model', '2030-01-02'],
  ['saved-mini', 'saved-mini-model', '2030-03-04'],
  ['saved-embedding', 'saved-embedding-model', 'saved-version-7'],
];
const view: InitialFoundryView = {
  foundries,
  selectedFoundryId: foundries[1].id,
  deployments: models.map(([deploymentName, modelName, version]) => ({
    id: `${foundries[1].id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  })),
};
const labels = foundries.map(
  (foundry) => `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`,
);

// With no releases, any accidental Azure acquisition cannot return a view.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('デプロイモデルを再閲覧する', async ({ page, app }) => {
  const savedFile = join(app.dataDir, 'foundry-state.json');
  const savedText = JSON.stringify(view, null, 2) + '\n';
  let timestamps: { mtimeMs: number; birthtimeMs: number };
  const progressFrames: string[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress')) progressFrames.push(frame);
    });
  });
  await page.addInitScript(() => {
    const observations = { count: 0 };
    Reflect.set(window, 'acquisitionDialogObservations', observations);
    const recordDialog = (element: Element) => {
      const dialogs = [
        ...(element.matches('[role="dialog"]') ? [element] : []),
        ...element.querySelectorAll('[role="dialog"]'),
      ];
      for (const dialog of dialogs) {
        if (dialog.textContent?.includes('デプロイモデルを取得しています')) {
          observations.count++;
        }
      }
    };
    new MutationObserver((records) => {
      for (const record of records) {
        // Added nodes remain inspectable even if a dialog was removed within the same task.
        for (const node of record.addedNodes) {
          if (node instanceof Element) recordDialog(node);
        }
      }
      recordDialog(document.documentElement);
    }).observe(document, { childList: true, subtree: true, characterData: true });
  });

  const assertRestored = async () => {
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveText(labels[1]);
    const rows = page.getByRole('table', { name: 'デプロイ済みモデル' }).locator('tbody tr');
    await expect(rows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(rows.nth(index).locator('td')).toHaveText(model);
    }
    await expect(page.getByRole('dialog', { name: 'デプロイモデルを取得しています' })).toHaveCount(
      0,
    );
    expect(
      await page.evaluate(() => Reflect.get(window, 'acquisitionDialogObservations').count),
    ).toBe(0);
    expect(progressFrames).toEqual([]);
    expect(readFileSync(savedFile, 'utf8')).toBe(savedText);
    const file = statSync(savedFile);
    expect({ mtimeMs: file.mtimeMs, birthtimeMs: file.birthtimeMs }).toEqual(timestamps);
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
    writeFileSync(savedFile, savedText);
    const oldTime = new Date('2001-02-03T04:05:06.000Z');
    utimesSync(savedFile, oldTime, oldTime);
    const file = statSync(savedFile);
    timestamps = { mtimeMs: file.mtimeMs, birthtimeMs: file.birthtimeMs };
    await app.restart();
  });

  await test.step('手順1', async () => {
    await page.goto(app.url);
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toBeVisible();
  });

  await test.step('手順2', async () => {
    await assertRestored();
  });

  await test.step('手順3', async () => {
    await page.getByRole('button', { name: 'Foundry', exact: true }).click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'false');
    for (const option of await page.getByRole('option').all()) {
      await expect(option).toBeVisible();
    }
  });

  await test.step('手順4', async () => {
    await page.keyboard.press('Escape');
    await expect(page.getByRole('option')).toHaveCount(0);
    await page.setViewportSize({ width: 640, height: 720 });
    const label = page
      .getByRole('button', { name: 'Foundry', exact: true })
      .locator('.mantine-InputPlaceholder-placeholder');
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
    await page.reload();
    await assertRestored();
    await app.restart();
    await page.goto(app.url);
    await assertRestored();
    await page.getByRole('button', { name: 'Foundry', exact: true }).click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true');
  });
});
