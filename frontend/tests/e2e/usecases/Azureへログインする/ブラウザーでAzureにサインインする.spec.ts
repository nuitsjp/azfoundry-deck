import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Page } from '@playwright/test';
import { test, expect, type IsolatedApp } from '../../fixtures';

// The e2e server build answers sign-in with the account below (Entra ID / ARM boundary)
// and writes the AuthenticationRecord JSON to the data directory instead of the
// Credential Manager. Real Azure sign-in and the real Credential Manager are out of scope.
const username = 'operator@contoso.onmicrosoft.com';
const tenantName = 'Contoso';
const tenantId = 'e2e-tenant';
const recordFile = (app: IsolatedApp) => join(app.dataDir, 'e2e-authentication-record.json');

function parts(page: Page) {
  const dialog = page.getByRole('dialog');
  return {
    header: page.getByRole('banner'),
    dialog,
    login: dialog.getByRole('button', { name: 'Azureにログイン' }),
    avatar: page.getByRole('banner').locator('.mantine-Avatar-root'),
  };
}

test('ブラウザーでサインインし、テナント名とアカウントを表示して保存する', async ({
  page,
  app,
}) => {
  const { header, dialog, login, avatar } = parts(page);
  let release = () => {};
  await test.step('開始条件', async () => {
    expect(existsSync(recordFile(app))).toBe(false);
  });
  await test.step('手順1', async () => {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button')).toHaveCount(1);
    await expect(login).toBeVisible();
    await page.keyboard.press('Escape');
    await page.mouse.click(30, 400);
    await expect(dialog).toBeVisible();
    await expect(avatar).toHaveCount(0);
  });
  await test.step('手順2', async () => {
    // Hold the Login call so the waiting state can be observed.
    const held = new Promise<void>((resolve) => (release = resolve));
    await page.route('**/wails/runtime**', async (route) => {
      await held;
      await route.continue();
    });
    await login.click();
    await expect(dialog.getByRole('status')).toHaveText('ブラウザーでサインインしてください');
    await expect(login).toHaveCount(0);
  });
  await test.step('手順3', async () => {
    release();
    await page.unrouteAll({ behavior: 'wait' });
    await expect.poll(() => existsSync(recordFile(app))).toBe(true);
  });
  await test.step('手順4', async () => {
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
    await expect(header).toContainText(tenantName);
    await avatar.hover();
    await expect(page.getByRole('tooltip')).toHaveText(username);
  });
  await test.step('受け入れ条件', async () => {
    const record = JSON.parse(readFileSync(recordFile(app), 'utf8')) as Record<string, string>;
    expect(record.username).toBe(username);
    expect(record.tenantId).toBe(tenantId);
    await expect(page.locator('body')).not.toContainText(tenantId);
  });
});

for (const [failure, label] of [
  ['signin', 'トークン取得・テナント名取得'],
  ['save', '保存'],
] as const) {
  test.describe(`${label}の失敗`, () => {
    test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: failure } });
    test(`${label}に失敗するとログイン済みにならず再試行できる`, async ({ page, app }) => {
      const { dialog, login, avatar } = parts(page);
      await test.step('受け入れ条件', async () => {
        await page.goto('/');
        await login.click();
        const alert = dialog.getByRole('alert');
        await expect(alert).toHaveText(/^LOGIN_FAILED: \S+/);
        await expect(dialog).toBeVisible();
        await expect(login).toBeEnabled();
        await expect(avatar).toHaveCount(0);
        expect(existsSync(recordFile(app))).toBe(false);
        await login.click();
        await expect(alert).toHaveText(/^LOGIN_FAILED: \S+/);
        await expect(login).toBeEnabled();
      });
    });
  });
}
