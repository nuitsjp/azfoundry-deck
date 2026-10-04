import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
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
const original = {
  foundries,
  selectedFoundryId: foundries[0].id,
  foundriesFetchedAt: savedAt,
};

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
  writeFileSync(stateFile, JSON.stringify(original));
  return { viewDir, stateFile };
}

test.describe('Foundryを削除する', () => {
  test('Foundry関連のみのリソースグループごと削除する', async ({ page, app }) => {
    let files: ReturnType<typeof seed>;
    const selectedButton = page.getByRole('button', { name: 'Foundry', exact: true });
    const deleteButton = page.getByRole('button', { name: 'Delete Foundry' });
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(selectedButton).toContainText(foundries[0].name);
      await expect(rows).toHaveCount(3);
    });

    await test.step('手順1', async () => {
      await expect(deleteButton).toBeVisible();
      await deleteButton.click();

      // 確認ダイアログの表示確認
      const dialog = page.getByRole('dialog', { name: 'Delete Foundry' });
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText(foundries[0].name);
      await expect(dialog).toContainText(foundries[0].resourceGroupName);
      await expect(dialog).toContainText('This cannot be undone.');

      // キャンセル時は何も削除しない
      await dialog.getByRole('button', { name: 'Cancel' }).click();
      await expect(dialog).not.toBeVisible();
      await expect(selectedButton).toContainText(foundries[0].name);
      await expect(rows).toHaveCount(3);
    });

    await test.step('手順2', async () => {
      await deleteButton.click();
      const dialog = page.getByRole('dialog', { name: 'Delete Foundry' });
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
    });

    await test.step('手順3', async () => {
      // 削除完了後にモーダルが閉じ、Foundry 一覧が更新され次の Foundry が選択される
      const dialog = page.getByRole('dialog', { name: 'Delete Foundry' });
      await expect(dialog).not.toBeVisible();
      const progressModal = page.getByRole('dialog', { name: 'Deleting Foundry' });
      await expect(progressModal).not.toBeVisible();

      // 残った Foundry が選択される
      await expect(selectedButton).toContainText(foundries[1].name);
    });

    await test.step('受け入れ条件', async () => {
      // 1. 状態ファイルが更新され、削除された Foundry が除外され次の Foundry が選択されていること
      const saved = JSON.parse(readFileSync(files.stateFile, 'utf8'));
      expect(saved.foundries.length).toBe(1);
      expect(saved.foundries[0].name).toBe(foundries[1].name);
      expect(saved.selectedFoundryId).toBe(foundries[1].id);

      // 2. ページ再読み込み後も状態が維持されていること
      await page.reload();
      await expect(selectedButton).toContainText(foundries[1].name);
    });
  });

  test.describe('削除失敗時の振る舞い', () => {
    test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'foundry-delete' } });

    test('Foundry関連のみのリソースグループごと削除する（削除失敗）', async ({ page, app }) => {
      let files: ReturnType<typeof seed>;
      const selectedButton = page.getByRole('button', { name: 'Foundry', exact: true });
      const deleteButton = page.getByRole('button', { name: 'Delete Foundry' });

      await test.step('開始条件', async () => {
        files = seed(app);
        await app.restart();
        await page.goto(app.url);
        await expect(selectedButton).toContainText(foundries[0].name);
      });

      await test.step('手順1', async () => {
        await deleteButton.click();
        const dialog = page.getByRole('dialog', { name: 'Delete Foundry' });
        await expect(dialog).toBeVisible();
      });

      await test.step('手順2', async () => {
        const dialog = page.getByRole('dialog', { name: 'Delete Foundry' });
        await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
      });

      await test.step('手順3', async () => {
        const progressModal = page.getByRole('dialog', { name: 'Deleting Foundry' });
        await expect(progressModal).not.toBeVisible();
        const errorBanner = page.getByText('FOUNDRY_DELETE_FAILED');
        await expect(errorBanner).toBeVisible();
      });

      await test.step('受け入れ条件', async () => {
        // 一覧・選択が維持され、状態ファイルも変更されていないこと
        await expect(selectedButton).toContainText(foundries[0].name);
        const saved = JSON.parse(readFileSync(files.stateFile, 'utf8'));
        expect(saved.foundries.length).toBe(2);
        expect(saved.selectedFoundryId).toBe(foundries[0].id);
      });
    });
  });
});
