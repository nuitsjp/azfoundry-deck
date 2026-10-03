import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { FoundryProgress } from '../../../../src/features/foundry/progress';

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
const foundry = (name: string, subscriptionName: string, group: string) => ({
  id: `/subscriptions/saved-fabrikam/resourceGroups/${group}/providers/Microsoft.CognitiveServices/accounts/${name}`,
  name,
  subscriptionName,
  resourceGroupName: group,
});
// Saved for Fabrikam only. The second Foundry is selected, and the fixed source never returns them.
const foundries = [
  foundry(
    'fabrikam-foundry-production',
    'Fabrikam Production Subscription',
    'rg-fabrikam-production',
  ),
  foundry('fabrikam-foundry-research', 'Fabrikam Research Subscription', 'rg-fabrikam-research'),
];
const models = [
  ['fabrikam-research-chat', 'gpt-4.1', '2025-04-14'],
  ['fabrikam-research-embedding', 'text-embedding-3-large', '1'],
];
const deployments = models.map(([deploymentName, modelName, version]) => ({
  id: `${foundries[1].id}/deployments/${deploymentName}`,
  deploymentName,
  modelName,
  version,
}));
const savedAt = '2026-09-01T09:00:00+09:00';
const label = (item: (typeof foundries)[number]) =>
  `${item.name}（${item.subscriptionName} - ${item.resourceGroupName}）`;
// The browser runs on the same machine, so local time matches.
const displayed = (value: string) => {
  const time = new Date(value);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(time.getHours())}:${pad(time.getMinutes())}`;
};

test('テナントを変更し再閲覧する', async ({ page, app }) => {
  const recordFile = join(app.dataDir, 'e2e-authentication-record.json');
  const fabrikamDir = viewDirOf(app.dataDir, 'e2e-fabrikam-tenant');
  const fabrikamState = join(fabrikamDir, 'foundry-state.json');
  const fabrikamModels = join(
    fabrikamDir,
    'foundry-models',
    `${createHash('sha256').update(foundries[1].id).digest('hex')}.json`,
  );
  const contosoState = join(viewDirOf(app.dataDir, 'e2e-azure-tenant'), 'foundry-state.json');
  const header = page.getByRole('banner');
  const tenantButton = header.getByRole('button', { name: 'テナント', exact: true });
  const selected = page.locator('button[aria-label="Foundry"]');
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const snapshots: FoundryProgress[] = [];
  let changeCalls = 0;
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress'))
        snapshots.push(JSON.parse(frame).data as FoundryProgress);
    });
  });
  page.on('request', (request) => {
    if (!request.url().includes('/wails/runtime') || request.method() !== 'POST') return;
    const body = request.postDataJSON() as { args?: { methodName?: string } };
    if (body.args?.methodName === 'azfoundrydeck/internal/azauth.Service.ChangeTenant') {
      changeCalls++;
    }
  });
  const assertFabrikam = async () => {
    await expect(tenantButton).toHaveText('Fabrikam');
    await expect(selected).toHaveText(label(foundries[1]));
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries()) {
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
    }
    await expect(page.getByText(`Foundry一覧の最終取得 ${displayed(savedAt)}`)).toBeVisible();
    await expect(page.getByText(`2 件・最終取得 ${displayed(savedAt)}`)).toBeVisible();
  };
  let fabrikamText = '';
  let fabrikamModified = 0;
  let modelsText = '';
  let contosoText = '';

  await test.step('分岐条件', async () => {
    writeFileSync(
      recordFile,
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
    mkdirSync(join(fabrikamDir, 'foundry-models'), { recursive: true });
    writeFileSync(
      fabrikamState,
      JSON.stringify({
        foundries,
        selectedFoundryId: foundries[1].id,
        deployments,
        foundriesFetchedAt: savedAt,
        deploymentsFetchedAt: savedAt,
      }),
    );
    writeFileSync(fabrikamModels, JSON.stringify({ fetchedAt: savedAt, deployments }, null, 2));
    fabrikamText = readFileSync(fabrikamState, 'utf8');
    fabrikamModified = statSync(fabrikamState).mtimeMs;
    modelsText = readFileSync(fabrikamModels, 'utf8');
    await app.restart();
    await page.goto(app.url);
    await expect(tenantButton).toHaveText('Contoso');
    await expect(modelRows).toHaveCount(3);
    contosoText = readFileSync(contosoState, 'utf8');
  });

  const before = snapshots.length;
  await test.step('手順1', async () => {
    await tenantButton.click();
    await expect(page.getByRole('option')).toHaveText(tenants.map((tenant) => tenant.displayName));
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
  });

  await test.step('手順2', async () => {
    await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await expect
      .poll(() => JSON.parse(readFileSync(recordFile, 'utf8')).selectedTenantId)
      .toBe('e2e-fabrikam-tenant');
  });

  await test.step('手順3', async () => {
    await expect(tenantButton).toHaveText('Fabrikam');
    await expect(header).not.toContainText('Contoso');
    // 保存済みの結果を読み込み、取得の進捗モーダルは表示しない。
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(modelRows).toHaveCount(models.length);
    expect(snapshots.length).toBe(before);
  });

  await test.step('手順4', async () => {
    await assertFabrikam();
  });

  await test.step('受け入れ条件', async () => {
    // 取得も再保存もせず、保存内容を変えない。変更前のテナントの保存も保持する。
    expect(changeCalls).toBe(1);
    expect(snapshots.length).toBe(before);
    expect(readFileSync(fabrikamState, 'utf8')).toBe(fabrikamText);
    expect(statSync(fabrikamState).mtimeMs).toBe(fabrikamModified);
    expect(readFileSync(fabrikamModels, 'utf8')).toBe(modelsText);
    expect(readFileSync(contosoState, 'utf8')).toBe(contosoText);
    // 再起動後は、最後に選択したテナントと保存済みの結果を復元する。
    await app.restart();
    await page.goto(app.url);
    await assertFabrikam();
    expect(readFileSync(fabrikamState, 'utf8')).toBe(fabrikamText);
  });
});
