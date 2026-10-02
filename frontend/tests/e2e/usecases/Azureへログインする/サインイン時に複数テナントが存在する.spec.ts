import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';

const tenants = [
  { id: 'e2e-azure-tenant', displayName: 'Contoso' },
  { id: 'e2e-development-tenant', displayName: 'Contoso Development' },
  { id: 'e2e-fabrikam-tenant', displayName: 'Fabrikam' },
  { id: 'e2e-northwind-tenant', displayName: 'Northwind' },
  { id: 'e2e-adventure-tenant', displayName: 'Adventure Works' },
  { id: 'e2e-woodgrove-tenant', displayName: 'Woodgrove' },
  { id: 'e2e-tailspin-tenant', displayName: 'Tailspin' },
];

test.use({ serverEnv: { AZFOUNDRYDECK_E2E_TENANTS: 'multiple' } });

test('すべての候補からテナントを選択し、保存後に閲覧を開始する', async ({ page, app }) => {
  const recordFile = join(app.dataDir, 'e2e-authentication-record.json');
  const selection = page.getByRole('dialog').filter({
    has: page.getByRole('heading', { name: 'テナントを選択' }),
  });
  const tenant = selection.getByRole('textbox', { name: 'テナント', exact: true });
  const confirm = selection.getByRole('button', { name: '確定' });
  const header = page.getByRole('banner');
  const progress: string[] = [];
  let foundryCalls = 0;
  page.on('request', (request) => {
    if (!request.url().includes('/wails/runtime') || request.method() !== 'POST') return;
    const body = request.postDataJSON() as { args?: { methodName?: string } };
    if (body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetInitialView') {
      foundryCalls++;
    }
  });
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      if (payload.toString().includes('foundry:progress')) progress.push(payload.toString());
    });
  });

  await test.step('分岐条件', async () => {
    expect(existsSync(recordFile)).toBe(false);
    await page.goto('/');
  });
  await test.step('手順1', async () => {
    await page.getByRole('button', { name: 'Azureにログイン' }).click();
    await expect(selection).toBeVisible();
    const record = JSON.parse(readFileSync(recordFile, 'utf8'));
    expect(record.tenants).toEqual(tenants);
    expect(record.selectedTenantId).toBe('');
    expect(record.tenantId).toBe('e2e-tenant');
    await expect(page.getByRole('button', { name: 'Azureにログイン' })).toHaveCount(0);
  });
  await test.step('手順2', async () => {
    await expect(tenant).toHaveValue('');
    await expect(tenant).toHaveAttribute('placeholder', 'テナントを選んでください');
    await expect(confirm).toBeDisabled();
    await page.keyboard.press('Escape');
    await page.mouse.click(5, 5);
    await expect(selection).toBeVisible();
    await expect(header.getByRole('button', { name: 'テナント', exact: true })).toHaveCount(0);
    await expect(header.locator('.mantine-Avatar-root')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveCount(0);
    expect(progress).toHaveLength(0);
    expect(foundryCalls).toBe(0);
    expect(existsSync(join(app.dataDir, 'azure-views'))).toBe(false);

    await tenant.click();
    await expect(page.getByRole('option')).toHaveText(tenants.map((item) => item.displayName));
    // Choose a candidate other than the first; the authentication tenant is not a candidate.
    await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
    await expect(tenant).toHaveValue('Fabrikam');
    await expect(confirm).toBeEnabled();
  });
  await test.step('手順3', async () => {
    let release = () => {};
    const held = new Promise<void>((resolve) => (release = resolve));
    await page.route('**/wails/runtime**', async (route) => {
      const request = route.request().postDataJSON() as { args?: { methodName?: string } };
      if (request.args?.methodName === 'azfoundrydeck/internal/azauth.Service.SelectTenant') {
        await held;
      }
      await route.continue();
    });
    try {
      await confirm.click();
      await expect(tenant).toBeDisabled();
      await expect(selection).toBeVisible();
      expect(JSON.parse(readFileSync(recordFile, 'utf8')).selectedTenantId).toBe('');
      expect(progress).toHaveLength(0);
      expect(foundryCalls).toBe(0);
      expect(existsSync(join(app.dataDir, 'azure-views'))).toBe(false);
    } finally {
      release();
      await page.unrouteAll({ behavior: 'wait' });
    }
    await expect(selection).toBeHidden();
    await expect(header.getByRole('button', { name: 'テナント', exact: true })).toHaveText(
      'Fabrikam',
    );
    await header.locator('.mantine-Avatar-root').hover();
    await expect(page.getByRole('tooltip')).toHaveText('operator@contoso.onmicrosoft.com');
    await expect(
      page.getByRole('table', { name: 'デプロイ済みモデル' }).locator('tbody tr'),
    ).toHaveCount(3);
  });
  await test.step('受け入れ条件', async () => {
    const record = JSON.parse(readFileSync(recordFile, 'utf8'));
    expect(record.tenants).toEqual(tenants);
    expect(record.selectedTenantId).toBe('e2e-fabrikam-tenant');
    expect(progress.length).toBeGreaterThan(0);
    expect(foundryCalls).toBeGreaterThan(0);
    const viewDir = join(
      app.dataDir,
      'azure-views',
      createHash('sha256')
        .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-fabrikam-tenant']))
        .digest('hex'),
    );
    expect(existsSync(join(viewDir, 'foundry-state.json'))).toBe(true);
    await header.getByRole('button', { name: 'テナント', exact: true }).click();
    await expect(page.getByRole('option')).toHaveText(tenants.map((item) => item.displayName));
    await expect(page.getByRole('option', { name: 'Fabrikam', exact: true })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });
});

