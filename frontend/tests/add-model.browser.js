async page => {
  const errors = [];
  page.on("pageerror", error => errors.push(String(error)));
  const assert = (value, message) => { if (!value) throw new Error(message); };
  const button = name => page.getByRole("button", { name, exact: true });
  const field = name => page.getByRole("dialog").getByLabel(name, { exact: true });
  const modelValue = (format, name) => `${format}::${name}`;
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
    await field("モデル").selectOption(modelValue("OpenAI", "gpt-4o"));
    assert(await field("バージョン").inputValue() === "2024-08-06", "既定バージョンの自動選択");
    assert(await field("SKU").inputValue() === "GlobalStandard", "返却順によらずGlobalStandardを優先選択");
    assert(await field("Capacity").inputValue() === "10", "SKUの既定Capacityを自動選択");
    assert(await field("デプロイ名").inputValue() === "gpt-4o", "デプロイ名の自動入力");
    await field("デプロイ名").fill("first-chat");
  };
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(page.url());
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
  await page.screenshot({ path: "docs/verification/f3-copy-list.png" });
  await button("＋ 新規追加").click();
  await page.setViewportSize({ width: 1080, height: 720 });
  await page.screenshot({ path: "docs/verification/f3-copy-open.png" });
  assert(await page.locator("dialog > form").evaluate(el => el.scrollHeight <= el.clientHeight + 1), "追加画面が1080×720で収まる: " + await page.locator("dialog > form").evaluate(el => `${el.scrollHeight}/${el.clientHeight}`));
  if (await field("サブスクリプション").count() !== 1) throw new Error(await page.locator("dialog").evaluate(el => el.outerHTML));
  await field("サブスクリプション").selectOption("subscription-001");
  await field("サブスクリプション").focus();
  assert(await field("サブスクリプション").evaluate(el => getComputedStyle(el).boxShadow.includes("inset")), "フォーカス枠を内側に表示");
  await page.screenshot({ path: "docs/verification/f3-copy-focus.png" });
  await field("リソースグループ").selectOption("rg-production");
  const inputPosition = () => field("モデル").evaluate(el => el.getBoundingClientRect().y);
  const positionBeforeLoad = await inputPosition();
  await field("Foundry").selectOption("contoso-first-model");
  assert(await inputPosition() === positionBeforeLoad, "Foundry特定と読込開始でモデル欄を動かさない");
  await page.getByRole("progressbar", { name: "モデル候補の読込状況" }).waitFor();
  assert(await field("モデル").isDisabled() && await button("確認").isDisabled(), "取得中の設定・確定を無効化");
  await page.getByText("モック：モデル取得の再現", { exact: true }).click();
  await field("モデル取得の状態").selectOption("slow");
  await button("モデル候補を再取得").click();
  await page.getByRole("progressbar", { name: "モデル候補の読込状況" }).waitFor();
  await page.screenshot({ path: "docs/verification/f3-copy-loading.png" });
  await field("モデル取得の状態").selectOption("empty");
  await field("Foundry").selectOption("contoso-chat-prod");
  const secondLoadPosition = await inputPosition();
  await page.getByText("利用できるモデル候補がありません", { exact: true }).waitFor();
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

  await field("モデル取得の状態").selectOption("no-default");
  await button("モデル候補を再取得").click();
  await page.getByRole("option", { name: "OpenAI / gpt-no-default", exact: true }).waitFor({ state: "attached" });
  await field("モデル").selectOption(modelValue("OpenAI", "gpt-no-default"));
  assert(await field("バージョン").inputValue() === "" && await field("SKU").inputValue() === "" && await field("Capacity").inputValue() === "", "既定版なしはバージョン・SKU・容量を自動選択しない");
  assert(await button("確認").isDisabled(), "既定版なしは確認不可");
  await field("バージョン").selectOption("2026-01-01");
  assert(await field("SKU").inputValue() === "GlobalStandard" && await field("Capacity").inputValue() === "5", "バージョン選択で単一SKUと既定Capacityを連動");
  await field("バージョン").selectOption("2026-02-01");
  assert(await field("SKU").inputValue() === "DataZoneStandard" && await field("Capacity").inputValue() === "5", "Globalがない版はDataZoneStandardをStandardより優先");
  await field("SKU").selectOption("Standard");
  assert(await field("Capacity").inputValue() === "", "自動選択後も手動変更でき、既定容量がなければ空にする");
  await field("モデル取得の状態").selectOption("missing-capacity");
  await button("モデル候補を再取得").click();
  await page.getByRole("option", { name: "OpenAI / gpt-capacity-unknown", exact: true }).waitFor({ state: "attached" });
  await field("モデル").selectOption(modelValue("OpenAI", "gpt-capacity-unknown"));
  assert(await field("バージョン").inputValue() === "2026-03-01" && await field("SKU").inputValue() === "GlobalStandard" && await field("Capacity").inputValue() === "", "容量欠損では容量を自動補完しない");
  assert(await page.getByText("容量の設定条件を取得できません。モデル候補を再取得してください。", { exact: true }).count() === 1 && await button("確認").isDisabled(), "容量条件不足は確認不可");
  await field("モデル取得の状態").selectOption("success");
  await button("モデル候補を再取得").click();
  await page.getByRole("option", { name: "OpenAI / gpt-4o", exact: true }).waitFor({ state: "attached" });
  await chooseModel();
  await field("デプロイ名").fill("CHAT-PRODUCTION");
  assert(await button("確認").isDisabled(), "同一Foundryの既存デプロイは大小文字を無視して拒否");
  await field("デプロイ名").fill("first-chat");
  await button("確認").click();
  assert((await page.locator(".add-summary").textContent()).includes("contoso-chat-prod（既存"), "デプロイありFoundryの選択確認");
  await button("変更").click();
  await field("Foundry").selectOption("contoso-first-model");
  await page.getByText("モック：モデル取得の再現", { exact: true }).click();
  await chooseModel();
  await field("デプロイ名").fill("chat-production");
  assert(await button("確認").isEnabled(), "別Foundryでは同名デプロイを許可");
  await field("デプロイ名").fill("first-chat");
  await field("バージョン").selectOption("2024-11-20");
  await field("SKU").selectOption("Standard");
  await field("モデル").selectOption(modelValue("OpenAI", "text-embedding-3-large"));
  assert(await field("バージョン").inputValue() === "1" && await field("SKU").inputValue() === "Standard" && await field("デプロイ名").inputValue() === "text-embedding-3-large", "モデル変更で既定値を再設定");
  assert(await field("Capacity").inputValue() === "1" && await field("Capacity").getAttribute("list") === "capacity-allowed-values", "許可値のCapacity契約を入力に反映");
  await field("Capacity").fill("2");
  assert(await button("確認").isDisabled(), "許可値外のCapacityを拒否");
  await field("Capacity").fill("5");
  assert(await button("確認").isEnabled(), "許可値内のCapacityを許可");
  for (const invalid of ["a", "chat production", "日本語", "a".repeat(65), "chat/prod"]) {
    await field("デプロイ名").fill(invalid);
    assert(await button("確認").isDisabled(), `デプロイ名不正の拒否: ${invalid}`);
  }
  await field("デプロイ名").fill("chat.prod_1-a");
  assert(await button("確認").isEnabled(), "デプロイ名有効の許可");
  await checkStableValidation("デプロイ名", "chat-prod", "a");
  await chooseModel();
  await field("バージョン").selectOption("2024-11-20");
  await field("SKU").selectOption("Standard");
  await field("Capacity").fill("42");
  await page.screenshot({ path: "docs/verification/f3-copy-existing.png" });
  await button("確認").click();
  assert((await page.locator(".add-summary").textContent()).includes("contoso-first-model（既存"), "0件Foundryの選択");
  await button("変更").click();
  assert(
    await field("サブスクリプション").inputValue() === "subscription-001" &&
    await field("リソースグループ").inputValue() === "rg-production" &&
    await field("Foundry").inputValue() === "contoso-first-model" &&
    await field("モデル").inputValue() === modelValue("OpenAI", "gpt-4o") &&
    await field("バージョン").inputValue() === "2024-11-20" &&
    await field("SKU").inputValue() === "Standard" &&
    await field("Capacity").inputValue() === "42" &&
    await field("デプロイ名").inputValue() === "first-chat",
    "確認から戻ると全設定（Capacity含む）を保持"
  );
  await field("サブスクリプション").selectOption("subscription-002");
  assert(await field("リソースグループ").inputValue() === "" && await field("Foundry").inputValue() === "", "上位変更で配置先解除");
  await button("Foundryを新規作成").click();
  await checkStableValidation("新しいFoundry名", "valid-foundry", "a");
  for (const invalid of ["a", "a".repeat(65), "-foundry", "foundry-", "bad_name", "bad name", "日本語"]) {
    await field("新しいFoundry名").fill(invalid);
    assert(await button("使用").isDisabled(), `Foundry不正名の拒否: ${invalid}`);
  }
  for (const valid of ["ab", "A".repeat(64), "valid-foundry"]) {
    await field("新しいFoundry名").fill(valid);
    assert(await button("使用").isEnabled(), "Foundry有効名の許可");
  }
  await field("新しいFoundry名").fill("new-foundry");
  await button("リソースグループを新規作成").click();
  await checkStableValidation("新しいリソースグループ名", "valid-rg", "bad name");
  for (const invalid of ["bad name", "name.", "bad/name", "a".repeat(91), "RG-SANDBOX"]) {
    await field("新しいリソースグループ名").fill(invalid);
    assert(await button("使用").isDisabled(), `RG不正名・重複の拒否: ${invalid}`);
  }
  for (const valid of ["a", "a".repeat(90), "日本語_開発(1)-rg"]) {
    await field("新しいリソースグループ名").fill(valid);
    assert(await button("使用").isEnabled(), "RG有効名の許可");
  }
  await field("新しいリソースグループ名").fill("rg-draft");
  await button("戻る").click();
  assert(
    await field("新しいFoundry名").inputValue() === "new-foundry" &&
    await field("リソースグループ").inputValue() === "" &&
    await page.getByRole("option", { name: "rg-draft（新規作成予定）", exact: true }).count() === 0,
    "RG入力から戻るとFoundry名を保持しRGを作成しない"
  );
  await button("リソースグループを新規作成").click();
  await field("新しいリソースグループ名").fill("rg-new");
  await page.screenshot({ path: "docs/verification/f3-copy-group.png" });
  await button("使用").click();
  assert(await field("新しいFoundry名").inputValue() === "new-foundry", "拡張操作から戻る入力保持");
  await page.screenshot({ path: "docs/verification/f3-copy-foundry.png" });
  await button("使用").click();
  await page.getByText("このリージョンの候補です。Foundryの作成後に再確認します。", { exact: true }).waitFor();
  await chooseModel();
  await button("確認").click();
  const summary = await page.locator(".add-summary").textContent();
  assert(summary.includes("rg-new（新規") && summary.includes("new-foundry（新規"), "両リソース新規作成");
  await page.getByText("新規Foundryの設定", { exact: true }).click();
  const proposal = await page.locator(".add-proposal").textContent();
  assert(proposal.includes("AIServices") && proposal.includes("S0") && proposal.includes("公開ネットワーク") && proposal.includes("有効") && proposal.includes("キー認証") && proposal.includes("Entra ID") && proposal.includes("new-foundry") && proposal.includes("システム割り当て") && proposal.includes("作成しない"), "新規Foundry設定案を確認画面に表示");
  await page.screenshot({ path: "docs/verification/f3-copy-review.png" });
  await button("追加").click();
  await page.getByRole("heading", { name: "モデルを追加しました" }).waitFor();
  assert(await button("閉じる").count() === 1, "完了画面の閉じるは1つだけ");
  await button("閉じる").click();

  const configureExisting = async () => {
    await button("＋ 新規追加").click();
    await field("サブスクリプション").selectOption("subscription-001");
    await field("リソースグループ").selectOption("rg-production");
    await field("Foundry").selectOption("contoso-first-model");
    await page.getByText("モデル候補 3件", { exact: true }).waitFor();
    await chooseModel();
  };
  const configureNew = async () => {
    await button("＋ 新規追加").click();
    await field("サブスクリプション").selectOption("subscription-001");
    await button("リソースグループを新規作成").click();
    await field("新しいリソースグループ名").fill("rg-result");
    await button("使用").click();
    await button("Foundryを新規作成").click();
    await field("新しいFoundry名").fill("new-result-foundry");
    await button("使用").click();
    await page.getByText("このリージョンの候補です。Foundryの作成後に再確認します。", { exact: true }).waitFor();
    await chooseModel();
  };
  const resultCases = [
    { value: "group-failure", configure: configureNew, stage: "Foundry「new-result-foundry」がAzureに残っている可能性", residual: "リソースグループ「rg-result」は作成済み" },
    { value: "foundry-candidate-mismatch", configure: configureNew, stage: "現在の候補と一致しません", residual: "Foundry「new-result-foundry」は作成済み" },
    { value: "deployment-failure", configure: configureExisting, stage: "デプロイ「first-chat」がAzureに残っている可能性", residual: "Azureで状態を確認してください" },
    { value: "unknown", configure: configureExisting, stage: "デプロイ「first-chat」の作成が完了したか確認できませんでした", residual: "処理が続いている可能性" },
    { value: "conflict", configure: configureExisting, stage: "デプロイ名「first-chat」は既に使われています", residual: "既存のデプロイは変更していません" },
    { value: "list-failure", configure: configureExisting, stage: "デプロイ「first-chat」は作成済み", residual: "再度追加する必要はありません" },
    { value: "interrupted", configure: configureExisting, stage: "デプロイ「first-chat」の作成処理はAzureで続いている可能性", residual: "作成を取り消せません" },
  ];
  for (const resultCase of resultCases) {
    await resultCase.configure();
    await page.getByText("モック：作成結果の再現", { exact: true }).click();
    await field("作成結果の状態").selectOption(resultCase.value);
    await button("確認").click();
    await button("追加").click();
    assert(await page.getByRole("heading", { name: /完了/ }).count() === 0, `${resultCase.value}: 成功見出しを使わない`);
    assert(await button("追加").count() === 0 && await button("確認").count() === 0, `${resultCase.value}: 結果画面から書込操作を出さない`);
    const resultText = await page.locator(".add-result-description").textContent();
    assert(resultText.includes(resultCase.stage) && resultText.includes(resultCase.residual), `${resultCase.value}: 対象名と停止段階を表示`);
    assert(!/再現|想定|未確認|送信|書き込み/.test(resultText), `${resultCase.value}: 本文にモック説明や実装用語を混ぜない`);
    await page.setViewportSize({ width: 1080, height: 720 });
    await page.screenshot({ path: `docs/verification/f3-copy-result-${resultCase.value}.png` });
    if (resultCase.value === "unknown" || resultCase.value === "interrupted") {
      await button("状態を確認").click();
      await page.getByText("まだ作成結果を確認できません。時間をおいて、もう一度確認してください。", { exact: true }).waitFor();
      assert(await page.getByText("まだ作成結果を確認できません。時間をおいて、もう一度確認してください。", { exact: true }).count() === 1, `${resultCase.value}: 読取再試行後も未知を維持`);
    } else if (resultCase.value === "list-failure") {
      await button("一覧を再取得").click();
      await page.getByText("一覧を更新しました。", { exact: true }).waitFor();
      assert(await page.getByText("一覧を更新しました。", { exact: true }).count() === 1, "一覧更新失敗: 一覧の読取だけを再試行");
    }
    assert(await button("追加").count() === 0 && await button("再送").count() === 0, `${resultCase.value}: 再追加・再送ボタンなし`);
    await button("閉じる").click();
    await page.getByRole("dialog").waitFor({ state: "detached" });
  }

  // An addition that an earlier run left unresolved is offered for a state
  // check as soon as the dialog opens, instead of staying hidden until the same
  // name is sent again. The mock reports one while the unknown result is
  // selected.
  await configureExisting();
  await page.getByText("モック：作成結果の再現", { exact: true }).click();
  await field("作成結果の状態").selectOption("unknown");
  await button("モデル追加を閉じる").click();
  await page.getByRole("dialog").waitFor({ state: "detached" });
  await button("＋ 新規追加").click();
  const pending = page.locator(".add-pending");
  await pending.waitFor();
  const pendingText = await pending.textContent();
  assert(pendingText.includes("chat-production-2"), "未解決の追加を対象名で示す");
  assert(!/再送|やり直/.test(pendingText), "未解決の通知から再送を促さない");
  assert(await pending.getByRole("button", { name: "追加" }).count() === 0, "未解決の通知に追加操作を置かない");
  await pending.getByRole("button", { name: "状態を確認", exact: true }).click();
  await pending.getByText("作成結果を確認できません", { exact: false }).waitFor();
  await page.setViewportSize({ width: 1080, height: 720 });
  await page.screenshot({ path: "docs/verification/f3-copy-pending-operation.png" });
  await page.setViewportSize({ width: 1440, height: 1000 });
  // The reopened dialog shows the default selection again while the mock still
  // holds the unknown result, so the notice is cleared by selecting a different
  // result rather than the one already displayed.
  await page.getByText("モック：作成結果の再現", { exact: true }).click();
  await field("作成結果の状態").selectOption("deployment-failure");
  await button("モデル追加を閉じる").click();
  await page.getByRole("dialog").waitFor({ state: "detached" });
  // The mock records the selected result without the screen waiting for it, so
  // the dialog is reopened until that selection is in effect. Only the mock
  // round trip is retried; the notice itself is read once per open.
  let cleared = false;
  for (let attempt = 0; attempt < 20 && !cleared; attempt += 1) {
    await button("＋ 新規追加").click();
    cleared = await page.locator(".add-pending").count() === 0;
    await button("モデル追加を閉じる").click();
    await page.getByRole("dialog").waitFor({ state: "detached" });
  }
  assert(cleared, "未解決がなければ通知を出さない");

  // The entry remains available when the deployment list itself is empty.
  await page.getByLabel("次回の取得状態", { exact: true }).selectOption("empty");
  await page.getByRole("button", { name: /(?:更新|再取得)$/ }).click();
  await page.getByRole("heading", { name: "デプロイはありません", exact: true }).waitFor();
  await button("＋ 新規追加").click();
  await field("サブスクリプション").selectOption("subscription-001");
  await button("リソースグループを新規作成").click();
  await field("新しいリソースグループ名").fill("rg-direct");
  await button("使用").click();
  assert(await field("リソースグループ").inputValue() === "rg-direct", "配置先欄からの直接作成");
  await button("Foundryを新規作成").click();
  await button("戻る").click();
  assert(await field("Foundry").inputValue() === "", "キャンセルでFoundryを作らない");
  await button("Foundryを新規作成").click();
  await field("新しいFoundry名").fill("discarded-foundry");
  await button("使用").click();
  await chooseModel();
  await field("バージョン").selectOption("2024-11-20");
  await field("SKU").selectOption("Standard");
  await field("Capacity").fill("77");
  await field("デプロイ名").fill("discarded-deploy");
  await button("キャンセル").click();
  await page.getByRole("dialog").waitFor({ state: "detached" });
  await button("＋ 新規追加").click();
  assert(
    await field("サブスクリプション").inputValue() === "" &&
    await field("リソースグループ").inputValue() === "" &&
    await field("Foundry").inputValue() === "" &&
    await field("モデル").inputValue() === "" &&
    await field("バージョン").inputValue() === "" &&
    await field("SKU").inputValue() === "" &&
    await field("Capacity").inputValue() === "" &&
    await field("デプロイ名").inputValue() === "" &&
    await page.getByRole("option", { name: "rg-direct（新規作成予定）", exact: true }).count() === 0,
    "追加キャンセル後の再開で入力を破棄"
  );
  await page.setViewportSize({ width: 1080, height: 720 });
  assert(await page.locator("dialog").evaluate(el => el.scrollWidth <= el.clientWidth), "ダイアログの横はみ出し");
  await page.keyboard.press("Escape");
  assert(await page.getByRole("dialog").count() === 0, "Escapeで閉じる");
  assert(errors.length === 0, errors.join("\n"));
  return { passed: true, checks: ["model-loading-empty-error-retry-stale", "stable-validation-and-input-mode", "list-button-placement", "model-defaults", "version-sku-capacity-contract", "missing-default-and-capacity-data", "deploy-name-validation", "deployment-conflict-scope", "resource-naming", "deployed-foundry-review", "empty-foundry", "review-and-back-all-settings", "subscription-reset", "group-cancel-preserves-foundry", "nested-resource-creation", "foundry-setting-proposal", "mock-completion", "create-result-states", "read-only-result-retry", "empty-list-entry", "direct-group-creation", "cancel-reopen-resets", "cancel", "responsive-and-escape"], pageErrors: errors };
}
