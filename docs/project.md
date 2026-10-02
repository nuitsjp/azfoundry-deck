# AzFoundryDeck のプロジェクト定義

プロジェクト共通の要件・制約、ユースケース一覧、確認した事実、および実行・検証手順の正本です。全体構造は [アーキテクチャ](architecture.md) を参照します。

## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 解決する問題・達成したい結果 | Azure 上の Microsoft Foundry とデプロイ済みモデルを、Azure SDK for Go を使うデスクトップアプリから管理できるようにする。 |
| 利用者・利用場面 | Azure アカウントを持ち、Foundry を運用する個人。Windows デスクトップで利用する。 |
| 今回の対象 | Azure へのログイン（ログイン情報の OS クレデンシャルマネージャーへの保存を含む）。 |
| 今回の対象外 | ログアウト、サブスクリプション・Foundry・モデルデプロイの参照と操作（別ユースケースとして順次追加する）。 |

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

<a id="design"></a>
## 4. 確認した事実

ログイン情報の保存手段（確認日 2026-10-02。情報源は Go モジュールキャッシュ内のソースで、対象版は `azidentity` v1.14.1、`azidentity/cache` v0.4.0、`microsoft-authentication-extensions-for-go/cache` v0.1.1、`microsoft-authentication-library-for-go` v1.8.0、`zalando/go-keyring` v0.2.6。確認範囲は Windows の保存処理）:

- `azidentity/cache` の永続キャッシュは、Windows では `%LOCALAPPDATA%\.IdentityService\<Name>`（CAE 用は `<Name>.cae`）に DPAPI（`CryptProtectData`）で暗号化したファイルとして保存する。資格情報マネージャーは使わない。`Name` の既定は `msal.cache` で他アプリと共有され得る。
- `azidentity.Cache` は内部型の別名で、外部モジュールからは `cache.New` 以外で作れない（`azidentity/internal` の import は `use of internal package ... not allowed` でビルドできないことを実測）。そのため `InteractiveBrowserCredential` のトークン保存先を資格情報マネージャーへ差し替えられず、リフレッシュトークンも取り出せない。
- MSAL はトークン取得時に永続キャッシュへ書き込み、書き込みの失敗をトークン取得の失敗として返す（`apps/internal/base/base.go` の `AuthResultFromToken`）。
- `AuthenticationRecord` は authority、clientId、homeAccountId、tenantId、username、version の6項目の JSON で、秘密情報を含まない。
- `InteractiveBrowserCredential` に `AuthenticationRecord`、`Cache`、`DisableAutomaticAuthentication: true` を渡すと、`GetToken` は記録のアカウントで `AcquireTokenSilent`（キャッシュのトークン、または更新トークンによる更新）だけを行い、失敗時は `AuthenticationRequiredError` を返してブラウザー認証（`AcquireTokenInteractive`）へ進まない（`azidentity` の `public_client.go` の `GetToken`）。`Authenticate` はキャッシュを使わず常にブラウザー認証を行う。記録の JSON の読み込みは `version` がない、または未対応の場合にエラーを返す（`authentication_record.go`）。
- `go-keyring` は Windows で `danieljoos/wincred` の汎用資格情報を使い、対象名は `<service>:<user>`、保存値が 2560 バイトを超えると `ErrSetDataTooBig` を返す。本アプリの `AzFoundryDeck:AuthenticationRecord` へ試験用レコード（283 バイト）を書き込み、`cmdkey /list` での表示、読み出しと一致確認、削除を実機で確認した。

- **確認した事実**: 外部仕様や既存コードの調査結果（情報源、対象版、確認日、確認範囲）。仮定と明確に区別します。外部システムの実測応答を保存する場合は `reference/` に配置して参照します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

作業ディレクトリはリポジトリのルートです。コマンドは PowerShell で実行します。

