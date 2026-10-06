import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Locator, Page } from '@playwright/test';
import { test, expect, type IsolatedApp } from '../../fixtures';

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
];
const models = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const updatedModels = [
  ['chat-production', 'gpt-4.1', '2024-11-20'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const developmentModels = [
  ['development-chat', 'gpt-4.1', '2025-04-14'],
  ['development-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['development-embedding', 'text-embedding-3-large', '1'],
];
const savedAt = '2001-02-03T13:05:06+09:00';
const foundryLabel = (index: number) => {
  const foundry = foundries[index];
  return `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`;
};
const displayed = (value: string) => {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
};
const identity = (path: string) => ({
  text: readFileSync(path, 'utf8'),
  mtimeMs: statSync(path).mtimeMs,
});

function seed(app: IsolatedApp) {
  writeFileSync(
    join(app.dataDir, 'e2e-authentication-record.json'),
    JSON.stringify({
      authority: 'login.microsoftonline.com',
      clientId: 'e2e-client',
      homeAccountId: 'e2e-object.e2e-tenant',
      tenantId: 'e2e-tenant',
      username: 'operator@contoso.onmicrosoft.com',
      version: '1.0',
      tenants: [
        { id: 'e2e-azure-tenant', displayName: 'Contoso' },
        { id: 'e2e-fabrikam-tenant', displayName: 'Fabrikam' },
      ],
      selectedTenantId: 'e2e-azure-tenant',
    }),
  );
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  mkdirSync(viewDir, { recursive: true });
  const stateFile = join(viewDir, 'foundry-state.json');
  writeFileSync(
    stateFile,
    JSON.stringify({
      foundries,
      selectedFoundryId: foundries[0].id,
      foundriesFetchedAt: savedAt,
    }),
  );
  // The deployments are fetched from Azure on startup, so the models gate is released up front.
  writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
  return { viewDir, stateFile };
}

function countCalls(page: Page, methodName: string) {
  let count = 0;
  page.on('request', (request) => {
    if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
    const body = request.postDataJSON() as { args?: { methodName?: string } };
    if (body.args?.methodName === methodName) count += 1;
  });
  return () => count;
}

async function assertRows(rows: Locator, modelList: string[][]) {
  await expect(rows).toHaveCount(modelList.length);
  for (const [index, model] of modelList.entries()) {
    await expect(rows.nth(index).locator('td:nth-child(-n + 3)')).toHaveText(model);
  }
}

async function assertDetail(details: Locator, version: string, capacity: string, policy: string) {
  await expect(
    details.getByRole('heading', { name: 'chat-production', exact: true }),
  ).toBeVisible();
  await expect(details.locator('dd')).toHaveText([
    'gpt-4.1',
    version,
    'GlobalStandard',
    capacity,
    'Succeeded',
    policy,
  ]);
}

// The hover label is absolutely positioned. At the right end it must stay inside the dialog
// so the modal does not gain a horizontal scrollbar.
async function assertCapacityLabelStaysInside(page: Page, dialog: Locator) {
  const slider = dialog.getByRole('slider');
  await slider.focus();
  await slider.press('End');
  await expect(slider).toHaveAttribute('aria-valuenow', '160000');
  const box = await slider.boundingBox();
  if (!box) throw new Error('スライダーの位置を取得できません');
  for (let i = 0; i < 8; i += 1) {
    await page.mouse.move(box.x + box.width - 1, box.y + box.height / 2);
    await page.mouse.move(box.x + box.width + 4, box.y + box.height / 2);
  }
  await slider.hover();
  const metrics = await dialog.evaluate((node) => {
    const content = node.getBoundingClientRect();
    const label = [...node.querySelectorAll('div')].find(
      (element) =>
        element.textContent === '160,000' && getComputedStyle(element).position === 'absolute',
    );
    const labelBox = label?.getBoundingClientRect();
    return {
      scrollWidth: node.scrollWidth,
      clientWidth: node.clientWidth,
      overflowX: getComputedStyle(node).overflowX,
      inside:
        labelBox != null &&
        labelBox.left >= content.left - 1 &&
        labelBox.right <= content.right + 1,
    };
  });
  expect(metrics.overflowX).toBe('hidden');
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
  expect(metrics.inside).toBe(true);
}

const stateKeys = ['foundries', 'foundriesFetchedAt', 'selectedFoundryId'];

// The saved file keeps only the Foundry list and the selection; deployments are never saved.
function assertNothingSavedBesidesFoundries(files: ReturnType<typeof seed>) {
  const state = JSON.parse(readFileSync(files.stateFile, 'utf8')) as Record<string, unknown>;
  expect(Object.keys(state).sort()).toEqual(stateKeys);
  expect(state.foundries).toEqual(foundries);
  expect(state.selectedFoundryId).toBe(foundries[0].id);
  expect(state.foundriesFetchedAt).toBe(savedAt);
  expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);
}

test.describe('変更成功', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

  test('一覧からデプロイモデルの設定を変更する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const selected = page.getByRole('button', { name: 'Foundry', exact: true });
    const modelsFetched = page.getByText(/ · Last fetched /);
    const foundriesFetched = page
      .locator('button[aria-label="Refresh Foundries"]')
      .locator('..')
      .getByText(/^Last fetched /);
    const editDialog = () => page.getByRole('dialog', { name: 'Edit deployment' });
    const progressDialog = () => page.getByRole('dialog', { name: 'Updating deployment' });
    const updates = countCalls(page, 'azfoundrydeck/internal/foundry.Service.UpdateDeployment');
    const maximums = countCalls(page, 'azfoundrydeck/internal/foundry.Service.GetCapacityMaximum');
    const settings = countCalls(
      page,
      'azfoundrydeck/internal/foundry.Service.GetDeploymentSettings',
    );
    const fetchedPattern = /^3 · Last fetched \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/;
    let files: ReturnType<typeof seed>;
    let stateFile: ReturnType<typeof identity>;

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(page.getByRole('banner')).toContainText('Contoso');
      await expect(selected).toHaveText(foundryLabel(0));
      await assertRows(rows, models);
      await expect(modelsFetched).toHaveText(fetchedPattern);
      await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
      await expect(details).toHaveText('Details');
      stateFile = identity(files.stateFile);
    });

    await test.step('手順1', async () => {
      await rows.nth(0).getByRole('button', { name: 'chat-production' }).click();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      writeFileSync(join(app.dataDir, 'e2e-foundry-detail-release'), '');
      await assertDetail(details, '2025-04-14', '50,000 / 160,000 TPM', 'Upgrade to new default');

      await details.getByRole('button', { name: 'Edit deployment' }).click();
      const dialog = editDialog();
      await expect(dialog.getByRole('status')).toHaveText('Loading deployment settings...');
      writeFileSync(join(app.dataDir, 'e2e-foundry-update-settings-release'), '');

      await expect(dialog.getByLabel('Deployment name')).toHaveValue('chat-production');
      await expect(dialog.getByLabel('Deployment name')).toHaveJSProperty('readOnly', true);
      await expect(dialog.getByLabel('Model')).toHaveValue('gpt-4.1');
      await expect(dialog.getByLabel('Model')).toHaveJSProperty('readOnly', true);
      await expect(dialog.getByLabel('SKU')).toHaveValue('GlobalStandard');
      await expect(dialog.getByLabel('SKU')).toHaveJSProperty('readOnly', true);
      await expect(dialog.getByLabel('Version')).toHaveValue('2025-04-14');
      await expect(dialog.getByLabel('Upgrade policy')).toHaveValue('Upgrade to new default');
      await expect(dialog.getByText('50,000 / 160,000 TPM')).toBeVisible();
      await expect(dialog.getByRole('textbox', { name: 'Capacity' })).toHaveValue('50000');
      await expect(dialog.getByRole('slider')).toHaveAttribute('aria-valuenow', '50000');
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeDisabled();

      await dialog.getByLabel('Version').click();
      await expect(page.getByRole('option')).toHaveText(['2025-04-14', '2024-11-20']);
      await page.getByRole('option', { name: '2025-04-14', exact: true }).click();
      await expect(page.getByRole('option')).toHaveCount(0);
      await dialog.getByLabel('Upgrade policy').click();
      await expect(page.getByRole('option')).toHaveText([
        'Upgrade to new default',
        'Upgrade on retirement',
        'No automatic upgrade',
      ]);
      await page.getByRole('option', { name: 'Upgrade to new default', exact: true }).click();
      await expect(page.getByRole('option')).toHaveCount(0);
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeDisabled();
      expect(updates()).toBe(0);
      expect(settings()).toBe(1);
      expect(maximums()).toBe(0);
      expect(identity(files.stateFile)).toEqual(stateFile);
    });

    await test.step('手順2', async () => {
      const dialog = editDialog();
      await dialog.getByLabel('Version').click();
      await page.getByRole('option', { name: '2024-11-20', exact: true }).click();
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeEnabled();

      const capacity = dialog.getByRole('textbox', { name: 'Capacity' });
      await capacity.fill('79000');
      await expect(capacity).toHaveValue('79000');
      await expect(dialog.getByText('79,000 / 160,000 TPM')).toBeVisible();
      await dialog.getByRole('slider').press('ArrowRight');
      await expect(capacity).toHaveValue('80000');
      await expect(dialog.getByRole('slider')).toHaveAttribute('aria-valuenow', '80000');
      await expect(dialog.getByText('80,000 / 160,000 TPM')).toBeVisible();

      await dialog.getByLabel('Upgrade policy').click();
      await page.getByRole('option', { name: 'Upgrade on retirement', exact: true }).click();
      await expect(page.getByRole('dialog', { name: 'Edit deployment' })).toBeVisible();
      await expect(page.locator('[role="dialog"]')).toHaveCount(1);

      // The capacity maximum must not be fetched again after the change, so hold it from here on.
      unlinkSync(join(app.dataDir, 'e2e-foundry-detail-release'));
      await dialog.getByRole('button', { name: 'Update', exact: true }).click();
      const progress = progressDialog();
      await expect(progress).toBeVisible();
      await expect(progress.getByText('Update', { exact: true })).toBeVisible();
      await expect(progress.getByText('chat-production', { exact: true })).toBeVisible();
      await expect(page.locator('[role="dialog"]')).toHaveCount(2);
      await page.keyboard.press('Escape');
      await page.mouse.click(5, 5);
      await expect(progress).toBeVisible();
      await expect(page.locator('[role="dialog"]')).toHaveCount(2);
      await assertRows(rows, models);
      expect(identity(files.stateFile)).toEqual(stateFile);

      writeFileSync(join(app.dataDir, 'e2e-foundry-update-release'), '');
    });

    await test.step('手順3', async () => {
      await expect(page.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15_000 });
      await assertRows(rows, updatedModels);
      await expect(rows.nth(0).getByRole('button')).toHaveAttribute('aria-pressed', 'true');
      await assertDetail(details, '2024-11-20', '80,000 / 160,000 TPM', 'Upgrade on retirement');
      await expect(details.getByRole('status')).toHaveCount(0);
      await expect(modelsFetched).toHaveText(fetchedPattern);
      await expect(foundriesFetched).toHaveText(`Last fetched ${displayed(savedAt)}`);
      expect(updates()).toBe(1);
      expect(maximums()).toBe(0);
    });

    await test.step('受け入れ条件', async () => {
      assertNothingSavedBesidesFoundries(files);

      // Each time the dialog opens, the shared quota is fetched again.
      unlinkSync(join(app.dataDir, 'e2e-foundry-update-settings-release'));
      await details.getByRole('button', { name: 'Edit deployment' }).click();
      const dialog = editDialog();
      await expect(dialog.getByRole('status')).toHaveText('Loading deployment settings...');
      writeFileSync(join(app.dataDir, 'e2e-foundry-update-settings-release'), '');
      await expect(dialog.getByLabel('Version')).toHaveValue('2024-11-20');
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeDisabled();
      expect(settings()).toBe(2);
      await assertCapacityLabelStaysInside(page, dialog);
      await dialog.getByRole('button', { name: 'Cancel' }).click();
      await expect(page.locator('[role="dialog"]')).toHaveCount(0);
      await assertRows(rows, updatedModels);
      await assertDetail(details, '2024-11-20', '80,000 / 160,000 TPM', 'Upgrade on retirement');
      expect(updates()).toBe(1);

      await details.getByRole('button', { name: 'Edit deployment' }).click();
      await editDialog().locator('header').getByRole('button').click();
      await expect(page.locator('[role="dialog"]')).toHaveCount(0);
      await assertRows(rows, updatedModels);
      expect(updates()).toBe(1);
      expect(maximums()).toBe(0);

      // The list is fetched from Azure each time, so coming back shows the changed deployment.
      await selected.click();
      await page.getByRole('option', { name: foundryLabel(1), exact: true }).click();
      await expect(selected).toHaveText(foundryLabel(1));
      await assertRows(rows, developmentModels);
      await selected.click();
      await page.getByRole('option', { name: foundryLabel(0), exact: true }).click();
      await expect(selected).toHaveText(foundryLabel(0));
      await assertRows(rows, updatedModels);
      assertNothingSavedBesidesFoundries({ ...files });
    });
  });
});

