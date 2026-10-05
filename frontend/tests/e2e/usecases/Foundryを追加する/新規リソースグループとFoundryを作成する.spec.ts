import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, type IsolatedApp } from '../../fixtures';
import type { FoundryCreateProgress } from '../../../../src/features/foundry/create-foundry';

const original = {
  foundries: [],
  selectedFoundryId: '',
  foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
};
const created = {
  id: '/subscriptions/review-production/resourceGroups/rg-direct/providers/Microsoft.CognitiveServices/accounts/foundry-direct',
  name: 'foundry-direct',
  subscriptionName: 'Contoso AI Production Subscription',
  resourceGroupName: 'rg-direct',
};
const label = `${created.name} (${created.subscriptionName} - ${created.resourceGroupName})`;

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
      tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
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
  writeFileSync(stateFile, JSON.stringify(original));
  writeFileSync(join(app.dataDir, 'e2e-foundry-discovery-release'), '');
  return { viewDir, stateFile };
}

test.use({
  serverEnv: {
    AZFOUNDRYDECK_E2E_FOUNDRIES: 'none',
    AZFOUNDRYDECK_E2E_FOUNDRY_ADD_REVIEW: '1',
    AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1',
  },
});

test('新規リソースグループとFoundryを作成する', async ({ page, app }) => {
  test.setTimeout(120_000);
  let files: ReturnType<typeof seed>;
  const add = page.getByRole('button', { name: 'Add Foundry' });
  const selected = page.getByRole('button', { name: 'Foundry', exact: true });
  const modal = page.getByRole('dialog', { name: 'Add Foundry', exact: true });
  const progress = page.getByRole('dialog', { name: 'Creating Foundry', exact: true });
  const resourceGroup = modal.getByLabel('Resource group name', { exact: true });
  const foundryName = modal.getByLabel('Foundry name', { exact: true });
  const keyword = modal.getByLabel('Keyword', { exact: true });
  const region = modal.getByLabel('Region', { exact: true });
  const create = modal.getByRole('button', { name: 'Create', exact: true });
  const rows = page.locator('table[aria-label="Deployments"] tbody tr');
  const groupStep = progress.getByText('Create resource group', { exact: true }).locator('../..');
  const foundryStep = progress.getByText('Create Foundry', { exact: true }).locator('../..');
  const homeStep = progress.getByText('Update Home', { exact: true }).locator('../..');
  const snapshots: FoundryCreateProgress[] = [];
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:create-progress'))
        snapshots.push(JSON.parse(frame).data as FoundryCreateProgress);
    });
  });

  await test.step('開始条件', async () => {
    files = seed(app);
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText('');
    await expect(rows).toHaveCount(0);
    await expect(add).toBeEnabled();
    await expect(page.getByRole('button', { name: 'Add deployment' })).toBeDisabled();
  });

  await test.step('手順1', async () => {
    const heading = page.getByText('Foundry', { exact: true });
    const headingBox = await heading.boundingBox();
    const addBox = await add.boundingBox();
    expect(headingBox).not.toBeNull();
    expect(addBox).not.toBeNull();
    expect(addBox!.x).toBeGreaterThan(headingBox!.x + headingBox!.width);
    expect(
      Math.abs(addBox!.y + addBox!.height / 2 - headingBox!.y - headingBox!.height / 2),
    ).toBeLessThan(2);
    await add.click();
    await expect(modal).toBeVisible();
    await expect(modal.getByLabel('Subscription', { exact: true })).toHaveValue(
      'Contoso Development',
    );
    await expect(region).toHaveValue('East US 2');
    await expect(keyword).toHaveValue('sample');
    await expect(resourceGroup).toHaveValue('rg-foundry-sample-eastus2');
    await expect(foundryName).toHaveValue('aif-foundry-sample-eastus2');
  });

  await test.step('手順2', async () => {
    // Both ways of closing before creation preserve the empty Home and saved state.
    await modal.getByRole('button', { name: 'Cancel' }).click();
    await expect(modal).toHaveCount(0);
    expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
    await add.click();
    await modal.getByRole('banner').getByRole('button').click();
    await expect(modal).toHaveCount(0);
    expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
    await add.click();
    await modal.getByLabel('Subscription', { exact: true }).click();
    await page
      .getByRole('option', { name: 'Contoso AI Production Subscription', exact: true })
      .click();
    await expect(region).toHaveValue('East US 2');
    await resourceGroup.fill('rg-before-region');
    await foundryName.fill('foundry-before-region');
    await region.click();
    await page.getByRole('option', { name: 'Japan East', exact: true }).click();
    await expect(resourceGroup).toHaveValue('rg-foundry-sample-japaneast');
    await expect(foundryName).toHaveValue('aif-foundry-sample-japaneast');
    await expect(modal.getByText('Edited', { exact: true })).toHaveCount(0);
    await resourceGroup.fill('rg-before-keyword');
    await foundryName.fill('foundry-before-keyword');
    await keyword.fill('  Team__AI--  ');
    await expect(resourceGroup).toHaveValue('rg-foundry-team-ai-japaneast');
    await expect(foundryName).toHaveValue('aif-foundry-team-ai-japaneast');
    await expect(modal.getByText('Edited', { exact: true })).toHaveCount(0);
    await keyword.fill('');
    await expect(create).toBeDisabled();
    await keyword.fill('team-ai');
    await resourceGroup.fill('');
    await expect(create).toBeDisabled();
    await resourceGroup.fill(created.resourceGroupName);
    await foundryName.fill('');
    await expect(create).toBeDisabled();
    await foundryName.fill(created.name);
    await expect(modal.getByText('Edited', { exact: true })).toHaveCount(2);
    await expect(create).toBeEnabled();
    await expect(selected).toHaveText('');
    expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
  });

  await test.step('手順3', async () => {
    const creationRequest = page.waitForRequest(
      (request) =>
        request.method() === 'POST' &&
        request.postData()?.includes('foundry.Service.CreateFoundry') === true,
    );
    await create.click();
    expect((await creationRequest).postDataJSON().args).toMatchObject({
      methodName: 'azfoundrydeck/internal/foundry.Service.CreateFoundry',
      args: [
        {
          subscriptionId: 'review-production',
          resourceGroupName: created.resourceGroupName,
          foundryName: created.name,
          region: 'japaneast',
        },
      ],
    });
    await expect(progress).toBeVisible();
    await expect(modal).toBeVisible();
    await expect(groupStep).toContainText(created.resourceGroupName);
    await expect(foundryStep).toContainText(created.name);
    await expect(groupStep).toContainText('In progress');
    await expect(foundryStep).toContainText('Waiting');
    await expect(homeStep).toContainText('Waiting');
    await expect(progress.getByRole('button')).toHaveCount(0);
    await expect(create).toBeDisabled();
    await expect(modal.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    await expect(modal.getByRole('banner').getByRole('button')).toHaveCount(0);
    await expect(resourceGroup).toBeDisabled();
    await expect(foundryName).toBeDisabled();
    await page.keyboard.press('Enter');
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(progress).toBeVisible();
    await expect(foundryStep).toContainText('In progress', { timeout: 10_000 });
    await expect(groupStep).toContainText('Completed');
    await expect(homeStep).toContainText('Waiting');
    await expect(homeStep).toContainText('In progress', { timeout: 10_000 });
    await expect(foundryStep).toContainText('Completed');
    // Azure model acquisition can take more than 30 seconds. Keep this boundary
    // held past the former server write timeout before completing the same request.
    await page.waitForTimeout(35_000);
    await expect(progress).toBeVisible();
    await expect(homeStep).toContainText('In progress');
    await expect(selected).toHaveText('');
    await expect(rows).toHaveCount(0);
    expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
    expect(snapshots.filter((snapshot) => snapshot.resourceGroupPhase === 'running')).toHaveLength(
      1,
    );
    writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
  });

  await test.step('手順4', async () => {
    await expect(progress).toHaveCount(0);
    await expect(modal).toHaveCount(0);
    await expect(selected).toHaveText(label);
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(rows).toHaveCount(0);
    await expect(page.getByText(/^0 · Last fetched /)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add deployment' })).toBeEnabled();
    await selected.click();
    await expect(page.getByRole('option')).toHaveCount(1);
    await expect(page.getByRole('option', { name: label, exact: true })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await page.keyboard.press('Escape');
  });

  await test.step('受け入れ条件', async () => {
    expect(snapshots.at(-1)).toEqual({
      resourceGroupName: created.resourceGroupName,
      foundryName: created.name,
      resourceGroupPhase: 'completed',
      foundryPhase: 'completed',
      viewPhase: 'completed',
    });
    const saved = JSON.parse(readFileSync(files.stateFile, 'utf8'));
    expect(saved).toEqual({
      foundries: [created],
      selectedFoundryId: created.id,
      foundriesFetchedAt: expect.any(String),
    });
    expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);
    await page.reload();
    await expect(selected).toHaveText(label);
    await expect(rows).toHaveCount(0);
    expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(saved);
  });
});
