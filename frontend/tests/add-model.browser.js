async page => {
  const errors = [];
  page.on("pageerror", error => errors.push(String(error)));
  const assert = (value, message) => { if (!value) throw new Error(message); };
  const button = name => page.getByRole("button", { name, exact: true });
  const field = name => page.getByRole("dialog").getByLabel(name, { exact: true });
  const checkStableValidation = async (label, valid, invalid) => {
    const input = field(label);
    await page.setViewportSize({ width: 1080, height: 720 });
    await input.fill(valid);
    const geometry = () => input.evaluate(el => {
      const dialog = el.closest("dialog");
      return { y: el.getBoundingClientRect().y, height: dialog.getBoundingClientRect().height, scroll: el.closest("form").scrollTop };
    });
    const before = await geometry();
    await input.fill(invalid);
    const during = await geometry();
    await input.fill(valid);
    const after = await geometry();
    assert(JSON.stringify(before) === JSON.stringify(during) && JSON.stringify(before) === JSON.stringify(after), `${label}: エラー切替で位置・高さ・スクロールを維持`);
    assert(await input.getAttribute("inputmode") === "url", `${label}: 英数字入力モード`);
    await page.setViewportSize({ width: 1440, height: 1000 });
  };
  const chooseModel = async () => {
    await field("モデル").selectOption("gpt-4o");
    assert(await field("バージョン").inputValue() === "2024-08-06", "既定バージョンの自動選択");
    assert(await field("SKU").inputValue() === "GlobalStandard", "既定SKUの自動選択");
    assert(await field("デプロイ名").inputValue() === "gpt-4o", "デプロイ名の自動入力");
    await field("デプロイ名").fill("first-chat");
  };
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("http://127.0.0.1:9246");
  await button("＋ 新規追加").waitFor();
  await page.waitForFunction(() => document.querySelectorAll("tbody tr").length === 24);
  const listY = (await page.locator(".deployments").boundingBox()).y;
  await page.getByLabel("次回の取得状態", { exact: true }).selectOption("loading");
  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).click();
  assert((await page.locator(".deployments").boundingBox()).y === listY, "一覧の読込開始で位置を維持");
  await page.getByLabel("次回の取得状態", { exact: true }).selectOption("success");
  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).click();
  await page.waitForFunction(() => document.querySelector(".fetch-status").textContent.includes("全対象の取得完了"));
  assert((await page.locator(".deployments").boundingBox()).y === listY, "一覧の取得完了で位置を維持");
  const titleBox = await page.locator("#deployments-heading").boundingBox();
  const addBox = await button("＋ 新規追加").boundingBox();
  assert(Math.abs(titleBox.y + titleBox.height / 2 - addBox.y - addBox.height / 2) < 3, "一覧見出しと追加ボタンの高さ");
  assert(await page.locator(".app-header").getByRole("button", { name: "＋ 新規追加" }).count() === 0, "ヘッダーから追加ボタンを移動");
  await page.screenshot({ path: "docs/verification/add-model-list.png" });
  await button("＋ 新規追加").click();
  await page.setViewportSize({ width: 1080, height: 720 });
  await page.screenshot({ path: "docs/verification/add-model-open.png" });
  assert(await page.locator("dialog > form").evaluate(el => el.scrollHeight <= el.clientHeight + 1), "追加画面が1080×720で収まる: " + await page.locator("dialog > form").evaluate(el => `${el.scrollHeight}/${el.clientHeight}`));
  if (await field("サブスクリプション").count() !== 1) throw new Error(await page.locator("dialog").evaluate(el => el.outerHTML));
  await field("サブスクリプション").selectOption("subscription-001");
  await field("サブスクリプション").focus();
  assert(await field("サブスクリプション").evaluate(el => getComputedStyle(el).boxShadow.includes("inset")), "フォーカス枠を内側に表示");
  await page.screenshot({ path: "docs/verification/add-model-focus.png" });
  await field("リソースグループ").selectOption("rg-production");
  const inputPosition = () => field("モデル").evaluate(el => el.getBoundingClientRect().y);
  const positionBeforeLoad = await inputPosition();
  await field("Foundry").selectOption("contoso-first-model");
  assert(await inputPosition() === positionBeforeLoad, "Foundry特定と読込開始でモデル欄を動かさない");
  await page.getByRole("progressbar", { name: "モデル候補の読込状況" }).waitFor();
  assert(await field("モデル").isDisabled() && await button("追加内容を確認").isDisabled(), "取得中の設定・確定を無効化");
  await page.getByText("モック：モデル取得の再現", { exact: true }).click();
  await field("モデル取得の状態").selectOption("slow");
  await button("モデル候補を再取得").click();
  await page.getByRole("progressbar", { name: "モデル候補の読込状況" }).waitFor();
  await page.screenshot({ path: "docs/verification/add-model-loading.png" });
  await field("モデル取得の状態").selectOption("empty");
  await field("Foundry").selectOption("contoso-chat-prod");
  const secondLoadPosition = await inputPosition();
  await page.getByText("モデル候補なし", { exact: true }).waitFor();
  // Wait beyond the old slow request and verify it cannot repopulate the new target.
  await page.waitForTimeout(8200);
  assert(await inputPosition() === secondLoadPosition, "空の取得結果でもモデル欄を動かさない");
  assert(await field("モデル").isDisabled(), "旧Foundryの遅延応答を破棄");
  await field("モデル取得の状態").selectOption("failure");
  await button("モデル候補を再取得").click();
  await button("モデル候補を再試行").waitFor();
  assert(await field("モデル").isDisabled(), "取得失敗時に設定を無効化");
  await field("モデル取得の状態").selectOption("success");
  await button("モデル候補を再試行").click();
  await field("Foundry").selectOption("contoso-first-model");
  await page.getByText("モック：モデル取得の再現", { exact: true }).click();
  await chooseModel();
  await field("バージョン").selectOption("2024-11-20");
  await field("SKU").selectOption("Standard");
  await field("モデル").selectOption("text-embedding-3-large");
  assert(await field("バージョン").inputValue() === "1" && await field("SKU").inputValue() === "Standard" && await field("デプロイ名").inputValue() === "text-embedding-3-large", "モデル変更で既定値を再設定");
  await chooseModel();
  await page.screenshot({ path: "docs/verification/add-model-existing.png" });
  await button("追加内容を確認").click();
  assert((await page.locator(".add-summary").textContent()).includes("contoso-first-model（既存"), "0件Foundryの選択");
  await button("入力内容を変更").click();
  assert(await field("デプロイ名").inputValue() === "first-chat", "確認から戻る入力保持");
  await field("サブスクリプション").selectOption("subscription-002");
  assert(await field("リソースグループ").inputValue() === "" && await field("Foundry").inputValue() === "", "上位変更で配置先解除");
  await button("Foundryを新規作成").click();
  await checkStableValidation("新しいFoundry名", "valid-foundry", "a");
  for (const invalid of ["a", "a".repeat(65), "-foundry", "foundry-", "bad_name", "bad name", "日本語"]) {
    await field("新しいFoundry名").fill(invalid);
    assert(await button("このFoundryを使用").isDisabled(), `Foundry不正名の拒否: ${invalid}`);
  }
  for (const valid of ["ab", "A".repeat(64), "valid-foundry"]) {
    await field("新しいFoundry名").fill(valid);
    assert(await button("このFoundryを使用").isEnabled(), "Foundry有効名の許可");
  }
  await field("新しいFoundry名").fill("new-foundry");
  await button("リソースグループを新規作成").click();
  await checkStableValidation("新しいリソースグループ名", "valid-rg", "bad name");
  for (const invalid of ["bad name", "name.", "bad/name", "a".repeat(91), "RG-SANDBOX"]) {
    await field("新しいリソースグループ名").fill(invalid);
    assert(await button("このリソースグループを使用").isDisabled(), `RG不正名・重複の拒否: ${invalid}`);
  }
  for (const valid of ["a", "a".repeat(90), "日本語_開発(1)-rg"]) {
    await field("新しいリソースグループ名").fill(valid);
    assert(await button("このリソースグループを使用").isEnabled(), "RG有効名の許可");
  }
  await field("新しいリソースグループ名").fill("rg-new");
  await page.screenshot({ path: "docs/verification/add-model-group.png" });
  await button("このリソースグループを使用").click();
  assert(await field("新しいFoundry名").inputValue() === "new-foundry", "拡張操作から戻る入力保持");
  await page.screenshot({ path: "docs/verification/add-model-foundry.png" });
  await button("このFoundryを使用").click();
  await chooseModel();
  await button("追加内容を確認").click();
  const summary = await page.locator(".add-summary").textContent();
  assert(summary.includes("rg-new（新規") && summary.includes("new-foundry（新規"), "両リソース新規作成");
  await page.screenshot({ path: "docs/verification/add-model-review.png" });
  await button("モデル追加を再現").click();
  await page.getByRole("heading", { name: "モデル追加の再現が完了しました" }).waitFor();
  await button("一覧に戻る").click();
  // The entry remains available when the deployment list itself is empty.
  await page.getByLabel("次回の取得状態", { exact: true }).selectOption("empty");
  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).click();
  await page.getByRole("heading", { name: "デプロイはありません", exact: true }).waitFor();
  await button("＋ 新規追加").click();
  await field("サブスクリプション").selectOption("subscription-001");
  await button("リソースグループを新規作成").click();
  await field("新しいリソースグループ名").fill("rg-direct");
  await button("このリソースグループを使用").click();
  assert(await field("リソースグループ").inputValue() === "rg-direct", "配置先欄からの直接作成");
  await button("Foundryを新規作成").click();
  await button("モデル追加に戻る").click();
  assert(await field("Foundry").inputValue() === "", "キャンセルでFoundryを作らない");
  await page.setViewportSize({ width: 1080, height: 720 });
  assert(await page.locator("dialog").evaluate(el => el.scrollWidth <= el.clientWidth), "ダイアログの横はみ出し");
  await page.keyboard.press("Escape");
  assert(await page.getByRole("dialog").count() === 0, "Escapeで閉じる");
  assert(errors.length === 0, errors.join("\n"));
  return { passed: true, checks: ["model-loading-empty-error-retry-stale", "stable-validation-and-input-mode", "list-button-placement", "model-defaults", "resource-naming", "empty-foundry", "review-and-back", "subscription-reset", "nested-resource-creation", "mock-completion", "empty-list-entry", "direct-group-creation", "cancel", "responsive-and-escape"], pageErrors: errors };
}