test.describe('設定の取得に失敗する', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'update-settings' } });

  test('一覧からデプロイモデルの設定を変更する（設定の取得に失敗）', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const updates = countCalls(page, 'azfoundrydeck/internal/foundry.Service.UpdateDeployment');
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await assertRows(rows, models);
      original = identity(files.stateFile);
    });

    await test.step('手順1', async () => {
      await rows.nth(0).getByRole('button', { name: 'chat-production' }).click();
      await assertDetail(details, '2025-04-14', '50,000 / 160,000 TPM', 'Upgrade to new default');
      await details.getByRole('button', { name: 'Edit deployment' }).click();
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await expect(dialog.getByRole('alert')).toContainText('DEPLOYMENT_UPDATE_FAILED');
      await expect(dialog.getByRole('alert')).toContainText(
        'Could not load the deployment settings.',
      );
      await expect(dialog.getByRole('button', { name: 'Retry' })).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toHaveCount(0);
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toHaveCount(0);
      expect(updates()).toBe(0);
    });

    await test.step('手順2', async () => {
      await page
        .getByRole('dialog', { name: 'Edit deployment' })
        .getByRole('button', {
          name: 'Retry',
        })
        .click();
    });

    await test.step('手順3', async () => {
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await expect(dialog.getByRole('alert')).toContainText('DEPLOYMENT_UPDATE_FAILED');
      await expect(dialog.getByRole('button', { name: 'Retry' })).toBeEnabled();
      await assertRows(rows, models);
      await expect(details.locator('dd')).toHaveText([
        'gpt-4.1',
        '2025-04-14',
        'GlobalStandard',
        '50,000 / Not set TPM',
        'DEPLOYMENT_DETAIL_FAILEDCould not retrieve the capacity maximum.Retry',
        'Succeeded',
        'Upgrade to new default',
      ]);
    });

    await test.step('受け入れ条件', async () => {
      expect(updates()).toBe(0);
      expect(identity(files.stateFile)).toEqual(original);
      await page
        .getByRole('dialog', { name: 'Edit deployment' })
        .getByRole('button', { name: 'Cancel' })
        .click();
      await expect(page.locator('[role="dialog"]')).toHaveCount(0);
      await assertRows(rows, models);
    });
  });
});

