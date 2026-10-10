# AzFoundryDeck のプロジェクト定義

プロジェクト共通の要件・制約、ユースケース一覧、確認した事実、および実行・検証手順の正本です。全体構造は [アーキテクチャ](architecture.md) を参照します。

## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 解決する問題・達成したい結果 | Azure 上の Microsoft Foundry とデプロイ済みモデルを、Azure SDK for Go を使うデスクトップアプリから管理できるようにする。 |
| 利用者・利用場面 | Azure アカウントを持ち、Foundry を運用する個人。Windows デスクトップで利用する。 |
| 今回の対象 | Azure へのログインとログアウト（ログイン情報の保存と破棄を含む）、利用対象のテナントの選択と変更、Home画面でのデプロイモデルの閲覧（Foundry 一覧と選択済み Foundry のファイル保存と復元、選択された Foundry の全デプロイ済みモデルの Azure からの取得とメモリでの保持、選択された Foundry の Azure OpenAI エンドポイントと API キーのマスク表示とコピー、選択された Foundry のサブスクリプション ID の表示とコピー、そのサブスクリプションの当月の利用金額の Azure Cost Management からの取得と表示と更新、Foundry の変更・テナントの変更の後の閲覧、Foundry 一覧とデプロイモデルの Azure からの更新を含む）、選択したデプロイモデルの明細（一覧の取得結果の即時表示と、容量上限の取得）の確認、選択中の Foundry のデプロイモデルの削除、選択中の Foundry へのデプロイモデルの追加、選択中の Foundry の既存デプロイモデルの設定変更。 |
| 今回の対象外 | サブスクリプションの変更操作、Foundry の新規作成以外の変更操作、および上記以外の参照（別ユースケースとして順次追加する）。 |


配布は、バージョンタグに対応するWindows x64用インストーラーのGitHub Releasesへの公開を対象とします。アプリは起動時に GitHub Releases の新版を確認・取得・検証し、利用者の操作で更新して再起動します。

Foundry の追加は、新規リソースグループと Foundry をセットで作成する操作を対象とします。

## 2. 制約・品質要求・受け入れ条件

- Azure への操作は Go 言語用の Azure SDK（`azidentity`、`armXXX` など）で行う。
- Windows デスクトップ版を対象とする。
- アカウント識別情報は OS のクレデンシャルマネージャー、トークンは Azure SDK の永続キャッシュ（Windows のユーザー単位の暗号化）に保存し、平文でファイル・画面・ログへ出さない。
- Foundry の API キーはメモリだけに保持し、ファイル・ログへ出さない。画面ではマスクして表示し、平文は利用者のコピー操作でクリップボードへ渡す場合に限る。
- 受け入れ条件は各ユースケースとシナリオに記載する。

動作環境、規模、費用、データ取扱、セキュリティ、運用上の制約と、客観的に判定可能な受け入れ条件を記述します。判断に必要な未確定事項は推測で埋めず、停止点で利用者に確認して確定した仕様のみを反映します。

<a id="usecases"></a>
## 3. ユースケース一覧

