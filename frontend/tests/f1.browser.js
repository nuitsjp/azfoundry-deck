async page => {
  const baseURL = page.url();
  const pageErrors = [];
  const httpErrors = [];
  const scenarios = [];
  const checks = [];

  page.on("pageerror", error => {
    pageErrors.push(error?.stack || error?.message || String(error));
  });
  page.on("response", response => {
    if (response.status() >= 400 && !response.url().endsWith("/favicon.ico")) {
      httpErrors.push(`${response.status()} ${response.url()}`);
    }
  });

  const assert = (condition, message) => {
    if (!condition) throw new Error(message);
  };
  const mark = (name, details = {}) => {
    checks.push(name);
    scenarios.push({ name, ...details });
  };
  const getStatus = () => page.locator(".fetch-status").textContent().then(value => value || "");
  const getRows = () => page.evaluate(() => Array.from(document.querySelectorAll("tbody tr")).map(row => {
    const cells = row.querySelectorAll("td");
    const accountSpan = cells[3]?.querySelector("span[title]");
    const tenantSpan = cells[0]?.querySelector("span[title]");
    const subscriptionSpan = cells[1]?.querySelector("span[title]");
    return {
      id: cells[5]?.querySelector("strong")?.getAttribute("title") || "",
      name: cells[5]?.querySelector("strong")?.textContent?.trim() || "",
      model: cells[4]?.childNodes[0]?.textContent?.trim() || "",
      region: cells[2]?.textContent?.trim() || "",
      accountId: accountSpan?.getAttribute("title") || "",
      tenantId: tenantSpan?.getAttribute("title") || "",
      subscriptionId: subscriptionSpan?.getAttribute("title") || "",
    };
  }));
  const waitForRows = expected => page.waitForFunction(
    expectedCount => document.querySelectorAll("tbody tr").length === expectedCount,
    expected,
    { timeout: 5000 },
  );
  const waitForIdle = (timeout = 15000) => page.waitForFunction(
    () => {
      const status = document.querySelector(".fetch-status")?.textContent || "";
      return !status.includes("取得中");
    },
    undefined,
    { timeout },
  );
  const filterSelects = () => page.locator(".filter-panel select");
  const optionValue = index => filterSelects().nth(index).evaluate(select => {
    const option = Array.from(select.options).find(candidate => candidate.value);
    return option?.value || "";
  });
  const optionValues = index => filterSelects().nth(index).evaluate(select => Array.from(select.options)
    .filter(option => option.value && !option.disabled)
    .map(option => option.value));
  const selectedOption = index => filterSelects().nth(index).evaluate(select => {
    const option = select.options[select.selectedIndex];
    return {
      value: option?.value || "",
      text: option?.textContent?.trim() || "",
      disabled: Boolean(option?.disabled),
    };
  });
  const clearFilters = async () => {
    const button = page.getByRole("button", { name: "すべて解除" });
    if (await button.isEnabled()) await button.click();
  };
  const selectScenarioAndRefresh = async value => {
    await page.getByRole("combobox", { name: "次回の取得状態" }).selectOption(value);
    await page.getByRole("button", { name: /(?:更新|再取得)$/ }).first().click();
  };
  const openErrorDetails = async () => {
    await page.getByRole("button", { name: "エラー詳細" }).click();
    await page.getByRole("heading", { name: "エラー詳細" }).waitFor();
  };
  const returnToDeployments = async () => {
    await page.getByRole("button", { name: "デプロイ一覧に戻る" }).click();
    await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  };

  await page.setViewportSize({ width: 1440, height: 920 });
  await page.goto(baseURL);
  await page.getByRole("heading", { name: "デプロイ一覧" }).waitFor();
  await page.waitForFunction(
    () => (document.querySelector(".fetch-status")?.textContent || "").includes("10 / 10 アカウント取得成功"),
    undefined,
    { timeout: 10000 },
  );

  assert(await page.locator('.mock-bar option[value="forbidden"]').count() === 0, "対象外のテナント権限不足がモック状態に残っています");
  const initialRows = await getRows();
  assert(initialRows.length === 24, `初回デプロイ件数が24ではありません: ${initialRows.length}`);
  assert(new Set(initialRows.map(row => row.tenantId)).size === 3, "テナント候補が3件ではありません");
  assert(new Set(initialRows.map(row => row.subscriptionId)).size === 5, "サブスクリプション候補が5件ではありません");
  assert(new Set(initialRows.map(row => row.accountId)).size === 10, "アカウント候補が10件ではありません");
  assert(initialRows.filter(row => row.name === "chat-prod").length === 10, "同名chatデプロイが10件ではありません");
  assert(await page.getByText("不明", { exact: true }).count() >= 3, "欠損数量が不明として表示されていません");
  const optionCounts = await filterSelects().evaluateAll(selects => selects.map(select => Array.from(select.options).filter(option => option.value).length));
  assert(optionCounts[0] === 3 && optionCounts[1] === 5 && optionCounts[3] === 10, `候補数が不正です: ${optionCounts.join(",")}`);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "24件", "初回の件数表示が24件ではありません");
  const wideMetrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
  assert(wideMetrics.scrollWidth <= wideMetrics.innerWidth + 1, `1440px表示で横overflowがあります: ${JSON.stringify(wideMetrics)}`);
  await page.screenshot({ path: "docs/verification/f1-browser-normal.png", fullPage: true });
  mark("success", { deployments: 24, tenants: 3, subscriptions: 5, accounts: 10, duplicateChat: 10 });

  const filterCases = [
    [0, "tenantId"],
    [1, "subscriptionId"],
    [2, "region"],
    [3, "accountId"],
    [4, "model"],
  ];
  const sortedUnique = values => [...new Set(values)].sort((left, right) => left.localeCompare(right));
  const matchesFilters = (row, active) => {
    const name = (active.name || "").trim().toLocaleLowerCase();
    return filterCases.every(([, key]) => !active[key] || row[key] === active[key])
      && (!name || row.name.toLocaleLowerCase().includes(name));
  };
  const expectedCandidateValues = (key, active = {}) => initialRows
    .filter(row => matchesFilters(row, { ...active, [key]: "" }))
    .map(row => row[key]);
  const assertCandidates = async (index, key, active, context) => {
    const actual = sortedUnique(await optionValues(index));
    const expected = sortedUnique(expectedCandidateValues(key, active));
    assert(JSON.stringify(actual) === JSON.stringify(expected), `${context}の候補が不正です: ${JSON.stringify(actual)} / ${JSON.stringify(expected)}`);
  };
  const restrictiveValue = key => sortedUnique(initialRows.map(row => row[key]))
    .sort((left, right) => initialRows.filter(row => row[key] === left).length - initialRows.filter(row => row[key] === right).length)[0];
  for (const [index, key] of filterCases) {
    const value = await optionValue(index);
    const expected = initialRows.filter(row => row[key] === value).length;
    await filterSelects().nth(index).selectOption(value);
    await waitForRows(expected);
    const visible = await getRows();
    assert(visible.length > 0 && visible.every(row => row[key] === value), `${key} selectが完全一致で絞り込めません`);
    assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === `${expected}/24件`, `${key}絞り込みの件数表示が不正です`);
    await filterSelects().nth(index).selectOption("");
    await waitForRows(24);
    assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "24件", `${key}解除後の件数表示が不正です`);
  }
  const nameInput = page.locator("input[type=search]");
  await nameInput.fill("  ChAt-PrOd  ");
  await waitForRows(10);
  assert((await getRows()).every(row => row.name.toLocaleLowerCase().includes("chat-prod")), "デプロイ名のtrim/大小無視部分一致が不正です");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "10/24件", "名前絞り込みの件数表示が不正です");
  const andValue = await optionValue(0);
  const andExpected = initialRows.filter(row => row.tenantId === andValue && row.name.toLocaleLowerCase().includes("chat-prod")).length;
  await filterSelects().nth(0).selectOption(andValue);
  await waitForRows(andExpected);
  const andRows = await getRows();
  const andTenant = await filterSelects().nth(0).inputValue();
  assert(andRows.length > 0 && andRows.every(row => row.tenantId === andTenant && row.name.toLocaleLowerCase().includes("chat-prod")), "絞り込み条件のANDが不正です");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === `${andExpected}/24件`, "AND絞り込みの件数表示が不正です");
  await nameInput.fill("");
  await clearFilters();
  await waitForRows(24);
  await nameInput.fill("no-such-deployment");
  await waitForRows(0);
  assert((await page.locator(".empty-state h3").textContent() || "").includes("条件に一致するデプロイがありません"), "条件0件の説明が不正です");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "0/24件", "0件絞り込みの件数表示が不正です");
  await nameInput.fill("");
  await waitForRows(24);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "24件", "0件解除後の件数表示が不正です");
  await clearFilters();
  await waitForRows(24);
  mark("filters", { exactSelects: 5, nameMatching: "trim/case-insensitive partial", and: true, zeroExplanation: true });

  for (const [index, key] of filterCases) {
    const value = restrictiveValue(key);
    const expectedRows = initialRows.filter(row => row[key] === value).length;
    await filterSelects().nth(index).selectOption(value);
    await waitForRows(expectedRows);
    const active = { [key]: value };
    for (const [targetIndex, targetKey] of filterCases) {
      await assertCandidates(targetIndex, targetKey, active, `${key}条件後の${targetKey}`);
    }
    const selfCandidates = sortedUnique(expectedCandidateValues(key, active));
    const otherValue = selfCandidates.find(candidate => candidate !== value);
    assert(otherValue, `${key}の自己条件を外した候補がありません`);
    await filterSelects().nth(index).selectOption(otherValue);
    await waitForRows(initialRows.filter(row => row[key] === otherValue).length);
    await clearFilters();
    await waitForRows(24);
    for (const [targetIndex, targetKey] of filterCases) {
      await assertCandidates(targetIndex, targetKey, {}, `${key}条件解除後の${targetKey}`);
    }
  }

  await nameInput.fill("chat");
  await waitForRows(initialRows.filter(row => row.name.toLocaleLowerCase().includes("chat")).length);
  const nameActive = { name: "chat" };
  for (const [index, key] of filterCases) {
    await assertCandidates(index, key, nameActive, `名前検索後の${key}`);
  }
  await nameInput.fill("");
  await waitForRows(24);

  const heldIndex = 0;
  const heldKey = "tenantId";
  const heldValue = restrictiveValue(heldKey);
  await filterSelects().nth(heldIndex).selectOption(heldValue);
  await waitForRows(initialRows.filter(row => row[heldKey] === heldValue).length);
  await nameInput.fill("no-such-deployment");
  await waitForRows(0);
  const heldOption = await selectedOption(heldIndex);
  assert(heldOption.value === heldValue && heldOption.disabled && heldOption.text === "選択中（現在の条件に該当なし）", `該当なしの選択中表示が不正です: ${JSON.stringify(heldOption)}`);
  assert((await optionValues(heldIndex)).length === 0, "該当なしの選択中に通常候補が残っています");
  await nameInput.fill("chat");
  const restoredRows = initialRows.filter(row => row[heldKey] === heldValue && row.name.toLocaleLowerCase().includes("chat")).length;
  await waitForRows(restoredRows);
  const restoredOption = await selectedOption(heldIndex);
  assert(restoredOption.value === heldValue && !restoredOption.disabled && restoredOption.text !== "選択中（現在の条件に該当なし）", `名前復元後の選択状態が不正です: ${JSON.stringify(restoredOption)}`);
  for (const [index, key] of filterCases) {
    await assertCandidates(index, key, { [heldKey]: heldValue, name: "chat" }, `名前復元後の${key}`);
  }
  await clearFilters();
  await waitForRows(24);
  mark("dependent-options", { selectFields: 5, nameAffectsSelects: 5, selfConditionExcluded: true, restoredAfterClear: true, disabledSelectionPreserved: true });

  await selectScenarioAndRefresh("empty");
  await waitForIdle();
  await waitForRows(0);
  let status = await getStatus();
  assert(status.includes("10 / 10 アカウント取得成功"), "emptyが成功0件として完了していません");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "0件", "emptyの件数表示が不正です");
  assert((await page.locator(".empty-state h3").textContent() || "").includes("デプロイはありません"), "emptyの説明が不正です");
  await page.screenshot({ path: "docs/verification/f1-browser-empty.png", fullPage: true });
  mark("empty", { deployments: 0, successfulAccounts: 10 });

  await selectScenarioAndRefresh("partial");
  await waitForIdle();
  await waitForRows(15);
  status = await getStatus();
  assert(status.includes("4アカウントで取得失敗（一覧は不完全）"), "partialの失敗件数表示が不正です");
  assert(!status.includes("6 / 10 アカウント取得成功") && !status.includes("取得エラー 4件"), "partialに旧来の重複件数表示が残っています");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "15件", "partialの件数表示が不正です");
  assert(await page.locator(".error-item").count() === 0, "partialの詳細が一覧画面に表示されています");
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 1, "partialのエラー詳細ボタンがありません");
  assert(await page.getByText("forbidden", { exact: true }).count() === 0 && await page.getByText("communication", { exact: true }).count() === 0, "partialの詳細コードが一覧画面に表示されています");
  await page.screenshot({ path: "docs/verification/f1-browser-partial.png", fullPage: true });
  const partialUpdateMeta = await page.locator(".update-meta span").last().textContent();
  await nameInput.fill("chat");
  await page.waitForFunction(
    () => {
      const rows = Array.from(document.querySelectorAll("tbody tr"));
      return rows.length > 0 && rows.every(row => (row.querySelector(".deployment-name strong")?.textContent || "").toLocaleLowerCase().includes("chat"));
    },
    undefined,
    { timeout: 5000 },
  );
  const partialRows = await getRows();
  const partialChatExpected = partialRows.length;
  assert(partialChatExpected > 0, "partialの名前検索chatが0件です");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === `${partialChatExpected}/15件`, "partialのchat絞り込み件数表示が不正です");
  await page.screenshot({ path: "docs/verification/f1-browser-filtered.png", fullPage: true });
  await openErrorDetails();
  assert(await page.getByRole("heading", { name: "エラー詳細" }).count() === 1, "エラー詳細画面に切り替わっていません");
  assert(await page.getByRole("combobox", { name: "次回の取得状態" }).count() === 1, "エラー詳細画面にモック状態選択がありません");
  assert(await page.getByRole("button", { name: /(?:更新|再取得)$/ }).count() >= 1, "エラー詳細画面に更新ボタンがありません");
  assert(await page.locator(".fetch-status").count() === 1, "エラー詳細画面に取得状態がありません");
  assert(await page.locator(".error-item").count() === 4, "partialの詳細件数が4ではありません");
  assert(await page.getByText("forbidden", { exact: true }).count() === 1, "partialのforbidden詳細がありません");
  assert(await page.getByText("communication", { exact: true }).count() === 1, "partialのcommunication詳細がありません");
  await page.screenshot({ path: "docs/verification/f1-browser-error-details.png", fullPage: true });
  await returnToDeployments();
  assert(await nameInput.inputValue() === "chat", "詳細画面から戻った後に名前フィルターが保持されていません");
  const returnedRows = await getRows();
  assert(returnedRows.length === partialChatExpected, "詳細画面から戻った後の一覧件数が保持されていません");
  assert(await page.locator(".update-meta span").last().textContent() === partialUpdateMeta, "画面切替だけで最終取得時刻が変わりました");
  await nameInput.fill("no-such-deployment");
  await waitForRows(0);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "0/15件", "partialの0件絞り込み表示が不正です");
  await openErrorDetails();
  assert(await page.locator(".error-item").count() === 4, "フィルター0件でもpartialの詳細4件が表示されていません");
  await returnToDeployments();
  await nameInput.fill("");
  await waitForRows(15);
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "15件", "partialの絞り込み解除後の件数表示が不正です");
  await page.reload();
  await waitForRows(15);
  assert(await page.getByRole("combobox", { name: "次回の取得状態" }).inputValue() === "partial", "再読み込み後の状態選択がGoの保持状態と一致しません");
  assert(await page.locator(".error-item").count() === 0, "再読み込み後の一覧にpartial詳細が表示されています");
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 1, "再読み込み後にエラー詳細ボタンがありません");
  mark("reload", { selectedScenario: "partial", deployments: 15, failures: 4, detailsOnSeparateScreen: true });
  await waitForRows(15);
  mark("partial", { deployments: 15, successfulAccounts: 6, failures: 4, errorsRemainOnDetails: true, detailsOnSeparateScreen: true });

  await selectScenarioAndRefresh("failure");
  await waitForIdle();
  await waitForRows(0);
  status = await getStatus();
  assert(status.includes("対象の探索に失敗しました（一覧は不完全）"), "failureの探索失敗表示がありません");
  assert(!status.includes("探索が完了していません") && !status.includes("取得エラー 1件"), "failureに旧来の重複件数表示が残っています");
  assert(await page.locator(".error-item").count() === 0, "failureの詳細が一覧画面に表示されています");
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 1, "failureのエラー詳細ボタンがありません");
  await page.screenshot({ path: "docs/verification/f1-browser-failure.png", fullPage: true });
  await openErrorDetails();
  assert(await page.locator(".error-item").count() === 1, "failureの詳細件数が1ではありません");
  assert(await page.getByText("not-logged-in", { exact: true }).count() === 1, "failureのコード詳細がありません");
  await returnToDeployments();
  mark("failure", { failures: 1, discoveryIncomplete: true, detailsOnSeparateScreen: true });

  await selectScenarioAndRefresh("cli-missing");
  await waitForIdle();
  await waitForRows(0);
  status = await getStatus();
  assert(status.includes("対象の探索に失敗しました（一覧は不完全）"), "cli-missingの探索失敗表示がありません");
  assert(!status.includes("探索が完了していません") && !status.includes("取得エラー 1件"), "cli-missingに旧来の重複件数表示が残っています");
  assert(await page.locator(".error-item").count() === 0, "cli-missingの詳細が一覧画面に表示されています");
  await openErrorDetails();
  assert(await page.locator(".error-item").count() === 1, "cli-missingの詳細件数が1ではありません");
  assert(await page.getByText("cli-missing", { exact: true }).count() === 1, "cli-missingのコード詳細がありません");
  await returnToDeployments();
  mark("cli-missing", { failures: 1, discoveryIncomplete: true, detailsOnSeparateScreen: true });

  await selectScenarioAndRefresh("all-accounts-failed");
  await waitForIdle();
  await waitForRows(0);
  status = await getStatus();
  assert(status.includes("10アカウントで取得失敗（一覧は不完全）"), "all-accounts-failedの取得失敗表示が不正です");
  assert(!status.includes("0 / 10") && !status.includes("取得エラー 10件"), "all-accounts-failedに旧来の重複件数表示が残っています");
  assert((await page.locator(".result-count").textContent() || "").replace(/\s+/g, "") === "0件", "all-accounts-failedの件数表示が不正です");
  assert(await page.locator(".error-item").count() === 0, "all-accounts-failedの詳細が一覧画面に表示されています");
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 1, "all-accounts-failedのエラー詳細ボタンがありません");
  assert((await page.locator(".empty-state h3").textContent() || "").includes("取得失敗のため表示できるデプロイがありません"), "全失敗時の0件説明が不正です");
  await openErrorDetails();
  assert(await page.locator(".error-item").count() === 10, "all-accounts-failedの詳細件数が10ではありません");
  mark("all-accounts-failed", { deployments: 0, failures: 10, incomplete: true, detailsOnSeparateScreen: true, scrollableDetails: true });
  await page.setViewportSize({ width: 1080, height: 720 });
  const errorList = page.locator(".error-list").first();
  const scrollMetrics = await errorList.evaluate(element => ({ scrollHeight: element.scrollHeight, clientHeight: element.clientHeight }));
  assert(scrollMetrics.scrollHeight > scrollMetrics.clientHeight, "all-accounts-failedの詳細一覧がスクロール可能ではありません");
  await errorList.evaluate(element => { element.scrollTop = element.scrollHeight; });
  assert(await errorList.evaluate(element => Math.ceil(element.scrollTop + element.clientHeight) >= element.scrollHeight), "all-accounts-failedの詳細一覧を最後までスクロールできません");
  assert(await page.getByRole("button", { name: "デプロイ一覧に戻る" }).isVisible(), "1080px表示で一覧へ戻るボタンを操作できません");
  assert(await page.getByRole("button", { name: /(?:更新|再取得)$/ }).first().isVisible(), "1080px表示で詳細画面の更新ボタンを操作できません");
  await selectScenarioAndRefresh("success");
  await waitForIdle();
  assert(await page.getByRole("heading", { name: "エラー詳細" }).count() === 1, "成功更新後に詳細画面が閉じられました");
  assert(await page.getByText("取得エラーはありません", { exact: true }).count() === 1, "成功更新後のエラーなし表示がありません");
  assert(await page.locator(".error-item").count() === 0, "成功更新後もエラー詳細が残っています");
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 0, "成功更新後もエラー詳細リンクが残っています");
  assert(await page.getByRole("button", { name: "デプロイ一覧に戻る" }).count() === 1, "成功更新後も一覧へ戻れません");
  await returnToDeployments();
  await waitForRows(24);
  assert(await page.getByRole("button", { name: "エラー詳細" }).count() === 0, "成功更新後の一覧にエラー詳細リンクが残っています");
  assert(!(await getStatus()).includes("不完全"), "成功した更新後も不完全表示が残っています");
  mark("error-details", { partial: 4, failure: 1, cliMissing: 1, allAccountsFailed: 10, topLevelSummaryOnly: true, preservesFiltersAndTimestamp: true, successClearsDetails: true });
  await page.setViewportSize({ width: 1440, height: 920 });

  await selectScenarioAndRefresh("loading");
  await page.waitForFunction(
    () => (document.querySelector(".fetch-status")?.textContent || "").includes("取得中"),
    undefined,
    { timeout: 3000 },
  );
  await page.screenshot({ path: "docs/verification/f1-browser-loading.png", fullPage: true });
  assert((await getStatus()).includes("前回の結果"), "loading中に前回結果の説明がありません");
  await nameInput.fill("chat");
  await waitForRows(10);
  assert((await getStatus()).includes("取得中"), "loading中の条件変更後に取得中表示が消えました");
  await page.waitForFunction(() => {
    const bar = document.querySelector(".fetch-progress progress");
    return bar?.max === 5 && bar.value > 0 && bar.value < 5;
  }, undefined, { timeout: 10000 });
  assert((await getStatus()).includes("対象を探索"), "探索段階が表示されていません");
  await page.screenshot({ path: "docs/verification/f1-browser-loading.png", fullPage: true });
  await page.waitForFunction(() => {
    const bar = document.querySelector(".fetch-progress progress");
    return bar?.max === 10 && bar.value > 0 && bar.value < 10;
  }, undefined, { timeout: 10000 });
  const completedAccounts = await page.locator(".fetch-progress progress").evaluate(bar => bar.value);
  const progressingRows = await getRows();
  assert(progressingRows.length === completedAccounts && progressingRows.length < 10, "応答したアカウントのchat行が完了前に表示されていません");
  assert(await nameInput.inputValue() === "chat", "途中結果の反映で絞り込み条件が消えました");
  assert((await getStatus()).includes("途中結果"), "途中結果と前回結果を区別できません");
  await nameInput.fill("");
  const unfilteredProgress = await getRows();
  assert(unfilteredProgress.length > 0 && unfilteredProgress.length < 24, "全件完了前に部分的なデプロイ一覧を確認できません");
  await page.screenshot({ path: "docs/verification/f1-browser-progress.png", fullPage: true });
  await nameInput.fill("chat");
  await waitForIdle(40000);
  await waitForRows(10);
  assert(await page.locator(".fetch-progress").count() === 0, "取得完了後も進捗表示が残っています");
  await nameInput.fill("");
  await waitForRows(24);
  mark("loading", { filtersChangeable: true, previousResultShown: true, subscriptionProgress: true, accountProgress: true, incrementalRows: true, completedAfter30s: true });

  await selectScenarioAndRefresh("delayed");
  await page.waitForFunction(
    () => (document.querySelector(".fetch-status")?.textContent || "").includes("取得中"),
    undefined,
    { timeout: 3000 },
  );
  const delayedMetaBefore = await page.locator(".update-meta span").last().textContent();
  await page.waitForTimeout(4000);
  assert((await page.locator(".update-meta span").last().textContent()) === delayedMetaBefore, "取得中に前回の最終取得時刻が変更されました");
  await selectScenarioAndRefresh("success");
  await waitForIdle();
  await waitForRows(24);
  const successMeta = await page.locator(".update-meta span").last().textContent();
  const immediateTpm = await page.locator("tbody tr").first().locator("td.tpm").textContent();
  assert((immediateTpm || "").includes("120,000") && !(immediateTpm || "").includes("111,000"), "後続successのTPMが直後に正しく反映されていません");
  await page.waitForTimeout(9000);
  const finalTpm = await page.locator("tbody tr").first().locator("td.tpm").textContent();
  assert((finalTpm || "").includes("120,000") && !(finalTpm || "").includes("111,000"), "遅延した旧応答が後続successを上書きしました");
  assert((await page.locator(".update-meta span").last().textContent()) === successMeta, "遅延応答後に最終取得時刻が巻き戻りました");
  assert(await page.locator(".fetch-progress").count() === 0, "古い要求の進捗で完了済みの画面が取得中へ戻りました");
  mark("delayed", { oldTpm: 111000, finalTpm: 120000, staleResponseSuppressed: true, staleProgressSuppressed: true, noPeriodicRefresh: true });

  await page.setViewportSize({ width: 1080, height: 720 });
  assert(await filterSelects().count() === 5, "1080px表示で5つのselectがありません");
  assert(await nameInput.isVisible(), "1080px表示でデプロイ名フィルターを操作できません");
  for (const [index, key] of filterCases) {
    const value = await optionValue(index);
    const expected = initialRows.filter(row => row[key] === value).length;
    await filterSelects().nth(index).selectOption(value);
    await waitForRows(expected);
    await filterSelects().nth(index).selectOption("");
    await waitForRows(24);
  }
  await nameInput.fill("chat");
  await waitForRows(10);
  await nameInput.fill("");
  await waitForRows(24);
  const narrowMetrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
  assert(narrowMetrics.scrollWidth <= narrowMetrics.innerWidth + 1, `1080px表示で横overflowがあります: ${JSON.stringify(narrowMetrics)}`);
  mark("responsive", { viewport: "1440x920 and 1080x720", sixFiltersOperable: true, noHorizontalOverflow: true });

  assert(pageErrors.length === 0, `pageerrorが発生しました: ${pageErrors.join(" | ")}`);
  assert(httpErrors.length === 0, `HTTPエラーが発生しました: ${httpErrors.join(" | ")}`);
  assert(checks.length === 13, `検証項目数が13ではありません: ${checks.length}`);
  return { passed: true, scenarios, checks, userAgent: await page.evaluate(() => navigator.userAgent), pageErrors, httpErrors };
}
