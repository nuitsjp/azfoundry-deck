import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';

const tenants = [
  { id: 'e2e-azure-tenant', displayName: 'Contoso' },
  { id: 'e2e-fabrikam-tenant', displayName: 'Fabrikam' },
  { id: 'e2e-northwind-tenant', displayName: 'Northwind' },
];
const homeAccountId = 'e2e-object.e2e-tenant';
const viewDirOf = (dataDir: string, tenantId: string) =>
  join(
    dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify([homeAccountId, tenantId]))
      .digest('hex'),
  );
const writeRecord = (dataDir: string) =>
  writeFileSync(
    join(dataDir, 'e2e-authentication-record.json'),
    JSON.stringify({
      authority: 'login.microsoftonline.com',
      clientId: 'e2e-client',
      homeAccountId,
      tenantId: 'e2e-tenant',
      username: 'operator@contoso.onmicrosoft.com',
      version: '1.0',
      tenants,
      selectedTenantId: 'e2e-azure-tenant',
    }),
  );

test('テナントを変更し閲覧する', async ({ page, app }) => {
  const recordFile = join(app.dataDir, 'e2e-authentication-record.json');
  const contosoState = join(viewDirOf(app.dataDir, 'e2e-azure-tenant'), 'foundry-state.json');
  const fabrikamState = join(viewDirOf(app.dataDir, 'e2e-fabrikam-tenant'), 'foundry-state.json');
  const header = page.getByRole('banner');
  const tenantButton = header.getByRole('button', { name: 'テナント', exact: true });
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const selectedFoundry = page.locator('button[aria-label="Foundry"]');
  const snapshots: { foundryPhase: string; modelPhase: string }[] = [];
  let changeCalls = 0;
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress')) snapshots.push(JSON.parse(frame).data);
    });
  });
  page.on('request', (request) => {
    if (!request.url().includes('/wails/runtime') || request.method() !== 'POST') return;
    const body = request.postDataJSON() as { args?: { methodName?: string } };
    if (body.args?.methodName === 'azfoundrydeck/internal/azauth.Service.ChangeTenant') {
      changeCalls++;
    }
  });
  const selectedTenantId = () => JSON.parse(readFileSync(recordFile, 'utf8')).selectedTenantId;
  let contosoText = '';

  await test.step('開始条件', async () => {
    writeRecord(app.dataDir);
    await app.restart();
    await page.goto(app.url);
    await expect(tenantButton).toHaveText('Contoso');
    await expect(modelRows).toHaveCount(3);
    expect(existsSync(contosoState)).toBe(true);
    expect(existsSync(fabrikamState)).toBe(false);
    contosoText = readFileSync(contosoState, 'utf8');
  });

  await test.step('手順1', async () => {
    await tenantButton.click();
    await expect(page.getByRole('option')).toHaveText(tenants.map((tenant) => tenant.displayName));
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
  });

  const before = snapshots.length;
  await test.step('手順2', async () => {
    await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await expect.poll(selectedTenantId).toBe('e2e-fabrikam-tenant');
    expect(JSON.parse(readFileSync(recordFile, 'utf8')).tenants).toEqual(tenants);
  });

  await test.step('手順3', async () => {
    await expect(tenantButton).toHaveText('Fabrikam');
    await expect(header).not.toContainText('Contoso');
    await expect
      .poll(() => snapshots.slice(before).some((s) => s.modelPhase === 'completed'))
      .toBe(true);
    await expect(page.getByRole('dialog')).toHaveCount(0);
  });

  await test.step('手順4', async () => {
    await expect(modelRows).toHaveCount(3);
    await expect(selectedFoundry).toBeVisible();
    const saved = JSON.parse(readFileSync(fabrikamState, 'utf8'));
    expect(saved.foundries.length).toBeGreaterThan(0);
    // The state file holds the Foundry list and selection only; deployments are never saved.
    expect(Object.keys(saved).sort()).toEqual([
      'foundries',
      'foundriesFetchedAt',
      'selectedFoundryId',
    ]);
    expect(existsSync(join(viewDirOf(app.dataDir, 'e2e-fabrikam-tenant'), 'foundry-models'))).toBe(
      false,
    );
    expect(readFileSync(contosoState, 'utf8')).toBe(contosoText);
  });

  await test.step('受け入れ条件', async () => {
    // 同じテナントを選んでも、呼び出し・取得・保存を行わない。
    const calls = changeCalls;
    const fabrikamText = readFileSync(fabrikamState, 'utf8');
    await tenantButton.click();
    await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    expect(changeCalls).toBe(calls);
    expect(readFileSync(fabrikamState, 'utf8')).toBe(fabrikamText);
    // 変更先に保存済みの Foundry 一覧・選択があれば、それを使い、デプロイ一覧だけを取得する。
    const northwindState = join(
      viewDirOf(app.dataDir, 'e2e-northwind-tenant'),
      'foundry-state.json',
    );
    const northwindFoundries = ['a', 'b'].map((suffix) => ({
      id: `/subscriptions/saved-northwind/resourceGroups/rg-northwind-${suffix}/providers/Microsoft.CognitiveServices/accounts/northwind-foundry-${suffix}`,
      name: `northwind-foundry-${suffix}`,
      subscriptionName: 'Northwind Subscription',
      resourceGroupName: `rg-northwind-${suffix}`,
    }));
    const northwindSaved = {
      foundries: northwindFoundries,
      selectedFoundryId: northwindFoundries[1].id,
      foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
    };
    mkdirSync(join(northwindState, '..'), { recursive: true });
    writeFileSync(northwindState, JSON.stringify(northwindSaved, null, 2));
    const fabrikamAfter = readFileSync(fabrikamState, 'utf8');
    const northwindBefore = snapshots.length;
    await tenantButton.click();
    await page.getByRole('option', { name: 'Northwind', exact: true }).click();
    await expect(tenantButton).toHaveText('Northwind');
    await expect(selectedFoundry).toHaveText(
      'northwind-foundry-b (Northwind Subscription - rg-northwind-b)',
    );
    await expect(modelRows).toHaveCount(3);
    await expect(page.getByRole('dialog')).toHaveCount(0);
    const northwindSnapshots = snapshots.slice(northwindBefore);
    expect(northwindSnapshots.length).toBeGreaterThan(0);
    expect(northwindSnapshots.every((snapshot) => snapshot.foundryPhase === 'completed')).toBe(
      true,
    );
    expect(JSON.parse(readFileSync(northwindState, 'utf8'))).toEqual(northwindSaved);
    expect(readFileSync(fabrikamState, 'utf8')).toBe(fabrikamAfter);
    // 再起動後は、最後に選択したテナントを復元する。
    await app.restart();
    await page.goto(app.url);
    await expect(tenantButton).toHaveText('Northwind');
    expect(selectedTenantId()).toBe('e2e-northwind-tenant');
  });
});

