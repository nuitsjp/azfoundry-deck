import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
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
const developmentModels = [
  ['saved-development-chat', 'saved-development-model', 'saved-version-3'],
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
  const developmentDeployments = deployments(foundries[1], developmentModels);

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
  const developmentModelFile = join(
    viewDir,
    'foundry-models',
    `${createHash('sha256').update(foundries[1].id).digest('hex')}.json`,
  );
  writeFileSync(stateFile, JSON.stringify(original));
  writeFileSync(
    productionModelFile,
    JSON.stringify({ fetchedAt: savedAt, deployments: productionDeployments }),
  );
  writeFileSync(
    developmentModelFile,
    JSON.stringify({ fetchedAt: savedAt, deployments: developmentDeployments }),
  );
  return { viewDir, stateFile, productionModelFile, developmentModelFile };
}

test.describe('デプロイモデルを削除する', () => {
  test('一覧からデプロイモデルを削除する', async ({ page, app }) => {
    let files: ReturnType<typeof seed>;
    let developmentFileBefore: { text: string; mtimeMs: number };
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    const details = page.getByRole('region', { name: 'Details', exact: true });

    await test.step('開始条件', async () => {
      files = seed(app);
      developmentFileBefore = {
        text: readFileSync(files.developmentModelFile, 'utf8'),
        mtimeMs: statSync(files.developmentModelFile).mtimeMs,
      };
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

      // 2. 状態ファイルとモデルファイルが更新され、chat-mini が含まれていないこと
      const stateContent = JSON.parse(readFileSync(files.stateFile, 'utf8')) as InitialFoundryView;
      expect(stateContent.deployments).toHaveLength(2);
      expect(stateContent.deployments.map((d) => d.deploymentName)).toEqual([
        'chat-production',
        'embeddings',
      ]);
      const productionModelContent = JSON.parse(
        readFileSync(files.productionModelFile, 'utf8'),
      ) as {
        deployments: { deploymentName: string }[];
      };
      expect(productionModelContent.deployments).toHaveLength(2);
      expect(productionModelContent.deployments.map((d) => d.deploymentName)).toEqual([
        'chat-production',
        'embeddings',
      ]);

      // 3. ほかの Foundry のモデルファイルは変更されていないこと
      expect(readFileSync(files.developmentModelFile, 'utf8')).toBe(developmentFileBefore.text);
      expect(statSync(files.developmentModelFile).mtimeMs).toBe(developmentFileBefore.mtimeMs);

      // 4. ページを再読み込みしても削除後の2件を表示すること
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
