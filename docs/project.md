# AzFoundryDeck のプロジェクト定義

プロジェクト共通の要件・制約、ユースケース一覧、確認した事実、および実行・検証手順の正本です。全体構造は [アーキテクチャ](architecture.md) を参照します。

## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 解決する問題・達成したい結果 | Azure 上の Microsoft Foundry とデプロイ済みモデルを、Azure SDK for Go を使うデスクトップアプリから管理できるようにする。 |
| 利用者・利用場面 | Azure アカウントを持ち、Foundry を運用する個人。Windows デスクトップで利用する。 |
| 今回の対象 | Azure へのログインとログアウト（ログイン情報の保存と破棄を含む）、利用対象のテナントの選択と変更、Home画面でのデプロイモデルの閲覧（Foundry 一覧・選択済み Foundry・選択された Foundry の全デプロイ済みモデルのファイル保存と、保存済みファイルからの復元、Foundry を変更した後の初回閲覧と再閲覧を含む）。 |
| 今回の対象外 | サブスクリプション・Foundry・モデルデプロイの操作、および上記以外の参照（別ユースケースとして順次追加する）。 |

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
| [Azureからログアウトする](usecases/Azureからログアウトする/README.md) | Foundry の運用者 | このアプリのログイン状態と保存したログイン情報を破棄する | 2 | [UCP-1](design/UCP-1.md) | 対象 |
| [デプロイモデルを閲覧する](usecases/デプロイモデルを閲覧する/README.md) | Foundry の運用者 | Home画面で Foundry 一覧と選択された Foundry のデプロイ済みモデルを確認する | 3 | [UCP-1](design/UCP-1.md) | 対象 |
| [テナントを変更する](usecases/テナントを変更する/README.md) | Azure にログイン済みの Foundry 運用者 | 認証済みのアカウントを維持したまま、Azure リソース操作の対象テナントを変更する | 4 | [UCP-1](design/UCP-1.md) | 対象 |

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

- **確認した事実**: 外部仕様や既存コードの調査結果（情報源、対象版、確認日、確認範囲）。仮定と明確に区別します。外部システムの実測応答を保存する場合は `reference/` に配置して参照します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

作業ディレクトリはリポジトリのルートです。コマンドは PowerShell で実行します。

