import { createHash } from 'node:crypto';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, type IsolatedApp } from '../../fixtures';

const foundries = [
  {
    id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast',
    name: 'contoso-foundry-production-japaneast',
    subscriptionName: 'Contoso AI Production Subscription',
    resourceGroupName: 'rg-ai-production-japaneast',
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
      tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
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
  test('デプロイ作成に失敗した場合は設定画面へ戻る', async ({ page, app }) => {
    seed(app);
    await app.restart();
    await page.goto(app.url);
    const rows = page.locator('table[aria-label="Deployments"] tbody tr');
    await expect(rows).toHaveCount(3);

    const modal = page.getByRole('dialog', { name: 'Add deployment' });

    await test.step('分岐条件', async () => {
      // 1. Add deployment モーダルを開く
      await page.getByRole('button', { name: 'Add deployment' }).click();
      await expect(modal).toBeVisible();

      // 2. fail-deploy という名称でデプロイを実行（E2Eでエラーを発生させる合成点）
      const nameInput = modal.getByLabel('Deployment name');
      await nameInput.fill('fail-deploy');
      const deployBtn = modal.getByRole('button', { name: 'Deploy', exact: true });
      await deployBtn.click();
    });

    await test.step('手順1', async () => {
      // 進捗モーダルが閉じてデプロイ作成モーダルへ戻る
      await expect(page.getByRole('dialog', { name: 'Deploying model' })).not.toBeVisible({
        timeout: 15000,
      });
      await expect(modal).toBeVisible();

      // 入力値（fail-deploy）が保持されていること
      const nameInput = modal.getByLabel('Deployment name');
      expect(await nameInput.inputValue()).toBe('fail-deploy');

      // 上部にエラーアラートが表示されていること
      const alert = modal.locator('.mantine-Alert-root');
      await expect(alert).toBeVisible();
      await expect(alert).toContainText('already exists');
    });

    await test.step('手順2', async () => {
      // 設定を修正して再度「Deploy」を押す
      const nameInput = modal.getByLabel('Deployment name');
      await nameInput.fill('retry-success');

      const deployBtn = modal.getByRole('button', { name: 'Deploy', exact: true });
      await expect(deployBtn).toBeEnabled();
      await deployBtn.click();

      // 進捗モーダルが表示され、成功して両方のモーダルが閉じる
      await expect(page.getByRole('dialog', { name: 'Deploying model' })).not.toBeVisible({
        timeout: 15000,
      });
      await expect(modal).not.toBeVisible();

      // 一覧に retry-success が追加されていること
      await expect(rows).toHaveCount(4);
      await expect(rows.filter({ hasText: 'retry-success' })).toHaveCount(1);
    });

    await test.step('受け入れ条件', async () => {
      // キャンセル時の確認: 再度モーダルを開いて Cancel を押したときに一覧が維持されること
      await page.getByRole('button', { name: 'Add deployment' }).click();
      await expect(modal).toBeVisible();
      await modal.getByRole('button', { name: 'Cancel' }).click();
      await expect(modal).not.toBeVisible();
      await expect(rows).toHaveCount(4);
    });
  });
});
