import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, statSync, utimesSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '../../fixtures';
import type { InitialFoundryView } from '../../../../src/features/foundry/models';

const foundries = [
  {
    id: '/subscriptions/saved-old/resourceGroups/rg-old/providers/Microsoft.CognitiveServices/accounts/saved-old-foundry',
    name: 'saved-old-foundry',
    subscriptionName: 'Saved Original Subscription',
    resourceGroupName: 'rg-old',
  },
  {
    id: '/subscriptions/saved-target/resourceGroups/rg-target-japaneast/providers/Microsoft.CognitiveServices/accounts/saved-target-foundry-production-japaneast',
    name: 'saved-target-foundry-production-japaneast',
    subscriptionName: 'Saved Target Production Subscription',
    resourceGroupName: 'rg-target-japaneast',
  },
];
const oldModels = [
  ['original-chat', 'original-model', 'original-v1'],
  ['original-mini', 'original-mini-model', 'original-v2'],
];
const targetModels = [
  ['cached-chat', 'cached-model', 'cached-v1'],
  ['cached-mini', 'cached-mini-model', 'cached-v2'],
  ['cached-embedding', 'cached-embedding-model', 'cached-v3'],
];
const deployments = (index: number, models: string[][]) =>
  models.map(([deploymentName, modelName, version]) => ({
    id: `${foundries[index].id}/deployments/${deploymentName}`,
    deploymentName,
    modelName,
    version,
  }));
const original: InitialFoundryView = {
  foundries,
  selectedFoundryId: foundries[0].id,
  deployments: deployments(0, oldModels),
  foundriesFetchedAt: '2001-02-03T13:05:06+09:00',
  deploymentsFetchedAt: '2001-02-03T13:05:07+09:00',
};
const expected: InitialFoundryView = {
  foundries,
  selectedFoundryId: foundries[1].id,
  deployments: deployments(1, targetModels),
  foundriesFetchedAt: original.foundriesFetchedAt,
  deploymentsFetchedAt: '2001-02-03T13:05:08+09:00',
};
const labels = foundries.map(
  (foundry) => `${foundry.name}（${foundry.subscriptionName} - ${foundry.resourceGroupName}）`,
);

// No external acquisition can finish: every source gate remains unreleased.
test.use({ serverEnv: { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } });

