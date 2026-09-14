async page => {
  const baseURL = "http://127.0.0.1:9248";
  const pageErrors = [];
  const httpErrors = [];
  const checks = [];

  page.on("pageerror", error => pageErrors.push(error?.stack || error?.message || String(error)));
  page.on("response", response => {
    if (response.status() >= 400 && !response.url().endsWith("/favicon.ico")) httpErrors.push(`${response.status()} ${response.url()}`);
  });

  const assert = (condition, message) => {
    if (!condition) throw new Error(message);
  };
  const mark = name => checks.push(name);
  const waitForDeploymentIdle = () => page.waitForFunction(
    () => !(document.querySelector(".fetch-status")?.textContent || "").includes("取得中"),
    undefined,
    { timeout: 60000 },
  );
  const waitForModelIdle = () => page.waitForFunction(
    () => !(document.querySelector(".model-fetch-status")?.textContent || "").includes("取得中"),
    undefined,
    { timeout: 60000 },
  );

  await page.setViewportSize({ width: 1440, height: 920 });
  await page.goto(baseURL);
  await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  await waitForDeploymentIdle();
  assert(await page.locator(".mock-bar").count() === 0, "実Azure画面にモック設定が表示されています");
  const deploymentRows = await page.locator("tbody tr").count();
  assert(deploymentRows > 0, "モデル候補を開けるデプロイ行がありません");
  mark("f1-real-target");

  const selectedButton = page.locator(".model-candidates-button").first();
  const selectedLabel = await selectedButton.getAttribute("aria-label");
  await selectedButton.click();
  await page.getByRole("heading", { name: "モデル候補", exact: true }).waitFor();
  await waitForModelIdle();
  assert(await page.locator(".mock-bar").count() === 0, "実AzureのF2画面にモック設定が表示されています");
  assert((await page.locator(".model-target").textContent() || "").includes(selectedLabel.replace("モデル候補 ", "")), "F2の対象アカウントがF1選択行と一致しません");
  const modelErrors = await page.locator(".model-error-list .error-item").count();
  const modelRows = await page.locator(".model-candidates-table tbody tr").count();
  assert(modelErrors > 0 || modelRows > 0, "F2の実接続が成功・対象エラーのどちらも表示していません");
  mark(modelErrors > 0 ? "real-error" : "real-success");

  const targetBeforeRefresh = await page.locator(".model-target").textContent();
  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).first().click();
  await waitForModelIdle();
  assert(await page.locator(".model-target").textContent() === targetBeforeRefresh, "F2更新で対象アカウントが変わりました");
  mark("real-refresh");

  await page.getByRole("button", { name: "デプロイ一覧に戻る" }).click();
  await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  assert(await page.locator("tbody tr").count() === deploymentRows, "F2から戻った後にF1の行数が変わりました");
  mark("back");

  await page.setViewportSize({ width: 1080, height: 720 });
  const metrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
  assert(metrics.scrollWidth <= metrics.innerWidth + 1, `1080px表示で横overflowがあります: ${JSON.stringify(metrics)}`);
  assert(pageErrors.length === 0, `pageerrorが発生しました: ${pageErrors.join(" | ")}`);
  assert(httpErrors.length === 0, `HTTPエラーが発生しました: ${httpErrors.join(" | ")}`);
  return { passed: true, checks, deploymentRows, modelRows, modelErrors, pageErrors, httpErrors, userAgent: await page.evaluate(() => navigator.userAgent) };
}