| 目的 | コマンド・設定 | 成功確認 |
| --- | --- | --- |
| 環境構築 | `mise trust`、`mise run setup`、`mise run setup:browser` | `frontend/bindings/azfoundrydeck/internal/azauth/` と `frontend/bindings/azfoundrydeck/internal/foundry/` が生成される |
| self-hosted runner の初期登録・追加 | 管理者 PowerShell 7 で `mise run setup:runner`。2台目以降は `mise run setup:runner -- 2` のように正整数の番号を指定する。GitHub の Settings → Actions → Runners → New self-hosted runner から登録トークンを取得し、プロンプトで入力する。サービス実行アカウントもプロンプトで指定する | GitHub の Runners に runner が Online と表示される。番号省略・1は既存の配置先と名前を使い、2以降はフォルダー名・登録名に `-2` などを付けて別サービスにする。[設定スクリプト](../scripts/setup-runner.ps1) は既存ファイルがある配置先を上書きしない |
| runner 本体の自動更新 | runner の標準自動更新を有効にしたままサービスを常駐させる。別の定期タスクは不要 | [GitHub の仕様](https://docs.github.com/en/actions/reference/runners/self-hosted-runners#communication)では、ジョブ割り当て時、または新バージョン公開後1週間以内に更新される |
| 起動（ブラウザー確認） | `mise run server` | `http://127.0.0.1:34115/` を開くと、Home を背景にログインのモーダルが表示される |
| 起動（デスクトップ） | `mise run dev` | ウィンドウにログインのモーダルが表示される（未検証） |
| 1件テナントのサインイン画面確認用起動 | `mise run server:review:login` | `http://127.0.0.1:34117/` が未ログインで開く。起動のたびに空の一時フォルダー `AzFoundryDeck-login-review-...` を作り、端末にパスを表示する。「Azureにログイン」を押すと、ブラウザーを開かずに固定応答で認証し、唯一の候補 ID `e2e-azure-tenant`、表示名 `Contoso` が選択される。認証記録のテナント ID `e2e-tenant` は候補 ID と異なる。ヘッダーのプルダウンとユーザーアイコンへのマウスオーバーで選択名・アカウント名 `operator@contoso.onmicrosoft.com` を確認する。実 Azure・資格情報マネージャー・永続キャッシュには触れず、選択の永続保存やテナント別の閲覧データ分離は未検証。構成は [サインイン設計](design/UCP-1.md#ブラウザーでazureにサインインする) を参照する |
| サインイン画面確認用構成の終了と実処理への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` または `mise run dev` | 確認用の一時データを通常データへ持ち込まず、実 Azure の認証経路に切り替わる。実認証・テナント一覧取得の変更と選択の永続保存は段階4で接続する対象であり、確認用画面の成功は本番認証・保存の成功を意味しない |
| ログイン | モーダルの「Azureにログイン」を押し、開いたブラウザーでサインインする | モーダルが閉じてヘッダーにテナント名とユーザーアイコンが表示され、`cmdkey /list:AzFoundryDeck:AuthenticationRecord` に資格情報が表示され、`%LOCALAPPDATA%\.IdentityService\azfoundrydeck.cae` が作成される（段階5で利用者が実 Azure で確認） |
| 自動ログイン（起動時の復元） | 保存済みのログイン情報がある状態で `mise run server` を起動し、`http://127.0.0.1:34115/` を開く | モーダルを表示せずに、ヘッダーにテナント名とユーザーアイコンが表示される。失敗時はモーダル内に `LOGIN_FAILED` と理由が表示され、ログに `operation":"azauth.Restore"` の `operation_failed` が記録される |
| ログアウト | ログイン済みの画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | ヘッダー右が消えてログインモーダルが出る。`cmdkey /list:AzFoundryDeck:AuthenticationRecord` が「なし」を表示し、`%LOCALAPPDATA%\.IdentityService\azfoundrydeck` と `azfoundrydeck.cae` が存在しない（実 Azure でのログアウトは未検証。段階5で利用者が確認） |
| 保存したログイン情報の手動削除 | `cmdkey /delete:AzFoundryDeck:AuthenticationRecord`、`Remove-Item "$env:LOCALAPPDATA\.IdentityService\azfoundrydeck*"` | `cmdkey /list:AzFoundryDeck:AuthenticationRecord` が「なし」を表示する |
| 終了 | 起動した端末で `Ctrl+C` | `http://127.0.0.1:34115/health` に応答しない |
| Home画面での初回閲覧 | データフォルダーに `foundry-state.json` が存在しない状態で `mise run server` を起動し、ログイン済みで Home画面を表示する | 取得・保存の進捗モーダルを経て、参照可能な全 Foundry と最初に発見した Foundry の全デプロイ済みモデルが表示され、データフォルダーの `foundry-state.json` に同じ内容が保存される。取得または保存に失敗した場合は Home画面に `FOUNDRY_LOAD_FAILED` が表示される |
| 画面確認用の起動（Home画面・ログアウトの UI 確認） | `mise run server:review` | `http://127.0.0.1:34115/` がログイン済み（ヘッダー右に `Contoso` とユーザーアイコン）で開く。保存済みファイルがなければ固定の Foundry 3件とモデル3件を取得して保存し、あればその保存内容を表示する。外部取得の固定応答は待ち時間なしで返るため、初回取得の進捗モーダルは短時間で閉じる。端末に `review data directory: <一時フォルダー>\AzFoundryDeck-review` が出る |
| Home画面での再閲覧 | 保存済みの `foundry-state.json` がある状態で `mise run server` を起動し、ログイン済みで `http://127.0.0.1:34115/` を開く。Home画面を再読み込みし、プルダウンを開閉して選択表示にマウスを合わせる | 保存された Foundry 一覧・選択・全モデルをそのまま表示する。展開時は全項目の全文、閉じた選択表示は幅に応じた省略と全文ツールチップを表示する。Azure からの一覧・モデル取得、進捗モーダルの表示、再保存を行わない。読み込みや JSON の復元に失敗した場合は `FOUNDRY_LOAD_FAILED` が表示される。処理は [再閲覧設計](design/UCP-1.md#デプロイモデルの再閲覧) を参照する |
| 再閲覧の再起動確認 | 上の通常ビルドを起動端末の `Ctrl+C` で終了し、同じデータフォルダーで `mise run server` を起動して Home画面を開く | 再起動前と同じ保存済みの一覧・選択・全モデルを表示し、取得の進捗モーダルを表示しない。保存ファイルの内容と更新時刻は変わらない |
| Foundry変更の画面確認用起動 | `node scripts/run.mjs server:review:foundry-change` | `http://127.0.0.1:34116/` がログイン済みで開く。通常と同じフロントエンドを E2E 用 Go サービスにつなぎ、保存済み状態がなければ固定の Foundry 3件と初期選択先のモデル3件を取得して保存する。保存済みならその内容を表示する。端末に確認用データフォルダーが表示される |
| Foundry変更の確認 | 変更先のモデルファイルが存在しない状態で、プルダウンから `contoso-foundry-development` を選択する。完了後に選択表示へマウスを合わせ、Home画面を再読み込みする | サブスクリプションと Foundry 一覧は再取得せず、変更先のモデル取得と保存の実際の進捗を表示する。応答が速い場合は短い状態を目視できないことがある。成功後に変更先とモデル3件へ切り替わり、省略表示と全文ツールチップを表示する。保存形式は [データ設計](design/data.md#foundry-とデプロイモデル) を参照する。再読み込み後も保存した選択とモデルを表示する |
| Foundry変更の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。通常構成は `mise run server` | Azure のモデル取得だけが確認用の固定応答から実処理に切り替わり、変更先の確認・選択更新・ファイル読み込みと保存・進捗表示は同じ処理を通る。処理と型は [変更シナリオ設計](design/UCP-1.md#foundryを変更し初回閲覧する) を参照する |
| Foundry変更後の再閲覧の画面確認用起動 | `node scripts/run.mjs server:review:foundry-revisit` | `http://127.0.0.1:34116/` がログイン済みで開く。起動のたびに `%TEMP%\AzFoundryDeck-foundry-revisit-review` に認証記録、Foundry 2件と各モデルファイル、Production を選択した状態ファイルを用意する。初期表示は `saved-production-chat` の1件。`AZFOUNDRYDECK_E2E_HOLD_FOUNDRY=1` で外部取得を保留したまま、本番と同じ Go・画面処理で表示する |
| Foundry変更後の再閲覧の確認 | プルダウンで `contoso-foundry-development` を選択し、Home画面を再読み込みする。必要に応じて Production に戻る | 取得や進捗モーダルなしで、ファイルから `saved-development-chat`・`saved-development-embedding` の2件を表示する。変更先のモデルファイルは内容と更新時刻を維持し、選択と表示モデルを状態ファイルに保存する。再読み込み後も選択を維持し、Production に戻すと保存済みの1件を表示する。再現境界は [再閲覧の構成](design/UCP-1.md#foundryを変更し再閲覧する) を参照する |
| Foundry変更後の再閲覧の再起動確認 | 起動端末で `Ctrl+C`。`$env:WAILS_DATA_DIR="$env:TEMP\AzFoundryDeck-foundry-revisit-review"; $env:WAILS_SERVER_PORT='34116'; $env:AZFOUNDRYDECK_E2E_HOLD_FOUNDRY='1'; .\bin\azfoundrydeck-server-e2e.exe` で直接起動する | 固定ファイルを置き直さず、変更後の選択と全モデルを復元する。外部取得を解放するファイルを作らずに表示でき、取得の進捗モーダルを表示しない。公開の確認用起動コマンドはファイルを初期化するため、この確認には使わない |
| Foundry変更後の再閲覧の終了と通常構成への切り替え | 起動端末で `Ctrl+C`。直接起動用に設定した場合は `Remove-Item Env:WAILS_DATA_DIR, Env:WAILS_SERVER_PORT, Env:AZFOUNDRYDECK_E2E_HOLD_FOUNDRY`。通常構成は `node scripts/run.mjs server` | 通常の認証とデータフォルダーを使用する。確認用の固定ファイルを通常データに持ち込まず、保存済みモデルの読み込みと選択更新は同じ処理を通る |
| 通常構成でのFoundry変更後の再閲覧 | `mise run server` で Home画面を開き、モデルファイルが保存済みの別の Foundry を選択する。再読み込みし、同じデータフォルダーでアプリを再起動する | 進捗モーダルなしで保存済みの全モデルを表示し、モデルファイルの内容・更新時刻は変わらない。選択とモデルは状態ファイルへ保存され、再起動後も復元される。同じ Foundry の選択は保存しない。保存形式の変更はなく、実装は [再閲覧の構成](design/UCP-1.md#foundryを変更し再閲覧する) を参照する |
| 初回閲覧の進捗の確認 | `foundry-state.json` が存在しない状態で通常ビルドの Home画面を表示し、モーダルの検索・サブスクリプション一覧・モデル取得・保存を確認する。処理中に Escape を押し、モーダル外をクリックする | 検索中と発見件数、待機中・取得中のサブスクリプションが発見順に表示される。完了した行は削除され、完了数と発見件数は削除した行も含めて集計される。選択先とモデルの取得件数、保存状態を表示し、保存成功後に自動で閉じる。処理中は Escape・外側クリックで閉じない。実際の応答時間に従って表示が進むため、短い処理の状態は目視できないことがある |
| Home画面の初回閲覧の再現 | 確認用データフォルダーに `foundry-state.json` が存在しない状態で `mise run server:review` を起動し、Foundry のプルダウンを開閉して選択表示にマウスを合わせる | 開いた一覧は Foundry 3件の全文を表示する。選択表示は幅に応じて省略し、ツールチップに全文を表示する。モデル一覧はデプロイ名・モデル名・バージョンの3列で3件を表示する。表示した一覧・初期選択・モデルが確認用データフォルダーの `foundry-state.json` に保存される |
| ログアウトの再現 | 上の画面でユーザーアイコンを押し、メニューの「ログアウト」を選ぶ | メニューにアカウント名 `operator@contoso.onmicrosoft.com` と「ログアウト」だけが出る。選ぶとヘッダー右が消え、閉じられないログインモーダルが出る。データフォルダーの `e2e-authentication-record.json` が削除される |
| ログアウト後の再起動の再現 | 終了後、`$env:WAILS_DATA_DIR="$env:TEMP\AzFoundryDeck-review"; .\bin\azfoundrydeck-server-e2e.exe`（確認後 `Remove-Item Env:WAILS_DATA_DIR`） | 自動ログインされず、ログインモーダルが出る（`server:review` は起動のたびに記録を置き直すため、記録を置かずに起動する） |
| ログアウト失敗の再現 | `$env:AZFOUNDRYDECK_E2E_FAIL='logout'; mise run server:review`（確認後 `Remove-Item Env:AZFOUNDRYDECK_E2E_FAIL`） | 「ログアウト」を選ぶと、メニューが開いたまま中に `LOGOUT_FAILED` と理由が出て、ヘッダー右はログイン済みのまま、「ログアウト」を再度押せる |
| 一括検証（生成・型検査・Lint・整形・単体テスト・Go の vet と test・文書検査・E2E 用ビルド・E2E） | `mise run verify` | 終了コード 0。文書検査が `NG 0 件`、Go の `internal/azauth` が `ok`、E2E が `11 passed` |
| E2E のみ再実行 | `mise run verify` を一度実行した後、`npm --prefix frontend run test:e2e` | `11 passed`。失敗時の記録は `frontend/playwright-report/` と `frontend/test-results/` |

- **E2E の対象と構成**: シナリオ「ブラウザーでAzureにサインインする」の E2E は `frontend/tests/e2e/usecases/Azureへログインする/ブラウザーでAzureにサインインする.spec.ts` です。各テストが `bin\azfoundrydeck-server-e2e.exe` を一時データディレクトリと空きポートで起動します。主成功（手順1〜4と保存の確認）、トークン取得・テナント名取得の失敗、保存の失敗の3件を検証します。シナリオ「保存済みのログイン情報で自動的にログイン済みになる」の E2E は同じディレクトリの `保存済みのログイン情報で自動的にログイン済みになる.spec.ts` で、記録を置いて再起動し、復元の成功（取得中にモーダル・スピナーがなくヘッダー右が空であること、ブラウザーのサインインが呼ばれないことを含む）と復元の失敗（モーダルとエラー、記録が残ること、その後の手動ログイン）の2件を検証します。シナリオ「ヘッダーのユーザーアイコンからログアウトする」の E2E は `frontend/tests/e2e/usecases/Azureからログアウトする/ヘッダーのユーザーアイコンからログアウトする.spec.ts` で、記録を置いて再起動したログイン済みの状態から、メニューの内容、記録ファイルの削除、閉じられないログインモーダル、確認ダイアログがないこと、再起動後に自動ログインされないことを検証する主成功と、削除の失敗（メニュー内のエラー、ログイン済みのまま、再押下でも同じ、記録が残る）の2件を検証します。E2E 用ビルドには永続キャッシュがないため、永続キャッシュの削除は E2E の対象外です。
- **Home画面の初回閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/デプロイモデルを初回閲覧する.spec.ts` で、保存済みの固定ログイン情報から Home画面を開き、検索・待機・取得・完了行の削除・モデル取得・保存後の結果表示、保存内容との一致、Foundry の全文表示と省略・ツールチップを検証します。`AZFOUNDRYDECK_E2E_HOLD_FOUNDRY=1` のテストだけで外部取得を保留し、一時データディレクトリの `e2e-foundry-<段階>-release` ファイルで解放します。保存の状態遷移は実際の進捗イベントで確認します。固定 Source は3件のため、8並列の制限は実 Azure の進捗通知で確認します。`internal/foundry/service_test.go` はモデル取得の先行開始、全取得後の保存、取得失敗時の中止と既存保存内容の維持を検証します。すべて `mise run verify` で再実行できます。
- **単体テスト**: 永続キャッシュの削除（`internal/azauth/token_cache_windows_test.go`）は、実際の `%LOCALAPPDATA%\.IdentityService` に試験用の名前 `azfoundrydeck-test-<時刻>` とその `.cae` のファイルを作り、削除されることと、存在しないときの再削除が成功することを確認して後始末します。本アプリの `azfoundrydeck` には触れません。`go test ./internal/...`（`mise run verify` に含まれる）で実行されます。本番の永続キャッシュと資格情報マネージャーのエントリが実際に削除されることは、段階5で利用者が実機で確認しました。
- **Home画面の再閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/デプロイモデルを再閲覧する.spec.ts` で、保存済みファイルの一覧・2件目の Foundry の選択・全モデルを復元し、再読み込みとアプリ再起動後も維持することを検証します。外部取得を保留したまま表示できること、取得進捗イベントとモーダルが出ないこと、保存ファイルの内容・更新時刻が変わらないこと、プルダウンの全文表示・省略・ツールチップも確認します。保存ファイルの読み込みと表示は本番と同じ処理を通します。
- **Foundry変更後の初回閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/Foundryを変更し初回閲覧する.spec.ts` で、変更先が未保存の状態から、モデル取得の進捗、処理中の旧表示と操作制限、保存後の切り替え、変更前と変更先のモデルファイル、再起動後の復元、同じ Foundry を選んだ場合に取得・保存しないことを検証します。サブスクリプションと Foundry の探索は解放せず、モデル取得だけを解放して再検索しないことを確認します。Go の単体テストでは、保存済みモデルを使う際の外部取得の抑止と、保存失敗時の変更前データの保持も確認します。
- **Foundry変更後の再閲覧の検証**: `frontend/tests/e2e/usecases/デプロイモデルを閲覧する/Foundryを変更し再閲覧する.spec.ts` で、保存済みの変更先モデルへの切り替え、変更前と変更先のモデルファイルの内容・更新時刻の維持、状態ファイルだけの更新、進捗イベントとモーダルがないこと、再起動後の復元、同じ Foundry を選んだ場合に保存しないこと、親ユースケースの表示条件を検証します。全外部取得を保留して解放せず、ファイルの読み込みと保存は本番と同じ処理を通します。
- **E2E 用ビルド**: `node scripts/build.mjs server-e2e`（`build:server:e2e` タスク）が `-tags server,production,e2e` でビルドします。`e2e` タグでは、外部境界のサインイン（Entra ID・ARM）が固定のアカウント（`operator@contoso.onmicrosoft.com`、テナント名 `Contoso`）を返し、アカウント識別情報は資格情報マネージャーの代わりにデータディレクトリの `e2e-authentication-record.json` に同じ JSON で保存されます。環境変数 `AZFOUNDRYDECK_E2E_FAIL=signin`、`save`、`restore`、`logout` で失敗を注入します。ログアウトではデータディレクトリの記録ファイルを削除します。起動前にデータディレクトリへ `e2e-authentication-record.json` を置くと、起動時の復元がその記録で成功します。サインインが呼ばれるとデータディレクトリに `e2e-signin-called` を作り、`AZFOUNDRYDECK_E2E_HOLD_RESTORE=1` のときは復元が `e2e-restore-release` の作成まで応答を保留します。
- **画面確認用の起動と実処理への切り替え**: `server:review` は E2E 用ビルド（`bin\azfoundrydeck-server-e2e.exe`）を、一時フォルダーの固定データディレクトリ `AzFoundryDeck-review` に認証記録 `e2e-authentication-record.json` を置いて起動します。認証と Foundry・モデル取得の外部境界は固定応答を返しますが、初期選択・進捗の合成と通知・ファイル保存・保存済みファイルの読み込み・結果表示は本番と同じ処理です。保存済みファイルがあれば外部取得を呼ばずに復元します。固定応答の内容と進捗の接続は [UCP-1](design/UCP-1.md#デプロイモデルの初回閲覧) を参照します。Azure・資格情報マネージャー・永続キャッシュには触れません。終了は起動した端末で `Ctrl+C` です。実 Azure の取得へ切り替える場合は終了後に `mise run server` で通常ビルドを起動します。読み込み・接続・保存に失敗した場合は固定応答に切り替わらず、Home画面にエラーが表示されることを確認します。本番ビルドのログアウトは、永続キャッシュのファイル（`azfoundrydeck`、`azfoundrydeck.cae`）、資格情報マネージャーのエントリの順に削除します。どちらも既に存在しなければ成功として扱い、途中で失敗するとログイン済みのまま `LOGOUT_FAILED` を返します。
- **本番に含まれないこと**: `internal/azauth/e2e.go`、`record_store_e2e.go`、`internal/foundry/e2e.go`、`foundry_source_e2e.go` は `e2e` タグのときだけコンパイルされ、`server`、`build`、`package`、`dev` のビルドには含まれません。`go list -tags server,production -f '{{.GoFiles}}' ./internal/azauth ./internal/foundry .` にこれらのファイルが現れないことで確認できます。実 Azure へのサインイン・Foundry 一覧とモデルの取得、実ブラウザーでの認証、実資格情報マネージャーへの保存は E2E の対象外です。

- **保存するもの**: アカウント識別情報（`azidentity.AuthenticationRecord` の JSON）を Windows 資格情報マネージャーの汎用資格情報 `AzFoundryDeck:AuthenticationRecord`（ユーザー名 `AuthenticationRecord`）に、トークンを `azidentity/cache` の永続キャッシュ（名前 `azfoundrydeck`）に保存します。Foundry とモデルの保存形式・保存先は [データ設計](design/data.md#foundry-とデプロイモデル) を参照します。
- **自動ログインの確認**: 2026-10-02 に `bin\azfoundrydeck-server.exe` で次を確認しました。この端末の実際の保存済みログイン情報では、ブラウザー操作なしにテナント名とユーザーアイコンが表示されました。保存済みログイン情報がない場合はエラーなしのモーダル、不正な JSON の場合と、キャッシュにないアカウント（`homeAccountId` を変えた記録）の場合はモーダルと `LOGIN_FAILED` が表示され、保存済みの値は変わりませんでした。いずれも取得中はモーダルとスピナーを表示せず、ヘッダー右は空でした。ブラウザー認証が始まらないことは、サーバーの待ち受けポートがアプリの 34115 だけで、ブラウザー認証時に MSAL が開く応答受信用のローカルポートが現れないことで確認しました。確認後に元の保存値へ戻しました。
- **失敗時の確認**: `mise run server` で作成した `bin\azfoundrydeck-server.exe` を、接続できないプロキシ（`HTTPS_PROXY=http://127.0.0.1:9`）を設定して起動し、「Azureにログイン」を押すと、モーダル内に `LOGIN_FAILED` と理由が表示され、ボタンが再び押せる状態に戻り、ログイン済みにならないことを確認しました。ログ（`%APPDATA%\AzFoundryDeck\logs\app.jsonl`）には `operation_failed` と接続失敗の原因が記録されます。

環境構築、作業ディレクトリ、実行コマンド、設定、期待結果を明記します。自動テストと実機確認の対象・条件を示し、最新コードで再実行できる手順を維持します。モック利用時は、起動・終了、モック有効/無効の確認、実処理への切り替え手順を記述します（接続失敗時にモックへフォールバックしないことの確認を含む）。
