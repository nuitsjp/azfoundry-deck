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

function releaseCatalog(app: IsolatedApp, stage: string) {
  writeFileSync(join(app.dataDir, `e2e-catalog-${stage}-release`), '');
}

function catalogCalls(app: IsolatedApp) {
  const path = join(app.dataDir, 'e2e-catalog-calls');
  return existsSync(path) ? readFileSync(path, 'utf8').trim().split(/\r?\n/) : [];
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
      // 1. 「Add deployment」ボタンを押す
      const addBtn = page.getByRole('button', { name: 'Add deployment' });
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
      const addBtn = page.getByRole('button', { name: 'Add deployment' });
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

test.describe('モデル定義と共有クォータの保持', () => {
  test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_CATALOG: '1' } });

  test('初回の取得を共有し、モデル定義だけで選択を開始できる', async ({ page, app }) => {
    const modal = page.getByRole('dialog', { name: 'Add deployment', exact: true });
    const add = page.getByRole('button', { name: 'Add deployment', exact: true });

    await test.step('開始条件', async () => {
      seed(app);
      await app.restart();
      await page.goto(app.url);
      await expect(page.locator('table[aria-label="Deployments"] tbody tr')).toHaveCount(3);
      await expect.poll(() => catalogCalls(app)).toEqual(['definitions']);
    });

    await test.step('手順1', async () => {
      await add.click();
      await expect(modal.getByText('Loading model catalog...', { exact: true })).toBeVisible();
      await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
      await expect(modal).not.toBeVisible();
      await add.click();
      await expect(modal.getByText('Loading model catalog...', { exact: true })).toBeVisible();
      expect(catalogCalls(app)).toEqual(['definitions']);

      releaseCatalog(app, 'definitions');
      await expect(modal.getByPlaceholder('Search models...')).toBeVisible();
      await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o');
      await expect(modal.getByRole('status')).toHaveText('Loading...');
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toBeDisabled();
      await expect(modal.getByRole('button', { name: 'Deploy', exact: true })).toBeDisabled();
      await modal.getByText('gpt-4o-mini', { exact: true }).click();
      await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o-mini');
      await expect(modal.getByRole('status')).toHaveText('Loading...');

      releaseCatalog(app, 'quota');
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toHaveValue(
        '125000',
      );
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toBeEnabled();
      await expect(modal.getByRole('button', { name: 'Deploy', exact: true })).toBeEnabled();
    });

    await test.step('受け入れ条件', async () => {
      await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
      await expect(modal).not.toBeVisible();
      // Allow the dialog animation, but keep the previous 700 ms loading timer unexpired.
      await page.clock.install();
      await page.clock.pauseAt(new Date());
      const catalogResponse = page.waitForResponse(
        (response) =>
          response.request().postData()?.includes('foundry.Service.GetModelCatalog') === true,
      );
      await add.click();
      await page.clock.runFor(200);
      await catalogResponse;
      await page.clock.runFor(200);
      await expect(modal.getByPlaceholder('Search models...')).toBeVisible();
      await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o-mini');
      expect(catalogCalls(app)).toEqual(['definitions', 'quota']);
      await page.clock.resume();
      await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
      await expect(page.locator('table[aria-label="Deployments"] tbody tr')).toHaveCount(3);
    });
  });

  for (const result of ['reduced', 'failed'] as const) {
    test(`作成成功後にクォータを非同期更新する（${result === 'reduced' ? '上限縮小' : '取得失敗'}）`, async ({
      page,
      app,
    }) => {
      const modal = page.getByRole('dialog', { name: 'Add deployment', exact: true });
      const rows = page.locator('table[aria-label="Deployments"] tbody tr');
      const capacity = modal.getByRole('textbox', { name: 'Capacity', exact: true });
      const deploy = modal.getByRole('button', { name: 'Deploy', exact: true });

      await test.step('開始条件', async () => {
        seed(app);
        releaseCatalog(app, 'definitions');
        releaseCatalog(app, 'quota');
        await app.restart();
        await page.goto(app.url);
        await expect(rows).toHaveCount(3);
      });

      await test.step('手順1', async () => {
        await page.getByRole('button', { name: 'Add deployment', exact: true }).click();
        await expect(capacity).toBeEnabled();
        await expect(capacity).toHaveValue('80000');
      });

      await test.step('手順2', async () => {
        await modal.getByLabel('Deployment name').fill('quota-refresh-created');
        await capacity.fill('120000');
        await expect(deploy).toBeEnabled();
        await deploy.click();
      });

      await test.step('手順3', async () => {
        // The new quota response remains held while creation completes and the list updates.
        await expect(rows).toHaveCount(4);
        await expect(rows.filter({ hasText: 'quota-refresh-created' })).toHaveCount(1);
        await expect(modal).not.toBeVisible();
        await expect(page.getByRole('dialog', { name: 'Deploying model' })).not.toBeVisible();
        await expect
          .poll(() => catalogCalls(app))
          .toEqual(['definitions', 'quota', 'quota-refresh']);
      });

      await test.step('受け入れ条件', async () => {
        await page.getByRole('button', { name: 'Add deployment', exact: true }).click();
        await expect(modal.getByPlaceholder('Search models...')).toBeVisible();
        await modal.getByLabel('Deployment name').fill('next-deployment');
        await expect(modal.getByRole('status')).toHaveText('Loading...');
        await expect(capacity).toBeDisabled();
        await expect(deploy).toBeDisabled();
        await expect(capacity).toHaveValue('120000');

        writeFileSync(
          join(app.dataDir, `e2e-catalog-quota-${result === 'reduced' ? 'reduced' : 'fail'}`),
          '',
        );
        releaseCatalog(app, 'quota-refresh');
        await expect(modal.getByRole('status')).toHaveCount(0);
        await expect(capacity).toHaveValue('120000');
        await expect(modal.getByLabel('Deployment name')).toHaveValue('next-deployment');
        await expect(deploy).toBeDisabled();
        if (result === 'reduced') {
          await expect(capacity).toBeEnabled();
          await expect(modal.getByText('20,000', { exact: true })).toBeVisible();
          await expect(
            modal.getByText('Enter a capacity within the available quota, in steps of 1,000.'),
          ).toBeVisible();
          await capacity.fill('20000');
          await expect(deploy).toBeEnabled();
        } else {
          await expect(modal.getByText('Quota unavailable', { exact: true })).toBeVisible();
          await expect(modal.getByText('Could not retrieve the shared quota.')).toBeVisible();
          await expect(capacity).toBeDisabled();
          await expect(modal.getByText('160,000', { exact: true })).toHaveCount(0);
          await expect(modal.getByText('Deployment failed', { exact: true })).toHaveCount(0);
          // Quota failure does not remove the catalog or prevent model selection.
          await modal.getByText('gpt-4o-mini', { exact: true }).click();
          await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o-mini');
          await expect(deploy).toBeDisabled();
          await modal.getByText('claude-3-5-sonnet', { exact: true }).click();
          await expect(modal.getByText('Serverless API (Pay-as-you-go)')).toBeVisible();
          await expect(capacity).toHaveCount(0);
          await expect(deploy).toBeEnabled();
        }
        expect(catalogCalls(app)).toEqual(['definitions', 'quota', 'quota-refresh']);
        await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
        await expect(rows.filter({ hasText: 'quota-refresh-created' })).toHaveCount(1);
      });
    });
  }

  test('Foundryとテナントの変更時に前のカタログとフォームを破棄する', async ({ page, app }) => {
    const modal = page.getByRole('dialog', { name: 'Add deployment', exact: true });
    const add = page.getByRole('button', { name: 'Add deployment', exact: true });
    const foundryButton = page.getByRole('button', { name: 'Foundry', exact: true });

    await test.step('開始条件', async () => {
      seed(app);
      releaseCatalog(app, 'definitions');
      releaseCatalog(app, 'quota');
      await app.restart();
      await page.goto(app.url);
      await expect(page.locator('table[aria-label="Deployments"] tbody tr')).toHaveCount(3);
    });

    await test.step('手順1', async () => {
      await add.click();
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toBeEnabled();
      await modal.getByPlaceholder('Search models...').fill('gpt');
      await modal.locator('input[value="All publishers"]').click();
      await page.getByRole('option', { name: 'OpenAI', exact: true }).click();
      await modal.locator('input[value="All options"]').click();
      await page.getByRole('option', { name: 'Standard', exact: true }).click();
      await modal.getByRole('checkbox', { name: 'Multimodal', exact: true }).check();
      await modal.getByLabel('Deployment name').fill('previous-foundry-name');
      await modal.getByLabel('Model version').click();
      await page.getByRole('option', { name: '2024-08-06', exact: true }).click();
      await modal.getByLabel('Deployment type (SKU)').click();
      await page.getByRole('option', { name: 'DataZoneStandard', exact: true }).click();
      await modal.getByRole('textbox', { name: 'Capacity', exact: true }).fill('10000');
      await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
    });

    await test.step('受け入れ条件', async () => {
      await foundryButton.click();
      const development = foundries[1];
      await page
        .getByRole('option', {
          name: `${development.name} (${development.subscriptionName} - ${development.resourceGroupName})`,
          exact: true,
        })
        .click();
      await expect(foundryButton).toContainText(development.name);
      await add.click();
      await expect(modal.locator('ul li')).toHaveCount(1);
      await expect(modal.getByPlaceholder('Search models...')).toHaveValue('');
      await expect(modal.locator('input[value="All publishers"]')).toBeVisible();
      await expect(modal.locator('input[value="All options"]')).toBeVisible();
      await expect(
        modal.getByRole('checkbox', { name: 'Multimodal', exact: true }),
      ).not.toBeChecked();
      await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o-mini');
      await expect(modal.getByLabel('Model version')).toHaveValue('2024-07-18 (Default)');
      await expect(modal.getByLabel('Deployment type (SKU)')).toHaveValue('GlobalStandard');
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toHaveValue(
        '125000',
      );

      await modal.getByPlaceholder('Search models...').fill('mini');
      await modal.getByRole('checkbox', { name: 'Chat', exact: true }).check();
      await modal.getByLabel('Deployment name').fill('previous-tenant-name');
      await modal.getByRole('textbox', { name: 'Capacity', exact: true }).fill('10000');
      await modal.getByRole('button', { name: 'Cancel', exact: true }).click();
      await page.getByRole('banner').getByRole('button', { name: 'テナント', exact: true }).click();
      await page.getByRole('option', { name: 'Fabrikam', exact: true }).click();
      await expect(foundryButton).toContainText(foundries[0].name);
      await add.click();
      await expect(modal.locator('ul li')).toHaveCount(10);
      await expect(modal.getByPlaceholder('Search models...')).toHaveValue('');
      await expect(modal.locator('input[value="All publishers"]')).toBeVisible();
      await expect(modal.locator('input[value="All options"]')).toBeVisible();
      await expect(modal.getByRole('checkbox', { name: 'Chat', exact: true })).not.toBeChecked();
      await expect(modal.getByLabel('Deployment name')).toHaveValue('gpt-4o');
      await expect(modal.getByLabel('Model version')).toHaveValue('2024-11-20 (Default)');
      await expect(modal.getByLabel('Deployment type (SKU)')).toHaveValue('GlobalStandard');
      await expect(modal.getByRole('textbox', { name: 'Capacity', exact: true })).toHaveValue(
        '80000',
      );
      expect(catalogCalls(app)).toEqual([
        'definitions',
        'quota',
        'definitions',
        'quota',
        'definitions',
        'quota',
      ]);
    });
  });
});
