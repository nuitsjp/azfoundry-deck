import { existsSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Page } from '@playwright/test';
import { test, expect, type IsolatedApp } from '../../fixtures';

// The e2e server build keeps the AuthenticationRecord in the data directory instead
// of the Credential Manager, and has no persistent token cache, so the token cache
// deletion is outside these tests. Logout deletes the record file.
const username = 'operator@contoso.onmicrosoft.com';
const tenantName = 'Contoso';
const record = JSON.stringify({
  authority: 'login.microsoftonline.com',
  clientId: 'e2e-client',
  homeAccountId: 'e2e-object.e2e-tenant',
  tenantId: 'e2e-tenant',
  username,
  version: '1.0',
  tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
  selectedTenantId: 'e2e-azure-tenant',
});
const recordFile = (app: IsolatedApp) => join(app.dataDir, 'e2e-authentication-record.json');

// The fixture starts without a record; restart so the saved record signs in at startup.
async function startSignedIn(page: Page, app: IsolatedApp) {
  writeFileSync(recordFile(app), record);
  await app.restart();
  await page.goto(app.url);
  await expect(page.getByRole('banner')).toContainText(tenantName);
}

function parts(page: Page) {
  const header = page.getByRole('banner');
  const menu = page.getByRole('menu');
  return {
    header,
    menu,
    icon: header.getByRole('button', { name: 'アカウント' }),
    logout: menu.getByRole('menuitem', { name: 'ログアウト' }),
    dialog: page
      .getByRole('dialog')
      .filter({ has: page.getByRole('heading', { name: 'AzFoundryDeck' }) }),
  };
}

test('ユーザーアイコンのメニューからログアウトし、ログインモーダルに戻る', async ({
  page,
  app,
}) => {
  const { header, menu, icon, logout, dialog } = parts(page);
  await test.step('開始条件', async () => {
    await startSignedIn(page, app);
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
    await expect(icon).toBeVisible();
    await expect(dialog).toHaveCount(0);
  });
  await test.step('手順1', async () => {
    await icon.click();
    await expect(menu).toBeVisible();
    await expect(menu).toHaveText(`${username}ログアウト`);
    await expect(menu.getByRole('menuitem')).toHaveText(['ログアウト']);
  });
  await test.step('手順2', async () => {
    await logout.click();
    await expect.poll(() => existsSync(recordFile(app))).toBe(false);
  });
  await test.step('手順3', async () => {
    await expect(header).toHaveText('AzFoundryDeck');
    await expect(icon).toHaveCount(0);
    await expect(menu).toHaveCount(0);
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Azureにログイン' })).toBeVisible();
    await page.keyboard.press('Escape');
    await page.mouse.click(30, 400);
    await expect(dialog).toBeVisible();
  });
  await test.step('受け入れ条件', async () => {
    // No confirmation dialog: the only dialog is the login modal, without an error.
    await expect(dialog).toHaveCount(1);
    await expect(dialog.getByRole('alert')).toHaveCount(0);
    expect(existsSync(recordFile(app))).toBe(false);
    // After a restart the app does not sign in automatically.
    await app.restart();
    await page.goto(app.url);
    await expect(dialog.getByRole('button', { name: 'Azureにログイン' })).toBeVisible();
    await expect(dialog.getByRole('alert')).toHaveCount(0);
    await expect(header).toHaveText('AzFoundryDeck');
  });
});

test.describe('削除の失敗', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'logout' } });
  test('削除に失敗するとログイン済みのままメニュー内にエラーを出し、再度押せる', async ({
    page,
    app,
  }) => {
    const { header, menu, icon, logout, dialog } = parts(page);
    await test.step('受け入れ条件', async () => {
      await startSignedIn(page, app);
      await icon.click();
      for (let attempt = 0; attempt < 2; attempt++) {
        await logout.click();
        await expect(menu.getByRole('alert')).toHaveText(/^LOGOUT_FAILED: \S+/);
        await expect(menu).toBeVisible();
        await expect(logout).toBeEnabled();
        await expect(header).toContainText(tenantName);
        await expect(icon).toBeVisible();
        await expect(dialog).toHaveCount(0);
        expect(existsSync(recordFile(app))).toBe(true);
      }
    });
  });
});
