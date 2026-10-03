import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, type IsolatedApp } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';

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

const models = [
  ['chat-production', 'gpt-4.1', '2025-04-14'],
  ['chat-mini', 'gpt-4.1-mini', '2025-04-14'],
  ['embeddings', 'text-embedding-3-large', '1'],
];

const savedAt = '2001-02-03T13:05:06+09:00';
const deployments = (foundry: { id: string }, modelList: string[][]) =>
  modelList.map(([deploymentName, modelName, version]) => ({
    id: `${foundry.id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  }));

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
  mkdirSync(join(viewDir, 'foundry-models'), { recursive: true });
  const productionDeployments = deployments(foundries[0], models);

  const original: InitialFoundryView = {
    foundries,
    selectedFoundryId: foundries[0].id,
    deployments: productionDeployments,
    foundriesFetchedAt: savedAt,
    deploymentsFetchedAt: savedAt,
  };
  const stateFile = join(viewDir, 'foundry-state.json');
  const productionModelFile = join(
    viewDir,
    'foundry-models',
    `${createHash('sha256').update(foundries[0].id).digest('hex')}.json`,
  );
  writeFileSync(stateFile, JSON.stringify(original));
  writeFileSync(
    productionModelFile,
    JSON.stringify({ fetchedAt: savedAt, deployments: productionDeployments }),
  );
  return { viewDir, stateFile, productionModelFile };
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

      // 5. 状態ファイルとモデルファイルに新しいデプロイが保存されていること
      const stateContent = JSON.parse(readFileSync(files.stateFile, 'utf8')) as InitialFoundryView;
      expect(stateContent.deployments).toHaveLength(4);
      expect(stateContent.deployments.some((d) => d.deploymentName === 'gpt-4o-new')).toBe(true);

      const productionModelContent = JSON.parse(
        readFileSync(files.productionModelFile, 'utf8'),
      ) as {
        deployments: { deploymentName: string }[];
      };
      expect(productionModelContent.deployments).toHaveLength(4);
      expect(
        productionModelContent.deployments.some((d) => d.deploymentName === 'gpt-4o-new'),
      ).toBe(true);

      // 6. 再起動後も追加後の4件を表示すること
      await app.restart();
      await page.goto(app.url);
      await expect(rows).toHaveCount(4);
      await expect(rows.filter({ hasText: 'gpt-4o-new' })).toHaveCount(1);
    });
  });
});