ユースケースの共通事項は `usecases/<名称>/README.md`、シナリオと固有の受け入れ条件は同じディレクトリの `scenarios/<名称>.md` に記載します。案の検討・保存は [提示と保存の手順](standards/mock-driven-development.md#discussion) に従います（未着手のユースケースは下表に名称だけを置き、本文とリンクは作りません）。

ユースケースの単位・系列の分割・モック適用は [モック標準のユースケース分割](standards/mock-driven-development.md#discussion) に従います。

| ユースケース | 主アクター | 目的 | 実装順序 | 実現パターン | モック適用 |
| --- | --- | --- | --- | --- | --- |
| [Azureへログインする](usecases/Azureへログインする/README.md) | Foundry の運用者 | Azure の認証済み状態を確立し、次回起動へ引き継ぐ | 1 | [UCP-1](design/UCP-1.md) | 対象 |
| [Azureからログアウトする](usecases/Azureからログアウトする/README.md) | Foundry の運用者 | このアプリのログイン状態と保存した認証・テナント・閲覧情報をすべて破棄する | 2 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを閲覧する](usecases/デプロイモデルを閲覧する/README.md) | Foundry の運用者 | Home画面で Foundry 一覧と選択された Foundry のデプロイ済みモデルを確認する | 3 | [UCP-1](design/UCP-1.md) | 対象 |
| [Foundryを変更する](usecases/Foundryを変更する/README.md) | Foundry の運用者 | Home画面で閲覧する Foundry を別の Foundry に変更し、そのデプロイ済みモデルを確認する | 4 | [UCP-1](design/UCP-1.md) | 対象 |
| [Foundry一覧を更新する](usecases/Foundry一覧を更新する/README.md) | Foundry の運用者 | Azure から Foundry 一覧を取得し直し、Home画面の一覧を最新にする | 5 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを更新する](usecases/デプロイモデルを更新する/README.md) | Foundry の運用者 | 選択中の Foundry のデプロイ済みモデルを Azure から取得し直し、Home画面のモデル一覧を最新にする | 6 | [UCP-1](design/UCP-1.md) | 対象 |
| [テナントを変更する](usecases/テナントを変更する/README.md) | Azure にログイン済みの Foundry 運用者 | 認証済みのアカウントを維持したまま、Azure リソース操作の対象テナントを変更する | 7 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルの詳細を確認する](usecases/デプロイモデルの詳細を確認する/README.md) | Azure にログイン済みの Foundry 運用者 | 一覧から選んだデプロイモデルの設定と状態を即時に確認し、容量の上限を確認する | 8 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを削除する](usecases/デプロイモデルを削除する/README.md) | Azure にログイン済みの Foundry 運用者 | 選択中の Foundry のデプロイモデルを Azure 上で削除し、Home画面の一覧を最新にする | 9 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを追加する](usecases/デプロイモデルを追加する/README.md) | Azure にログイン済みの Foundry 運用者 | Home画面で選択中の Foundry に新しいデプロイモデルを作成し、一覧へ反映する | 10 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルの設定を変更する](usecases/デプロイモデルの設定を変更する/README.md) | Azure にログイン済みの Foundry 運用者 | Home画面で選択中の Foundry にある既存のデプロイモデルの設定を Azure 上で変更し、一覧と明細を最新にする | 11 | [UCP-1](design/UCP-1.md) | 対象 |
| [インストーラーをReleasesへ発行する](usecases/インストーラーをReleasesへ発行する/README.md) | リリース担当者 | 指定したバージョンのWindows用インストーラーを公開する | 12 | [UCP-2](design/UCP-2.md) | UI確認不要 |
| [アプリをインストールする](usecases/アプリをインストールする/README.md) | Windowsでアプリを利用する人 | アプリをインストールし、起動方法を用意する | 13 | [UCP-2](design/UCP-2.md) | 対象 |
| [Foundryを追加する](usecases/Foundryを追加する/README.md) | Azure にログイン済みの Foundry 運用者 | 新規リソースグループと Foundry を作成し、Home画面で選択する | 14 | [UCP-1](design/UCP-1.md#新規リソースグループとfoundryを作成する) | 対象 |
| [Foundryを削除する](usecases/Foundryを削除する/README.md) | Azure にログイン済みの Foundry 運用者 | 選択中の Foundry を削除する。リソースグループに Foundry 関連しかなければ、リソースグループごと削除する | 15 | [UCP-1](design/UCP-1.md) | 対象 |
| [新版を確認してアプリを更新する](usecases/新版を確認してアプリを更新する/README.md) | Windowsでアプリを利用する人 | 起動時にバックグラウンドで GitHub Releases の新版を取得・検証し、利用者の操作で更新して再起動する | 16 | [UCP-3](design/UCP-3.md) | 対象 |
| [利用金額を更新する](usecases/利用金額を更新する/README.md) | Azure にログイン済みの Foundry 運用者 | 選択中の Foundry のサブスクリプションの当月の利用金額を取得し直し、Home画面の金額を最新にする | 17 | [UCP-1](design/UCP-1.md) | 対象 |

<a id="design"></a>
## 4. 確認した事実

ログイン情報の保存手段（確認日 2026-10-02。情報源は Go モジュールキャッシュ内のソースで、対象版は `azidentity` v1.14.1、`azidentity/cache` v0.4.0、`microsoft-authentication-extensions-for-go/cache` v0.1.1、`microsoft-authentication-library-for-go` v1.8.0、`zalando/go-keyring` v0.2.6。確認範囲は Windows の保存処理）:

- `azidentity/cache` の永続キャッシュは、Windows では `%LOCALAPPDATA%\.IdentityService\<Name>`（CAE 用は `<Name>.cae`）に DPAPI（`CryptProtectData`）で暗号化したファイルとして保存する。資格情報マネージャーは使わない。`Name` の既定は `msal.cache` で他アプリと共有され得る。
- `azidentity.Cache` は内部型の別名で、外部モジュールからは `cache.New` 以外で作れない（`azidentity/internal` の import は `use of internal package ... not allowed` でビルドできないことを実測）。そのため `InteractiveBrowserCredential` のトークン保存先を資格情報マネージャーへ差し替えられず、リフレッシュトークンも取り出せない。
- `azidentity/cache` v0.4.0 の公開 API は `New` と `Options` だけで、キャッシュの削除 API はない。Windows の保存先ファイルは `cacheFilePath`（`windows.KnownFolderPath(FOLDERID_LocalAppData)` 配下の `.IdentityService\<Name>`）と CAE 用の `<Name>.cae` で、データ本体を DPAPI で暗号化してそのパスに書く（`accessor/windows.go`）。排他用の `<パス>.lockfile` は操作中だけ作られ、解放時に削除される（`internal/lock/lock.go`）。この端末で `azfoundrydeck` と `azfoundrydeck.cae` がこのパスに存在することを確認した（読み取りのみ）。
- MSAL はトークン取得時に永続キャッシュへ書き込み、書き込みの失敗をトークン取得の失敗として返す（`apps/internal/base/base.go` の `AuthResultFromToken`）。
- `AuthenticationRecord` は authority、clientId、homeAccountId、tenantId、username、version の6項目の JSON で、秘密情報を含まない。
- `InteractiveBrowserCredential` に `AuthenticationRecord`、`Cache`、`DisableAutomaticAuthentication: true` を渡すと、`GetToken` は記録のアカウントで `AcquireTokenSilent`（キャッシュのトークン、または更新トークンによる更新）だけを行い、失敗時は `AuthenticationRequiredError` を返してブラウザー認証（`AcquireTokenInteractive`）へ進まない（`azidentity` の `public_client.go` の `GetToken`）。`Authenticate` はキャッシュを使わず常にブラウザー認証を行う。記録の JSON の読み込みは `version` がない、または未対応の場合にエラーを返す（`authentication_record.go`）。
- `go-keyring` は Windows で `danieljoos/wincred` の汎用資格情報を使い、対象名は `<service>:<user>`、保存値が 2560 バイトを超えると `ErrSetDataTooBig` を返す。本アプリの `AzFoundryDeck:AuthenticationRecord` へ試験用レコード（283 バイト）を書き込み、`cmdkey /list` での表示、読み出しと一致確認、削除を実機で確認した。

Foundry 一覧の取得方式（確認日 2026-10-03。情報源は開発者のサブスクリプション2件・Foundry 5件に対する実測。認証には Azure CLI のトークンを使った）:

- サブスクリプションごとの `armcognitiveservices` のアカウント一覧は、1ページ目に5件すべてが入り、後続の3ページは0件だった。1サブスクリプションの合計は3回の測定で 4.9〜12.2 秒で、全体の所要時間はこの最も遅い1件でほぼ決まる。
- Azure Resource Graph（`POST https://management.azure.com/providers/Microsoft.ResourceGraph/resources?api-version=2022-10-01`）の1回のクエリは、Reader 権限のまま同じ5件を 0.46〜0.55 秒で返した。`resourcecontainers` の `microsoft.resources/subscriptions` と `subscriptionId` で結合すると、サブスクリプション名まで 0.51〜0.78 秒で返した。
- 反映の遅れは、Foundry（AIServices アカウント）の作成が約0.6秒、削除が約9.3秒（各1回の測定）。計測用のリソースは削除と完全消去（purge）済み。
- 選択した Foundry のデプロイ済みモデル取得は 1.6〜3.4 秒だった（この Foundry のモデルは0件）。

デプロイ一覧と容量上限の取得時間（確認日 2026-10-04。情報源は実 Azure の Foundry 1件・デプロイ7件に対する各3回の実測。通常ビルドと同じ Azure SDK の呼び出しの所要時間）:

- デプロイ一覧（Deployments List）は 0.22〜0.81 秒、デプロイ単体の取得（Deployments Get）は1件 0.20〜0.38 秒だった。
- アカウント取得（Accounts Get）は 0.27〜0.44 秒、モデル定義一覧（Accounts ListModels）は 0.52〜2.5 秒、共有クォータ一覧（Usages List）は約1.3 秒だった。

Azure OpenAI エンドポイントの取得元（確認日 2026-10-05。情報源は開発者のサブスクリプションの AIServices アカウント2件に対する Azure CLI の `az cognitiveservices account show` の読み取り）:

- `properties.endpoints` は API 名をキーとする対応表で、`OpenAI Language Model Instance API` の値が `https://<アカウント名>.openai.azure.com/` だった。同じ URL は `OpenAI Realtime API` など他の OpenAI 系のキーにも入っており、`AI Foundry API` は `https://<アカウント名>.services.ai.azure.com/` で別の URL だった。

利用金額の取得（確認日 2026-10-08。情報源は開発者のサブスクリプション11件に対する Azure CLI の `az rest` での Cost Management Query API（`POST /subscriptions/<ID>/providers/Microsoft.CostManagement/query?api-version=2025-03-01`、種別 `ActualCost`、期間 `MonthToDate`、`Cost` の合計）の各2回の実測と、`armcostmanagement` v3.0.0 の `QueryClient.Usage` による3件の実測）:

- 応答は 0.77〜4.05 秒だった。列は `Cost` と `Currency` の2列で、金額のある5件はすべて `JPY` の1行（例: `25524.778183714`）を返した。
- 当月の利用がないサブスクリプション4件は、エラーではなく行0件を返した。
- 権限のないサブスクリプション2件は 403 `AuthorizationFailed`（`Microsoft.CostManagement/Query/read`）または 401 `RBACAccessDenied` を返した。
- 22回を連続して実行すると、16回目以降に 429 `Too many requests. Please retry.` を返した。
- 同じサブスクリプションへの連続した問い合わせは、4〜5回目で 429 になった（確認日 2026-10-08、1件に対する Python からの連続実行）。429 の応答は標準の `Retry-After` を持たず、`x-ms-ratelimit-microsoft.costmanagement-entity-retry-after` に 53〜56 秒、`x-ms-ratelimit-remaining-microsoft.costmanagement-entity-requests` に `DefaultQuota:0` を返した。成功時の応答はこれらの Cost Management 固有のヘッダーを返さなかった。
- 待機して問い合わせ直す実装で1件に8回を連続して実行すると、8回すべて成功し、うち2回は 20.4 秒と 54.8 秒かかった。

- **確認した事実**: 外部仕様や既存コードの調査結果（情報源、対象版、確認日、確認範囲）。仮定と明確に区別します。外部システムの実測応答を保存する場合は `reference/` に配置して参照します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

作業ディレクトリはリポジトリのルートです。コマンドは PowerShell で実行します。閲覧保存の基準フォルダーは [データ設計](design/data.md#閲覧データの保存範囲) に従い、アプリのデータフォルダー内でアカウントと選択テナントの組み合わせごとに分離します。以下の `foundry-state.json` はこの基準フォルダー内のファイルを指します。デプロイ一覧はファイルに保存しません。

| 目的 | コマンド・設定 | 成功確認 |
| --- | --- | --- |
| 環境構築 | `mise trust`、`mise run setup`、`mise run setup:browser` | 固定版のgolangci-lint・actionlint・govulncheckと依存関係を導入し、`frontend/bindings/azfoundrydeck/internal/azauth/` と `frontend/bindings/azfoundrydeck/internal/foundry/` が生成される。CIでは解析ツールを版・Go版・OS・アーキテクチャ別にキャッシュする |
| 編集時の整形・診断 | VS Codeでリポジトリの推奨拡張（Go・ESLint・Prettier）を導入し、環境構築後に編集する | Goはgofmt、フロントエンドは既存Prettierで保存時に整形する。gopls・TypeScript・ESLintが編集時に診断し、CLI・CIでも検査する |
| Go・TypeScriptの整形 | `mise run format` | Goはgofmt、TypeScriptはPrettierで整形される |
| Go・TypeScriptの整形確認 | `mise run format:check` | 終了コード0。整形が必要なファイルがあれば失敗する |
| TypeScript・設定・運用スクリプトのLint | `mise run lint` | 型情報を使い、Promiseの処理漏れ・Promiseを渡せない場所への指定・非Promiseのawaitを検査する。Hooks、設定ファイル、Node運用スクリプト、workflowも検査し、指摘があれば失敗する |
| Goのエラー処理・検査除外の検査 | `mise run lint:go` | 画面のビルド後、本番・E2E構成のerrcheckとnolintlintが合格する。エラーの明示的な破棄と、検査除外の対象解析器・理由・不要な除外を検査する。ツールの導入だけなら `node scripts/run.mjs lint:go:install`。固定版は `scripts/run.mjs` に指定する |
| self-hosted runner の初期登録・追加 | 管理者 PowerShell 7 で `mise run setup:runner`。2台目以降は `mise run setup:runner -- 2` のように正整数の番号を指定する。GitHub の Settings → Actions → Runners → New self-hosted runner から登録トークンを取得し、プロンプトで入力する。サービス実行アカウントもプロンプトで指定する | GitHub の Runners に runner が Online と表示される。番号省略・1は既存の配置先と名前を使い、2以降はフォルダー名・登録名に `-2` などを付けて別サービスにする。[設定スクリプト](../scripts/setup-runner.ps1) は既存ファイルがある配置先を上書きしない |
| runner 本体の自動更新 | runner の標準自動更新を有効にしたままサービスを常駐させる。別の定期タスクは不要 | [GitHub の仕様](https://docs.github.com/en/actions/reference/runners/self-hosted-runners#communication)では、ジョブ割り当て時、または新バージョン公開後1週間以内に更新される |
| 起動（ブラウザー確認） | `mise run server` | `http://127.0.0.1:34115/` を開くと、Home を背景にログインのモーダルが表示される |
| 起動（デスクトップ） | `mise run dev` | ウィンドウにログインのモーダルが表示される（未検証） |
| モデル明細の画面確認用起動 | `mise run server:review:deployment-detail` | `http://127.0.0.1:34123/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。`AZFOUNDRYDECK_E2E_CAPACITY_REVIEW=1` を自動設定し、容量上限と接続情報（Azure OpenAI Endpoint・API key）の固定応答を4秒遅延させる。起動直後と Foundry の変更直後は1行に並んだ Azure OpenAI Endpoint・API key に回転表示と Loading... を表示してコピーボタンを無効にし、取得後に `https://contoso-foundry-production-japaneast.openai.azure.com/openai/v1` と伏せ字に末尾 `prd1` を添えたキーを表示する。コピーボタンは全文をクリップボードへ書き込み、約2秒間 `Copied` を表示する。起動直後は一覧の全行の Capacity の分母に回転表示と Loading... を表示し、追加操作なしに全行が更新されることを確認できる。行を選ぶと Details も同じ取得結果で更新され、取得開始から4秒以上待ってから選ぶと即時表示を確認できる。デプロイ一覧は3件、明細は未選択で空。`chat-production` の行のどこを押しても、明細にモデル `gpt-4.1`、SKU `GlobalStandard`、Capacity の現在値 `50,000`、Provisioning state、Upgrade policy `Upgrade to new default` を即時に表示する。上限情報が取得済みなら `50,000 / 160,000 TPM` を即時表示する。取得中なら上限だけを `Loading...` にし、取得完了後に更新する。`chat-mini` は `100,000 / 250,000 TPM` と `Upgrade on retirement`、`embeddings` は SKU `Standard`、`20,000 / 80,000 TPM` と `No automatic upgrade` となる。上限取得済みの場合は同じ Foundry の他の行でも即時表示する。認証と外部取得のみ固定応答で、起動時の上限の並行取得と表示は通常と同じ処理を通す。実 Azure と本番保存先には触れない |
| 明細の画面確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` または `mise run dev` | 通常ビルドは固定応答を含まない。明細は一覧の取得結果から即時表示し、対象 Foundry が確定した時点でデプロイ一覧と並行して実 Azure からモデル定義と共有クォータを取得する。上限の保持状態と完了通知から明細を表示・更新する（Foundry の変更・画面の読み込みまで保持する）。Capacity は設定済み容量／割り当て可能上限と単位を表示し、上限の取得に失敗したときは上限の位置に `Not set`、`DEPLOYMENT_DETAIL_FAILED` と `Retry` を表示する。接続情報も同じ時点に実 Azure から Accounts Get のエンドポイント（`/openai/v1` を付けて表示する）と Accounts ListKeys の Key1 を取得し、メモリだけに保持する。画面確認用データを本番へ持ち込まない |
| デプロイモデル削除の画面確認用起動 | `mise run server:review:deployment-delete` | `http://127.0.0.1:34124/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。モデル一覧は `chat-production`・`chat-mini`・`embeddings` の3件。外部の取得と削除だけが固定応答で、削除したデプロイは以降の取得から除かれる。保存・削除後の再取得・表示は通常と同じ処理を通す。実 Azure には触れない |
| デプロイモデル削除の確認 | 起動後、`chat-mini` 行を選んで右側の明細を表示し、明細下部右端のゴミ箱アイコン（Delete）を押してダイアログで「Cancel」を押す。もう一度ゴミ箱アイコンを押し、ダイアログの「Delete」を押す | ダイアログは Foundry 名・デプロイ名・「This cannot be undone.」を表示し、Cancel では一覧が3件のまま。削除後は進捗モーダル「Deleting deployment」（応答が速いため短い状態は目視できないことがある）が閉じ、一覧が `chat-production`・`embeddings` の2件になり、件数と最終取得日時が更新される。再読み込み後も2件のまま |
| デプロイモデル削除の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='delete'` を設定し、`mise run server:review:deployment-delete` を起動する。同じ手順で削除する | 一覧は3件のまま、ページ本文の先頭に `DEPLOYMENT_DELETE_FAILED` と理由を赤いバナーで表示し、「×」で閉じられる。「Delete」を再度押せる。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| デプロイモデル削除の確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | 通常ビルドは固定応答を含まない。実 Azure への削除は `armcognitiveservices.DeploymentsClient` で実行され、完了後に選択中 Foundry のデプロイ一覧を Azure から再取得して表示を更新する |
| 通常構成でのデプロイモデルの削除 | `mise run server` で Home画面を開き、モデル明細の右下にあるゴミ箱アイコンを押して削除する | 実 Azure 上でデプロイを削除し、デプロイ一覧を Azure から取得し直して更新する（未検証。実装フェーズで利用者が確認） |
| デプロイモデル追加の画面確認用起動 | `mise run server:review:deployment-add` | `http://127.0.0.1:34125/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。モデル一覧は `chat-production`・`chat-mini`・`embeddings` の3件。追加カタログは `internal/foundry/e2e.go` の固定応答を使い、取得状態・保持・画面表示は通常と同じ Go サービスと画面を通す。認証・外部操作も固定応答であり、実 Azure には接続しない |
| デプロイモデル追加の確認 | 起動後、「+ Add deployment」を押す。フィルターを試し、モデルを選択して設定を入力し「Deploy」を押す。閉じた後に Foundry を development へ切り替え、もう一度開く | 固定待機を挟まず左右2ペインを表示する。モデル選択時に Deployment name が自動設定され、Standard の Capacity は上限の50%となり、直接入力とスライダーが連動する。Pay-as-you-go は料金レートを表示する。Deploy で進捗を表示し、完了後に両モーダルが閉じて一覧へ追加される。Foundry 切替後は検索・フィルター・入力が初期化され、新しい Foundry のカタログが表示される |
| 追加カタログの初回取得中の確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_CATALOG_REVIEW='1'` を設定し、`mise run server:review:deployment-add` を起動する。Home 表示直後、または Foundry 切替直後に Add deployment を押す | Go の固定外部境界がモデル定義の取得を2秒遅延する。取得中はモーダル内にプログレスを表示し、完了後にカタログを表示する。取得中に閉じて開き直しても同じ取得の完了を待ち、取得済みなら待機しない。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_CATALOG_REVIEW` で解除する |
| 追加画面のクォータ取得・更新中の確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_QUOTA_REVIEW='1'` を設定し、`mise run server:review:deployment-add` を起動する。Home 表示直後に Add deployment を開く。上限の取得後に新しい名前で Deploy し、完了後すぐ再び開く | モデル定義の取得後、初回のクォータ取得と作成成功後のクォータ更新をそれぞれ4秒遅延する。モデル一覧と選択は利用でき、Standard の上限は Loading...、Capacity と Deploy は無効となる。完了後に上限を表示する。初期値は上限の50%とし、入力済み値は保持する。モデル定義の2秒遅延も設定した場合は、その後にクォータの4秒遅延が始まる。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_QUOTA_REVIEW` で解除する |
| 追加画面のクォータ取得失敗の確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='quota'` を設定し、`mise run server:review:deployment-add` を起動して Add deployment を押す | モデル定義は取得でき、モデル一覧と選択を表示する。クォータ取得は失敗し、Standard では原因を表示して、上限を Not available、Capacity と Deploy を無効にする。Pay-as-you-go の選択と料金表示は利用できる。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で解除する |
| デプロイモデル追加の確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | 通常ビルドは固定応答を含まない。実 Azure へのデプロイ作成は実 SDK で実行され、完了後に選択中 Foundry のデプロイ一覧を Azure から再取得して表示を更新する |
| 通常構成でのデプロイモデルの追加 | `mise run server` で Home画面を開き、「+ Add deployment」ボタンを押して新規デプロイを追加する | 実 Azure 上でモデルをデプロイし、デプロイ一覧を Azure から取得し直して更新する |
| Foundry削除の画面確認用起動 | `mise run server:review:foundry-delete` | `http://127.0.0.1:34128/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。Foundry は `contoso-foundry-production-japaneast`・`contoso-foundry-development`・`contoso-foundry-research` の3件で、各リソースグループは Foundry 関連のみ。外部の削除・消去だけが固定応答で、削除した Foundry は以降の取得から除かれる。削除は遅さを再現するためFoundryの削除に約3秒、消去に約2秒、リソースグループの削除に約3秒かかる。保存・一覧の更新・表示は通常と同じ処理を通す。実 Azure には触れない |
| Foundry削除の確認 | 起動後、Foundry 見出し行の右端にある「Delete Foundry」アイコンボタンを押して確認ダイアログの「Cancel」を押す。もう一度「Delete Foundry」アイコンボタンを押し、ダイアログの「Delete」を押す | ダイアログは Foundry 名・リソースグループ名・リソースグループごと削除される旨・「This cannot be undone.」を表示し、Cancel では何も変わらない。削除中は進捗モーダル「Deleting Foundry」を表示し、「Delete Foundry」「Purge Foundry」「Delete resource group」「Update Home」の4行が待機・実行中・完了と進み、Escape や外側クリックでは閉じない。完了後に次の Foundry（`contoso-foundry-development`）が選択されてデプロイ3件が表示される。最後の1件を削除すると Foundry 一覧とモデル一覧は空になり、「Delete Foundry」は無効になる |
| Foundry削除の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='foundry-delete'` を設定し、`mise run server:review:foundry-delete` を起動する。同じ手順で削除する | 進捗モーダルが閉じ、一覧と選択は変わらず、ページ本文の先頭に `FOUNDRY_DELETE_FAILED` と理由を赤いバナーで表示する（未検証）。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| Foundry削除の確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | 通常ビルドは固定応答を含まず、実 Azure への削除は実装フェーズで接続する（現在は未接続のため `FOUNDRY_DELETE_FAILED` となる） |
| デプロイモデル設定変更の画面確認用起動 | `mise run server:review:deployment-update` | `http://127.0.0.1:34126/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。モデル一覧は `chat-production`・`chat-mini`・`embeddings` の3件。認証と外部取得だけが固定応答で、設定の取得・変更と変更後の再取得は通常と同じ処理を通す。実 Azure には触れない |
| デプロイモデル設定変更の確認 | 起動後、`chat-production` 行を選んで明細を表示し、明細見出しの Edit deployment を押す。Version を `2024-11-20`、Capacity を `80,000`、Upgrade policy を `Upgrade on retirement` に変えて Update を押す | 設定モーダル Edit deployment が開き、Deployment name・Model・SKU は変更できない。初期値は Version `2025-04-14`、Capacity `50,000 / 160,000 TPM`、Upgrade policy `Upgrade to new default` で、値が変わるまで Update は無効。Update 後は進捗モーダル Updating deployment（Step: Update）が閉じ、一覧の Version が `2024-11-20` になり、同じ行の明細が Capacity `80,000 / 160,000 TPM` と Upgrade policy `Upgrade on retirement` になる。件数は3件のまま、デプロイ一覧の最終取得日時が更新される |
| デプロイモデル設定変更の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='update'` を設定し、`mise run server:review:deployment-update` を起動する。同じ手順で Update を押す | 進捗モーダルだけが閉じ、設定モーダルは入力値を保持したまま開いている。モーダル内に `DEPLOYMENT_UPDATE_FAILED` と理由が表示され、Update を再度押せる。一覧は変更前のまま。編集モーダルの共有クォータの取り直しの失敗は `$env:AZFOUNDRYDECK_E2E_FAIL='update-settings'` で、変更を開始せず同じエラーコードと Retry をモーダル内に表示する。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| 新版の更新の画面確認用起動 | `mise run server:review:update` | `http://127.0.0.1:34129/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。データフォルダーの `e2e-release` に起動ごとの鍵で署名した v0.2.0 の `update.json` とインストーラーを作り、GitHub Releases の代わりに更新元とする。起動の約5秒後に新版を確認し、Home画面の先頭に更新の区画を表示する。確認・取得・検証・適用前の再検証は通常と同じ処理を通し、インストーラーは実行せずにログに記録してサーバーを終了する |
| 新版の再検証の失敗確認 | `mise run server:review:update-untrusted` | `http://127.0.0.1:34130/` で同じく更新の区画を表示する。取得後に置いたインストーラーを書き換えてあるため、`更新して再起動` を押すとサーバーは終了せず、区画に `更新を検証できませんでした。次回の起動時に確認し直します。` を表示する |
| デプロイモデル設定変更の確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run dev` | 通常ビルドは固定応答を含まない。実 Azure への設定変更は `DeploymentsClient.BeginCreateOrUpdate` で実行され、完了後に選択中 Foundry のデプロイ一覧を Azure から再取得し、同じデプロイの明細を新しい一覧から表示する |
| 通常構成でのデプロイモデルの設定変更 | `mise run dev` で Home画面を開き、明細見出しの Edit deployment から Version、Capacity、Upgrade policy を変えて Update を押す | 実 Azure 上のデプロイが変わり、デプロイ一覧と明細が更新される（未検証。実装フェーズで利用者が確認） |

| 1件テナントのサインイン画面確認用起動 | `mise run server:review:login` | `http://127.0.0.1:34117/` が未ログインで開く。起動のたびに空の一時フォルダー `AzFoundryDeck-login-review-...` を作り、端末にパスを表示する。「Azureにログイン」を押すと、ブラウザーを開かずに固定応答で認証し、唯一の候補 ID `e2e-azure-tenant`、表示名 `Contoso` が選択される。認証記録のテナント ID `e2e-tenant` は候補 ID と異なる。ヘッダーのプルダウンとユーザーアイコンへのマウスオーバーで選択名・アカウント名 `operator@contoso.onmicrosoft.com` を確認する。実 Azure・資格情報マネージャー・永続キャッシュには触れない。一覧・選択のファイル保存とアカウント・テナント別の閲覧保存先決定は通常と同じ処理を通す。実 Azure と本番の保存先はこの確認用構成の検証対象に含めない。構成は [サインイン設計](design/UCP-1.md#ブラウザーでazureにサインインする) を参照する |
| サインイン画面確認用構成の終了と実処理への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` または `mise run dev` | 確認用の一時データを通常データへ持ち込まず、実 Azure の認証経路に切り替わる。通常構成には実認証・テナント一覧取得と一覧・選択の永続保存を接続している。実 Azure の認証と本番保存先の確認には通常構成を使う |
| 複数テナントの選択画面確認用起動 | `mise run server:review:login-multiple` | `http://127.0.0.1:34118/` が未ログインで開く。起動のたびに空の一時ディレクトリを作り、端末にパスを表示する。「Azureにログイン」を押すと、ブラウザーを開かずに固定応答で認証し、「テナントを選択」画面を表示する。固定候補は `Contoso`、`Contoso Development`、`Fabrikam`、`Northwind`、`Adventure Works`、`Woodgrove`、`Tailspin` の7件。初期状態の「テナント」は未選択で「テナントを選んでください」を表示し、「確定」は無効。候補を選んで確定すると、Home と選択したテナントのヘッダープルダウン、ユーザーアイコンを表示する。構成は [複数テナントのサインイン設計](design/UCP-1.md#サインイン時に複数テナントが存在する) を参照する。ログイン後のテナント変更は対象外 |
| 複数テナントの選択保存失敗の確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='select-save'` を設定し、`mise run server:review:login-multiple` を起動する | 候補を選んで「確定」を押すと、選択保存だけが失敗する。テナント選択画面にエラーを表示し、選択値を保持して再試行できる。Home の閲覧を開始しない。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| 複数テナントの画面確認用構成の終了と実処理への切り替え | 起動端末で `Ctrl+C`。失敗注入を使った場合は解除し、通常構成を `mise run server` または `mise run dev` で起動する | 固定候補の取得に使う `AZFOUNDRYDECK_E2E_TENANTS=multiple` は確認用起動コマンドが子プロセスだけに設定する。終了後の通常ビルドは実 Azure の認証・一覧取得を使い、確認用の一時データを持ち込まない。通常構成で実 Azure の認証、選択先のトークン取得、資格情報と一覧・選択の保存を確認する |
| テナント変更の画面確認用起動 | `mise run server:review:tenant-change` | `http://127.0.0.1:34119/` がログイン済み（アカウント `operator@contoso.onmicrosoft.com`、テナント `Contoso`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-tenant-change-review` を作り直し、テナント3件（`Contoso`、`Fabrikam`、`Northwind`）の認証記録を置く。保存済みの閲覧結果はないため、最初の Home は固定応答で Foundry 一覧とデプロイ一覧を取得する。トークン取得は固定応答で成功し、一覧・選択の保存と読み込みは通常と同じ処理を通す |
| テナント変更の確認（保存済みの Foundry 一覧がない） | 起動後、ヘッダーのテナントプルダウンを開いて `Fabrikam` を選ぶ。続けてプルダウンを開き、`Fabrikam` を選び直す | 選択後にプルダウンが閉じ、処理中は Foundry の変更と更新ボタンが無効になる。変更前の Foundry・モデル表示が消えて、進捗モーダルで固定応答の Foundry 一覧とデプロイ一覧の取得を行い、完了後に `Fabrikam` の Home を表示する。ヘッダーのテナント表示は `Fabrikam` になる。同じテナントの選び直しでは何も起きない。再起動後の選択の復元は、この確認用起動が起動のたびにデータを作り直すため、この手順の対象外とする。仕様は [テナントを変更し閲覧する](usecases/テナントを変更する/scenarios/テナントを変更し閲覧する.md) を参照する |
| テナント変更の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='select-save'` を設定し、`mise run server:review:tenant-change` を起動する。`Fabrikam` を選ぶ | 選択保存だけが失敗し、ページ本文の先頭（「Home」見出しの下）に、赤いバナーで `SELECT_TENANT_FAILED` と理由を表示し、右上の「×」だけで閉じられる。テナント表示と Home は `Contoso` のまま変わらない。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| テナント変更で保存済みの Foundry 一覧を使う画面確認用起動 | `mise run server:review:tenant-revisit` | `http://127.0.0.1:34122/` がログイン済み（テナント `Contoso`、候補 `Contoso`・`Fabrikam`・`Northwind`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-tenant-revisit-review` を作り直し、`Fabrikam` の閲覧保存先に、Foundry 2件（2件目の `fabrikam-foundry-research` を選択）の状態ファイルを用意する。トークン取得とデプロイ取得は固定応答で、読み込み・表示は通常と同じ処理を通す |
| テナント変更で保存済みの Foundry 一覧を使う確認 | 起動後、ヘッダーのテナントプルダウンを開いて `Fabrikam` を選ぶ | 保存済みの Foundry 一覧と選択（`fabrikam-foundry-research`）をそのまま使い、Foundry 一覧は取得し直さない。進捗モーダルで選択中の Foundry のデプロイ一覧を固定応答から取得して表示し、容量上限情報も並行して取得する。Foundry 一覧の最終取得日時は保存時の `2026-09-01 09:00` になる。`Fabrikam` の状態ファイルには Foundry 一覧と選択だけが保存され、デプロイ一覧は保存されない。仕様は [テナントを変更し閲覧する](usecases/テナントを変更する/scenarios/テナントを変更し閲覧する.md) を参照する |
| Foundryが存在しない状態の画面確認用起動 | `mise run server:review:no-foundry` | `http://127.0.0.1:34120/` がログイン済み（テナント `Contoso`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-no-foundry-review` を作り直し、保存済みの閲覧結果はない。外部取得は固定応答で、Foundry を返さない（`AZFOUNDRYDECK_E2E_FOUNDRIES=none` を起動コマンドが子プロセスにだけ設定する）。保存・読み込み・表示は通常と同じ処理を通す |
| Foundryが存在しない状態の確認 | 起動後、Home画面を開く。続けて Home画面を再読み込みする | 取得の進捗モーダルのあと、Foundry のプルダウン（選択肢なし）と空のモデル一覧（0 件）を表示し、`FOUNDRY_LOAD_FAILED` は表示しない。「Refresh Foundries」ボタンと Foundry 一覧の最終取得日時を表示し、「Refresh models」ボタンは無効になる。`foundry-state.json` に空の一覧・選択なし・取得日時が保存され、再読み込み後も取得せずに同じ表示になる。「Refresh Foundries」を押して0件のままの場合は、次の0件への更新の確認で扱う。仕様は [Foundryが存在しない状態で閲覧する](usecases/デプロイモデルを閲覧する/scenarios/Foundryが存在しない状態で閲覧する.md) を参照する |
| Foundry一覧の更新で0件になる画面確認用起動 | `mise run server:review:foundry-empty` | `http://127.0.0.1:34121/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-empty-review` を作り直し、保存済みの Foundry 3件・選択（Production）の状態ファイルを用意する。外部取得は固定応答で、更新の取得結果は Foundry 0件（`AZFOUNDRYDECK_E2E_FOUNDRIES=none` を起動コマンドが子プロセスにだけ設定する）。保存・削除・読み込み・表示は通常と同じ処理を通す |
| Foundry一覧の更新で0件になる確認 | 起動後、Foundry のプルダウンの右にある「Refresh Foundries」ボタンを押す。続けて Home画面を再読み込みする | 進捗モーダル（「Deployments」は待機中のまま）のあと、Foundry のプルダウンとモデル一覧が空（0 件）になり、`FOUNDRY_LOAD_FAILED` は出ない。「Refresh models」ボタンは無効、「Refresh Foundries」ボタンは有効。`foundry-state.json` に空の一覧・選択なし・取得日時が保存される。再読み込み後も取得せずに同じ表示になる。仕様は [Foundryが存在しない状態へ一覧を更新する](usecases/Foundry一覧を更新する/scenarios/Foundryが存在しない状態へ一覧を更新する.md) を参照する |
| ログイン | モーダルの「Azureにログイン」を押し、開いたブラウザーでサインインする | 一覧が1件なら自動選択し、複数件なら候補を選んで「確定」を押す。モーダルが閉じてヘッダーにテナントプルダウンとユーザーアイコンが表示され、`cmdkey /list:AzFoundryDeck:AuthenticationRecord` に資格情報が表示され、`%LOCALAPPDATA%\.IdentityService\azfoundrydeck.cae` が作成される |
| 自動ログイン（起動時の復元） | 保存済みの認証記録・テナント一覧・選択がある状態で `mise run server` を起動し、`http://127.0.0.1:34115/` を開く | 保存済みの一覧と選択を読み込み、ARM の一覧を取得し直さず、ヘッダーにテナントプルダウンとユーザーアイコンを表示する。失敗時はモーダル内に `LOGIN_FAILED` と理由を表示し、保存情報を自動削除しない。仕様は [自動復元シナリオ](usecases/Azureへログインする/scenarios/保存済みのログイン情報で自動的にログイン済みになる.md) を参照する。再起動後も同じ一覧・選択と、そのアカウント・テナントの保存済み Foundry 一覧・選択を使い、デプロイ一覧は Azure から取得して表示する |
| ログアウト | ログイン済みの画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | 認証・テナント・全アカウントの閲覧保存情報とメモリ上の状態をすべて削除し、閲覧結果とヘッダー右を消してログインモーダルを表示する。削除範囲は [データ設計](design/data.md#ログアウト時の削除範囲) を参照する。この変更の実機動作は未検証 |
| 保存したログイン情報の手動削除 | `cmdkey /delete:AzFoundryDeck:AuthenticationRecord`、`Remove-Item "$env:LOCALAPPDATA\.IdentityService\azfoundrydeck*"` | `cmdkey /list:AzFoundryDeck:AuthenticationRecord` が「なし」を表示する |
| 終了 | 起動した端末で `Ctrl+C` | `http://127.0.0.1:34115/health` に応答しない |
| Home画面での閲覧（状態ファイルなし） | 閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で `mise run server` を起動し、ログイン済みで Home画面を表示する | 進捗モーダルを経て、参照可能な全 Foundry（サブスクリプション名、Foundry 名の昇順）と先頭の Foundry の全デプロイ済みモデルが表示され、閲覧保存の基準フォルダーの `foundry-state.json` に Foundry 一覧と選択が保存される（デプロイ一覧は保存しない）。取得または保存に失敗した場合は Home画面に `FOUNDRY_LOAD_FAILED` が表示される |
| 画面確認用の起動（Home画面・ログアウトの UI 確認） | `mise run server:review` | `http://127.0.0.1:34115/` がログイン済み（ヘッダー右に `Contoso` とユーザーアイコン）で開く。状態ファイルがなければ固定の Foundry 3件を取得して先頭を選択・保存し、あればその一覧と選択を使う。どちらの場合も選択中の Foundry のデプロイ3件を固定応答から取得する。外部取得の固定応答は待ち時間なしで返るため、進捗モーダルは短時間で閉じる。端末に `review data directory: <一時フォルダー>\AzFoundryDeck-review` が出る |
| サブスクリプションの利用金額の画面確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_CAPACITY_REVIEW='1'` を設定し、`mise run server:review` を起動して `http://127.0.0.1:34115/` を開く。Foundry のプルダウンで `contoso-foundry-development`、`contoso-foundry-research`、`contoso-foundry-production-japaneast` を順に選ぶ。続けて金額の右の「Refresh cost」ボタンにマウスを合わせて押し、最後に「Refresh models」を押す | プルダウンの下の `Subscription ID` 行の右に `This month` を表示する。取得中は約4秒間 `Loading...` を表示し、その後 Production は `¥12,346`、Development は `1,234.56 USD`、Research は `COST_LOAD_FAILED: Could not retrieve the month-to-date cost.` を赤字で表示する。「Refresh cost」はツールチップに名称を表示し、押すと取得中はボタンが無効になり、取得中の表示を経て金額を表示する。「Refresh models」では金額を取得し直さない。通常ビルドは固定応答を含まず、Azure Cost Management から取得する |
| 通常構成での利用金額の表示 | `mise run server` でログイン済みの Home画面を開き、Foundry を切り替え、「Refresh cost」を押す | 選択中の Foundry のサブスクリプションの当月の実コストを `This month` に表示し、切り替えと「Refresh cost」のたびに取得中の表示を経て取得し直す。権限のないサブスクリプションは `COST_LOAD_FAILED` を表示する |
| Home画面での閲覧（状態ファイルあり） | 閲覧保存の基準フォルダーに保存済みの `foundry-state.json` がある状態で `mise run server` を起動し、ログイン済みで `http://127.0.0.1:34115/` を開く。Home画面を再読み込みし、プルダウンを開閉して選択表示にマウスを合わせる | 保存された Foundry 一覧と選択を復元し、Foundry 一覧は取得し直さない。選択中の Foundry のデプロイ一覧を Azure から取得して進捗モーダルを表示し、取得後に全モデルを表示する。展開時は全項目の全文、閉じた選択表示は幅に応じた省略と全文ツールチップを表示する。読み込みや JSON の復元に失敗した場合は `FOUNDRY_LOAD_FAILED` が表示される。処理は [閲覧設計](design/UCP-1.md#デプロイモデルを閲覧する) を参照する |
| 閲覧の再起動確認 | 上の通常ビルドを起動端末の `Ctrl+C` で終了し、同じデータフォルダーで `mise run server` を起動して Home画面を開く | 再起動前と同じ保存済みの Foundry 一覧・選択を復元し、デプロイ一覧を Azure から取得し直して表示する。保存ファイルには Foundry 一覧と選択だけがある |
| Foundry変更の画面確認用起動 | `node scripts/run.mjs server:review:foundry-change` | `http://127.0.0.1:34116/` がログイン済みで開く。通常と同じフロントエンドを E2E 用 Go サービスにつなぎ、状態ファイルがなければ固定の Foundry 3件を取得して先頭を選択・保存し、選択中の Foundry のデプロイ3件を取得して表示する。保存済みならその一覧と選択を使う。端末に確認用データフォルダーが表示される |
| Foundry変更の確認 | プルダウンから `contoso-foundry-development` を選択する。完了後に選択表示へマウスを合わせ、Home画面を再読み込みする | Foundry 一覧は再取得せず、「Deployments」の1行だけの進捗モーダルで変更先のデプロイ取得の実際の進捗を表示する。応答が速い場合は短い状態を目視できないことがある。成功後に変更先とデプロイ3件へ切り替わり、省略表示と全文ツールチップを表示する。保存するのは変更先の選択だけで、保存形式は [データ設計](design/data.md#foundry-とデプロイモデル) を参照する。再読み込み後も保存した選択を維持し、デプロイ一覧は取得し直して表示する。Production に戻した場合も取得し直す |
| Foundry変更の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure のデプロイ取得だけが確認用の固定応答から実処理に切り替わり、変更先の確認・選択更新・状態ファイルの読み込みと保存・進捗表示は同じ処理を通る。処理と型は [変更シナリオ設計](design/UCP-1.md#foundryを変更し閲覧する) を参照する |
| Foundry一覧の更新の画面確認用起動 | `node scripts/run.mjs server:review:foundry-refresh` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-refresh-review` を初期化し、保存済みの Foundry 3件（Production・Development・Legacy）と Production の選択、Foundry 一覧の最終取得日時 `2026-09-01 09:00` を用意する。起動時に Production のデプロイ3件を固定応答から取得して表示する。外部取得だけが固定応答（Production・Development・Research）になる |
| Foundry一覧の更新の確認 | プルダウン右の更新ボタンにマウスを合わせて押し、進捗中に Escape を押す。完了後にプルダウンを開き、再読み込みする。起動し直し、プルダウンで `contoso-foundry-legacy` を選んでから更新ボタンを押す | ツールチップ「Refresh Foundries」が出る。「Refreshing Foundries」のモーダルが「Foundries」「Deployments」の2行を最初から表示し、大きさを変えず、Escape で閉じない。応答が速いため短い状態は目視できないことがある。1回目は「Deployments」が待機中のままで、一覧が Production・Development・Research に変わり、選択とデプロイ一覧、デプロイの最終取得日時を維持し、Foundry一覧の最終取得日時だけが現在時刻になる。再読み込み後は保存した一覧と選択を使い、デプロイ一覧を取得し直す。Legacy を選んだ場合は「Deployments」も取得中になり、Production とデプロイ3件に切り替わり、両方の最終取得日時が現在時刻になる。保存形式は [データ設計](design/data.md#foundry-とデプロイモデル) を参照する |
| Foundry一覧の更新の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure の一覧・デプロイ取得だけが固定応答から実処理に切り替わり、更新・保存・進捗表示は同じ処理を通る。処理は [更新シナリオ設計](design/UCP-1.md#foundry一覧を更新する) を参照する |
| 通常構成での Foundry一覧の更新 | `mise run server` で Home画面を開き、更新ボタンを押す | 実 Azure から参照可能な全 Foundry を取得し、一覧と Foundry一覧の最終取得日時を更新して保存する。選択中の Foundry が一覧から消えた場合は最初の Foundry とそのデプロイに切り替わる（未検証。実装フェーズで利用者が確認） |
| デプロイモデルの更新の画面確認用起動 | `node scripts/run.mjs server:review:deployment-refresh` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-deployment-refresh-review` を初期化し、Foundry一覧の更新と同じ状態ファイル（Production を選択、Foundry 一覧の最終取得日時 `2026-09-01 09:00`）を用意し、起動時に Production のデプロイ3件を固定応答から取得して表示する。外部取得だけが固定応答になる |
| デプロイモデルの更新の確認 | 見出し「デプロイ済みモデル」右の更新ボタンにマウスを合わせて押し、進捗中に Escape を押す。完了後にモデル一覧と日時を確認し、再読み込みする | ツールチップ「Refresh models」が出る。「Refreshing models」のモーダルが「Deployments」の1行だけで選択中の Foundry 名と取得件数を表示し、Escape で閉じない。応答が速いため短い状態は目視できないことがある。完了後にデプロイ一覧が3件のまま、デプロイの最終取得日時だけが現在時刻になる。Foundry 一覧の最終取得日時は変わらない。再読み込み後はデプロイ一覧を取得し直して表示する |
| デプロイモデルの更新の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure のデプロイ取得だけが固定応答から実処理に切り替わり、進捗表示は同じ処理を通る。処理は [更新シナリオ設計](design/UCP-1.md#デプロイモデルを更新する) を参照する |
| 通常構成でのデプロイモデルの更新 | `mise run server` で Home画面を開き、見出し「デプロイ済みモデル」右の更新ボタンを押す | 実 Azure から選択中の Foundry の全デプロイを取得し、デプロイ一覧とデプロイの最終取得日時を更新する（未検証。実装フェーズで利用者が確認） |
| 閲覧の進捗の確認 | 閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で通常ビルドの Home画面を表示し、モーダルの「Foundries」「Deployments」を確認する。処理中に Escape を押し、モーダル外をクリックする | 2行を最初から表示し、モーダルの大きさは完了まで変わらない。実行中の行に回転表示が出て、一覧の取得後に Foundry の件数、デプロイ取得中に選択先の名称と取得件数を行の右側に表示する。経過時間とファイル保存は表示しない。保存成功後に自動で閉じる。処理中は Escape・外側クリックで閉じない。実際の応答時間に従って表示が進むため、短い処理の状態は目視できないことがある |
| Home画面の閲覧の再現 | 確認用アカウント・テナントの閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で `mise run server:review` を起動し、Foundry のプルダウンを開閉して選択表示にマウスを合わせる | 開いた一覧は Foundry 3件の全文を表示する。選択表示は幅に応じて省略し、ツールチップに全文を表示する。モデル一覧は Deployment・Model・Version・Capacity の4列で3件を表示する。Capacity は Details と同じ数値・単位を一行に表示する。表示した Foundry 一覧と初期選択が確認用アカウント・テナントの閲覧保存の基準フォルダーの `foundry-state.json` に保存される |
| ログアウトの再現 | 上の画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | メニューにアカウント名 `operator@contoso.onmicrosoft.com` と「ログアウト」だけが出る。選ぶとヘッダー右が消え、閉じられないログインモーダルが出る。データフォルダーの `e2e-authentication-record.json` と全アカウント・テナントの閲覧保存データが削除される |
| ログアウト後の再起動の再現 | 終了後、`$env:WAILS_DATA_DIR="$env:TEMP\AzFoundryDeck-review"; .\bin\azfoundrydeck-server-e2e.exe`（確認後 `Remove-Item Env:WAILS_DATA_DIR`） | 自動ログインされず、ログインモーダルが出る（`server:review` は起動のたびに記録を置き直すため、記録を置かずに起動する） |
| ログアウト失敗の再現 | `$env:AZFOUNDRYDECK_E2E_FAIL='logout'; mise run server:review`（確認後 `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL`） | 「ログアウト」を選ぶと、メニューが開いたまま中に `LOGOUT_FAILED` と理由が出て、ヘッダー右はログイン済みのまま、「ログアウト」を再度押せる |
| 日常の検証（生成・画面ビルドと型検査・型情報を使うLint・設定と運用スクリプトのLint・GoとTypeScriptの整形確認・単体テスト・Goのvetとerrcheckと検査除外・文書検査） | `mise run verify` | 編集中の確認に使う。終了コード 0。文書検査が `NG 0 件`、単体テストと静的解析が合格。E2E用ビルドとE2Eは実行しない |
| E2Eを含む検証 | `mise run verify:e2e` | 作業完了前や画面とGoの連携を変えたときに使う。日常の検証後に最新のE2E用サーバーをビルドし、すべてのserver E2Eが合格する |
| CIの検証と本番ビルド | `node scripts/run.mjs ci` | 日常の検証の合格後にブラウザーを導入し、E2E用サーバーをビルドする。その完了後、server E2Eと本番デスクトップビルドを並列実行する。CIではE2Eを省略しない |
| 依存関係の脆弱性検査 | `mise run audit` | npmの開発依存を含む既知脆弱性と、Goの本番・E2E構成から到達する既知脆弱性がないこと。外部DB通信を伴い、日常の検証には含めない。`Dependency checks` がPR・mainへのpush・毎週月曜の09:00（日本時間）に実行する |
| E2E のみ再実行 | ソースを変更していない場合に限り、`mise run verify:e2e` を一度実行した後、`npm --prefix frontend run test:e2e` | すべてのserver E2Eが合格。失敗時の記録は `frontend/playwright-report/` と `frontend/test-results/`。ソースを変更した場合は `mise run verify:e2e` でビルドからやり直す |

- **E2E の対象と構成**: シナリオ「ブラウザーでAzureにサインインする」の E2E は `frontend/tests/e2e/usecases/Azureへログインする/ブラウザーでAzureにサインインする.spec.ts` です。各テストが `bin\azfoundrydeck-server-e2e.exe` を一時データディレクトリと空きポートで起動します。主成功（手順1〜4と保存の確認）、認証境界の失敗、保存の失敗の3件を検証します。シナリオ「保存済みのログイン情報で自動的にログイン済みになる」の E2E は同じディレクトリの `保存済みのログイン情報で自動的にログイン済みになる.spec.ts` で、記録を置いて再起動し、復元の成功（取得中にモーダル・スピナーがなくヘッダー右が空であること、ブラウザーのサインインが呼ばれないことを含む）と復元の失敗（モーダルとエラー、記録が残ること、その後の手動ログイン）の2件を検証します。シナリオ「ヘッダーのユーザーアイコンからログアウトする」の E2E は `frontend/tests/e2e/usecases/Azureからログアウトする/ヘッダーのユーザーアイコンからログアウトする.spec.ts` で、記録を置いて再起動したログイン済みの状態から、メニューの内容、記録ファイルの削除、閉じられないログインモーダル、確認ダイアログがないこと、再起動後に自動ログインされないことを検証する主成功と、削除の失敗（メニュー内のエラー、ログイン済みのまま、再押下でも同じ、記録が残る）の2件を検証します。E2E 用ビルドには永続キャッシュがないため、永続キャッシュの削除は E2E の対象外です。
- **Home画面の閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/デプロイモデルを閲覧する.spec.ts` で、保存済みの固定ログイン情報から Home画面を開き、Foundry 一覧の取得・デプロイ取得の2行（待機・取得中・完了）、モーダルの大きさが完了まで変わらないこと、結果表示、状態ファイルに Foundry 一覧と選択だけが保存されデプロイ一覧が保存されないこと、保存済みの一覧と選択がある状態でのデプロイ取得と再起動後の取得し直し、Foundry の全文表示と省略・ツールチップを検証します。`AZFOUNDRYDECK_E2E_HOLD_FOUNDRY=1` のテストだけで外部取得を保留し、一時データディレクトリの `e2e-foundry-<段階>-release` ファイルで解放します。進捗の遷移は実際の進捗イベントで確認します。Foundry 一覧の取得は実 Azure の Resource Graph で確認します（[確認した事実](#design)）。`internal/foundry/service_test.go` は並び順と先頭の選択、一覧の取得完了後のデプロイ取得、一覧の取得失敗時の中止と既存保存内容の維持、保存済みの状態ファイルからの復元とデプロイ取得を検証します。すべて `mise run verify:e2e` で再実行できます。
- **モデル明細の検証**: `frontend/tests/e2e/usecases/デプロイモデルの詳細を確認する/` の3シナリオ（一覧からデプロイモデルの明細を表示する・明細の容量上限を表示する・容量上限の取得に失敗する）で、行全体とキーボードでの選択、上限以外の項目の即時表示、上限の取得中の `Loading...` と操作制限、同じ Foundry の別のデプロイでの上限の再利用、Foundry の変更後の取り直し、更新・Foundry・テナント変更後の明細破棄、失敗時の `Not set`・エラー・`Retry` と一覧の維持を検証します。`internal/foundry/azure_detail_test.go` はモデル別の単位換算、欠落したレート定義、容量ゼロ、PTUの刻みと許可値、編集モーダルへ渡す容量の最小値と刻みを、`internal/foundry/limits_test.go` は上限の取得が Foundry ごとに1回であることと Foundry の変更での破棄、編集モーダルで共有クォータだけを取り直すことを検証します。実 Azure の確認は通常構成で、実在するデプロイの明細と共有クォータに基づく上限を照合します。
- **単体テスト**: 永続キャッシュの削除（`internal/azauth/token_cache_windows_test.go`）は、実際の `%LOCALAPPDATA%\.IdentityService` に試験用の名前 `azfoundrydeck-test-<時刻>` とその `.cae` のファイルを作り、削除されることと、存在しないときの再削除が成功することを確認して後始末します。本アプリの `azfoundrydeck` には触れません。`go test ./internal/...`（`mise run verify` に含まれる）で実行されます。新しい実装による本番の永続キャッシュ・資格情報・全閲覧保存データの削除は実機未検証です。
- **Foundry変更後の閲覧の検証**: `frontend/tests/e2e/usecases/Foundryを変更する/Foundryを変更し閲覧する.spec.ts` で、デプロイ取得の進捗、処理中の旧表示と操作制限、取得後の切り替え、状態ファイルに変更先の選択だけが保存されること、再起動後に保存した選択を復元してデプロイ一覧を取得し直すこと、同じ Foundry を選んだ場合に取得・保存しないことを検証します。Foundry 一覧の取得は解放せず、デプロイ取得だけを解放して一覧を再取得しないことを確認します。Go の単体テストでは、同じ Foundry の選択での取得の抑止と、取得失敗時の変更前の選択の保持も確認します。
- **Foundry一覧の更新の検証**: `frontend/tests/e2e/usecases/Foundry一覧を更新する/Foundry一覧を更新する.spec.ts` で、保存済みの一覧（固定 Source にない Legacy を含む）から更新ボタンを押し、Foundry 一覧の取得の進捗（デプロイの行は待機中のまま）、処理中の旧表示と操作制限、Escape・外側クリックで閉じないこと、更新後の一覧・選択・デプロイ一覧・最終取得日時の表示と状態ファイルの内容、デプロイを取得しないこと、再起動後の復元を検証します。選択中の Foundry が消える場合は、`frontend/tests/e2e/usecases/Foundry一覧を更新する/Foundry一覧の更新で選択先が変わる.spec.ts` で、一覧の取得後に最初の Foundry のデプロイ取得の進捗（モーダルの大きさが変わらないこと）、状態ファイルの保存、両方の最終取得日時の更新と再起動後の復元を検証します。外部取得は `e2e-foundry-<段階>-release` ファイルで段階ごとに解放します。
- **デプロイモデルの更新の検証**: `frontend/tests/e2e/usecases/デプロイモデルを更新する/デプロイモデルを更新する.spec.ts` で、状態ファイルがある状態から更新ボタンを押し、デプロイ取得の1行だけの進捗、Foundry 一覧の取得を表示しないこと、処理中の旧表示と操作制限、Escape・外側クリックで閉じないこと、取得後のデプロイ一覧・件数・デプロイの最終取得日時の表示、Foundry 一覧の最終取得日時と状態ファイルの内容が変わらないこと、再起動後と別の Foundry へ変更して戻った後に取得し直した一覧を表示することを検証します。外部取得はデプロイ取得だけを `e2e-foundry-models-release` ファイルで解放します。
- **デプロイモデルの削除の検証**: `frontend/tests/e2e/usecases/デプロイモデルを削除する/一覧からデプロイモデルを削除する.spec.ts` で、一覧からモデルを選んで明細を表示し、ゴミ箱アイコンから確認ダイアログを開き、キャンセル時の状態維持、削除実行後の進捗と削除完了、一覧からの除外（件数と最終取得日時の更新）、表示中明細の破棄、状態ファイルに Foundry 一覧と選択だけが保存されること、再読み込み後の維持を検証します。また `AZFOUNDRYDECK_E2E_FAIL=delete` で削除失敗時にエラーコード `DEPLOYMENT_DELETE_FAILED` を表示し、一覧が維持され再試行可能であることを検証します。
- **デプロイモデルの追加の検証**: 「+ Add deployment」ボタン押下によるカタログ取得中表示、左右2ペインのモーダル表示、フィルター（検索・Publisher・Deployment option・Tasks）のリアルタイム反映、モデル選択時の Deployment name 自動同期、Capacity（直接入力とスライダー連動）または従量課金料金レートの表示、Deploy 押下時の進捗モーダル「Deploying model」、完了後の一覧への新規デプロイ追加反映（件数・最終取得日時の更新）、状態ファイルに Foundry 一覧と選択だけが保存されることを検証します。 `internal/foundry/azure_catalog_test.go` は SKU ごとの単位・最小値・刻みとレート定義の読み取りを、`frontend/tests/unit/AddDeploymentModal.test.tsx` は RPM の刻みでの調整と SKU の capacity への換算を検証します。
- **追加カタログとクォータ保持の検証**: `frontend/tests/e2e/usecases/デプロイモデルを追加する/新規デプロイモデルを追加する.spec.ts` は、初回取得の共有、モデル定義の先行表示、700ミリ秒の待機がないこと、クォータ取得中・失敗時の操作制限、作成成功後の非同期更新、入力保持と上限超過、Foundry・テナント切替時の初期化を検証します。固定外部境界を `AZFOUNDRYDECK_E2E_HOLD_CATALOG=1` で保留し、`e2e-catalog-definitions-release`、`e2e-catalog-quota-release`、`e2e-catalog-quota-refresh-release` で解放します。`internal/foundry/catalog_cache_test.go` は並列取得の共有、保持済み情報の再利用、操作ロックの解放、旧対象の結果排除と操作成功後の更新条件を、`internal/foundry/azure_catalog_test.go` はクォータ不明・ゼロと SKU 上限の境界を補強します。`node scripts/run.mjs verify:e2e` でまとめて再実行できます。
- **デプロイモデルの設定変更の検証**: `frontend/tests/e2e/usecases/デプロイモデルの設定を変更する/一覧からデプロイモデルの設定を変更する.spec.ts` で、一覧から Edit deployment を開き、設定取得中の表示、Deployment name・Model・SKU の読み取り専用、カタログに無い現在バージョンを選択肢に含めること、Upgrade policy の3表示名、Capacity の数値入力とスライダーの 1,000 単位の連動（固定応答の刻み）、未変更時の Update 無効、確認ダイアログがないこと、進捗モーダル「Updating deployment」が Escape と外側クリックで閉じないこと、完了まで一覧を変えないこと、完了後の一覧・件数・最終取得日時・同じ行の選択・新しい一覧からの明細表示、再読み込み・再起動・別の Foundry へ移して戻った後に取得し直した一覧、Cancel と × では変更を呼ばないこと、Capacity を右端にしてホバーしても設定モーダルに横スクロールが出ないことを検証します。外部取得は `e2e-foundry-detail-release`（上限の初回取得）、`e2e-foundry-update-settings-release`（編集モーダルの共有クォータの取り直し）、`e2e-foundry-update-release`、`e2e-foundry-models-release` で段階ごとに解放します。`AZFOUNDRYDECK_E2E_FAIL=update-settings` では変更を開始せずモーダル内に `DEPLOYMENT_UPDATE_FAILED` と Retry を表示し、`AZFOUNDRYDECK_E2E_FAIL=update` では進捗モーダルだけを閉じて入力値とエラーを設定モーダルに残し、Update を再度押せることを検証します。固定応答の3件はいずれも Standard であり、Pay-as-you-go と、初期 Capacity が上限を超える場合はこの E2E では検証しません。
- **Foundryの削除の検証**: `frontend/tests/e2e/usecases/Foundryを削除する/Foundry関連のみのリソースグループごと削除する.spec.ts` で、選択中の Foundry 見出し行のゴミ箱アイコンから確認ダイアログを開き、キャンセル時の状態維持、削除実行後の進捗と削除完了、一覧からの除外と次の Foundry 自動選択、状態ファイルへの保存、再読み込み後の維持を検証します。また `AZFOUNDRYDECK_E2E_FAIL=foundry-delete` で削除失敗時にエラーコード `FOUNDRY_DELETE_FAILED` を表示し、一覧と選択が維持されることを検証します。

- **E2E 用ビルド**: `node scripts/build.mjs server-e2e`（`build:server:e2e` タスク）が `-tags server,production,e2e` でビルドします。`e2e` タグでは、サインインの外部境界が固定の認証記録（アカウント名 `operator@contoso.onmicrosoft.com`、認証テナント ID `e2e-tenant`）を返し、一覧取得の外部境界が唯一の候補（ID `e2e-azure-tenant`、表示名 `Contoso`）を返します。唯一の候補の自動選択と認証記録・一覧・選択の保存は通常と同じサービス処理を通し、資格情報マネージャーの代わりにデータディレクトリの `e2e-authentication-record.json` へ保存します。環境変数 `AZFOUNDRYDECK_E2E_FAIL=signin`、`save`、`restore`、`logout` で失敗を注入します。ログアウトでは記録ファイルと全アカウント・テナントの閲覧保存データを削除します。起動前にデータディレクトリへ `e2e-authentication-record.json` を置くと、起動時の復元がその記録で成功します。サインインが呼ばれるとデータディレクトリに `e2e-signin-called` を作り、`AZFOUNDRYDECK_E2E_HOLD_RESTORE=1` のときは復元が `e2e-restore-release` の作成まで応答を保留します。
- **画面確認用の起動と実処理への切り替え**: `server:review` は E2E 用ビルド（`bin\azfoundrydeck-server-e2e.exe`）を、一時フォルダーの固定データディレクトリ `AzFoundryDeck-review` に認証記録・テナント一覧・選択を含む `e2e-authentication-record.json` を置いて起動します。認証と Foundry・デプロイ取得の外部境界は固定応答を返しますが、初期選択・進捗の合成と通知・Foundry 一覧と選択のファイル保存・保存済みファイルの読み込み・結果表示は本番と同じ処理です。保存済みファイルがあれば Foundry 一覧を取得せずに復元し、選択中の Foundry のデプロイだけを固定応答から取得します。固定応答の内容と進捗の接続は [UCP-1](design/UCP-1.md#デプロイモデルを閲覧する) を参照します。Azure・資格情報マネージャー・永続キャッシュには触れません。終了は起動した端末で `Ctrl+C` です。実 Azure の取得へ切り替える場合は終了後に `mise run server` で通常ビルドを起動します。読み込み・接続・保存に失敗した場合は固定応答に切り替わらず、Home画面にエラーが表示されることを確認します。本番のログアウトで削除する対象は [データ設計](design/data.md#ログアウト時の削除範囲) に従います。閲覧保存データの削除も同じ実処理を通しますが、本番の永続キャッシュと資格情報マネージャーを含む実機動作は未検証です。
- **本番に含まれないこと**: `internal/azauth/e2e.go`、`record_store_e2e.go`、`internal/foundry/e2e.go`、`foundry_source_e2e.go`、`update_source_e2e.go` は `e2e` タグのときだけコンパイルされ、`server`、`build`、`package`、`dev` のビルドには含まれません。`go list -tags server,production -f '{{.GoFiles}}' ./internal/azauth ./internal/foundry .` にこれらのファイルが現れないことで確認できます。実 Azure へのサインイン・Foundry 一覧とモデルの取得、実ブラウザーでの認証、実資格情報マネージャーへの保存は E2E の対象外です。

- **保存するもの**: アカウント識別情報・テナント一覧・選択を Windows 資格情報マネージャーの汎用資格情報 `AzFoundryDeck:AuthenticationRecord`（ユーザー名 `AuthenticationRecord`）に、トークンを `azidentity/cache` の永続キャッシュ（名前 `azfoundrydeck`）に保存します。通常起動時とテナント変更時は保存した一覧を使い、ブラウザーでの再サインイン時だけ ARM の一覧を取得し直します。保存形式・アカウントとテナントごとの閲覧保存先・ログアウトの削除範囲は [データ設計](design/data.md) を参照します。
- **自動ログインの実機確認**: ブラウザーでのサインイン成功後にアプリを終了し、同じデータフォルダーで通常ビルドを再起動します。ブラウザーを開かずに保存済みの一覧・選択からヘッダーを復元し、ARM のテナント一覧を取得し直さないこと、同じアカウント・テナントの閲覧結果を復元することを確認します。
- **失敗時の実機確認**: 接続できないプロキシを設定した通常ビルドで「Azureにログイン」を押し、ログインモーダルにエラーのコードと理由が表示され、再試行できること、閲覧を開始しないことを確認します。新しい実装での動作は未検証です。診断ログの場所は `%APPDATA%\\AzFoundryDeck\\logs\\app.jsonl` です。

環境構築、作業ディレクトリ、実行コマンド、設定、期待結果を明記します。自動テストと実機確認の対象・条件を示し、最新コードで再実行できる手順を維持します。モック利用時は、起動・終了、モック有効/無効の確認、実処理への切り替え手順を記述します（接続失敗時にモックへフォールバックしないことの確認を含む）。

### インストーラーの配布

リポジトリルートで、リリース対象の変更をコミットしたクリーンな作業ツリーから実行します。Gitのユーザー情報とoriginへのpush権限が必要です。

| 操作 | コマンド | 期待結果 |
| --- | --- | --- |
| バージョンを指定して公開 | `mise run release:tag -- 0.2.0` | 指定版をコミットし、タグとブランチをoriginへpushする |
| 末尾を1増やして公開 | `mise run release:tag` | `build/app.json` のパッチ番号を1増やして同じ処理を行う |
| インストーラーのローカル生成 | `mise run package` | `bin/azfoundrydeck-X.Y.Z-amd64-setup.exe` を生成する |
| インストーラーのE2E検証 | `mise run test:installer` | 完了画面の4組み合わせ・起動・リンク・アンインストールを検証し、確認用の登録とファイルを削除する |
| 文書の検査 | `python scripts/doc_check.py .` | `NG 0 件` |
| 更新用の署名鍵の作成（初回のみ） | `go run ./cmd/release keygen -out <リポジトリ外のパス>` | 秘密鍵をファイルに書き、公開鍵を表示する。公開鍵を `build/app.json` の `updatePublicKey` に、秘密鍵ファイルの内容を GitHub の Secrets の `UPDATE_SIGNING_KEY` に登録する（`gh secret set UPDATE_SIGNING_KEY < <パス>`）。秘密鍵はリポジトリに置かない。現在の鍵は開発者の `%USERPROFILE%\.azfoundrydeck\update-signing.key` にある |
| 新版の更新の実機確認 | `go run ./cmd/release keygen` で確認用の鍵を作る。環境変数 `BUILD_UPDATE_SOURCE` に手元のフォルダー、`BUILD_UPDATE_PUBLIC_KEY` に確認用の公開鍵、`BUILD_APP_VERSION` に古い版（例 `0.1.90`）と新しい版（例 `0.1.91`）を順に指定して `node scripts/run.mjs package` を実行する。新しい版のインストーラーをそのフォルダーに置き、`go run ./cmd/release manifest -key <鍵> -installer <インストーラー> -app-id AzFoundryDeck -version 0.1.91 -out <フォルダー>` で `update.json` を作る。古い版を `/S` でインストールして起動する | Home画面の先頭に `バージョン 0.1.91 の準備ができました。` が表示され、`更新して再起動` でアプリが終了し、0.1.91 が画面なしでインストールされて起動する。ログイン状態と閲覧の保存は残り、更新の区画は表示しない。確認後は GitHub Releases の正式なインストーラーで入れ直す |
| 新版の更新の E2E | `mise run test:desktop` | 「起動時に取得した新版で更新して再起動する」の E2E が合格する。v0.1.0 と v0.2.0 のインストーラーを確認用の鍵と手元の更新元で作り、v0.1.0 をインストールして更新し、署名の不一致と適用前の再検証の失敗も確かめてから、最後にアンインストールする。実際にインストールし、資格情報マネージャーのログインを使うため、Azure Foundry Deck を終了・アンインストールし、ログイン済みの状態で手元で実行する。NSIS がない場合は `NSIS_EXE` に `makensis.exe` を指定する。`verify:e2e` と CI には含めない（`verify:e2e` ではスキップと表示される） |
| Foundry追加の画面確認用起動 | `mise run server:review:foundry-add` | `http://127.0.0.1:34127/` が固定アカウントでログイン済み、Foundry 0件で起動する。起動ごとに空の一時フォルダーを作り、パスを表示する。Add Foundry で候補とデフォルト名を表示し、Create でリソースグループ作成・Foundry作成・Home更新の進捗を表示して新しい Foundry を選択する。候補取得と作成・デプロイ取得だけが固定応答で、サービスの進捗通知・選択・保存・表示は通常の処理を通す。実 Azure と本番保存先には触れない |
| Foundry追加の画面確認用構成の終了 | 起動端末で `Ctrl+C` | 確認用サーバーを停止する。通常起動は `mise run server` または `mise run dev`。通常構成は実 Azure から候補を取得してリソースを作成し、固定応答へフォールバックしない |
| Foundry追加の実処理確認 | 通常構成のHomeで Add Foundry を開き、作成先のサブスクリプション、eastus2、検証用キーワードを指定して Create を押す | 作成中の3段階の進捗表示後、新規 Foundry が選択される。Azure に新規リソースグループと AIServices アカウントが存在し、通常構成の画面再読み込みでも選択が復元され、デプロイ0件を取得する。作成済みリソースは自動削除しない |
| Foundry削除の画面確認用起動 | `node scripts/run.mjs server:review:foundry-delete` | `http://127.0.0.1:34128/` が固定アカウントでログイン済み、Foundry 3件で起動する。Delete Foundry で確認ダイアログを表示し、Delete で Foundry削除（Purge含む）・リソースグループ削除の進捗を表示して次の Foundry を選択する。外部削除処理だけが固定応答で、進捗通知・選択・保存・表示は通常の処理を通す。実 Azure と本番保存先には触れない |
| Foundry削除の画面確認用構成の終了 | 起動端末で `Ctrl+C` | 確認用サーバーを停止する。通常起動は `mise run server` または `mise run dev`。通常構成は実 Azure のリソースを削除し、固定応答へフォールバックしない |
| Foundry削除の実処理確認（実装フェーズ） | 通常構成（`mise run server` または `mise run dev`）のHomeで削除対象の Foundry を選び、ゴミ箱アイコンから Delete を押す | 進捗モーダル（Delete Foundry、Delete resource group。リソースグループ保持時は Delete Foundry のみ）が表示され、Azure 上の AIServices アカウント（および対象時はリソースグループ）が実際に削除・完全消去される。Home画面のFoundry一覧から削除対象が消え、次のFoundryが選択される |
| CIの静的検査 | `node scripts/run.mjs check:workflow`（日常の検証に含む） | 固定版actionlintで全workflowを検査し、エラー出力なしで終了する。ShellCheck・Pyflakesの外部ツール検査は省略する |

公開用タグのpushはWindows CIを起動します。`release` ジョブはインストーラーの版・サイズ・ハッシュを Secrets の `UPDATE_SIGNING_KEY` で署名した `update.json` を作り、インストーラーと同じ Release に添付します。Secrets が未登録なら公開せずに失敗します。各PCのアプリは次の起動時に `https://github.com/nuitsjp/azfoundry-deck/releases/latest/download/update.json` から新版を取得します。GitHub Actionsの `Windows checks` でビルド・検証と `release` ジョブの成功を確認し、ジョブが表示するURLからインストーラーを取得します。Git操作が失敗した場合は出力とローカルのバージョン・コミット・タグを確認し、自動再試行は行いません。

タグ付与の手動検証では、一時フォルダーに `scripts/release-tag.mjs`、`build/app.json` と `release:tag` タスクを配置し、Gitリポジトリを初期化してローカルのbareリポジトリをoriginに設定します。バージョン指定と未指定を別々のリポジトリで実行し、バージョン、変更コミットの対象ファイル、originのブランチとタグが一致することを確認します。不正値、同じ版・古い版、範囲超過、作業ツリーの変更、ローカルとoriginのタグ重複では、ファイル・コミット・タグが変わらず失敗することを確認します。この確認はGitHub CIとRelease公開の検証を代替しません。

### インストーラーの画面確認

リポジトリルートで `pwsh -NoProfile -File scripts/preview-installer.ps1` を実行します。NSISとWindowsのC#コンパイラーが必要です。一時フォルダーに本番と同じNSISスクリプトを配置し、製品定義と配置する実行ファイルだけを確認用に差し替えます。起動するインストーラーの名前は `AzFoundryDeck (画面確認)` で、インストール先は一時フォルダーです。

インストールを進め、完了画面の初期状態と選択後の結果を [対象シナリオ](usecases/アプリをインストールする/scenarios/インストールを完了して起動方法を選ぶ.md) に照らして確認します。起動を選ぶと、確認用アプリに起動済みの表示が出ます。確認用アプリを閉じ、確認用インストール先の `uninstall.exe` を実行して終了します。画面確認では本番アプリの処理を実行せず、本番の登録やショートカットを変更しません。

実処理へ切り替える場合は `mise run package` で生成した製品用インストーラーを使用します。確認用の製品定義と実行ファイルは製品用ビルドに含めません。

インストーラーのE2E検証はWindowsとPowerShell 7、NSIS、WindowsのC#コンパイラーを使用します。同じNSISスクリプトを一時フォルダーでビルドし、起動先だけを起動記録を書いて終了する確認用実行ファイルに差し替えます。非表示のネイティブウィンドウへWin32メッセージを送り、画面の選択とインストーラーの実処理を検証します。マウス・キーボードのフォーカスや画面キャプチャは使用しません。Windows CIでは製品側のインストーラー生成後に同じE2Eを実行し、成功後にartifactを保存します。
