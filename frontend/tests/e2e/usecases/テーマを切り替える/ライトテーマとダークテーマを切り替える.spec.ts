import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';

const record = JSON.stringify({
  authority: 'login.microsoftonline.com',
  clientId: 'e2e-client',
  homeAccountId: 'e2e-object.e2e-tenant',
  tenantId: 'e2e-tenant',
  username: 'operator@contoso.onmicrosoft.com',
  version: '1.0',
  tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
  selectedTenantId: 'e2e-azure-tenant',
});

test('ライトテーマとダークテーマを切り替える', async ({ page, app }) => {
  const settingsFile = join(app.dataDir, 'settings.json');
  const recordFile = join(app.dataDir, 'e2e-authentication-record.json');
  const toLightButton = page.getByRole('button', { name: 'ライトテーマに切り替え' });
  const toDarkButton = page.getByRole('button', { name: 'ダークテーマに切り替え' });
  const detailsPane = page.getByRole('region', { name: 'Details', exact: true });
  const modelRows = page.locator('table[aria-label="Deployments"] tbody tr');

  await test.step('開始条件', async () => {
    writeFileSync(recordFile, record);
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(modelRows).toHaveCount(3);
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'dark');
    expect(existsSync(settingsFile)).toBe(false);
  });

  await test.step('手順1', async () => {
    await toLightButton.hover();
    await expect(page.getByRole('tooltip')).toHaveText('ライトテーマに切り替え');
  });

  await test.step('手順2', async () => {
    await toLightButton.click();
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'light');
    await expect(toDarkButton).toBeVisible();
    await toDarkButton.hover();
    await expect(page.getByRole('tooltip')).toHaveText('ダークテーマに切り替え');
    await expect.poll(() => existsSync(settingsFile)).toBe(true);
    expect(JSON.parse(readFileSync(settingsFile, 'utf8'))).toEqual({ theme: 'light' });
  });

  await test.step('手順3', async () => {
    // Select a row to verify details pane and open Add deployment modal in light theme.
    await page.getByRole('button', { name: 'chat-production' }).click();
    await expect(detailsPane).toContainText('gpt-4.1');
    const bodyBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    expect(bodyBg).toBe('rgb(255, 255, 255)');

    await page.getByRole('button', { name: 'Add deployment' }).click();
    const addDialog = page.getByRole('dialog', { name: 'Add deployment' });
    await expect(addDialog).toBeVisible();
    await expect(addDialog.getByText('gpt-4o-mini').first()).toBeVisible();
    await addDialog.getByRole('button', { name: 'Cancel' }).click();
    await expect(addDialog).toHaveCount(0);
  });

  await test.step('手順4', async () => {
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'light');
    await expect(toDarkButton).toBeVisible();
  });

  await test.step('手順5', async () => {
    await toDarkButton.click();
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'dark');
    await expect(toLightButton).toBeVisible();
    await expect.poll(() => JSON.parse(readFileSync(settingsFile, 'utf8')).theme).toBe('dark');
  });

  await test.step('受け入れ条件', async () => {
    // When saving fails while signed in, keep the previous theme and show THEME_SAVE_FAILED banner.
    rmSync(settingsFile, { force: true });
    mkdirSync(join(settingsFile, 'block'), { recursive: true });
    await toLightButton.click();
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'dark');
    await expect(page.getByRole('alert')).toContainText('THEME_SAVE_FAILED');

    // Recovering lets the toggle succeed and clears the error banner.
    rmSync(settingsFile, { recursive: true, force: true });
    await page.getByRole('button', { name: 'chat-production' }).click();
    await expect(detailsPane).toContainText('gpt-4.1');
    await toLightButton.click();
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'light');
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(modelRows).toHaveCount(3);
    await expect(detailsPane).toContainText('gpt-4.1');

    // Logging out does not delete settings.json and theme toggle remains usable on login modal.
    await page.getByRole('button', { name: 'アカウント' }).click();
    await page.getByRole('menuitem', { name: 'ログアウト' }).click();
    await expect(
      page.getByRole('dialog').getByRole('button', { name: 'Azureにログイン' }),
    ).toBeVisible();
    expect(existsSync(settingsFile)).toBe(true);
    expect(JSON.parse(readFileSync(settingsFile, 'utf8'))).toEqual({ theme: 'light' });
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'light');

    await toDarkButton.click();
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'dark');
    await expect.poll(() => JSON.parse(readFileSync(settingsFile, 'utf8')).theme).toBe('dark');

    // When loading corrupted settings.json fails, stay dark and show THEME_LOAD_FAILED.
    writeFileSync(settingsFile, '{broken');
    await app.restart();
    await page.goto(app.url);
    await expect(page.locator('html')).toHaveAttribute('data-mantine-color-scheme', 'dark');
    await expect(page.getByRole('dialog').getByRole('alert')).toHaveText(/^THEME_LOAD_FAILED: \S+/);
  });
});
