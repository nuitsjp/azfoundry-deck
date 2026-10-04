import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
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
// The state file holds only the Foundry list, the selection and the list's fetch time.
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

test.describe('デプロイモデルを削除する', () => {
  test('一覧からデプロイモデルを削除する', async ({ page, app }) => {
    let files: ReturnType<typeof seed>;
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(rows).toHaveCount(3);
    });

    await test.step('手順1', async () => {
      // 削除したいモデルを行クリックで選び、明細を表示
      await rows.nth(1).click();
      await expect(details.getByRole('heading', { level: 3, name: 'chat-mini' })).toBeVisible();

      // 明細の下部にある削除ボタン（ゴミ箱アイコン）を押す
      const deleteButton = details.getByRole('button', { name: 'Delete deployment' });
      await deleteButton.click();

      // 確認ダイアログの表示確認
      const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText('chat-mini');
      await expect(dialog).toContainText(foundries[0].name);
      await expect(dialog).toContainText('This cannot be undone.');

      // キャンセル操作の確認（キャンセル時は何も変更しない）
      await dialog.getByRole('button', { name: 'Cancel' }).click();
      await expect(dialog).not.toBeVisible();
      await expect(rows).toHaveCount(3);
      await expect(details.getByRole('heading', { level: 3, name: 'chat-mini' })).toBeVisible();
    });

    await test.step('手順2', async () => {
      const deleteButton = details.getByRole('button', { name: 'Delete deployment' });
      await deleteButton.click();
      const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
    });

    await test.step('手順3', async () => {
      // 削除完了後にモーダルが閉じ、モデル一覧が更新される
      const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
      await expect(dialog).not.toBeVisible();
      await expect(rows).toHaveCount(2);
      await expect(rows.nth(0)).toContainText('chat-production');
      await expect(rows.nth(1)).toContainText('embeddings');
    });

    await test.step('受け入れ条件', async () => {
      // 1. 表示中だった明細が破棄されていること
      await expect(details.getByRole('heading', { level: 3, name: 'chat-mini' })).not.toBeVisible();

      // 2. デプロイ一覧は保存されず、状態ファイルの内容が変わらないこと
      expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
      expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);

      // 3. ページを再読み込みすると Azure から取得し直した削除後の2件を表示すること
      //    （E2E の固定応答は削除済みのデプロイをプロセス内で覚えている）
      await page.reload();
      await expect(rows).toHaveCount(2);
      await expect(rows.nth(0)).toContainText('chat-production');
      await expect(rows.nth(1)).toContainText('embeddings');
    });
  });

  test.describe('削除失敗時の振る舞い', () => {
    test.use({ serverEnv: { AZFOUNDRYDECK_E2E_FAIL: 'delete' } });

    test('一覧からデプロイモデルを削除する（削除失敗）', async ({ page, app }) => {
      const rows = page.locator('table[aria-label="Deployments"] tbody tr');
      const details = page.getByRole('region', { name: 'Details', exact: true });

      await test.step('開始条件', async () => {
        seed(app);
        await app.restart();
        await page.goto(app.url);
        await expect(rows).toHaveCount(3);
      });

      await test.step('手順1', async () => {
        await rows.nth(0).click();
        await expect(
          details.getByRole('heading', { level: 3, name: 'chat-production' }),
        ).toBeVisible();
        const deleteButton = details.getByRole('button', { name: 'Delete deployment' });
        await deleteButton.click();
        const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
        await expect(dialog).toBeVisible();
      });

      await test.step('手順2', async () => {
        const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
        await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
      });

      await test.step('手順3', async () => {
        // モーダルが閉じ、エラーバナーが表示され、一覧は維持される
        const dialog = page.getByRole('dialog', { name: 'Delete deployment' });
        await expect(dialog).not.toBeVisible();
        const errorBanner = page.getByText('DEPLOYMENT_DELETE_FAILED');
        await expect(errorBanner).toBeVisible();
      });

      await test.step('受け入れ条件', async () => {
        // 一覧は3件のまま維持され、再度操作可能であること
        await expect(rows).toHaveCount(3);
        const deleteButton = details.getByRole('button', { name: 'Delete deployment' });
        await expect(deleteButton).toBeEnabled();
      });
    });
  });
});
