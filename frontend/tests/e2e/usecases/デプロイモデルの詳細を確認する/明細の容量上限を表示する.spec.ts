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
  {
    id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development',
    name: 'contoso-foundry-development',
    subscriptionName: 'Contoso Development',
    resourceGroupName: 'rg-ai-development',
  },
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
  writeFileSync(join(app.dataDir, 'e2e-foundry-models-release'), '');
  return { viewDir, stateFile };
}

test.describe('容量上限の表示', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

  test('明細の容量上限を表示する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const selected = page.getByRole('button', { name: 'Foundry', exact: true });
    const refresh = page.getByRole('button', { name: 'Refresh models', exact: true });
    const tenant = page.getByRole('button', { name: 'テナント', exact: true });
    const capacity = details.locator('dd').nth(3);
    const release = join(app.dataDir, 'e2e-foundry-detail-release');
    let maximumRequests = 0;
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON() as { args?: { methodName?: string } };
      if (body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetCapacityMaximum')
        maximumRequests += 1;
    });
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;

    await test.step('分岐条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(selected).toHaveText(label(0));
      await expect(rows).toHaveCount(3);
      await expect(details).toHaveText('Details');
      original = identity(files.stateFile);
    });

    await test.step('手順1', async () => {
      await rows.nth(0).locator('td').nth(1).click();
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      await expect(details.locator('dd')).toHaveText([
        'gpt-4.1',
        '2025-04-14',
        'GlobalStandard',
        /^50,000 \/ Loading\.\.\. TPM$/,
        'Succeeded',
        'Upgrade to new default',
      ]);
      await expect(details.getByRole('status')).toHaveText('Loading...');
      await expect(details).toHaveAttribute('aria-busy', 'true');
      await expect(rows.nth(0).getByRole('button')).toHaveAttribute('aria-pressed', 'true');
      for (const button of await rows.getByRole('button').all())
        await expect(button).toBeDisabled();
      await expect(selected).toBeDisabled();
      await expect(refresh).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Refresh Foundries' })).toBeDisabled();
      await expect(tenant).toBeDisabled();
      await expect(details.getByRole('button', { name: 'Edit deployment' })).toBeDisabled();
      await expect(details.getByRole('button', { name: 'Delete deployment' })).toBeDisabled();
      // A click in another row is ignored while the first fetch is pending.
      await rows.nth(1).locator('td').nth(2).click();
      expect(maximumRequests).toBe(1);
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      expect(identity(files.stateFile)).toEqual(original);
    });

    await test.step('手順2', async () => {
      writeFileSync(release, '');
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
      await expect(details.getByRole('status')).toHaveCount(0);
      await expect(details).toHaveAttribute('aria-busy', 'false');
      await expect(selected).toBeEnabled();
      await expect(refresh).toBeEnabled();
      await expect(tenant).toBeEnabled();
      await expect(details.getByRole('button', { name: 'Edit deployment' })).toBeEnabled();
    });

    await test.step('手順3', async () => {
      // Without the release file a new fetch would hang, so an immediate value proves the reuse.
      unlinkSync(release);
      await rows.nth(1).click();
      await expect(details.getByRole('heading', { name: 'chat-mini' })).toBeVisible();
      await expect(capacity).toHaveText('100,000 / 250,000 TPM');
      await rows.nth(2).click();
      await expect(details.getByRole('heading', { name: 'embeddings' })).toBeVisible();
      await expect(capacity).toHaveText('20,000 / 80,000 TPM');
      await rows.nth(0).click();
      await expect(details.getByRole('heading', { name: 'chat-production' })).toBeVisible();
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
      await expect(details.getByRole('status')).toHaveCount(0);
      await expect(refresh).toBeEnabled();
    });

    await test.step('受け入れ条件', async () => {
      // A refresh keeps the retained result: the maximum shows at once with the fetch held.
      await refresh.click();
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(details).toHaveText('Details');
      await rows.nth(1).click();
      await expect(capacity).toHaveText('100,000 / 250,000 TPM');
      // Changing the Foundry discards it, for the new Foundry and again when coming back.
      await selected.click();
      await page.getByRole('option', { name: label(1), exact: true }).click();
      await expect(selected).toHaveText(label(1));
      await expect(details).toHaveText('Details');
      await rows.nth(0).click();
      await expect(details.getByRole('heading', { name: 'development-chat' })).toBeVisible();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      writeFileSync(release, '');
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
      unlinkSync(release);
      await selected.click();
      await page.getByRole('option', { name: label(0), exact: true }).click();
      await expect(selected).toHaveText(label(0));
      await rows.nth(0).click();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      writeFileSync(release, '');
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
      unlinkSync(release);
      // Changing the tenant discards it. Its Foundry list is discovered first.
      writeFileSync(join(app.dataDir, 'e2e-foundry-discovery-release'), '');
      await tenant.click();
      await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
      await expect(tenant).toHaveText('Fabrikam');
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(rows).toHaveCount(3);
      await rows.nth(0).click();
      await expect(details.getByRole('status')).toHaveText('Loading...');
      writeFileSync(release, '');
      await expect(capacity).toHaveText('50,000 / 160,000 TPM');
      // Nothing of the details is saved.
      const state = JSON.parse(readFileSync(files.stateFile, 'utf8')) as Record<string, unknown>;
      expect(Object.keys(state).sort()).toEqual([
        'foundries',
        'foundriesFetchedAt',
        'selectedFoundryId',
      ]);
      expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);
    });
  });
});
