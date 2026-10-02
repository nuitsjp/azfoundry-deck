import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Page } from '@playwright/test';
import { test, expect, type IsolatedApp } from '../../fixtures';

// The e2e server build restores from the record file in the data directory instead
// of the Credential Manager, and answers with tenant name Contoso instead of Entra ID / ARM.
// It leaves e2e-signin-called when the browser sign-in runs, and with
// AZFOUNDRYDECK_E2E_HOLD_RESTORE=1 waits for e2e-restore-release before answering.
const username = 'operator@contoso.onmicrosoft.com';
const tenantName = 'Contoso';
const record = JSON.stringify({
  authority: 'login.microsoftonline.com',
  clientId: 'e2e-client',
  homeAccountId: 'e2e-object.e2e-tenant',
  tenantId: 'e2e-tenant',
  username,
  version: '1.0',
});
const file = (app: IsolatedApp, name: string) => join(app.dataDir, name);
const recordFile = (app: IsolatedApp) => file(app, 'e2e-authentication-record.json');
const signInCalled = (app: IsolatedApp) => existsSync(file(app, 'e2e-signin-called'));

// The fixture starts without a record; restart so the saved record is read at startup.
async function startWithSavedRecord(app: IsolatedApp) {
  writeFileSync(recordFile(app), record);
  await app.restart();
}

function parts(page: Page) {
  const header = page.getByRole('banner');
  return {
    header,
    dialog: page.getByRole('dialog'),
    avatar: header.locator('.mantine-Avatar-root'),
    loaders: page.locator('.mantine-Loader-root'),
  };
}

test.describe('復元に成功する', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_RESTORE: '1' } });
  test('保存済みのログイン情報でブラウザー操作なしにログイン済みになる', async ({ page, app }) => {
    const { header, dialog, avatar, loaders } = parts(page);
    await test.step('分岐条件', async () => {
      await startWithSavedRecord(app);
      expect(existsSync(recordFile(app))).toBe(true);
    });
    await test.step('手順1', async () => {
      await page.goto(app.url);
      await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
      // The restore is held here, so this is the screen while it is in progress.
      await page.waitForTimeout(1000);
      await expect(dialog).toHaveCount(0);
      await expect(loaders).toHaveCount(0);
      await expect(header).toHaveText('AzFoundryDeck');
      expect(signInCalled(app)).toBe(false);
      writeFileSync(file(app, 'e2e-restore-release'), '');
    });
    await test.step('手順2', async () => {
      await expect(header).toContainText(tenantName);
      await avatar.hover();
      await expect(page.getByRole('tooltip')).toHaveText(username);
    });
    await test.step('受け入れ条件', async () => {
      // Same screen as after a browser sign-in: Home, tenant name and account icon, no modal.
      await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
      await expect(dialog).toHaveCount(0);
      await expect(loaders).toHaveCount(0);
      expect(signInCalled(app)).toBe(false);
    });
  });
});

test.describe('復元に失敗する', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'restore' } });
  test('ログイン済みにならず、記録を残したままモーダルにエラーを出す', async ({ page, app }) => {
    const { dialog, avatar } = parts(page);
    await test.step('受け入れ条件', async () => {
      await startWithSavedRecord(app);
      await page.goto(app.url);
      await expect(dialog.getByRole('alert')).toHaveText(/^LOGIN_FAILED: \S+/);
      await expect(dialog.getByRole('button', { name: 'Azureにログイン' })).toBeEnabled();
      await expect(avatar).toHaveCount(0);
      expect(signInCalled(app)).toBe(false);
      expect(readFileSync(recordFile(app), 'utf8')).toBe(record);
      // The next manual login still works; it also shows the sign-in probe above is live.
      await dialog.getByRole('button', { name: 'Azureにログイン' }).click();
      await expect(dialog).toHaveCount(0);
      await expect(avatar).toBeVisible();
      expect(signInCalled(app)).toBe(true);
    });
  });
});