test.describe('変更の実行に失敗する', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'update' } });

  test('一覧からデプロイモデルの設定を変更する（変更の実行に失敗）', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const updates = countCalls(page, 'azfoundrydeck/internal/foundry.Service.UpdateDeployment');
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await assertRows(rows, models);
      original = identity(files.stateFile);
      await rows.nth(0).getByRole('button', { name: 'chat-production' }).click();
      await assertDetail(details, '2025-04-14', '50,000 / 160,000 TPM', 'Upgrade to new default');
    });

    await test.step('手順1', async () => {
      await details.getByRole('button', { name: 'Edit deployment' }).click();
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await expect(dialog.getByLabel('Version')).toHaveValue('2025-04-14');
      await expect(dialog.getByRole('textbox', { name: 'Capacity' })).toHaveValue('50000');
      await expect(dialog.getByLabel('Upgrade policy')).toHaveValue('Upgrade to new default');
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeDisabled();
    });

    await test.step('手順2', async () => {
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await dialog.getByLabel('Version').click();
      await page.getByRole('option', { name: '2024-11-20', exact: true }).click();
      await dialog.getByRole('textbox', { name: 'Capacity' }).fill('80000');
      await dialog.getByLabel('Upgrade policy').click();
      await page.getByRole('option', { name: 'Upgrade on retirement', exact: true }).click();
      await expect(page.locator('[role="dialog"]')).toHaveCount(1);
      await dialog.getByRole('button', { name: 'Update', exact: true }).click();
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toBeVisible();
      await page.keyboard.press('Escape');
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toBeVisible();
    });

    await test.step('手順3', async () => {
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toHaveCount(0);
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByLabel('Version')).toHaveValue('2024-11-20');
      await expect(dialog.getByRole('textbox', { name: 'Capacity' })).toHaveValue('80000');
      await expect(dialog.getByLabel('Upgrade policy')).toHaveValue('Upgrade on retirement');
      await expect(dialog.getByRole('alert')).toContainText('DEPLOYMENT_UPDATE_FAILED');
      await expect(dialog.getByRole('alert')).toContainText(
        'Could not update the deployment or refresh the deployed models.',
      );
      await assertRows(rows, models);
    });

    await test.step('受け入れ条件', async () => {
      const dialog = page.getByRole('dialog', { name: 'Edit deployment' });
      await expect(dialog.getByRole('button', { name: 'Update', exact: true })).toBeEnabled();
      expect(identity(files.stateFile)).toEqual(original);
      await assertDetail(details, '2025-04-14', '50,000 / 160,000 TPM', 'Upgrade to new default');
      await dialog.getByRole('button', { name: 'Update', exact: true }).click();
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toBeVisible();
      await expect(page.getByRole('dialog', { name: 'Updating deployment' })).toHaveCount(0);
      await expect(dialog.getByRole('alert')).toContainText('DEPLOYMENT_UPDATE_FAILED');
      await expect(dialog.getByLabel('Version')).toHaveValue('2024-11-20');
      await assertRows(rows, models);
      expect(updates()).toBe(2);
      expect(identity(files.stateFile)).toEqual(original);
    });
  });
});
