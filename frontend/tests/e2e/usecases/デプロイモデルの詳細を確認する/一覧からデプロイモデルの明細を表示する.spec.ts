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

// The saved file holds only the Foundry list and the selection. The deployments are fetched on
// startup, so the models gate is released up front.
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

test.describe('明細表示', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

  test('一覧からデプロイモデルの明細を表示する', async ({ page, app }) => {
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });
    const selected = page.getByRole('button', { name: 'Foundry', exact: true });
    const refresh = page.getByRole('button', { name: 'Refresh models', exact: true });
    const release = join(app.dataDir, 'e2e-foundry-detail-release');
    const calls: string[] = [];
    page.on('request', (request) => {
      if (request.method() !== 'POST' || !request.url().includes('/wails/runtime')) return;
      const body = request.postDataJSON() as { args?: { methodName?: string } };
      const method = body.args?.methodName ?? '';
      if (method.startsWith('azfoundrydeck/internal/foundry.Service.'))
        calls.push(method.slice(method.lastIndexOf('.') + 1));
    });
    let files: ReturnType<typeof seed>;
    let original: ReturnType<typeof identity>;
    const checkRows = async () => {
      await expect(rows).toHaveCount(3);
      for (const [index, model] of models.entries())
        await expect(rows.nth(index).locator('td:nth-child(-n + 3)')).toHaveText(model);
    };
    const checkDetail = async (name: string, capacity: string | RegExp, policy: string) => {
      const model = models.find((item) => item[0] === name)!;
      await expect(details.getByRole('heading', { name, exact: true })).toBeVisible();
      await expect(details.locator('dd')).toHaveText([
        model[1],
        model[2],
        name === 'embeddings' ? 'Standard' : 'GlobalStandard',
        capacity,
        'Succeeded',
        policy,
      ]);
    };

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(selected).toHaveText(label(0));
      await checkRows();
      await expect(
        page.getByText(/^3 · Last fetched \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/),
      ).toBeVisible();
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      original = identity(files.stateFile);
      calls.length = 0;
    });

    await test.step('手順1', async () => {
      // Selecting the Model cell exercises the row outside the name button. The capacity maximum
      // is held, yet everything else is already displayed from the fetched list.
      await rows.nth(0).locator('td').nth(1).click();
      await checkDetail(
        'chat-production',
        /^50,000 \/ Loading\.\.\. TPM$/,
        'Upgrade to new default',
      );
      await expect(rows.nth(0).getByRole('button')).toHaveAttribute('aria-pressed', 'true');
      await expect(selected).toHaveText(label(0));
      await checkRows();
      expect(calls.filter((method) => method !== 'GetCapacityState')).toEqual([]);
      expect(identity(files.stateFile)).toEqual(original);
      writeFileSync(release, '');
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      await expect(refresh).toBeEnabled();
    });

    await test.step('手順2', async () => {
      await rows.nth(0).locator('td').nth(2).click();
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
      // Showing details never fetches the list again.
      expect(new Set(calls.filter((method) => method !== 'GetCapacityState'))).toEqual(new Set());
    });

    await test.step('受け入れ条件', async () => {
      expect(identity(files.stateFile)).toEqual(original);
      const state = JSON.parse(readFileSync(files.stateFile, 'utf8')) as Record<string, unknown>;
      expect(Object.keys(state).sort()).toEqual([
        'foundries',
        'foundriesFetchedAt',
        'selectedFoundryId',
      ]);
      expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);
      await expect(details.getByText('Last fetched')).toHaveCount(0);
      await expect(details.locator('footer')).toHaveCount(0);
      const left = await page.locator('.deployment-list-pane').boundingBox();
      const right = await details.boundingBox();
      expect(Math.abs(left!.width - right!.width * 1.5)).toBeLessThanOrEqual(1);
      expect(left!.y).toBe(right!.y);
      await expect(details.locator('dt')).toHaveText([
        'Model',
        'Version',
        'SKU',
        'Capacity',
        'Provisioning state',
        'Upgrade policy',
      ]);
      // A refresh discards the selection and the displayed details.
      await refresh.click();
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      await rows.nth(0).click();
      await checkDetail('chat-production', '50,000 / 160,000 TPM', 'Upgrade to new default');
      // Changing the Foundry discards them too.
      await selected.click();
      await page.getByRole('option', { name: label(1), exact: true }).click();
      await expect(selected).toHaveText(label(1));
      await expect(details).toHaveText('Details');
      await expect(rows.locator('button[aria-pressed="true"]')).toHaveCount(0);
      await rows.nth(0).click();
      await expect(
        details.getByRole('heading', { name: 'development-chat', exact: true }),
      ).toBeVisible();
      // So does the restart.
      await app.restart();
      await page.goto(app.url);
      await expect(selected).toHaveText(label(1));
      await expect(rows).toHaveCount(3);
      await expect(details).toHaveText('Details');
      await rows.nth(0).click();
      await expect(
        details.getByRole('heading', { name: 'development-chat', exact: true }),
      ).toBeVisible();
      // Changing the tenant discards them. Its Foundry list is discovered first.
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
      unlinkSync(release);
    });
  });
});
