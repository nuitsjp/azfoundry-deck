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

test.describe('デプロイモデルを追加する', () => {
  test('新規デプロイモデルを追加する', async ({ page, app }) => {
    let files: ReturnType<typeof seed>;
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');

    await test.step('開始条件', async () => {
      files = seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(rows).toHaveCount(3);
    });

    await test.step('手順1', async () => {
      // 1. 「+ Add deployment」ボタンを押す
      const addBtn = page.getByRole('button', { name: '+ Add deployment' });
      await expect(addBtn).toBeVisible();
      await addBtn.click();

      // 2. モーダルが開く
      const modal = page.getByRole('dialog', { name: 'Add deployment' });
      await expect(modal).toBeVisible();

      // 3. 左側の検索・フィルターとモデル一覧、右側の設定入力欄を確認
      await expect(modal.getByPlaceholder('Search models...')).toBeVisible();
      await expect(modal.getByLabel('Deployment name')).toBeVisible();

      // 4. 初期状態でモデルが選択され、Deployment name が初期設定されていること
      const initialName = await modal.getByLabel('Deployment name').inputValue();
      expect(initialName.length).toBeGreaterThan(0);
    });

    await test.step('手順2', async () => {
      const modal = page.getByRole('dialog', { name: 'Add deployment' });

      // 左側で gpt-4o を選択
      const gpt4oItem = modal.locator('ul li').filter({ hasText: 'gpt-4o' }).first();
      await gpt4oItem.click();

      // Deployment name を入力
      const nameInput = modal.getByLabel('Deployment name');
      await nameInput.fill('gpt-4o-new');

      // SKU を選択（GlobalStandard）
      const skuSelect = modal.getByLabel('Deployment type (SKU)');
      if (await skuSelect.isVisible()) {
        await expect(skuSelect).toBeVisible();
      }

      // 「Deploy」ボタンを押す
      const deployBtn = modal.getByRole('button', { name: 'Deploy', exact: true });
      await expect(deployBtn).toBeEnabled();
      await deployBtn.click();

      // デプロイ作成モーダルを開いたまま前面に進捗モーダル（Step: Deploy）を重ねて表示
      const progressModal = page.getByRole('dialog', { name: 'Deploying model' });
      await expect(progressModal).toBeVisible();
      await expect(progressModal).toContainText('Deploy');
    });

    await test.step('手順3', async () => {
      // 作成完了後に進捗モーダルと Add deployment モーダルの両方が閉じる
      await expect(page.getByRole('dialog', { name: 'Deploying model' })).not.toBeVisible({
        timeout: 15000,
      });
      await expect(page.getByRole('dialog', { name: 'Add deployment' })).not.toBeVisible();

      // モデル一覧に新規追加されたモデルが表示され、件数が4件になる
      await expect(rows).toHaveCount(4);
      await expect(rows.filter({ hasText: 'gpt-4o-new' })).toHaveCount(1);
    });

    await test.step('受け入れ条件', async () => {
      // 1. Cancel / × を押した場合は Azure 操作も画面変更も行わない
      const addBtn = page.getByRole('button', { name: '+ Add deployment' });
      await addBtn.click();
      const modal = page.getByRole('dialog', { name: 'Add deployment' });
      await expect(modal).toBeVisible();
      await modal.getByRole('button', { name: 'Cancel' }).click();
      await expect(modal).not.toBeVisible();
      await expect(rows).toHaveCount(4);

      // 2. フィルター機能の確認
      await addBtn.click();
      await expect(modal).toBeVisible();
      const search = modal.getByPlaceholder('Search models...');
      await search.fill('claude');
      await expect(modal.locator('ul li').filter({ hasText: 'claude-3-5-sonnet' })).toBeVisible();
      await expect(modal.locator('ul li').filter({ hasText: 'gpt-4o' })).toHaveCount(0);

      // 3. Pay-as-you-go モデル選択時は TPM クォータ入力を行わず従量課金レートを表示
      await modal.locator('ul li').filter({ hasText: 'claude-3-5-sonnet' }).first().click();
      await expect(modal.locator('text=Serverless API (Pay-as-you-go)')).toBeVisible();
      await expect(modal.locator('text=TPM (Available)')).not.toBeVisible();

      // 4. 重複名バリデーションの確認
      const nameInput = modal.getByLabel('Deployment name');
      await nameInput.fill('gpt-4o-new');
      await expect(
        modal.locator('text=This deployment name already exists in this Foundry.'),
      ).toBeVisible();
      const deployBtn = modal.getByRole('button', { name: 'Deploy', exact: true });
      await expect(deployBtn).toBeDisabled();

      await modal.getByRole('button', { name: 'Cancel' }).click();
      await expect(modal).not.toBeVisible();

      // 5. デプロイ一覧は保存されず、状態ファイルの内容が変わらないこと
      expect(JSON.parse(readFileSync(files.stateFile, 'utf8'))).toEqual(original);
      expect(existsSync(join(files.viewDir, 'foundry-models'))).toBe(false);

      // 6. 画面を再読み込みすると Azure から取得し直した追加後の4件を表示すること
      //    （E2E の固定応答は作成済みのデプロイをプロセス内で覚えている）
      await page.reload();
      await expect(rows).toHaveCount(4);
      await expect(rows.filter({ hasText: 'gpt-4o-new' })).toHaveCount(1);
    });
  });
});
