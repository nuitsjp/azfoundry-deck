import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, type IsolatedApp } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';

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
const savedAt = '2001-02-03T13:05:06+09:00';
const label = (index: number) => {
  const foundry = foundries[index];
  return `${foundry.name} (${foundry.subscriptionName} - ${foundry.resourceGroupName})`;
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
  mkdirSync(join(viewDir, 'foundry-models'), { recursive: true });
  const deployments = models.map(([deploymentName, modelName, version]) => ({
    id: `${foundries[0].id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  }));
  const original: InitialFoundryView = {
    foundries,
    selectedFoundryId: foundries[0].id,
    deployments,
    foundriesFetchedAt: savedAt,
    deploymentsFetchedAt: savedAt,
  };
  const stateFile = join(viewDir, 'foundry-state.json');
  const modelFile = join(
    viewDir,
    'foundry-models',
    `${createHash('sha256').update(foundries[0].id).digest('hex')}.json`,
  );
  writeFileSync(stateFile, JSON.stringify(original));
  writeFileSync(modelFile, JSON.stringify({ fetchedAt: savedAt, deployments }));
  return [stateFile, modelFile];
}

test.describe('明細取得成功', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

  test('一覧からデプロイモデルの明細を表示する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const selected = page.getByRole('button', { name: 'Foundry', exact: true });
    const refresh = page.getByRole('button', { name: 'Refresh models', exact: true });
    const release = join(app.dataDir, 'e2e-foundry-detail-release');
    let detailRequests = 0;
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON() as { args?: { methodName?: string } };
      if (body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetDeploymentDetail')
        detailRequests += 1;
    });
    let paths: string[];
    let originalFiles: ReturnType<typeof identity>[];
    const checkRows = async () => {
      await expect(rows).toHaveCount(3);
      for (const [index, model] of models.entries())
        await expect(rows.nth(index).locator('td')).toHaveText(model);
    };
    const checkDetail = async (name: string, capacity: string, policy: string) => {
      await expect(details.getByRole('heading', { name, exact: true })).toBeVisible();
      await expect(details.locator('dd')).toHaveText([
        models.find((model) => model[0] === name)![1],
        models.find((model) => model[0] === name)![2],
        name === 'embeddings' ? 'Standard' : 'GlobalStandard',
        capacity,
        'Succeeded',
        policy,
      ]);
      await expect(details.locator('footer')).toHaveText(
        /^Last fetched \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/,
      );
    };

    await test.step('開始条件', async () => {
      paths = seed(app);
      originalFiles = paths.map(identity);
      await app.restart();
      await page.goto(app.url);
      await expect(selected).toHaveText(label(0));
      await checkRows();
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
    });

    await test.step('手順1', async () => {
      // Selecting the Model cell exercises the row outside the name button.
      await rows.nth(0).locator('td').nth(1).click();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      await expect(details).toHaveAttribute('aria-busy', 'true');
      await expect(rows.nth(0).getByRole('button')).toHaveAttribute('aria-pressed', 'true');
      for (const button of await rows.getByRole('button').all())
        await expect(button).toBeDisabled();
      await expect(selected).toBeDisabled();
      await expect(refresh).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Refresh Foundries' })).toBeDisabled();
      await expect(page.getByRole('button', { name: 'テナント', exact: true })).toBeDisabled();
      // A click in another row is ignored while the first request is pending.
      await rows.nth(1).locator('td').nth(2).click();
      expect(detailRequests).toBe(1);
      expect(paths.map(identity)).toEqual(originalFiles);
    });

    await test.step('手順2', async () => {
      writeFileSync(release, '');
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      await expect(selected).toHaveText(label(0));
      await checkRows();
      await expect(refresh).toBeEnabled();
    });

    await test.step('手順3', async () => {
      // Hold the same-row refetch to prove old contents disappear before completion.
      unlinkSync(release);
      await rows.nth(0).locator('td').nth(2).click();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      await expect(details.locator('dd')).toHaveCount(0);
      expect(detailRequests).toBe(2);
      writeFileSync(release, '');
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      // Click whitespace at the right edge of a row.
      const box = await rows.nth(1).boundingBox();
      await rows.nth(1).click({ position: { x: box!.width - 4, y: box!.height / 2 } });
      await checkDetail('chat-mini', '100,000 / 250,000 TPM', 'Upgrade on retirement');
      await rows.nth(2).getByRole('button').focus();
      await page.keyboard.press('Enter');
      await checkDetail('embeddings', '20,000 / 80,000 TPM', 'No automatic upgrade');
      await rows.nth(0).getByRole('button').focus();
      await page.keyboard.press('Space');
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      expect(detailRequests).toBe(5);
    });

    await test.step('受け入れ条件', async () => {
      expect(paths.map(identity)).toEqual(originalFiles);
      const left = await page.locator('.deployment-list-pane').boundingBox();
      const right = await details.boundingBox();
      expect(Math.abs(left!.width - right!.width)).toBeLessThanOrEqual(1);
      expect(left!.y).toBe(right!.y);
      await expect(details.locator('dt')).toHaveText([
        'Model',
        'Version',
        'SKU',
        'Capacity',
        'Provisioning state',
        'Upgrade policy',
      ]);
      writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
      await refresh.click();
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      await rows.nth(0).click();
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      await selected.click();
      await page.getByRole('option', { name: label(1), exact: true }).click();
      await expect(selected).toHaveText(label(1));
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      await app.restart();
      await page.goto(app.url);
      await expect(selected).toHaveText(label(1));
      await expect(details).toHaveText('Details');
      expect(detailRequests).toBe(6);
      await rows.nth(0).click();
      await expect(
        details.getByRole('heading', { name: 'development-chat', exact: true }),
      ).toBeVisible();
      writeFileSync(join(app.dataDir, 'e2e-foundry-discovery-release'), '');
      const tenant = page.getByRole('button', { name: 'テナント', exact: true });
      await tenant.click();
      await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
      await expect(tenant).toHaveText('Fabrikam');
      await expect(selected).toHaveText(label(0));
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await checkRows();
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      await expect(refresh).toBeEnabled();
      expect(detailRequests).toBe(7);
    });
  });
});

test.describe('明細取得失敗', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'detail' } });

  test('取得失敗時も一覧を維持して再試行する', async ({ page, app }) => {
    const paths = seed(app);
    const originalFiles = paths.map(identity);
    await app.restart();
    await page.goto(app.url);
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    let requests = 0;
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON() as { args?: { methodName?: string } };
      if (body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetDeploymentDetail')
        requests += 1;
    });
    await expect(rows).toHaveCount(3);
    await rows.nth(0).click();
    await expect(details).toContainText('DEPLOYMENT_DETAIL_FAILED');
    await expect(details).toContainText(
      'Could not retrieve deployment details from the source or read the current Foundry selection.',
    );
    await expect(details.locator('dd')).toHaveCount(0);
    for (const [index, model] of models.entries())
      await expect(rows.nth(index).locator('td')).toHaveText(model);
    await details.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(details).toContainText('DEPLOYMENT_DETAIL_FAILED');
    await expect.poll(() => requests).toBe(2);
    await expect(details.getByRole('button', { name: 'Retry' })).toBeEnabled();
    expect(paths.map(identity)).toEqual(originalFiles);
  });
});
