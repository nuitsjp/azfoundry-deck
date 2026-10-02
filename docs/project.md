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
- 認証情報は OS のクレデンシャルマネージャーにのみ保存し、平文でファイル・画面・ログへ出さない。
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

未確認。ログイン情報の保存先として使う SDK の永続キャッシュと Windows 資格情報マネージャーの関係は、段階4の前に SDK のソースで確認して記録する。

- **確認した事実**: 外部仕様や既存コードの調査結果（情報源、対象版、確認日、確認範囲）。仮定と明確に区別します。外部システムの実測応答を保存する場合は `reference/` に配置して参照します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

作業ディレクトリはリポジトリのルートです。コマンドは PowerShell で実行します。

| 目的 | コマンド・設定 | 成功確認 |
| --- | --- | --- |
| 環境構築 | `mise trust`、`mise run setup`、`mise run setup:browser` | `frontend/bindings/azfoundrydeck/internal/azauth/` が生成される |
| モック起動（ブラウザー確認） | `mise run server:mock` | `http://127.0.0.1:34115/` に右上の「試験用モック」バッジと固定データの注記が表示され、サーバーの出力に `mock authenticator enabled` が出る |
| モックの失敗再現 | `$env:AZFOUNDRYDECK_MOCK_LOGIN_FAIL='1'; mise run server:mock`（確認後 `Remove-Item Env:AZFOUNDRYDECK_MOCK_LOGIN_FAIL`） | 「Azureにログイン」を押して約3秒後に `LOGIN_FAILED` の理由が表示され、ボタンが再び押せる未ログイン画面に戻る |
| モック起動（デスクトップ） | `mise run dev:mock` | ウィンドウに「試験用モック」バッジが表示される（未検証） |
| 実処理起動 | `mise run server`（ブラウザー確認、URL は同上）、`mise run dev`（デスクトップ） | 「試験用モック」バッジがなく、ログ（`%APPDATA%\AzFoundryDeck\logs\app.jsonl`）に `mock authenticator enabled` が出ない |
| 終了 | 起動した端末で `Ctrl+C` | `http://127.0.0.1:34115/health` に応答しない |

- **モックの範囲**: 合成点は `main.go` で `internal/azauth` の `Authenticator` を選ぶ1箇所です。モックの `Fixed` は約3秒待ってから固定のアカウント名 `operator@contoso.onmicrosoft.com` とテナントID `00000000-0000-0000-0000-000000000001` を返します。Azure への接続と OS のクレデンシャルマネージャーへの保存は行いません。
- **モック有効の条件**: `production` タグなしのビルドで、環境変数 `WAILS_FRONTEND_MODE=mock` のときだけ有効です。`server:mock` と `dev:mock` がこの値を設定します。`production` タグ付きのビルド（`server`、`build`、`package`）は環境変数に関係なく実処理（`azidentity.InteractiveBrowserCredential`）を使います。
- **実処理への切り替え**: `server:mock` を終了し、`mise run server` または `mise run dev` で起動します。
- **フォールバックしないことの確認**: `mise run server` で作成した `bin\azfoundrydeck-server.exe` を、`WAILS_FRONTEND_MODE=mock` と接続できないプロキシ（`HTTPS_PROXY=http://127.0.0.1:9`）を設定して起動し、「Azureにログイン」を押すと `LOGIN_FAILED` が表示され、ログイン済みにならないことを確認しました。ログには `operation_failed` と接続失敗の原因が記録されます。

環境構築、作業ディレクトリ、実行コマンド、設定、期待結果を明記します。自動テストと実機確認の対象・条件を示し、最新コードで再実行できる手順を維持します。モック利用時は、起動・終了、モック有効/無効の確認、実処理への切り替え手順を記述します（接続失敗時にモックへフォールバックしないことの確認を含む）。
