async page => {
  // Environment-specific negative case agreed by the owner on 2026-09-13.
  // This fixture must not become a production exclusion or trigger az login.
  const expectedFailure = "Visual Studio Professional（中村MSDN）";
  const assert = (condition, message) => { if (!condition) throw new Error(message); };
  const pageErrors = [];
  const httpErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("response", response => {
    if (response.status() >= 400) httpErrors.push(`${response.status()} ${response.url()}`);
  });
  // Reload after registering listeners to cover initial application startup.
  await page.reload();
  const waitIdle = () => page.waitForFunction(() =>
    document.querySelector(".deployments")?.getAttribute("aria-busy") === "false" &&
    document.querySelector(".update-meta")?.textContent.includes("最終取得"),
    undefined, { timeout: 60000 });
  const readRows = () => page.locator("tbody tr").evaluateAll(elements => elements.map(row => ({
    id: row.querySelector(".deployment-name strong")?.getAttribute("title"),
    cells: [...row.querySelectorAll("td")].map(cell => cell.textContent.trim())
  })));
  const checkList = async () => {
    await waitIdle();
    const rows = await readRows();
    assert(rows.length > 0, "成功した実デプロイが表示されていません");
    assert(await page.locator(".mock-bar").count() === 0, "実接続でモックが表示されています");
    const status = await page.locator(".fetch-status").innerText();
    assert(status.includes("対象の探索に失敗しました") && status.includes("一覧は不完全"), "探索失敗と不完全性が表示されていません");
    assert(!status.includes("全対象の取得完了"), "認証失敗を全対象成功として表示しています");
    assert(!(await page.locator("body").innerText()).includes("\uFFFD"), "一覧に置換文字があります");
    return rows;
  };
  const checkDetails = async () => {
    await page.getByRole("button", { name: "エラー詳細", exact: true }).click();
    await page.getByRole("heading", { name: "エラー詳細", exact: true }).waitFor();
    const errors = page.locator(".error-item");
    assert(await errors.count() === 1, "想定した認証エラー1件以外の失敗があります");
    assert((await errors.first().innerText()).includes(expectedFailure), "指定サブスクリプションの認証エラーが保持されていません");
    assert(await errors.locator("code").innerText() === "azure-cli-not-logged-in", "認証エラーのコードが異なります");
    assert((await errors.locator(".error-action").innerText()).length > 0, "対処の説明がありません");
    assert(!(await page.locator("body").innerText()).includes("\uFFFD"), "エラー詳細に置換文字があります");
  };
  const initialRows = await checkList();
  await checkDetails();
  await page.getByRole("button", { name: "デプロイ一覧に戻る" }).click();
  assert(JSON.stringify(await readRows()) === JSON.stringify(initialRows), "一覧復帰で取得結果が変わりました");

  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).first().click();
  await page.locator(".fetch-progress").waitFor();
  const refreshedRows = await checkList();
  // Parallel account requests can complete in a different order on refresh.
  const initialById = initialRows.toSorted((left, right) => left.id.localeCompare(right.id));
  const refreshedById = refreshedRows.toSorted((left, right) => left.id.localeCompare(right.id));
  assert(JSON.stringify(refreshedById) === JSON.stringify(initialById), "更新前後で実デプロイの表示値が異なります。リソース変更の有無を確認してください");

  const nameInput = page.getByPlaceholder("名前の一部で検索");
  await nameInput.fill("__f1_negative_case_no_deployment__");
  assert(await page.locator("tbody tr").count() === 0, "名前フィルターが適用されていません");
  const fetchedAt = await page.locator(".update-meta").innerText();
  await checkDetails();
  await page.screenshot({ path: "docs/verification/f1-azure-auth-error.png", fullPage: true });
  await page.getByRole("button", { name: "デプロイ一覧に戻る" }).click();
  assert(await nameInput.inputValue() === "__f1_negative_case_no_deployment__", "一覧復帰でフィルターが失われました");
  assert(await page.locator(".update-meta").innerText() === fetchedAt, "画面切り替えで最終取得時刻が変わりました");
  await page.getByRole("button", { name: "すべて解除" }).click();
  assert(JSON.stringify(await readRows()) === JSON.stringify(refreshedRows), "条件解除で成功行に戻れません");
  await page.screenshot({ path: "docs/verification/f1-azure-headless.png", fullPage: true });
  assert(pageErrors.length === 0, `JavaScriptエラー: ${pageErrors.join(" | ")}`);
  assert(httpErrors.length === 0, `アプリHTTPエラー: ${httpErrors.join(" | ")}`);
  return {
    passed: true, mock: false, initialRows: initialRows.length, refreshedRows: refreshedRows.length,
    expectedFailure, failureCode: "azure-cli-not-logged-in", failureCount: 1,
    initialAndRefreshFailureVerified: true, incompleteRetained: true,
    failureVisibleWithEmptyFilter: true, returnStatePreserved: true,
    replacementCharacters: 0, pageErrors, httpErrors,
    userAgent: await page.evaluate(() => navigator.userAgent)
  };
}