test.describe('選択保存の失敗', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'select-save' } });

  test('変更前のテナントと表示を維持し、本文先頭のバナーに表示する', async ({ page, app }) => {
    const recordFile = join(app.dataDir, 'e2e-authentication-record.json');
    const header = page.getByRole('banner');
    const tenantButton = header.getByRole('button', { name: 'テナント', exact: true });
    const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
    writeRecord(app.dataDir);
    const recordText = readFileSync(recordFile, 'utf8');
    await app.restart();
    await page.goto(app.url);
    await expect(modelRows).toHaveCount(3);

    await tenantButton.click();
    await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();

    const alert = page.getByRole('alert');
    await expect(alert).toContainText('SELECT_TENANT_FAILED');
    await expect(alert).toContainText('テナントを変更できませんでした');
    await expect(tenantButton).toHaveText('Contoso');
    await expect(modelRows).toHaveCount(3);
    expect(readFileSync(recordFile, 'utf8')).toBe(recordText);
    expect(existsSync(viewDirOf(app.dataDir, 'e2e-fabrikam-tenant'))).toBe(false);
    // 閉じる操作は右上の「×」のみ。
    await expect(alert.getByRole('button')).toHaveCount(1);
    await alert.getByRole('button').click();
    await expect(alert).toHaveCount(0);
  });
});
