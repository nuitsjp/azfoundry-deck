# 作業計画と未決事項

本書は引き継ぎの正本です。新しいセッションは本書だけで作業を再開できます。仕様・合意の根拠と検証の詳細は [プロジェクト定義](docs/project.md)、画面実装のルールは [frontend/src/AGENTS.md](frontend/src/AGENTS.md)、行動指針は [AGENTS.md](AGENTS.md) にあります。

## 1. 現在の到達点（2026-09-15）

**F3「モデルを追加する」は実処理まで完了しました。** 配置先の探索、モデル候補と容量条件の取得、既存デプロイ名の照会、リソースグループ・Foundry・デプロイの作成をGoサービスで実装し、画面を接続しています。指定サブスクリプションで作成から削除までを実施し、作成した資源はすべて削除済みです。次はF3の変更・削除とF4クォータという新機能か、下記の未検証項目の消化に進みます。

| 機能 | 状態 |
| --- | --- |
| F1：アカウント横断のデプロイ一覧 | 完了。モックと実Azure読取を実装済み。残件は[Issue #1](https://github.com/nuitsjp/azfoundry-deck/issues/1)。 |
| F2：モデル候補参照（旧独立画面） | 実装済み。主導線がモデル追加に移ったため、画面を残すかは未判断。 |
| F3：モデル追加 | 完了。読取・作成とも実装し、実Azureで作成と削除を検証済み。未検証は第3節のU4。 |
| F3：既存デプロイの変更・削除 | 未着手。ユースケースも未合意。 |
| F4：クォータ参照 | 未着手。詳細未合意。 |

主ユースケースは「モデルを追加する」で、必要時に「Foundryを追加する」が拡張し、それを「リソースグループを追加する」が拡張します。画面では一覧見出し右の「＋ 新規追加」からサブスクリプション・リソースグループ・Foundryを選び、後ろ2つはその場で新規作成できます。デプロイ0件のFoundryも選べます。

## 2. 次に進める作業

1. **U4の未検証を消化する。** プロセス再起動をまたぐ操作記録の再開、送信直後の異常終了、監視URLの失効、外部からの同名作成との実競合。いずれも実装済み機能の検証で、異常系の再現手段づくりが要ります。詳細は第3節。
2. **旧F2独立画面の扱いを決める。** 利用者判断です。モデル追加が主導線になったため、参照専用の単独画面を残すか、削除して追加画面に一本化するかを確認します。
3. **F3の変更・削除を具体化する。** 既存デプロイのSKU・Capacity変更と削除。追加とは別のユースケースとして、ヒアリングとモックから始めます。
4. **F4クォータ参照を具体化する。** 操作入口、欠損値の表示、比較対象を定義するところから。
5. **Issue #1の残件を扱う。** 実Deployment APIの継続ページと別RBAC構成。後続機能を止める条件にはしません。

## 3. 未決・未検証事項

| ID | 内容 | 扱い |
| --- | --- | --- |
| U2 | Windows IMEの実際のOFF切替と高コントラスト表示が未検証。入力モード属性とheadless画面は確認済み。 | ネイティブを操作・観測できる環境で確認する。IMEのOFFを確認済みと報告しない。 |
| U3 | F1の実Deployment API複数ページと、別RBAC構成での挙動が未検証。[Issue #1](https://github.com/nuitsjp/azfoundry-deck/issues/1)の残件。 | Issueの再開条件に従う。認証できないサブスクリプションは合意済みの異常系で、再ログイン成功を要求しない。 |
| U4 | 作成処理の異常系が未検証。(1) 送信直後・応答保存前にプロセスが終了した場合の操作記録、(2) 再起動後に未解決記録を読んで再開する経路、(3) 非同期処理の監視URLが失効した場合、(4) 外部操作が同名デプロイを先に作った場合の競合。 | 正常系と停止条件は実Azureで検証済み。上記は再現手段が要る。実装は `internal/azurego/create.go` と `internal/azurego/operations.go`。 |
| U5 | デプロイ名の正式な長さ・文字規則が未確定。公式命名表にもOpenAPIにも定義がない。 | 現行の暫定UI規則（2〜64文字の半角英数字・アンダースコア・ハイフン・ピリオド）を維持する。リソースグループ名の1文字許容は再審議しない。 |

未決の利用者判断は、上記U4の検証をどこまで行うか、および第2節2の旧F2画面の扱いです。

## 4. 確定済みで再審議しない事項

新しいセッションで蒸し返さないための一覧です。根拠は [プロジェクト定義](docs/project.md) にあります。

- **モック操作の合意**: 2026-09-15、利用者がモデル追加の画面操作全体に合意済み（[記録](docs/project.md#f3-u1-review)）。再確認を前提にしない。
- **外部競合の方針**: 条件付き作成ヘッダーは公開OpenAPIにも固定SDKにも存在しない。送信直前の再照会と競合時停止だけを根拠とし、再照会から送信までの間に外部が同名を作った場合は上書き更新になる限界を受け入れる（[根拠](docs/project.md#f3-external-contract)）。409等で自動改名や更新へ切り替えない。
- **新規Foundryの作成値**: `kind=AIServices`、SKU `S0`、`publicNetworkAccess=Enabled`、ネットワーク既定Allow、`disableLocalAuth=true`、`identity=SystemAssigned`、`customSubDomainName` はFoundry名、プロジェクト・ストレージ・Key Vault・検索・ロール割当は作成しない、デプロイは `versionUpgradeOption=NoAutoUpgrade`（[契約](docs/project.md#f3-create-contract)）。実作成結果がこの値と一致することを確認済み。
- **容量条件の規則**: `allowedValues` があればその集合、なければ `maximum` があれば入力可能。下限は `minimum`、提示がなければアプリの制限として1。刻みは提示されたときだけ適用する。実測では全SKUに `maximum` があり、`minimum`/`step` は予約容量SKUのみ、`allowedValues` は皆無だった（[経緯](docs/project.md#f3-create-implementation)）。欠損を0や10で補完しない。
- **廃止予定モデル**: `Deprecating` と `Deprecated` は追加の候補から除外する。作成を要求してもリソースプロバイダーが `ServiceModelDeprecating` で拒否する。F2の参照表示は除外しない。
- **書込の自動再試行**: `policy.RetryOptions.MaxRetries` に負値を設定して無効化する。`0` は既定の3回になるため使わない。
- **操作記録**: 送信前に記録し、同期・置換を確認してから一度だけ送る。記録に失敗したら送信しない。トークン・キー・応答本文は保存しない。SDKの `ResumeToken` は形式が不透明なため永続化しない。

## 5. 実装の所在

| 対象 | 場所 |
| --- | --- |
| 画面 | [AddModel.tsx](frontend/src/AddModel.tsx)、[App.tsx](frontend/src/App.tsx)、[ModelCandidates.tsx](frontend/src/ModelCandidates.tsx)、[style.css](frontend/src/style.css) |
| Goサービス結果の画面向け変換 | [addModelSource.ts](frontend/src/addModelSource.ts)（候補のグループ化、容量条件、廃止予定の除外、配置先の組み立て） |
| 共通サービス | [モデル候補](internal/service/models.go)、[配置先とデプロイ名](internal/service/placements.go)、[作成](internal/service/creation.go)、[デプロイ一覧](internal/service/deployments.go) |
| Azure境界 | [読取](internal/azurego/provider.go)、[作成](internal/azurego/create.go)、[操作記録](internal/azurego/operations.go) |
| モック固定データ | [追加画面用](internal/mock/addmodel.go)（配置先・候補・取得状態・作成結果）、[F1/F2用](internal/mock/data.go) |
| 生成バインディング | `frontend/bindings/`（`mise run bindings` で再生成。Goサービスを変更したら必ず実行する） |
| 操作記録の実体 | `%APPDATA%\AzFoundryDeck\operations\<操作ID>.json`。アプリは自動削除しない。 |

モックの選択例：`Contoso Production` → `rg-production` → `contoso-first-model`（デプロイ0件）。同一Foundryの重複拒否は `contoso-chat-prod` の既存名 `chat-production` で確認できます。取得状態と作成結果は追加フォーム末尾の「モック：〜の再現」で切り替えます。

## 6. 実行と検証の手順

すべてリポジトリのルートで実行します。初回は `mise trust`、`mise install`、`mise run setup` を先に行ってください。

### 単体検査

```powershell
mise run check
mise run test
```

`mise run check` はバインディング生成・`go vet`・TypeScript検査を行います。Git Bashでは `mise run bindings` 内の `node -e` が失敗するため、PowerShellで実行してください。失敗する場合は `mise exec -- wails3 generate bindings -ts -d frontend/bindings` を直接実行します。

### モックの起動

```powershell
mise run mock:browser
```

`http://127.0.0.1:9245` で開きます。別ポートで起動する場合は `mise run build:browser` の後に `AZFOUNDRY_MOCK=1`、`WAILS_SERVER_HOST=127.0.0.1`、`WAILS_SERVER_PORT=<ポート>` を設定して `.\bin\azfoundry-deck-browser.exe` を実行します。実Azure接続は `AZFOUNDRY_MOCK=0` です。Windowsは起動中の実行ファイルを上書きできないため、ビルド前に対象ポートのプロセスだけを停止します。他のプロセスを一括終了しないでください。

### 画面のheadless検証

`scripts/Test-F1Browser.ps1` と `scripts/Test-F2Browser.ps1` はPlaywright CLIでEdgeを起動しますが、**この環境ではEdge 152が終了コード `3221225477` で落ちて使えません**。同梱Chromiumを直接起動する [run-suite.mjs](frontend/tests/run-suite.mjs) を使ってください。サーバーを任意のポートで起動してから実行します。

```powershell
node frontend/tests/run-suite.mjs http://127.0.0.1:9252 frontend/tests/add-model.browser.js docs/verification/add-model-result.json
```

- 一覧は `frontend/tests/f1.browser.js`、旧F2画面は `frontend/tests/f2.browser.js`。f2のシナリオはポート **9246** を前提にしています。
- シナリオの状態選択はサーバー側に残ります。一覧の初期24件を前提にするf1の前には**サーバーを再起動**してください。
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
