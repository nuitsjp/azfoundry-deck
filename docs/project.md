# AzFoundryDeck のプロジェクト定義

プロジェクト共通の要件・制約、ユースケース一覧、確認した事実、および実行・検証手順の正本です。全体構造は [アーキテクチャ](architecture.md) を参照します。

## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 解決する問題・達成したい結果 | Azure 上の Microsoft Foundry とデプロイ済みモデルを、Azure SDK for Go を使うデスクトップアプリから管理できるようにする。 |
| 利用者・利用場面 | Azure アカウントを持ち、Foundry を運用する個人。Windows デスクトップで利用する。 |
| 今回の対象 | Azure へのログインとログアウト（ログイン情報の保存と破棄を含む）、利用対象のテナントの選択と変更、Home画面でのデプロイモデルの閲覧（Foundry 一覧・選択済み Foundry・選択された Foundry の全デプロイ済みモデルのファイル保存と、保存済みファイルからの復元、Foundry を変更した後の初回閲覧と再閲覧、Foundry 一覧とデプロイモデルの Azure からの更新を含む）、選択したデプロイモデルの最新の明細の確認、選択中の Foundry のデプロイモデルの削除。 |
| 今回の対象外 | サブスクリプション・Foundry の操作、モデルデプロイの作成・変更、および上記以外の参照（別ユースケースとして順次追加する）。 |

## 2. 制約・品質要求・受け入れ条件

- Azure への操作は Go 言語用の Azure SDK（`azidentity`、`armXXX` など）で行う。
- Windows デスクトップ版を対象とする。
- アカウント識別情報は OS のクレデンシャルマネージャー、トークンは Azure SDK の永続キャッシュ（Windows のユーザー単位の暗号化）に保存し、平文でファイル・画面・ログへ出さない。
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
| [デプロイモデルの詳細を確認する](usecases/デプロイモデルの詳細を確認する/README.md) | Azure にログイン済みの Foundry 運用者 | 一覧から選んだデプロイモデルの最新の設定と状態を確認する | 8 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを削除する](usecases/デプロイモデルを削除する/README.md) | Azure にログイン済みの Foundry 運用者 | 選択中の Foundry のデプロイモデルを Azure 上で削除し、Home画面の一覧を最新にする | 9 | [UCP-1](design/UCP-1.md) | 対象 |

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

- **確認した事実**: 外部仕様や既存コードの調査結果（情報源、対象版、確認日、確認範囲）。仮定と明確に区別します。外部システムの実測応答を保存する場合は `reference/` に配置して参照します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

