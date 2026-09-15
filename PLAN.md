# 作業計画と未決事項

本書は引き継ぎの正本です。新しいセッションは本書だけで作業を再開できます。**最初に読むのは第2.1節「次のセッションの開始手順」です。**仕様・合意の根拠と検証の詳細は [プロジェクト定義](docs/project.md)、画面実装のルールは [frontend/src/AGENTS.md](frontend/src/AGENTS.md)、行動指針は [AGENTS.md](AGENTS.md) にあります。

## 1. 現在の到達点（2026-09-15）

**F3「モデルを追加する」は実処理と異常系の検証まで完了しました。** 配置先の探索、モデル候補と容量条件の取得、既存デプロイ名の照会、リソースグループ・Foundry・デプロイの作成をGoサービスで実装し、画面を接続しています。指定サブスクリプションで作成から削除までを実施し、作成した資源はすべて削除済みです。2026-09-15にU4の異常系4件を決定的なテストで検証し、そこで見つかった3件の不具合と1件の導線欠落を是正しました（[記録](docs/project.md#f3-u4-verification)）。

**同日、利用者が残りの未決事項をすべて判断しました。** 旧F2独立画面は削除済みです。次の作業はF3の「既存デプロイの変更」で、**削除は変更のあとに行います**。F4クォータは[Issue #3](https://github.com/nuitsjp/azfoundry-deck/issues/3)に登録して着手しません。[Issue #1](https://github.com/nuitsjp/azfoundry-deck/issues/1)は後回し、実Azureでの異常系再現は行いません。

| 機能 | 状態 |
| --- | --- |
| F1：アカウント横断のデプロイ一覧 | 完了。モックと実Azure読取を実装済み。残件は[Issue #1](https://github.com/nuitsjp/azfoundry-deck/issues/1)。 |
| F2：モデル候補参照（旧独立画面） | 削除済み（2026-09-15）。モデル候補の参照はモデル追加画面に一本化。 |
| F3：モデル追加 | 完了。読取・作成とも実装し、実Azureで作成と削除を検証済み。U4の異常系も検証・是正済みで、残るのは実Azureでの異常系再現のみ。 |
| F3：既存デプロイの変更 | 未着手。次の作業。ユースケースはこれからヒアリングする。 |
| F3：既存デプロイの削除 | 未着手。**変更のあとに行う**。取り消せない操作のため、確認の作法を別に詰める。 |
| F4：クォータ参照 | 着手しない。[Issue #3](https://github.com/nuitsjp/azfoundry-deck/issues/3)で追跡する。 |

主ユースケースは「モデルを追加する」で、必要時に「Foundryを追加する」が拡張し、それを「リソースグループを追加する」が拡張します。画面では一覧見出し右の「＋ 新規追加」からサブスクリプション・リソースグループ・Foundryを選び、後ろ2つはその場で新規作成できます。デプロイ0件のFoundryも選べます。

## 2. 次に進める作業

1. **F3「既存デプロイの変更」を具体化する。** 次の作業はこれです。既存デプロイのSKU・Capacityの変更を、追加とは別のユースケースとしてヒアリングとモックから始めます。最初に決めるのは操作の入口で、一覧の行から開くのか、追加と同じダイアログにするのかです。追加と違って**動いているデプロイに触る**操作である点を、モックの確認事項に含めます。
2. **F3「既存デプロイの削除」を、変更のあとに行う。** 変更と同時には進めません。削除は取り消せないため、確認の作法を独立して詰めます。**この項目を落とさないこと。** 変更が終わった時点で、削除のユースケースのヒアリングから始めます。

以下は着手しないと決めた項目です。再審議の対象にしません。

3. **F4クォータ参照** — [Issue #3](https://github.com/nuitsjp/azfoundry-deck/issues/3)に登録済み。操作入口・欠損値の表示・比較対象の3点が合意されたときに再開します。
4. **Issue #1の残件** — 後回し。実Deployment APIの継続ページと別RBAC構成は未検証のままにします。再現条件に当たったときに再開します。
5. **実Azureでの異常系再現** — 行いません。U4は決定的なテストで検証済みで、課金対象の資源作成に見合いません。

## 2.1 次のセッションの開始手順

次の担当が最初に行うことを、実行順に書きます。**ここから始めてください。**

### 手順1：状態を確かめる（作業前）

作業ツリーが `main` の最新であること、`git status` が空であることを確認します。ビルドと検査の入口は第6節にあります。`mise run check` はこの環境で落ちるため、第6節に書いた個別コマンドを使ってください。

### 手順2：ヒアリングする（実装前）

F3「既存デプロイの変更」はユースケースが未合意です。**モックも実装も作る前に、次の5点を利用者に確認します。** これらは利用者にしか答えられません。

| # | 確認すること | なぜ先に要るか |
| --- | --- | --- |
| 1 | 操作の入口。デプロイ一覧の行から開くのか、「＋ 新規追加」と同じダイアログを流用するのか。 | 画面構成が変わる。一覧の行に操作を足すなら、列レイアウトの再合意が要る。 |
| 2 | 変更できる項目。SKUとCapacityの両方か、Capacityだけか。モデルのバージョンを上げる操作を含めるか。 | 含める項目によって、送信前に再確認すべき候補の範囲が変わる。 |
| 3 | 変更前後の見せ方。現在値と変更後の値を並べて確認させるか、追加と同じ確認画面の形にするか。 | 動いているデプロイに触るため、何がどう変わるかを確認できないと危険。 |
| 4 | 変更中の失敗の扱い。Azureが受理したあとに結果が分からなくなった場合、追加と同じ「結果不明 → 状態を確認」でよいか。 | 追加で決めた結果区分をそのまま使えるかどうかが決まる。 |
| 5 | 実Azureでの検証対象。どのサブスクリプション・デプロイで試すか、費用と後片付けの条件。 | 書込を伴う検証は、対象と削除条件の明示承認なしに行わない（第6節）。 |

### 手順3：ヒアリング結果を記録してからモックへ

回答は [プロジェクト定義](docs/project.md) に日付つきの節として記録し、本書の第1節の表を更新します。そのうえでモックを作り、利用者の操作合意を得てから実処理へ進みます。追加のときと同じ順序です。

### 質問してはいけないこと（決定済み）

次は既に決まっています。ヒアリングで蒸し返さないでください。根拠は第4節と [プロジェクト定義](docs/project.md) にあります。

- 削除は変更のあと。変更と同時に進めない。
- F4クォータ、Issue #1、実Azureでの異常系再現は着手しない。
- 容量条件の規則、廃止予定モデルの除外、書込の自動再試行の無効化、操作記録の作法、受理後の失敗の扱い、排他ロックの方式。

### 変更機能で再利用できるもの（調査済み）

ヒアリング前に読んでおくと判断が早くなります。**いずれも読むだけで、変更しないでください。**

- 既存デプロイへのPUTは同じIDを更新する契約です。追加の導線では送信直前に同名を見つけたら競合として停止しますが（[create.go](internal/azurego/create.go)）、変更では逆に**対象が存在することが前提**になります。この停止条件をそのまま流用できません。
- 容量条件の判定（`capacityMismatch`）、書込の自動再試行の無効化、操作記録の保存と未解決の扱いは、追加と変更で同じ規則を使えます。
- 結果区分は8種類が [creation.go](internal/service/creation.go) にあります。変更で不足するものがあるかは、上のヒアリング4で決まります。

## 3. 未決・未検証事項

| ID | 内容 | 扱い |
| --- | --- | --- |
| U2 | Windows IMEの実際のOFF切替と高コントラスト表示が未検証。入力モード属性とheadless画面は確認済み。 | ネイティブを操作・観測できる環境で確認する。IMEのOFFを確認済みと報告しない。 |
| U3 | F1の実Deployment API複数ページと、別RBAC構成での挙動が未検証。[Issue #1](https://github.com/nuitsjp/azfoundry-deck/issues/1)の残件。 | Issueの再開条件に従う。認証できないサブスクリプションは合意済みの異常系で、再ログイン成功を要求しない。 |
| U4 | 作成処理の異常系(1)〜(4)は、Azureの読取・書込と非同期監視を差し替えた決定的なテストで検証・是正済み（[記録](docs/project.md#f3-u4-verification)）。 | 2026-09-15、利用者判断で**実Azureでの再現は行わない**と決定。テストは `internal/azurego/create_recovery_test.go`。実環境で確認したとは報告しない。 |
| U5 | デプロイ名の正式な長さ・文字規則が未確定。公式命名表にもOpenAPIにも定義がない。 | 現行の暫定UI規則（2〜64文字の半角英数字・アンダースコア・ハイフン・ピリオド）を維持する。リソースグループ名の1文字許容は再審議しない。 |

**未決の利用者判断はありません。** 2026-09-15にすべて判断済みです。次に判断が要るのは、F3「変更」のヒアリング内容です。

## 4. 確定済みで再審議しない事項

新しいセッションで蒸し返さないための一覧です。根拠は [プロジェクト定義](docs/project.md) にあります。

- **モック操作の合意**: 2026-09-15、利用者がモデル追加の画面操作全体に合意済み（[記録](docs/project.md#f3-u1-review)）。再確認を前提にしない。
- **外部競合の方針**: 条件付き作成ヘッダーは公開OpenAPIにも固定SDKにも存在しない。送信直前の再照会と競合時停止だけを根拠とし、再照会から送信までの間に外部が同名を作った場合は上書き更新になる限界を受け入れる（[根拠](docs/project.md#f3-external-contract)）。409等で自動改名や更新へ切り替えない。
- **新規Foundryの作成値**: `kind=AIServices`、SKU `S0`、`publicNetworkAccess=Enabled`、ネットワーク既定Allow、`disableLocalAuth=true`、`identity=SystemAssigned`、`customSubDomainName` はFoundry名、プロジェクト・ストレージ・Key Vault・検索・ロール割当は作成しない、デプロイは `versionUpgradeOption=NoAutoUpgrade`（[契約](docs/project.md#f3-create-contract)）。実作成結果がこの値と一致することを確認済み。
- **容量条件の規則**: `allowedValues` があればその集合、なければ `maximum` があれば入力可能。下限は `minimum`、提示がなければアプリの制限として1。刻みは提示されたときだけ適用する。実測では全SKUに `maximum` があり、`minimum`/`step` は予約容量SKUのみ、`allowedValues` は皆無だった（[経緯](docs/project.md#f3-create-implementation)）。欠損を0や10で補完しない。
- **旧F2独立画面の削除**: 2026-09-15、利用者判断で削除した。モデル候補の参照はモデル追加画面に一本化する。同じ候補を2か所で見せると、片方の変更がもう片方とずれるため。`ModelService` は追加画面が使い続けるので残す。復活させない。
- **F3の着手順**: 変更が先、削除が後。削除は取り消せないため、確認の作法を独立して詰める。
- **廃止予定モデル**: `Deprecating` と `Deprecated` は追加の候補から除外する。作成を要求してもリソースプロバイダーが `ServiceModelDeprecating` で拒否する。
- **書込の自動再試行**: `policy.RetryOptions.MaxRetries` に負値を設定して無効化する。`0` は既定の3回になるため使わない。
- **操作記録**: 送信前に記録し、同期・置換を確認してから一度だけ送る。記録に失敗したら送信しない。トークン・キー・応答本文は保存しない。SDKの `ResumeToken` も、ARMの操作状態URLも永続化しない。再起動後の照合は対象のデプロイを読み直して行う。
- **受理後の失敗の扱い**: Azureが要求を受理したあとに起きた失敗（監視URLの失効、待機中の切断）は結果不明であり、作成失敗とは報告しない。送信自体が拒否された場合だけ失敗とする。
- **排他ロック**: 所有プロセスがロックファイルを開いたまま保持する。生きている所有者のロックは奪わず、異常終了で残ったロックは引き継ぐ。二重送信を防ぐのはロックではなく未解決記録である。

## 5. 実装の所在

| 対象 | 場所 |
| --- | --- |
| 画面 | [AddModel.tsx](frontend/src/AddModel.tsx)、[App.tsx](frontend/src/App.tsx)、[style.css](frontend/src/style.css) |
| Goサービス結果の画面向け変換 | [addModelSource.ts](frontend/src/addModelSource.ts)（候補のグループ化、容量条件、廃止予定の除外、配置先の組み立て） |
| 共通サービス | [モデル候補](internal/service/models.go)、[配置先とデプロイ名](internal/service/placements.go)、[作成](internal/service/creation.go)、[デプロイ一覧](internal/service/deployments.go) |
| Azure境界 | [読取](internal/azurego/provider.go)、[作成](internal/azurego/create.go)、[操作記録](internal/azurego/operations.go)、[異常系の検証](internal/azurego/create_recovery_test.go) |
| モック固定データ | [追加画面用](internal/mock/addmodel.go)（配置先・候補・取得状態・作成結果）、[F1用](internal/mock/data.go) |
| 生成バインディング | `frontend/bindings/`（`mise run bindings` で再生成。Goサービスを変更したら必ず実行する） |
| 操作記録の実体 | `%APPDATA%\AzFoundryDeck\operations\<操作ID>.json`。アプリは自動削除しない。 |

モックの選択例：`Contoso Production` → `rg-production` → `contoso-first-model`（デプロイ0件）。同一Foundryの重複拒否は `contoso-chat-prod` の既存名 `chat-production` で確認できます。取得状態と作成結果は追加フォーム末尾の「モック：〜の再現」で切り替えます。作成結果に「結果不明」を選んでダイアログを開き直すと、前回の未解決として1件が表示されます。この選択はサーバー側に残るため、別の結果へ切り替えるまで表示され続けます。

## 6. 実行と検証の手順

すべてリポジトリのルートで実行します。初回は `mise trust`、`mise install`、`mise run setup` を先に行ってください。

### 単体検査

```powershell
mise run check
mise run test
```

`mise run check` はバインディング生成・`go vet`・TypeScript検査を行います。**この環境では `bindings` タスク内の `node -e` がPowerShellでも構文エラーで落ちます**（入れ子の引用符が壊れる）。`check`・`test`・`build:browser` はいずれも `bindings` に依存するため同じ理由で止まります。次を直接実行してください。

```powershell
New-Item -ItemType Directory -Force frontend/dist | Out-Null
mise exec -- wails3 generate bindings -ts -d frontend/bindings
go vet ./...
npm --prefix frontend run typecheck
go test ./...
npm --prefix frontend test
```

ブラウザー実行ファイルは `npm --prefix frontend run build` のあと `mise exec -- go build -tags server,production -trimpath -o bin/<名前>.exe .` で作ります。

### モックの起動

```powershell
mise run mock:browser
```

`http://127.0.0.1:9245` で開きます。別ポートで起動する場合は `mise run build:browser` の後に `AZFOUNDRY_MOCK=1`、`WAILS_SERVER_HOST=127.0.0.1`、`WAILS_SERVER_PORT=<ポート>` を設定して `.\bin\azfoundry-deck-browser.exe` を実行します。実Azure接続は `AZFOUNDRY_MOCK=0` です。Windowsは起動中の実行ファイルを上書きできないため、ビルド前に対象ポートのプロセスだけを停止します。他のプロセスを一括終了しないでください。

### 画面のheadless検証

`scripts/Test-F1Browser.ps1` と `scripts/Test-AddModelBrowser.ps1` はPlaywright CLIでEdgeを起動しますが、**この環境ではEdge 152が終了コード `3221225477` で落ちて使えません**。同梱Chromiumを直接起動する [run-suite.mjs](frontend/tests/run-suite.mjs) を使ってください。サーバーを任意のポートで起動してから実行します。

```powershell
node frontend/tests/run-suite.mjs http://127.0.0.1:9252 frontend/tests/add-model.browser.js docs/verification/add-model-result.json
```

- 一覧は `frontend/tests/f1.browser.js`。旧F2画面のシナリオは画面とともに削除しました。
- シナリオの状態選択はサーバー側に残ります。一覧の初期24件を前提にするf1と、未解決通知のない初期状態を前提にするadd-modelの前には、**サーバーを再起動**してください。
- 各シナリオは `docs/verification/` へスクリーンショットを上書き保存します。既存の証跡を残したい場合は実行前に退避するか、実行後に `git checkout -- docs/verification` で戻してください。
- 既存のEdgeやログイン済みプロファイルには接続しません。

### 実Azureの検証

読取は `mise run test:azure`。追加の配置先読取は次のとおりです。

```powershell
$env:AZFOUNDRY_LIVE='1'; go test ./internal/azurego -run TestLiveAzurePlacements -count=1 -v
```

書込を伴う検証は既存デプロイを保護する停止条件の確認だけが自動化されています。対象を明示したうえで実行してください。

```powershell
$env:AZFOUNDRY_LIVE_WRITE='1'
$env:AZFOUNDRY_LIVE_SUBSCRIPTION='<サブスクリプションID>'
$env:AZFOUNDRY_LIVE_GROUP='<リソースグループ>'
$env:AZFOUNDRY_LIVE_FOUNDRY='<Foundry名>'
$env:AZFOUNDRY_LIVE_DEPLOYMENT='<既存のデプロイ名>'
go test ./internal/azurego -run TestLiveAzureCreate -count=1 -v
```

実Azureへ資源を作成する検証は、費用・対象・保持期間・削除条件について利用者の明示承認を得てから行います。2026-09-15の検証では承認のうえで作成し、すべて削除しました。

### 文書の確認

変更後は `git diff HEAD --check` と、本書および [プロジェクト定義](docs/project.md) の相対リンク・明示アンカーの整合を確認します。`docs/project.md` の[参照資料](docs/project.md#references)にある外部の絶対パスは検査対象外です。

## 7. 環境上の注意

- **Edgeのheadless起動が失敗します。** Edge 152.0.4191.66 で `3221225477`（アクセス違反）が出ます。原因は未特定です。同梱Chromiumは正常に起動するため、第6節の手順で検証できます。Playwright CLIのセッションデーモンは、Chromium設定では応答が返らないため使いません。
- **Git Bashのヒアドキュメントはバックスラッシュを畳みます。** `\\n` が改行に化けるため、`\` を含むスクリプトをヒアドキュメントで渡さず、ファイルに書いてから実行してください。
- **`bin/` の実行ファイルは使い捨てです。** Git管理外で、`mise run build` と `mise run build:browser` が生成します。過去のセッションで利用者確認用に保持していた `bin/azfoundry-deck-browser.exe`（9250番）と `bin/azfoundry-deck-u4-browser.exe`（9251番）は、その確認が完了して[記録](docs/project.md#f3-u4-review)に残ったため、もう保持する必要はありません。標準タスクで上書きしてかまいません。
