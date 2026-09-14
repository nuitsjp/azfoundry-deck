async page => {
  const baseURL = "http://127.0.0.1:9246";
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
  const modelScenario = () => page.getByRole("combobox", { name: "モデル候補の次回の取得状態" });
  const update = () => page.getByRole("button", { name: /(?:更新|再取得)$/ }).first();
  const waitForModelIdle = (timeout = 5000) => page.waitForFunction(
    () => !(document.querySelector(".model-fetch-status")?.textContent || "").includes("取得中"),
    undefined,
    { timeout },
  );
  const waitForRows = expected => page.waitForFunction(
    expectedCount => document.querySelectorAll("tbody tr").length === expectedCount,
    expected,
    { timeout: 5000 },
  );
  const selectModelScenarioAndRefresh = async value => {
    await modelScenario().selectOption(value);
    await update().click();
  };

  await page.setViewportSize({ width: 1440, height: 920 });
  await page.goto(baseURL);
  await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  await page.waitForFunction(
    () => (document.querySelector(".fetch-status")?.textContent || "").includes("10 / 10 アカウント取得成功"),
    undefined,
    { timeout: 10000 },
  );
  await page.locator("input[type=search]").fill("chat");
  await page.waitForFunction(() => document.querySelectorAll("tbody tr").length === 10, undefined, { timeout: 5000 });
  const deploymentMeta = await page.locator(".update-meta span").last().textContent();

  await page.locator(".model-candidates-button").first().click();
  await page.getByRole("heading", { name: "モデル候補", exact: true }).waitFor();
  await page.waitForFunction(
    () => (document.querySelector(".model-fetch-status")?.textContent || "").includes("モデル候補の取得完了"),
    undefined,
    { timeout: 5000 },
  );
  await waitForRows(6);
  assert((await page.locator(".model-target").textContent() || "").includes("contoso-chat-prod"), "選択したアカウント名が表示されていません");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "6件", "モデル候補件数が6件ではありません");
  assert(await page.locator(".model-candidates-table tbody tr").filter({ hasText: "gpt-4o" }).count() === 2, "gpt-4oの全バージョンが保持されていません");
  assert(await page.getByText("Deprecating", { exact: true }).count() === 1, "ライフサイクルが表示されていません");
  assert(await page.getByText("既定", { exact: true }).count() === 3, "既定バージョンの表示数が不正です");
  assert(await page.getByText("GlobalStandard", { exact: true }).count() >= 3 && await page.getByText("Standard", { exact: true }).count() >= 2, "利用可能SKUが表示されていません");
  assert(await page.locator(".model-candidates-table tbody tr").filter({ hasText: "custom-model" }).getByText("不明", { exact: true }).count() >= 3, "欠損モデル属性が不明として表示されていません");
  mark("success-and-display");

  const modelSearch = page.getByRole("searchbox", { name: "モデル候補を検索" });
  await modelSearch.fill("gpt-4o");
  await waitForRows(2);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "2/6件", "モデル名検索の件数表示が不正です");
  await modelSearch.fill("2024-11-20");
  await waitForRows(1);
  assert((await page.locator(".model-candidates-table tbody tr").first().textContent() || "").includes("Deprecating"), "バージョン検索結果が不正です");
  await modelSearch.fill("");
  await waitForRows(6);
  mark("search");

  await selectModelScenarioAndRefresh("empty");
  await waitForModelIdle();
  await waitForRows(0);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "0件", "モデル候補emptyの件数が不正です");
  assert((await page.locator(".model-empty-state h3").textContent() || "").includes("モデル候補はありません"), "候補なしの説明が不正です");
  mark("empty");

  await page.getByRole("button", { name: "デプロイ一覧に戻る" }).click();
  await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  await page.waitForFunction(() => document.querySelectorAll("tbody tr").length === 10, undefined, { timeout: 5000 });
  assert(await page.locator("input[type=search]").inputValue() === "chat", "モデル画面から戻った後にF1の検索条件が消えました");
  assert(await page.locator(".update-meta span").last().textContent() === deploymentMeta, "モデル画面から戻った後にF1取得時刻が変わりました");
  mark("back-preserves-f1");

  await page.locator(".model-candidates-button").first().click();
  await page.getByRole("heading", { name: "モデル候補", exact: true }).waitFor();
  await waitForModelIdle();
  assert(await modelScenario().inputValue() === "empty", "F2モック状態が再遷移後に保持されていません");
  await selectModelScenarioAndRefresh("failure");
  await waitForModelIdle();
  await waitForRows(0);
  assert(await page.locator(".model-error-list .error-item").count() === 1, "F2失敗の詳細件数が1ではありません");
  assert(await page.getByText("model-catalog-forbidden", { exact: true }).count() === 1, "F2失敗コードが表示されていません");
  assert((await page.locator(".model-error-list").textContent() || "").includes("contoso-chat-prod"), "F2失敗対象アカウントが表示されていません");
  mark("failure");

  await selectModelScenarioAndRefresh("success");
  await waitForModelIdle();
  await waitForRows(6);
  assert(await page.locator(".model-error-list").count() === 0, "成功再試行後もF2エラーが残っています");
  mark("retry");

  await selectModelScenarioAndRefresh("loading");
  await page.getByText("モデル候補を取得中…", { exact: true }).waitFor();
  assert(await page.locator(".model-candidates-table tbody tr").count() === 6, "読込中に前回の候補が表示されていません");
  await selectModelScenarioAndRefresh("success");
  await waitForModelIdle();
  await waitForRows(6);
  mark("loading");

  await selectModelScenarioAndRefresh("delayed");
  await page.getByText("モデル候補を取得中…", { exact: true }).waitFor();
  assert(await page.locator(".model-candidates-table tbody tr").count() === 6, "遅延中に前回の候補が表示されていません");
  await selectModelScenarioAndRefresh("success");
  await waitForModelIdle();
  await waitForRows(6);
  await page.waitForTimeout(2500);
  assert(await page.locator(".model-candidates-table tbody tr").count() === 6, "遅延した旧応答が新しい結果を上書きしました");
  mark("loading-and-stale");

  await page.setViewportSize({ width: 1080, height: 720 });
  assert(await modelSearch.isVisible(), "1080px表示でモデル検索を操作できません");
  assert(await modelScenario().isVisible(), "1080px表示でF2モック状態を操作できません");
  const metrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
  assert(metrics.scrollWidth <= metrics.innerWidth + 1, `1080px表示で横overflowがあります: ${JSON.stringify(metrics)}`);
  mark("responsive");

  assert(pageErrors.length === 0, `pageerrorが発生しました: ${pageErrors.join(" | ")}`);
  assert(httpErrors.length === 0, `HTTPエラーが発生しました: ${httpErrors.join(" | ")}`);
  assert(checks.length === 9, `F2検証項目数が9ではありません: ${checks.length}`);
  return { passed: true, checks, pageErrors, httpErrors, userAgent: await page.evaluate(() => navigator.userAgent) };
}
