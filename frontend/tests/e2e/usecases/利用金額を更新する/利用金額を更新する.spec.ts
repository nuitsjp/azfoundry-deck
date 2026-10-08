import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';

const foundry = (
  key: string,
  name: string,
  subscriptionName: string,
  resourceGroupName: string,
) => ({
  id: `/subscriptions/review-${key}/resourceGroups/${resourceGroupName}/providers/Microsoft.CognitiveServices/accounts/${name}`,
  name,
  subscriptionName,
  resourceGroupName,
});
const production = foundry(
  'production',
  'contoso-foundry-production-japaneast',
  'Contoso AI Production Subscription',
  'rg-ai-production-japaneast',
);
const development = foundry(
  'development',
  'contoso-foundry-development',
  'Contoso Development',
  'rg-ai-development',
);
const research = foundry(
  'research',
  'contoso-foundry-research',
  'Contoso Research',
  'rg-ai-research',
);
const original = {
  foundries: [production, development, research],
  selectedFoundryId: production.id,
  foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
};
const label = (value: typeof production) =>
  `${value.name} (${value.subscriptionName} - ${value.resourceGroupName})`;
const failure = 'COST_LOAD_FAILED: Could not retrieve the month-to-date cost.';

// The cost waits for its release file; the deployments and connection are released at once.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('利用金額を更新する', async ({ page, app }) => {
  const viewDir = join(
    app.dataDir,
    'azure-views',
    createHash('sha256')
      .update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant']))
      .digest('hex'),
  );
  const stateFile = join(viewDir, 'foundry-state.json');
  const release = (stage: string) =>
    writeFileSync(join(app.dataDir, `e2e-foundry-${stage}-release`), '');
  const hold = (stage: string) => rmSync(join(app.dataDir, `e2e-foundry-${stage}-release`));
  const selected = page.getByRole('button', { name: 'Foundry', exact: true });
  const refreshCost = page.getByRole('button', { name: 'Refresh cost' });
  const costValue = page.getByLabel('This month', { exact: true });
  const costLoading = page.getByRole('status', { name: 'This month loading' });
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');
  const modelsFetched = page.getByText(/ · Last fetched /);
  const foundriesFetched = page.getByText(/^Last fetched /);
  const endpoint = page.getByLabel('Azure OpenAI Endpoint', { exact: true });
  let before = { models: '', foundries: '' };

  await test.step('開始条件', async () => {
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
    mkdirSync(viewDir, { recursive: true });
    writeFileSync(stateFile, JSON.stringify(original));
    for (const stage of ['models', 'detail', 'connection', 'cost']) release(stage);
    await app.restart();
    await page.goto(app.url);
    await expect(selected).toHaveText(label(production));
    await expect(modelRows).toHaveCount(3);
    await expect(endpoint).toHaveText(
      'https://contoso-foundry-production-japaneast.openai.azure.com/openai/v1',
    );
    await expect(costValue).toHaveText('¥12,346');
    await expect(refreshCost).toBeEnabled();
    before = {
      models: await modelsFetched.innerText(),
      foundries: await foundriesFetched.innerText(),
    };
  });

  await test.step('手順1', async () => {
    hold('cost');
    await refreshCost.hover();
    await expect(page.getByRole('tooltip')).toHaveText('Refresh cost');
    await refreshCost.click();
    await expect(costLoading).toHaveText('Loading...');
    await expect(costValue).toHaveCount(0);
    await expect(refreshCost).toBeDisabled();
    await expect(page.getByRole('dialog')).toHaveCount(0);
  });

  await test.step('手順2', async () => {
    release('cost');
    await expect(costLoading).toHaveCount(0);
    await expect(costValue).toHaveText('¥12,346');
    await expect(refreshCost).toBeEnabled();
  });

  await test.step('受け入れ条件', async () => {
    // Only the cost is fetched again; the models, connection and fetch times stay as they were.
    await expect(modelRows).toHaveCount(3);
    await expect(modelsFetched).toHaveText(before.models);
    await expect(foundriesFetched).toHaveText(before.foundries);
    await expect(endpoint).toHaveText(
      'https://contoso-foundry-production-japaneast.openai.azure.com/openai/v1',
    );
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(original);

    // Other operations stay available while the cost is fetched, and switching the Foundry shows
    // the newly selected Foundry's cost instead of the refreshed one.
    hold('cost');
    await refreshCost.click();
    await expect(costLoading).toBeVisible();
    await expect(selected).toBeEnabled();
    await selected.click();
    await page.getByRole('option', { name: label(development), exact: true }).click();
    await expect(selected).toHaveText(label(development));
    await expect(costLoading).toBeVisible();
    release('cost');
    await expect(costValue).toHaveText('1,234.56 USD');
    await expect(refreshCost).toBeEnabled();

    // A failure stays inline, keeps the button enabled and is retried with it.
    await selected.click();
    await page.getByRole('option', { name: label(research), exact: true }).click();
    await expect(page.getByText(failure, { exact: true })).toBeVisible();
    await expect(refreshCost).toBeEnabled();
    await expect(page.getByRole('alert')).toHaveCount(0);
    hold('cost');
    await refreshCost.click();
    await expect(costLoading).toBeVisible();
    await expect(page.getByText(failure, { exact: true })).toHaveCount(0);
    release('cost');
    await expect(page.getByText(failure, { exact: true })).toBeVisible();
    await expect(refreshCost).toBeEnabled();

    // The cost is never saved to the data folder.
    for (const file of readdirSync(app.dataDir, { recursive: true, withFileTypes: true })) {
      if (!file.isFile()) continue;
      const text = readFileSync(join(file.parentPath, file.name), 'utf8');
      expect(text).not.toContain('12345.6');
      expect(text).not.toContain('1234.56');
    }
  });
});