test('Foundryを変更し再閲覧する', async ({ page, app }) => {
  const stateFile = join(app.dataDir, 'foundry-state.json');
  const cacheFile = (id: string) =>
    join(app.dataDir, 'foundry-models', `${createHash('sha256').update(id).digest('hex')}.json`);
  const cacheFiles = foundries.map((foundry) => cacheFile(foundry.id));
  const fileIdentity = (path: string) => ({
    bytes: readFileSync(path),
    mtimeMs: statSync(path).mtimeMs,
  });
  let originalCaches: ReturnType<typeof fileIdentity>[];
  const progressFrames: string[] = [];
  let changeCalls = 0;
  let requestReady = () => {};
  let releaseRequest = () => {};
  const pendingRequest = new Promise<void>((resolve) => {
    requestReady = resolve;
  });
  const requestRelease = new Promise<void>((resolve) => {
    releaseRequest = resolve;
  });
  let responseReady = () => {};
  let releaseResponse = () => {};
  const savedResponse = new Promise<void>((resolve) => {
    responseReady = resolve;
  });
  const responseRelease = new Promise<void>((resolve) => {
    releaseResponse = resolve;
  });
  page.on('websocket', (socket) => {
    socket.on('framereceived', ({ payload }) => {
      const frame = payload.toString();
      if (frame.includes('foundry:progress')) progressFrames.push(frame);
    });
  });
  await page.addInitScript(() => {
    const observations = { count: 0 };
    Reflect.set(window, 'acquisitionDialogObservations', observations);
    const inspect = (element: Element) => {
      const dialogs = [
        ...(element.matches('[role="dialog"]') ? [element] : []),
        ...element.querySelectorAll('[role="dialog"]'),
      ];
      for (const dialog of dialogs) {
        if (dialog.textContent?.includes('デプロイモデルを取得しています')) observations.count++;
      }
    };
    new MutationObserver((records) => {
      // A removed dialog still appears in the recorded added nodes.
      for (const record of records)
        for (const node of record.addedNodes) if (node instanceof Element) inspect(node);
      inspect(document.documentElement);
    }).observe(document, { childList: true, subtree: true, characterData: true });
  });
  const selected = page.locator('button[aria-label="Foundry"]');
  const dialog = page.getByRole('dialog', { name: 'デプロイモデルを取得しています' });
  const modelRows = page.locator('table[aria-label="デプロイ済みモデル"] tbody tr');
  const assertModels = async (models: string[][]) => {
    await expect(modelRows).toHaveCount(models.length);
    for (const [index, model] of models.entries())
      await expect(modelRows.nth(index).locator('td')).toHaveText(model);
  };
  const assertNoAcquisition = async () => {
    await expect(dialog).toHaveCount(0);
    expect(
      await page.evaluate(() => Reflect.get(window, 'acquisitionDialogObservations').count),
    ).toBe(0);
    expect(progressFrames).toEqual([]);
    expect(cacheFiles.map(fileIdentity)).toEqual(originalCaches);
  };
  const assertTarget = async () => {
    await expect(selected).toHaveText(labels[1]);
    await assertModels(targetModels);
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(expected);
    await assertNoAcquisition();
  };

  await test.step('分岐条件', async () => {
    writeFileSync(
      join(app.dataDir, 'e2e-authentication-record.json'),
      JSON.stringify({
        authority: 'login.microsoftonline.com',
        clientId: 'e2e-client',
        homeAccountId: 'e2e-object.e2e-tenant',
        tenantId: 'e2e-tenant',
        username: 'operator@contoso.onmicrosoft.com',
        version: '1.0',
      }),
    );
    writeFileSync(stateFile, JSON.stringify(original));
    mkdirSync(join(app.dataDir, 'foundry-models'));
    for (const [index, view] of [original, expected].entries())
      writeFileSync(
        cacheFiles[index],
        JSON.stringify(
          { fetchedAt: view.deploymentsFetchedAt, deployments: view.deployments },
          null,
          2,
        ) + '\n',
      );
    for (const file of cacheFiles)
      utimesSync(file, new Date('2001-02-03T04:05:06Z'), new Date('2001-02-03T04:05:06Z'));
    originalCaches = cacheFiles.map(fileIdentity);
    await app.restart();
    await page.goto(app.url);
    await expect(page.getByRole('banner')).toContainText('Contoso');
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    await assertNoAcquisition();
    // Hold the real request before Go reads/saves, then hold its successful response before UI commit.
    await page.route('**/wails/runtime**', async (route) => {
      const request = route.request().postDataJSON() as { args?: { methodName?: string } };
      if (request.args?.methodName !== 'azfoundrydeck/internal/foundry.Service.ChangeFoundry') {
        await route.continue();
        return;
      }
      changeCalls++;
      requestReady();
      await requestRelease;
      const response = await route.fetch();
      expect(response.ok()).toBe(true);
      responseReady();
      await responseRelease;
      await route.fulfill({ response });
    });
  });

  await test.step('手順1', async () => {
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
    for (const option of await page.getByRole('option').all()) await expect(option).toBeVisible();
  });

  await test.step('手順2', async () => {
    await page.getByRole('option', { name: labels[1], exact: true }).click();
    await pendingRequest;
    await expect(page.getByRole('option')).toHaveCount(0);
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(original);
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    await assertNoAcquisition();
    releaseRequest();
    await savedResponse;
    await expect(page.getByRole('option')).toHaveCount(0);
    // Go has saved successfully; the UI keeps its committed view until it receives that success.
    expect(JSON.parse(readFileSync(stateFile, 'utf8'))).toEqual(expected);
    await expect(selected).toHaveText(labels[0]);
    await assertModels(oldModels);
    await assertNoAcquisition();
  });

  await test.step('手順3', async () => {
    releaseResponse();
    await assertTarget();
  });

  await test.step('手順4', async () => {
    await page.setViewportSize({ width: 640, height: 720 });
    const label = selected.locator('.mantine-InputPlaceholder-placeholder');
    expect(
      await label.evaluate((element) => ({
        overflow: getComputedStyle(element).overflow,
        textOverflow: getComputedStyle(element).textOverflow,
        whiteSpace: getComputedStyle(element).whiteSpace,
        truncated: element.scrollWidth > element.clientWidth,
      })),
    ).toEqual({
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      truncated: true,
    });
    await label.hover();
    await expect(page.getByRole('tooltip')).toHaveText(labels[1]);
  });

  await test.step('受け入れ条件', async () => {
    await page.reload();
    await assertTarget();
    await app.restart();
    await page.goto(app.url);
    await assertTarget();
    const savedState = fileIdentity(stateFile);
    await selected.click();
    await expect(page.getByRole('option')).toHaveText(labels);
    await expect(page.getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true');
    await page.getByRole('option', { name: labels[1], exact: true }).click();
    await expect(page.getByRole('option')).toHaveCount(0);
    await assertTarget();
    expect(fileIdentity(stateFile)).toEqual(savedState);
    expect(changeCalls).toBe(1);
  });
});