test.describe('選択保存の失敗', () => {
  test.use({
    serverEnv: {
      AZFOUNDRYDECK_E2E_TENANTS: 'multiple',
      AZFOUNDRYDECK_E2E_FAIL: 'select-save',
    },
  });
  test('選択値を保持して再試行でき、閲覧を開始しない', async ({ page, app }) => {
    let foundryCalls = 0;
    page.on('request', (request) => {
      if (!request.url().includes('/wails/runtime') || request.method() !== 'POST') return;
      const body = request.postDataJSON() as { args?: { methodName?: string } };
      if (body.args?.methodName === 'azfoundrydeck/internal/foundry.Service.GetInitialView') {
        foundryCalls++;
      }
    });
    await test.step('受け入れ条件', async () => {
      await page.goto('/');
      await page.getByRole('button', { name: 'Azureにログイン' }).click();
      const selection = page.getByRole('dialog').filter({
        has: page.getByRole('heading', { name: 'テナントを選択' }),
      });
      const tenant = selection.getByRole('textbox', { name: 'テナント', exact: true });
      const confirm = selection.getByRole('button', { name: '確定' });
      await tenant.click();
      await page.getByRole('option', { name: 'Northwind', exact: true }).click();
      for (let attempt = 1; attempt <= 2; attempt++) {
        await confirm.click();
        await expect
          .poll(
            () =>
              readFileSync(join(app.dataDir, 'logs', 'app.jsonl'), 'utf8')
                .split('\n')
                .filter((line) => line.includes('azauth.SelectTenant')).length,
          )
          .toBe(attempt);
        await expect(selection.getByRole('alert')).toHaveText(/^SELECT_TENANT_FAILED: \S+/);
        await expect(selection).toBeVisible();
        await expect(tenant).toHaveValue('Northwind');
        await expect(tenant).toBeEnabled();
        await expect(confirm).toBeEnabled();
        await expect(page.getByRole('button', { name: 'Foundry', exact: true })).toHaveCount(0);
        await expect(page.getByRole('table', { name: 'デプロイ済みモデル' })).toHaveCount(0);
        await expect(page.getByRole('banner').locator('.mantine-Avatar-root')).toHaveCount(0);
        const record = JSON.parse(
          readFileSync(join(app.dataDir, 'e2e-authentication-record.json'), 'utf8'),
        );
        expect(record.tenants).toEqual(tenants);
        expect(record.selectedTenantId).toBe('');
        expect(existsSync(join(app.dataDir, 'azure-views'))).toBe(false);
        expect(foundryCalls).toBe(0);
      }
    });
  });
});
