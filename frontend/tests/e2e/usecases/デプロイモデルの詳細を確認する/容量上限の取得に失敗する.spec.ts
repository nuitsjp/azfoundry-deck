import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, type IsolatedApp } from '../../fixtures';

const foundries = [
  {
    id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    name: 'contoso-foundry-production-japaneast',
    subscriptionName: 'Contoso AI Production Subscription',
    resourceGroupName: 'rg-ai-production-japaneast',
  },
];
const models = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];
const savedAt = '2001-02-03T13:05:06+09:00';
const identity = (path: string) => ({
  text: readFileSync(path, 'utf8'),
  mtimeMs: statSync(path).mtimeMs,
});

test.describe('容量上限の再試行成功', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

  test('選択前の取得失敗から再試行で回復する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const capacity = details.locator('dd').nth(3);
    const retry = details.getByRole('button', { name: 'Retry', exact: true });
    const release = join(app.dataDir, 'e2e-foundry-detail-release');
    const failure = join(app.dataDir, 'e2e-foundry-detail-fail');
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;
    let retries = 0;
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON();
      if (
        body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetCapacityState' &&
        body.args?.args?.[0] === true
      )
        retries += 1;
    });

    await test.step('分岐条件', async () => {
      files = seed(app);
      writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
      writeFileSync(failure, '');
      writeFileSync(release, '');
      await app.restart();
      const failed = page.waitForResponse(async (response) => {
        if (response.request().method() !== 'POST' || !response.url().includes('/wails/runtime'))
          return false;
        if (
          response.request().postDataJSON()?.args?.methodName !==
          'azfoundrydeck/internal/foundry.Service.GetCapacityState'
        )
          return false;
        const state = (await response.json()) as { error: { code: string } | null };
        return state.error?.code === 'DEPLOYMENT_DETAIL_FAILED';
      });
      await page.goto(app.url);
      await failed;
      await expect(rows).toHaveCount(3);
      await expect(details).toHaveText('Details');
      original = identity(files.stateFile);
    });

    await test.step('手順1', async () => {
      await rows.nth(0).click();
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      await expect(capacity).toHaveText('50,000 / Not set TPM');
      await expect(details.getByRole('status')).toHaveCount(0);
    });

    await test.step('手順2', async () => {
      await expect(details).toContainText('DEPLOYMENT_DETAIL_FAILED');
      await expect(retry).toBeEnabled();
      expect(retries).toBe(0);
      unlinkSync(failure);
      unlinkSync(release);
      await rows.nth(1).click();
      await expect(capacity).toHaveText('100,000 / Not set TPM');
      await expect(retry).toBeEnabled();
      expect(retries).toBe(0);
    });

    await test.step('手順3', async () => {
      await retry.click();
      await expect(capacity).toHaveText('100,000 / Loading... TPM');
      await rows.nth(0).click();
      await expect(capacity).toHaveText('50,000 / Loading... TPM');
      writeFileSync(release, '');
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
    });

    await test.step('受け入れ条件', async () => {
      await expect(details.getByRole('alert')).toHaveCount(0);
      await expect(retry).toHaveCount(0);
      await expect(details.getByRole('status')).toHaveCount(0);
      expect(retries).toBe(1);
      expect(identity(files.stateFile)).toEqual(original);
      await expect(rows).toHaveCount(3);
      unlinkSync(release);
      await rows.nth(1).click();
      await expect(capacity).toHaveText('100,000 / 250,000 TPM');
      expect(retries).toBe(1);
    });
  });
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
  writeFileSync(
    stateFile,
    JSON.stringify({
      foundries,
      selectedFoundryId: foundries[0].id,
      foundriesFetchedAt: savedAt,
    }),
  );
  return { viewDir, stateFile };
}

test.describe('容量上限の取得失敗', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'detail' } });

  test('容量上限の取得に失敗する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const retry = details.getByRole('button', { name: 'Retry', exact: true });
    const calls: string[] = [];
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON() as { args?: { methodName?: string; args?: unknown[] } };
      const method = body.args?.methodName ?? '';
      if (method.endsWith('.GetCapacityState') && body.args?.args?.[0] !== true) return;
      if (method.startsWith('azfoundrydeck/internal/foundry.Service.'))
        calls.push(method.slice(method.lastIndexOf('.') + 1));
    });
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;
    const checkRows = async () => {
      await expect(rows).toHaveCount(3);
      for (const [index, model] of models.entries())
        await expect(rows.nth(index).locator('td')).toHaveText(model);
    };
    // The other fields stay; the maximum is replaced by Not set and the error row follows.
    const checkFailed = async () => {
      await expect(details).toContainText('DEPLOYMENT_DETAIL_FAILED');
      await expect(details).toContainText('Could not retrieve the capacity maximum.');
      await expect(retry).toBeEnabled();
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      await expect(details.locator('dd').nth(0)).toHaveText('gpt-4.1');
      await expect(details.locator('dd').nth(1)).toHaveText('2025-04-14');
      await expect(details.locator('dd').nth(2)).toHaveText('GlobalStandard');
      await expect(details.locator('dd').nth(3)).toHaveText('50,000 / Not set TPM');
      await expect(details.locator('dd').nth(5)).toHaveText('Succeeded');
      await expect(details.locator('dd').nth(6)).toHaveText('Upgrade to new default');
      await expect(details.getByRole('status')).toHaveCount(0);
      await checkRows();
    };

    await test.step('分岐条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await checkRows();
      await expect(details).toHaveText('Details');
      original = identity(files.stateFile);
      calls.length = 0;
    });

    await test.step('手順1', async () => {
      await rows.nth(0).click();
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      await expect(details.locator('dd').nth(0)).toHaveText('gpt-4.1');
    });

    await test.step('手順2', async () => {
      await checkFailed();
      expect(calls).toEqual([]);
    });

    await test.step('手順3', async () => {
      await retry.click();
      await expect.poll(() => calls.length).toBe(1);
      await checkFailed();
      // Only the maximum is fetched again; the list is not.
      expect(calls).toEqual(['GetCapacityState']);
    });

    await test.step('受け入れ条件', async () => {
      expect(identity(files.stateFile)).toEqual(original);
      expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);
      // The retained failed result is displayed for another row without fetching again.
      await rows.nth(1).click();
      await expect(details.getByRole('heading', { name: 'chat-mini' })).toBeVisible();
      await expect(details).toContainText('DEPLOYMENT_DETAIL_FAILED');
      await expect(details.locator('dd').nth(3)).toHaveText('100,000 / Not set TPM');
      expect(calls.length).toBe(1);
      await checkRows();
    });
  });
});