| 目的 | コマンド・設定 | 成功確認 |
| --- | --- | --- |
| 環境構築 | `mise trust`、`mise run setup`、`mise run setup:browser` | `frontend/bindings/azfoundrydeck/internal/azauth/` が生成される |
| 起動（ブラウザー確認） | `mise run server` | `http://127.0.0.1:34115/` を開くと、Home を背景にログインのモーダルが表示される |
| 起動（デスクトップ） | `mise run dev` | ウィンドウにログインのモーダルが表示される（未検証） |
| ログイン | モーダルの「Azureにログイン」を押し、開いたブラウザーでサインインする | モーダルが閉じてヘッダーにテナント名とユーザーアイコンが表示され、`cmdkey /list:AzFoundryDeck:AuthenticationRecord` に資格情報が表示され、`%LOCALAPPDATA%\.IdentityService\azfoundrydeck`（CAE 用は `azfoundrydeck.cae`）が作成される（段階5で利用者が実 Azure で確認） |
| 自動ログイン（起動時の復元） | 保存済みのログイン情報がある状態で `mise run server` を起動し、`http://127.0.0.1:34115/` を開く | モーダルを表示せずに、ヘッダーにテナント名とユーザーアイコンが表示される。失敗時はモーダル内に `LOGIN_FAILED` と理由が表示され、ログに `operation":"azauth.Restore"` の `operation_failed` が記録される |
| 保存したログイン情報の削除 | `cmdkey /delete:AzFoundryDeck:AuthenticationRecord`、`Remove-Item "$env:LOCALAPPDATA\.IdentityService\azfoundrydeck*"` | `cmdkey /list:AzFoundryDeck:AuthenticationRecord` が「なし」を表示する |
| 終了 | 起動した端末で `Ctrl+C` | `http://127.0.0.1:34115/health` に応答しない |
| 一括検証（生成・型検査・Lint・整形・単体テスト・Go の vet と test・文書検査・E2E 用ビルド・E2E） | `mise run verify` | 終了コード 0。文書検査が `NG 0 件`、E2E が `3 passed` |
| E2E のみ再実行 | `mise run verify` を一度実行した後、`npm --prefix frontend run test:e2e` | `3 passed`。失敗時の記録は `frontend/playwright-report/` と `frontend/test-results/` |

- **E2E の対象と構成**: シナリオ「ブラウザーでAzureにサインインする」の E2E は `frontend/tests/e2e/usecases/Azureへログインする/ブラウザーでAzureにサインインする.spec.ts` です。各テストが `bin\azfoundrydeck-server-e2e.exe` を一時データディレクトリと空きポートで起動します。主成功（手順1〜4と保存の確認）、トークン取得・テナント名取得の失敗、保存の失敗の3件を検証します。
- **E2E 用ビルド**: `node scripts/build.mjs server-e2e`（`build:server:e2e` タスク）が `-tags server,production,e2e` でビルドします。`e2e` タグでは、外部境界のサインイン（Entra ID・ARM）が固定のアカウント（`operator@contoso.onmicrosoft.com`、テナント名 `Contoso`）を返し、アカウント識別情報は資格情報マネージャーの代わりにデータディレクトリの `e2e-authentication-record.json` に同じ JSON で保存されます。環境変数 `AZFOUNDRYDECK_E2E_FAIL=signin`、`save`、`restore` で失敗を注入します。起動前にデータディレクトリへ `e2e-authentication-record.json` を置くと、起動時の復元がその記録で成功します。
- **本番に含まれないこと**: `internal/azauth/e2e.go` と `record_store_e2e.go` は `e2e` タグのときだけコンパイルされ、`server`、`build`、`package`、`dev` のビルドには含まれません。`go list -tags server,production -f '{{.GoFiles}}' ./internal/azauth .` に `e2e.go`・`record_store_e2e.go` が現れないことで確認できます。実 Azure へのサインイン、実ブラウザーでの認証、実資格情報マネージャーへの保存は E2E の対象外です。

- **保存するもの**: アカウント識別情報（`azidentity.AuthenticationRecord` の JSON）を Windows 資格情報マネージャーの汎用資格情報 `AzFoundryDeck:AuthenticationRecord`（ユーザー名 `AuthenticationRecord`）に、トークンを `azidentity/cache` の永続キャッシュ（名前 `azfoundrydeck`）に保存します。
- **自動ログインの確認**: 2026-10-02 に `bin\azfoundrydeck-server.exe` で次を確認しました。この端末の実際の保存済みログイン情報では、ブラウザー操作なしにテナント名とユーザーアイコンが表示されました。保存済みログイン情報がない場合はエラーなしのモーダル、不正な JSON の場合と、キャッシュにないアカウント（`homeAccountId` を変えた記録）の場合はモーダルと `LOGIN_FAILED` が表示され、保存済みの値は変わりませんでした。いずれも取得中はモーダルとスピナーを表示せず、ヘッダー右は空でした。ブラウザー認証が始まらないことは、サーバーの待ち受けポートがアプリの 34115 だけで、ブラウザー認証時に MSAL が開く応答受信用のローカルポートが現れないことで確認しました。確認後に元の保存値へ戻しました。
- **失敗時の確認**: `mise run server` で作成した `bin\azfoundrydeck-server.exe` を、接続できないプロキシ（`HTTPS_PROXY=http://127.0.0.1:9`）を設定して起動し、「Azureにログイン」を押すと、モーダル内に `LOGIN_FAILED` と理由が表示され、ボタンが再び押せる状態に戻り、ログイン済みにならないことを確認しました。ログ（`%APPDATA%\AzFoundryDeck\logs\app.jsonl`）には `operation_failed` と接続失敗の原因が記録されます。

環境構築、作業ディレクトリ、実行コマンド、設定、期待結果を明記します。自動テストと実機確認の対象・条件を示し、最新コードで再実行できる手順を維持します。モック利用時は、起動・終了、モック有効/無効の確認、実処理への切り替え手順を記述します（接続失敗時にモックへフォールバックしないことの確認を含む）。