作業ディレクトリはリポジトリのルートです。コマンドは PowerShell で実行します。閲覧保存の基準フォルダーは [データ設計](design/data.md#閲覧データの保存範囲) に従い、アプリのデータフォルダー内でアカウントと選択テナントの組み合わせごとに分離します。以下の `foundry-state.json` と `foundry-models/` はこの基準フォルダー内のファイルを指します。

| 目的 | コマンド・設定 | 成功確認 |
| --- | --- | --- |
| 環境構築 | `mise trust`、`mise run setup`、`mise run setup:browser` | `frontend/bindings/azfoundrydeck/internal/azauth/` と `frontend/bindings/azfoundrydeck/internal/foundry/` が生成される |
| self-hosted runner の初期登録・追加 | 管理者 PowerShell 7 で `mise run setup:runner`。2台目以降は `mise run setup:runner -- 2` のように正整数の番号を指定する。GitHub の Settings → Actions → Runners → New self-hosted runner から登録トークンを取得し、プロンプトで入力する。サービス実行アカウントもプロンプトで指定する | GitHub の Runners に runner が Online と表示される。番号省略・1は既存の配置先と名前を使い、2以降はフォルダー名・登録名に `-2` などを付けて別サービスにする。[設定スクリプト](../scripts/setup-runner.ps1) は既存ファイルがある配置先を上書きしない |
| runner 本体の自動更新 | runner の標準自動更新を有効にしたままサービスを常駐させる。別の定期タスクは不要 | [GitHub の仕様](https://docs.github.com/en/actions/reference/runners/self-hosted-runners#communication)では、ジョブ割り当て時、または新バージョン公開後1週間以内に更新される |
| 起動（ブラウザー確認） | `mise run server` | `http://127.0.0.1:34115/` を開くと、Home を背景にログインのモーダルが表示される |
| 起動（デスクトップ） | `mise run dev` | ウィンドウにログインのモーダルが表示される（未検証） |
| モデル明細の画面確認用起動 | `mise run server:review:deployment-detail` | `http://127.0.0.1:34123/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。モデル一覧は3件、明細は未選択で空。`chat-production` の行のどこを押しても明細にモデル `gpt-4.1`、SKU `GlobalStandard`、Capacity `50,000 / 160,000 TPM`、Upgrade policy `Upgrade to new default` を表示する。`chat-mini` は `100,000 / 250,000 TPM` と `Upgrade on retirement`、`embeddings` は SKU `Standard`、`20,000 / 80,000 TPM` と `No automatic upgrade` となる。同じモデルを再選択しても取得日時を更新する。認証と外部取得のみ固定応答で、明細の取得呼び出しと表示は通常と同じ処理を通す。実 Azure と本番保存先には触れない |
| 明細の画面確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` または `mise run dev` | 通常ビルドは固定応答を含まない。明細を選ぶと実 Azure からデプロイ・モデル定義・共有クォータを毎回取得する。Capacity は設定済み容量／割り当て可能上限と単位を表示し、失敗時は `DEPLOYMENT_DETAIL_FAILED` と `Retry` を表示する。画面確認用データを本番へ持ち込まない |
| デプロイモデル削除の画面確認用起動 | `mise run server:review:deployment-delete` | `http://127.0.0.1:34124/` が固定アカウントでログイン済みとなる。起動ごとに空の一時フォルダーを作り、パスを端末に表示する。モデル一覧は `chat-production`・`chat-mini`・`embeddings` の3件。外部の取得と削除だけが固定応答で、削除したデプロイは以降の取得から除かれる。保存・削除後の再取得・表示は通常と同じ処理を通す。実 Azure には触れない |
| デプロイモデル削除の確認 | 起動後、`chat-mini` 行を選んで右側の明細を表示し、明細下部右端のゴミ箱アイコン（Delete）を押してダイアログで「Cancel」を押す。もう一度ゴミ箱アイコンを押し、ダイアログの「Delete」を押す。デプロイ名右端の鉛筆アイコン（Edit）は仮のボタンで、押しても何も起きない | ダイアログは Foundry 名・デプロイ名・「This cannot be undone.」を表示し、Cancel では一覧が3件のまま。削除後は進捗モーダル「Deleting deployment」（応答が速いため短い状態は目視できないことがある）が閉じ、一覧が `chat-production`・`embeddings` の2件になり、件数と最終取得日時が更新される。再読み込み後も2件のまま |
| デプロイモデル削除の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='delete'` を設定し、`mise run server:review:deployment-delete` を起動する。同じ手順で削除する | 一覧は3件のまま、ページ本文の先頭に `DEPLOYMENT_DELETE_FAILED` と理由を赤いバナーで表示し、「×」で閉じられる。「Delete」を再度押せる。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| デプロイモデル削除の確認用構成の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | 通常ビルドは固定応答を含まない。実 Azure への削除は段階4で接続するまで `DEPLOYMENT_DELETE_FAILED` になる（未実装） |
| 1件テナントのサインイン画面確認用起動 | `mise run server:review:login` | `http://127.0.0.1:34117/` が未ログインで開く。起動のたびに空の一時フォルダー `AzFoundryDeck-login-review-...` を作り、端末にパスを表示する。「Azureにログイン」を押すと、ブラウザーを開かずに固定応答で認証し、唯一の候補 ID `e2e-azure-tenant`、表示名 `Contoso` が選択される。認証記録のテナント ID `e2e-tenant` は候補 ID と異なる。ヘッダーのプルダウンとユーザーアイコンへのマウスオーバーで選択名・アカウント名 `operator@contoso.onmicrosoft.com` を確認する。実 Azure・資格情報マネージャー・永続キャッシュには触れない。一覧・選択のファイル保存とアカウント・テナント別の閲覧保存先決定は通常と同じ処理を通す。実 Azure と本番の保存先はこの確認用構成の検証対象に含めない。構成は [サインイン設計](design/UCP-1.md#ブラウザーでazureにサインインする) を参照する |
| サインイン画面確認用構成の終了と実処理への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` または `mise run dev` | 確認用の一時データを通常データへ持ち込まず、実 Azure の認証経路に切り替わる。通常構成には実認証・テナント一覧取得と一覧・選択の永続保存を接続している。実 Azure の認証と本番保存先の確認には通常構成を使う |
| 複数テナントの選択画面確認用起動 | `mise run server:review:login-multiple` | `http://127.0.0.1:34118/` が未ログインで開く。起動のたびに空の一時ディレクトリを作り、端末にパスを表示する。「Azureにログイン」を押すと、ブラウザーを開かずに固定応答で認証し、「テナントを選択」画面を表示する。固定候補は `Contoso`、`Contoso Development`、`Fabrikam`、`Northwind`、`Adventure Works`、`Woodgrove`、`Tailspin` の7件。初期状態の「テナント」は未選択で「テナントを選んでください」を表示し、「確定」は無効。候補を選んで確定すると、Home と選択したテナントのヘッダープルダウン、ユーザーアイコンを表示する。構成は [複数テナントのサインイン設計](design/UCP-1.md#サインイン時に複数テナントが存在する) を参照する。ログイン後のテナント変更は対象外 |
| 複数テナントの選択保存失敗の確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='select-save'` を設定し、`mise run server:review:login-multiple` を起動する | 候補を選んで「確定」を押すと、選択保存だけが失敗する。テナント選択画面にエラーを表示し、選択値を保持して再試行できる。Home の閲覧を開始しない。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| 複数テナントの画面確認用構成の終了と実処理への切り替え | 起動端末で `Ctrl+C`。失敗注入を使った場合は解除し、通常構成を `mise run server` または `mise run dev` で起動する | 固定候補の取得に使う `AZFOUNDRYDECK_E2E_TENANTS=multiple` は確認用起動コマンドが子プロセスだけに設定する。終了後の通常ビルドは実 Azure の認証・一覧取得を使い、確認用の一時データを持ち込まない。通常構成で実 Azure の認証、選択先のトークン取得、資格情報と一覧・選択の保存を確認する |
| テナント変更の画面確認用起動 | `mise run server:review:tenant-change` | `http://127.0.0.1:34119/` がログイン済み（アカウント `operator@contoso.onmicrosoft.com`、テナント `Contoso`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-tenant-change-review` を作り直し、テナント3件（`Contoso`、`Fabrikam`、`Northwind`）の認証記録を置く。保存済みの閲覧結果はないため、最初の Home は固定応答の初回取得を行う。トークン取得は固定応答で成功し、一覧・選択の保存と閲覧の保存・読み込みは通常と同じ処理を通す |
| テナント変更の確認（初回閲覧） | 起動後、ヘッダーのテナントプルダウンを開いて `Fabrikam` を選ぶ。続けてプルダウンを開き、`Fabrikam` を選び直す | 選択後にプルダウンが閉じ、処理中は Foundry の変更と更新ボタンが無効になる。変更前の Foundry・モデル表示が消えて、進捗モーダルで固定応答の初回取得を行い、完了後に `Fabrikam` の Home を表示する。ヘッダーのテナント表示は `Fabrikam` になる。同じテナントの選び直しでは何も起きない。再起動後の選択の復元は、この確認用起動が起動のたびにデータを作り直すため、この手順の対象外とする。仕様は [テナントを変更し初回閲覧する](usecases/テナントを変更する/scenarios/テナントを変更し初回閲覧する.md) を参照する |
| テナント変更の失敗確認 | 起動前に `$env:AZFOUNDRYDECK_E2E_FAIL='select-save'` を設定し、`mise run server:review:tenant-change` を起動する。`Fabrikam` を選ぶ | 選択保存だけが失敗し、ページ本文の先頭（「Home」見出しの下）に、赤いバナーで `SELECT_TENANT_FAILED` と理由を表示し、右上の「×」だけで閉じられる。テナント表示と Home は `Contoso` のまま変わらない。終了後に `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL` で失敗注入を解除する |
| テナント変更後の再閲覧の画面確認用起動 | `mise run server:review:tenant-revisit` | `http://127.0.0.1:34122/` がログイン済み（テナント `Contoso`、候補 `Contoso`・`Fabrikam`・`Northwind`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-tenant-revisit-review` を作り直し、`Fabrikam` の閲覧保存先に、Foundry 2件（2件目の `fabrikam-foundry-research` を選択）と固定応答にないモデル2件の保存済みファイルを用意する。トークン取得は固定応答で成功し、読み込み・表示は通常と同じ処理を通す |
| テナント変更後の再閲覧の確認 | 起動後、ヘッダーのテナントプルダウンを開いて `Fabrikam` を選ぶ | 進捗モーダルを表示せず、保存済みの Foundry 一覧・選択（`fabrikam-foundry-research`）・モデル2件（`fabrikam-research-chat`、`fabrikam-research-embedding`）をそのまま表示し、最終取得日時は保存時の `2026-09-01 09:00` になる。`Fabrikam` の保存済みファイルの内容と更新時刻は変わらない。仕様は [テナントを変更し再閲覧する](usecases/テナントを変更する/scenarios/テナントを変更し再閲覧する.md) を参照する |
| Foundryが存在しない状態の画面確認用起動 | `mise run server:review:no-foundry` | `http://127.0.0.1:34120/` がログイン済み（テナント `Contoso`）で開く。起動のたびに `%TEMP%\AzFoundryDeck-no-foundry-review` を作り直し、保存済みの閲覧結果はない。外部取得は固定応答で、Foundry を返さない（`AZFOUNDRYDECK_E2E_FOUNDRIES=none` を起動コマンドが子プロセスにだけ設定する）。保存・読み込み・表示は通常と同じ処理を通す |
| Foundryが存在しない状態の確認 | 起動後、Home画面を開く。続けて Home画面を再読み込みする | 取得の進捗モーダルのあと、Foundry のプルダウン（選択肢なし）と空のモデル一覧（0 件）を表示し、`FOUNDRY_LOAD_FAILED` は表示しない。「Refresh Foundries」ボタンと Foundry 一覧の最終取得日時を表示し、「Refresh models」ボタンは無効になる。`foundry-state.json` に空の一覧・選択なし・取得日時が保存され、再読み込み後は進捗モーダルなしで同じ表示になる。「Refresh Foundries」を押して0件のままの場合の扱いは、別シナリオで定める（現状は `FOUNDRY_LOAD_FAILED`）。仕様は [Foundryが存在しない状態で初回閲覧する](usecases/デプロイモデルを閲覧する/scenarios/Foundryが存在しない状態で初回閲覧する.md) を参照する |
| Foundry一覧の更新で0件になる画面確認用起動 | `mise run server:review:foundry-empty` | `http://127.0.0.1:34121/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-empty-review` を作り直し、保存済みの Foundry 3件・選択（Production）・一部のモデルファイルを用意する。外部取得は固定応答で、更新の取得結果は Foundry 0件（`AZFOUNDRYDECK_E2E_FOUNDRIES=none` を起動コマンドが子プロセスにだけ設定する）。保存・削除・読み込み・表示は通常と同じ処理を通す |
| Foundry一覧の更新で0件になる確認 | 起動後、Foundry のプルダウンの右にある「Refresh Foundries」ボタンを押す。続けて Home画面を再読み込みする | 進捗モーダル（「Deployments」は待機中のまま）のあと、Foundry のプルダウンとモデル一覧が空（0 件）になり、`FOUNDRY_LOAD_FAILED` は出ない。「Refresh models」ボタンは無効、「Refresh Foundries」ボタンは有効。`foundry-state.json` に空の一覧・選択なし・取得日時が保存され、`foundry-models/` のファイルはすべて削除される。再読み込み後は進捗モーダルなしで同じ表示になる。仕様は [Foundryが存在しない状態へ一覧を更新する](usecases/Foundry一覧を更新する/scenarios/Foundryが存在しない状態へ一覧を更新する.md) を参照する |
| ログイン | モーダルの「Azureにログイン」を押し、開いたブラウザーでサインインする | 一覧が1件なら自動選択し、複数件なら候補を選んで「確定」を押す。モーダルが閉じてヘッダーにテナントプルダウンとユーザーアイコンが表示され、`cmdkey /list:AzFoundryDeck:AuthenticationRecord` に資格情報が表示され、`%LOCALAPPDATA%\.IdentityService\azfoundrydeck.cae` が作成される |
| 自動ログイン（起動時の復元） | 保存済みの認証記録・テナント一覧・選択がある状態で `mise run server` を起動し、`http://127.0.0.1:34115/` を開く | 保存済みの一覧と選択を読み込み、ARM の一覧を取得し直さず、ヘッダーにテナントプルダウンとユーザーアイコンを表示する。失敗時はモーダル内に `LOGIN_FAILED` と理由を表示し、保存情報を自動削除しない。仕様は [自動復元シナリオ](usecases/Azureへログインする/scenarios/保存済みのログイン情報で自動的にログイン済みになる.md) を参照する。再起動後も同じ一覧・選択と、そのアカウント・テナントの保存済み閲覧結果を表示する |
| ログアウト | ログイン済みの画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | 認証・テナント・全アカウントの閲覧保存情報とメモリ上の状態をすべて削除し、閲覧結果とヘッダー右を消してログインモーダルを表示する。削除範囲は [データ設計](design/data.md#ログアウト時の削除範囲) を参照する。この変更の実機動作は未検証 |
| 保存したログイン情報の手動削除 | `cmdkey /delete:AzFoundryDeck:AuthenticationRecord`、`Remove-Item "$env:LOCALAPPDATA\.IdentityService\azfoundrydeck*"` | `cmdkey /list:AzFoundryDeck:AuthenticationRecord` が「なし」を表示する |
| 終了 | 起動した端末で `Ctrl+C` | `http://127.0.0.1:34115/health` に応答しない |
| Home画面での初回閲覧 | 閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で `mise run server` を起動し、ログイン済みで Home画面を表示する | 進捗モーダルを経て、参照可能な全 Foundry（サブスクリプション名、Foundry 名の昇順）と先頭の Foundry の全デプロイ済みモデルが表示され、閲覧保存の基準フォルダーの `foundry-state.json` に同じ内容が保存される。取得または保存に失敗した場合は Home画面に `FOUNDRY_LOAD_FAILED` が表示される |
| 画面確認用の起動（Home画面・ログアウトの UI 確認） | `mise run server:review` | `http://127.0.0.1:34115/` がログイン済み（ヘッダー右に `Contoso` とユーザーアイコン）で開く。保存済みファイルがなければ固定の Foundry 3件とモデル3件を取得して保存し、あればその保存内容を表示する。外部取得の固定応答は待ち時間なしで返るため、初回取得の進捗モーダルは短時間で閉じる。端末に `review data directory: <一時フォルダー>\AzFoundryDeck-review` が出る |
| Home画面での再閲覧 | 閲覧保存の基準フォルダーに保存済みの `foundry-state.json` がある状態で `mise run server` を起動し、ログイン済みで `http://127.0.0.1:34115/` を開く。Home画面を再読み込みし、プルダウンを開閉して選択表示にマウスを合わせる | 保存された Foundry 一覧・選択・全モデルをそのまま表示する。展開時は全項目の全文、閉じた選択表示は幅に応じた省略と全文ツールチップを表示する。Azure からの一覧・モデル取得、進捗モーダルの表示、再保存を行わない。読み込みや JSON の復元に失敗した場合は `FOUNDRY_LOAD_FAILED` が表示される。処理は [再閲覧設計](design/UCP-1.md#デプロイモデルの再閲覧) を参照する |
| 再閲覧の再起動確認 | 上の通常ビルドを起動端末の `Ctrl+C` で終了し、同じデータフォルダーで `mise run server` を起動して Home画面を開く | 再起動前と同じ保存済みの一覧・選択・全モデルを表示し、取得の進捗モーダルを表示しない。保存ファイルの内容と更新時刻は変わらない |
| Foundry変更の画面確認用起動 | `node scripts/run.mjs server:review:foundry-change` | `http://127.0.0.1:34116/` がログイン済みで開く。通常と同じフロントエンドを E2E 用 Go サービスにつなぎ、保存済み状態がなければ固定の Foundry 3件と初期選択先のモデル3件を取得して保存する。保存済みならその内容を表示する。端末に確認用データフォルダーが表示される |
| Foundry変更の確認 | 変更先のモデルファイルが存在しない状態で、プルダウンから `contoso-foundry-development` を選択する。完了後に選択表示へマウスを合わせ、Home画面を再読み込みする | Foundry 一覧は再取得せず、「Deployments」の1行だけの進捗モーダルで変更先のモデル取得の実際の進捗を表示する。応答が速い場合は短い状態を目視できないことがある。成功後に変更先とモデル3件へ切り替わり、省略表示と全文ツールチップを表示する。保存形式は [データ設計](design/data.md#foundry-とデプロイモデル) を参照する。再読み込み後も保存した選択とモデルを表示する |
| Foundry変更の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure のモデル取得だけが確認用の固定応答から実処理に切り替わり、変更先の確認・選択更新・ファイル読み込みと保存・進捗表示は同じ処理を通る。処理と型は [変更シナリオ設計](design/UCP-1.md#foundryを変更し初回閲覧する) を参照する |
| Foundry変更後の再閲覧の画面確認用起動 | `node scripts/run.mjs server:review:foundry-revisit` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-revisit-review` に一覧・選択を含む認証記録と、閲覧保存の基準フォルダー内に Foundry 2件と各モデルファイル、Production を選択した状態ファイルを用意する。初期表示は `saved-production-chat` の1件。`AZFOUNDRYDECK_E2E_HOLD_FOUNDRY=1` で外部取得を保留したまま、本番と同じ Go・画面処理で表示する |
| Foundry変更後の再閲覧の確認 | プルダウンで `contoso-foundry-development` を選択し、Home画面を再読み込みする。必要に応じて Production に戻る | 取得や進捗モーダルなしで、ファイルから `saved-development-chat`・`saved-development-embedding` の2件を表示する。変更先のモデルファイルは内容と更新時刻を維持し、選択と表示モデルを状態ファイルに保存する。再読み込み後も選択を維持し、Production に戻すと保存済みの1件を表示する。再現境界は [再閲覧の構成](design/UCP-1.md#foundryを変更し再閲覧する) を参照する |
| Foundry変更後の再閲覧の再起動確認 | 起動端末で `Ctrl+C`。`$env:WAILS_DATA_DIR="$env:TEMP\AzFoundryDeck-foundry-revisit-review"; $env:WAILS_SERVER_PORT='34116'; $env:AZFOUNDRYDECK_E2E_HOLD_FOUNDRY='1'; .\bin\azfoundrydeck-server-e2e.exe` で直接起動する | 固定ファイルを置き直さず、変更後の選択と全モデルを復元する。外部取得を解放するファイルを作らずに表示でき、取得の進捗モーダルを表示しない。公開の確認用起動コマンドはファイルを初期化するため、この確認には使わない |
| Foundry変更後の再閲覧の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。直接起動用に設定した場合は `Remove-Item Env:WAILS_DATA_DIR, Env:WAILS_SERVER_PORT, Env:AZFOUNDRYDECK_E2E_HOLD_FOUNDRY`。通常構成は `node scripts/run.mjs server` | 通常の認証とデータフォルダーを使用する。確認用の固定ファイルを通常データに持ち込まず、保存済みモデルの読み込みと選択更新は同じ処理を通る |
| 通常構成でのFoundry変更後の再閲覧 | `mise run server` で Home画面を開き、モデルファイルが保存済みの別の Foundry を選択する。再読み込みし、同じデータフォルダーでアプリを再起動する | 進捗モーダルなしで保存済みの全モデルを表示し、モデルファイルの内容・更新時刻は変わらない。選択とモデルは状態ファイルへ保存され、再起動後も復元される。同じ Foundry の選択は保存しない。保存形式の変更はなく、実装は [再閲覧の構成](design/UCP-1.md#foundryを変更し再閲覧する) を参照する |
| Foundry一覧の更新の画面確認用起動 | `node scripts/run.mjs server:review:foundry-refresh` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-refresh-review` を初期化し、保存済みの Foundry 3件（Production・Development・Legacy）、Production と Legacy のモデルファイル、Production のモデル2件と両方の最終取得日時 `2026-09-01 09:00` を用意して表示する。外部取得だけが固定応答（Production・Development・Research）になる |
| Foundry一覧の更新の確認 | プルダウン右の更新ボタンにマウスを合わせて押し、進捗中に Escape を押す。完了後にプルダウンを開き、再読み込みする。起動し直し、プルダウンで `contoso-foundry-legacy` を選んでから更新ボタンを押す | ツールチップ「Refresh Foundries」が出る。「Refreshing Foundries」のモーダルが「Foundries」「Deployments」の2行を最初から表示し、大きさを変えず、Escape で閉じない。応答が速いため短い状態は目視できないことがある。1回目は「Deployments」が待機中のままで、一覧が Production・Development・Research に変わり、選択とモデル、モデルの最終取得日時 `2026-09-01 09:00` を維持し、Foundry一覧の最終取得日時だけが現在時刻になる。Legacy のモデルファイルは削除され、再読み込み後も更新後の状態を表示する。Legacy を選んだ場合は「Deployments」も取得中になり、Production とモデル3件に切り替わり、両方の最終取得日時が現在時刻になる。保存形式は [データ設計](design/data.md#foundry-とデプロイモデル) を参照する |
| Foundry一覧の更新の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure の一覧・モデル取得だけが固定応答から実処理に切り替わり、更新・保存・削除・進捗表示は同じ処理を通る。処理は [更新シナリオ設計](design/UCP-1.md#foundry一覧を更新する) を参照する |
| 通常構成での Foundry一覧の更新 | `mise run server` で Home画面を開き、更新ボタンを押す | 実 Azure から参照可能な全 Foundry を取得し、一覧と Foundry一覧の最終取得日時を更新して保存する。選択中の Foundry が一覧から消えた場合は最初の Foundry とそのモデルに切り替わる。以前の形式の保存ファイルは読み込めないため、事前に閲覧保存の基準フォルダーの `foundry-state.json` と `foundry-models` を削除する（未検証。段階5で利用者が確認） |
| デプロイモデルの更新の画面確認用起動 | `node scripts/run.mjs server:review:deployment-refresh` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-deployment-refresh-review` を初期化し、Foundry一覧の更新と同じ保存済みの状態（Production を選択、モデル2件、最終取得日時 `2026-09-01 09:00`）を表示する。外部取得だけが固定応答（Production のモデル3件）になる |
| デプロイモデルの更新の確認 | 見出し「デプロイ済みモデル」右の更新ボタンにマウスを合わせて押し、進捗中に Escape を押す。完了後にモデル一覧と日時を確認し、再読み込みする | ツールチップ「Refresh models」が出る。「Refreshing models」のモーダルが「Deployments」の1行だけで選択中の Foundry 名と取得件数を表示し、Escape で閉じない。応答が速いため短い状態は目視できないことがある。完了後にモデルが `embeddings` を加えた3件になり、モデルの最終取得日時だけが現在時刻になる。Production のモデルファイルが置き換わり、Legacy のモデルファイルは変わらない。再読み込み後も更新後の状態を表示する |
| デプロイモデルの更新の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure のモデル取得だけが固定応答から実処理に切り替わり、保存・進捗表示は同じ処理を通る。処理は [更新シナリオ設計](design/UCP-1.md#デプロイモデルを更新する) を参照する |
| 通常構成でのデプロイモデルの更新 | `mise run server` で Home画面を開き、見出し「デプロイ済みモデル」右の更新ボタンを押す | 実 Azure から選択中の Foundry の全モデルを取得し、モデル一覧とモデルの最終取得日時を更新して保存する。再起動後も更新後のモデルを表示する（未検証。段階5で利用者が確認） |
| 初回閲覧の進捗の確認 | 閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で通常ビルドの Home画面を表示し、モーダルの「Foundries」「Deployments」を確認する。処理中に Escape を押し、モーダル外をクリックする | 2行を最初から表示し、モーダルの大きさは完了まで変わらない。実行中の行に回転表示が出て、一覧の取得後に Foundry の件数、モデル取得中に選択先の名称と取得件数を行の右側に表示する。経過時間とファイル保存は表示しない。保存成功後に自動で閉じる。処理中は Escape・外側クリックで閉じない。実際の応答時間に従って表示が進むため、短い処理の状態は目視できないことがある |
| Home画面の初回閲覧の再現 | 確認用アカウント・テナントの閲覧保存の基準フォルダーに `foundry-state.json` が存在しない状態で `mise run server:review` を起動し、Foundry のプルダウンを開閉して選択表示にマウスを合わせる | 開いた一覧は Foundry 3件の全文を表示する。選択表示は幅に応じて省略し、ツールチップに全文を表示する。モデル一覧はデプロイ名・モデル名・バージョンの3列で3件を表示する。表示した一覧・初期選択・モデルが確認用アカウント・テナントの閲覧保存の基準フォルダーの `foundry-state.json` に保存される |
| ログアウトの再現 | 上の画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | メニューにアカウント名 `operator@contoso.onmicrosoft.com` と「ログアウト」だけが出る。選ぶとヘッダー右が消え、閉じられないログインモーダルが出る。データフォルダーの `e2e-authentication-record.json` と全アカウント・テナントの閲覧保存データが削除される |
| ログアウト後の再起動の再現 | 終了後、`$env:WAILS_DATA_DIR="$env:TEMP\AzFoundryDeck-review"; .\bin\azfoundrydeck-server-e2e.exe`（確認後 `Remove-Item Env:WAILS_DATA_DIR`） | 自動ログインされず、ログインモーダルが出る（`server:review` は起動のたびに記録を置き直すため、記録を置かずに起動する） |
| ログアウト失敗の再現 | `$env:AZFOUNDRYDECK_E2E_FAIL='logout'; mise run server:review`（確認後 `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL`） | 「ログアウト」を選ぶと、メニューが開いたまま中に `LOGOUT_FAILED` と理由が出て、ヘッダー右はログイン済みのまま、「ログアウト」を再度押せる |
| 一括検証（生成・型検査・Lint・整形・単体テスト・Go の vet と test・文書検査・E2E 用ビルド・E2E） | `mise run verify` | 終了コード 0。文書検査が `NG 0 件`、Go の `internal/azauth` が `ok`、すべての E2E テストが合格 |
| E2E のみ再実行 | `mise run verify` を一度実行した後、`npm --prefix frontend run test:e2e` | すべての E2E テストが合格。失敗時の記録は `frontend/playwright-report/` と `frontend/test-results/` |

- **E2E の対象と構成**: シナリオ「ブラウザーでAzureにサインインする」の E2E は `frontend/tests/e2e/usecases/Azureへログインする/ブラウザーでAzureにサインインする.spec.ts` です。各テストが `bin\azfoundrydeck-server-e2e.exe` を一時データディレクトリと空きポートで起動します。主成功（手順1〜4と保存の確認）、認証境界の失敗、保存の失敗の3件を検証します。シナリオ「保存済みのログイン情報で自動的にログイン済みになる」の E2E は同じディレクトリの `保存済みのログイン情報で自動的にログイン済みになる.spec.ts` で、記録を置いて再起動し、復元の成功（取得中にモーダル・スピナーがなくヘッダー右が空であること、ブラウザーのサインインが呼ばれないことを含む）と復元の失敗（モーダルとエラー、記録が残ること、その後の手動ログイン）の2件を検証します。シナリオ「ヘッダーのユーザーアイコンからログアウトする」の E2E は `frontend/tests/e2e/usecases/Azureからログアウトする/ヘッダーのユーザーアイコンからログアウトする.spec.ts` で、記録を置いて再起動したログイン済みの状態から、メニューの内容、記録ファイルの削除、閉じられないログインモーダル、確認ダイアログがないこと、再起動後に自動ログインされないことを検証する主成功と、削除の失敗（メニュー内のエラー、ログイン済みのまま、再押下でも同じ、記録が残る）の2件を検証します。E2E 用ビルドには永続キャッシュがないため、永続キャッシュの削除は E2E の対象外です。
- **Home画面の初回閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/デプロイモデルを初回閲覧する.spec.ts` で、保存済みの固定ログイン情報から Home画面を開き、Foundry 一覧の取得・モデル取得の2行（待機・取得中・完了）、モーダルの大きさが完了まで変わらないこと、保存後の結果表示、保存内容との一致、Foundry の全文表示と省略・ツールチップを検証します。`AZFOUNDRYDECK_E2E_HOLD_FOUNDRY=1` のテストだけで外部取得を保留し、一時データディレクトリの `e2e-foundry-<段階>-release` ファイルで解放します。進捗の遷移は実際の進捗イベントで確認します。Foundry 一覧の取得は実 Azure の Resource Graph で確認します（[確認した事実](#design)）。`internal/foundry/service_test.go` は並び順と先頭の選択、一覧の取得完了後のモデル取得、一覧の取得失敗時の中止と既存保存内容の維持を検証します。すべて `mise run verify` で再実行できます。
- **モデル明細の検証**: `frontend/tests/e2e/usecases/デプロイモデルの詳細を確認する/一覧からデプロイモデルの明細を表示する.spec.ts` は、行全体とキーボードでの選択、同じ行の再取得、取得中の表示と操作制限、明細項目、保存ファイルの不変、更新・Foundry・テナント変更後の明細破棄、失敗時の一覧維持と再試行を検証します。`internal/foundry/azure_detail_test.go` はモデル別の単位換算、欠落したレート定義、容量ゼロ、PTUの刻みと許可値を検証します。実 Azure の確認は通常構成で、実在するデプロイの明細と共有クォータに基づく上限を照合します。
- **単体テスト**: 永続キャッシュの削除（`internal/azauth/token_cache_windows_test.go`）は、実際の `%LOCALAPPDATA%\.IdentityService` に試験用の名前 `azfoundrydeck-test-<時刻>` とその `.cae` のファイルを作り、削除されることと、存在しないときの再削除が成功することを確認して後始末します。本アプリの `azfoundrydeck` には触れません。`go test ./internal/...`（`mise run verify` に含まれる）で実行されます。新しい実装による本番の永続キャッシュ・資格情報・全閲覧保存データの削除は実機未検証です。
- **Home画面の再閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/デプロイモデルを再閲覧する.spec.ts` で、保存済みファイルの一覧・2件目の Foundry の選択・全モデルを復元し、再読み込みとアプリ再起動後も維持することを検証します。外部取得を保留したまま表示できること、取得進捗イベントとモーダルが出ないこと、保存ファイルの内容・更新時刻が変わらないこと、プルダウンの全文表示・省略・ツールチップも確認します。保存ファイルの読み込みと表示は本番と同じ処理を通します。
- **Foundry変更後の初回閲覧の検証**: `frontend/tests/e2e/usecases/Foundryを変更する/Foundryを変更し初回閲覧する.spec.ts` で、変更先が未保存の状態から、モデル取得の進捗、処理中の旧表示と操作制限、保存後の切り替え、変更前と変更先のモデルファイル、再起動後の復元、同じ Foundry を選んだ場合に取得・保存しないことを検証します。Foundry 一覧の取得は解放せず、モデル取得だけを解放して一覧を再取得しないことを確認します。Go の単体テストでは、保存済みモデルを使う際の外部取得の抑止と、保存失敗時の変更前データの保持も確認します。
- **Foundry変更後の再閲覧の検証**: `frontend/tests/e2e/usecases/Foundryを変更する/Foundryを変更し再閲覧する.spec.ts` で、保存済みの変更先モデルへの切り替え、変更前と変更先のモデルファイルの内容・更新時刻の維持、状態ファイルだけの更新、進捗イベントとモーダルがないこと、再起動後の復元、同じ Foundry を選んだ場合に保存しないこと、親ユースケースの表示条件を検証します。全外部取得を保留して解放せず、ファイルの読み込みと保存は本番と同じ処理を通します。
- **Foundry一覧の更新の検証**: `frontend/tests/e2e/usecases/Foundry一覧を更新する/Foundry一覧を更新する.spec.ts` で、保存済みの一覧（固定 Source にない Legacy を含む）から更新ボタンを押し、Foundry 一覧の取得の進捗（モデルの行は待機中のまま）、処理中の旧表示と操作制限、Escape・外側クリックで閉じないこと、更新後の一覧・選択・モデル・最終取得日時の表示と状態ファイルの内容、一覧にない Foundry のモデルファイルの削除、モデルを取得しないこと、再起動後の復元を検証します。選択中の Foundry が消える場合は、`frontend/tests/e2e/usecases/Foundry一覧を更新する/Foundry一覧の更新で選択先が変わる.spec.ts` で、一覧の取得後に最初の Foundry のモデル取得の進捗（モーダルの大きさが変わらないこと）、モデルファイルと状態ファイルの保存、一覧にない Foundry のモデルファイルの削除、両方の最終取得日時の更新と再起動後の復元を検証します。外部取得は `e2e-foundry-<段階>-release` ファイルで段階ごとに解放します。
- **デプロイモデルの更新の検証**: `frontend/tests/e2e/usecases/デプロイモデルを更新する/デプロイモデルを更新する.spec.ts` で、保存済みのモデルファイルがある状態から更新ボタンを押し、モデル取得の1行だけの進捗、Foundry 一覧の取得を表示しないこと、処理中の旧表示と操作制限、Escape・外側クリックで閉じないこと、取得後のモデル・件数・最終取得日時の表示、状態ファイルと選択中の Foundry のモデルファイルの置き換え、ほかの Foundry のモデルファイルの内容・更新時刻の維持、再起動後と別の Foundry へ変更して戻った後の表示を検証します。外部取得はモデル取得だけを `e2e-foundry-models-release` ファイルで解放します。
- **E2E 用ビルド**: `node scripts/build.mjs server-e2e`（`build:server:e2e` タスク）が `-tags server,production,e2e` でビルドします。`e2e` タグでは、サインインの外部境界が固定の認証記録（アカウント名 `operator@contoso.onmicrosoft.com`、認証テナント ID `e2e-tenant`）を返し、一覧取得の外部境界が唯一の候補（ID `e2e-azure-tenant`、表示名 `Contoso`）を返します。唯一の候補の自動選択と認証記録・一覧・選択の保存は通常と同じサービス処理を通し、資格情報マネージャーの代わりにデータディレクトリの `e2e-authentication-record.json` へ保存します。環境変数 `AZFOUNDRYDECK_E2E_FAIL=signin`、`save`、`restore`、`logout` で失敗を注入します。ログアウトでは記録ファイルと全アカウント・テナントの閲覧保存データを削除します。起動前にデータディレクトリへ `e2e-authentication-record.json` を置くと、起動時の復元がその記録で成功します。サインインが呼ばれるとデータディレクトリに `e2e-signin-called` を作り、`AZFOUNDRYDECK_E2E_HOLD_RESTORE=1` のときは復元が `e2e-restore-release` の作成まで応答を保留します。
- **画面確認用の起動と実処理への切り替え**: `server:review` は E2E 用ビルド（`bin\azfoundrydeck-server-e2e.exe`）を、一時フォルダーの固定データディレクトリ `AzFoundryDeck-review` に認証記録・テナント一覧・選択を含む `e2e-authentication-record.json` を置いて起動します。認証と Foundry・モデル取得の外部境界は固定応答を返しますが、初期選択・進捗の合成と通知・ファイル保存・保存済みファイルの読み込み・結果表示は本番と同じ処理です。保存済みファイルがあれば外部取得を呼ばずに復元します。固定応答の内容と進捗の接続は [UCP-1](design/UCP-1.md#デプロイモデルの初回閲覧) を参照します。Azure・資格情報マネージャー・永続キャッシュには触れません。終了は起動した端末で `Ctrl+C` です。実 Azure の取得へ切り替える場合は終了後に `mise run server` で通常ビルドを起動します。読み込み・接続・保存に失敗した場合は固定応答に切り替わらず、Home画面にエラーが表示されることを確認します。本番のログアウトで削除する対象は [データ設計](design/data.md#ログアウト時の削除範囲) に従います。閲覧保存データの削除も同じ実処理を通しますが、本番の永続キャッシュと資格情報マネージャーを含む実機動作は未検証です。
- **本番に含まれないこと**: `internal/azauth/e2e.go`、`record_store_e2e.go`、`internal/foundry/e2e.go`、`foundry_source_e2e.go` は `e2e` タグのときだけコンパイルされ、`server`、`build`、`package`、`dev` のビルドには含まれません。`go list -tags server,production -f '{{.GoFiles}}' ./internal/azauth ./internal/foundry .` にこれらのファイルが現れないことで確認できます。実 Azure へのサインイン・Foundry 一覧とモデルの取得、実ブラウザーでの認証、実資格情報マネージャーへの保存は E2E の対象外です。

- **保存するもの**: アカウント識別情報・テナント一覧・選択を Windows 資格情報マネージャーの汎用資格情報 `AzFoundryDeck:AuthenticationRecord`（ユーザー名 `AuthenticationRecord`）に、トークンを `azidentity/cache` の永続キャッシュ（名前 `azfoundrydeck`）に保存します。通常起動時とテナント変更時は保存した一覧を使い、ブラウザーでの再サインイン時だけ ARM の一覧を取得し直します。保存形式・アカウントとテナントごとの閲覧保存先・ログアウトの削除範囲は [データ設計](design/data.md) を参照します。
- **自動ログインの実機確認**: ブラウザーでのサインイン成功後にアプリを終了し、同じデータフォルダーで通常ビルドを再起動します。ブラウザーを開かずに保存済みの一覧・選択からヘッダーを復元し、ARM のテナント一覧を取得し直さないこと、同じアカウント・テナントの閲覧結果を復元することを確認します。
- **失敗時の実機確認**: 接続できないプロキシを設定した通常ビルドで「Azureにログイン」を押し、ログインモーダルにエラーのコードと理由が表示され、再試行できること、閲覧を開始しないことを確認します。新しい実装での動作は未検証です。診断ログの場所は `%APPDATA%\\AzFoundryDeck\\logs\\app.jsonl` です。

環境構築、作業ディレクトリ、実行コマンド、設定、期待結果を明記します。自動テストと実機確認の対象・条件を示し、最新コードで再実行できる手順を維持します。モック利用時は、起動・終了、モック有効/無効の確認、実処理への切り替え手順を記述します（接続失敗時にモックへフォールバックしないことの確認を含む）。
