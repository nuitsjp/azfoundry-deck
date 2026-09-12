# AzFoundry Deck プロジェクト定義

本書は、プロジェクト固有の要件・仕様・設計・検証内容を管理する正本です。作業進捗と未決事項は [PLAN.md](../PLAN.md)、文書運用ルールは [文書方針](document-policy.md) を参照してください。

- **合意事項**: 仮実装の正式実装に向け、`aidd-project-template` に従い実装前の状態を整える（判断者: リポジトリ所有者、日付: 2026-09-13）。
- **仕様の位置づけ**: 第1〜4節の要件・仕様・設計候補は仮実装から整理した案であり、正式版としては未合意です。未決事項は [PLAN.md（U1〜U4）](../PLAN.md) で追跡します。

<a id="requirements"></a>
## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 目的 | Microsoft Foundry の操作対象を明示し、モデル候補・デプロイ・クォータをデスクトップ画面で確認・管理可能にする。 |
| 利用者・利用場面 | Azure CLI でサインインし、対象リソースのアクセス権を持つ開発者・運用担当者によるデプロイの確認・管理。 |
| 対象リソース候補 | Azure Public Cloud の既存 `Microsoft.CognitiveServices/accounts`（`kind` が `AIServices` または `OpenAI`）。管理プレーンのデプロイ、モデル・SKU 候補、およびアカウントのリージョンに対応する Subscription クォータ。 |
| 初期スコープ（F1） | Tenant、Subscription、Resource Group、Foundry アカウントを選択し、デプロイ一覧を表示する読取フロー。 |
| 対象外候補 | Foundry プロジェクト管理、Hub・Azure ML エンドポイント、アカウント新規作成、推論、ファインチューニング、課金取得、クォータ引き上げ申請、Sovereign Cloud。 |
| プラットフォーム | Windows 11 を主対象とする（Linux・macOS への対応は現時点では対象外）。 |

## 2. 制約・品質要求・受け入れ条件

仮実装から引き継いだ要件候補です。正式採用の範囲は仕様合意（U1）で決定します。

| 観点 | 制約・期待する振る舞い | 合否の確認方法 |
| --- | --- | --- |
| 認証と操作対象 | 認証はログイン済み Azure CLI に委譲し、アプリから自動ログインや `az account set` を行わない。各要求に操作対象を明示して渡す。 | F1 の実接続呼出しを検査し、選択対象との一致、およびログインや既定 Subscription の変更がないことを確認する。 |
| データの取扱い | トークンや API キーを画面・ログに表示・保存しない。欠損した数量を推測で 0 に置き換えない。 | 応答表示とログを検査し、欠損値を含むデータが正しく表示されることを確認する。 |
| エラーと対象切替 | 認証・権限・通信のエラーを適切に表示する。別 API・スコープ・デモへ自動切替せず、旧対象の応答を新対象に混在させない。 | F1-S3〜S6 のモック再現、および実接続での認証・権限・通信エラー発生時の挙動を検証する。 |
| モックと実処理 | モックは明示的に有効化し画面で識別可能とする。本番構成では無効化する。 | F1-S7 を確認し、モック無効時に実際のリソースを参照することを検証する。 |
| 変更操作（F3） | 対象名の完全一致確認を必須とし、作成および SKU・Capacity 変更時は費用確認を要求する。既存デプロイは上書きしない。 | F3 着手時に、未確認・既存名・競合・結果不明時の受け入れ条件を定める（U4）。 |
| 開発・配布 | タスクは `mise run ...` に統一し、Windows では WebView2 を利用する。 | 正式版のコード生成、型検査、テスト、ビルド、および Windows でのネイティブ起動を検証する（U2）。 |

<a id="features"></a>
## 3. 機能仕様と合意

<a id="f1"></a>
### F1. 操作対象の選択とデプロイ一覧

目的: 操作対象（Tenant / Subscription / Resource Group / Account）と配下のデプロイ一覧（対象名、モデル名・バージョン、SKU、Capacity、状態）を確認可能にし、後続機能の前提を確立する（検索・詳細表示は動作確認時に採否を判断）。

| シナリオ ID | 初期条件・入力 | 操作・契機 | 期待結果・観測内容 |
| --- | --- | --- | --- |
| F1-S1 | 起動直後、操作対象未選択（Tenant ID は省略または指定）。 | Subscription、Resource Group、Foundry アカウントを順次選択。 | 選択に応じた下位候補を表示（アカウント候補は `AIServices` / `OpenAI` のみ）。全選択完了までデプロイ一覧は取得しない。Tenant 省略時は Azure CLI の現行テナントを使用。 |
| F1-S2 | アカウント選択済み（デプロイあり）。 | 一覧の取得または再取得。 | 対象のデプロイ一覧を表示。読込中表示を行い、数量欠損時は不明と明示。 |
| F1-S3 | 下位選択・一覧表示済み、または要求処理中。 | 上位スコープ（Tenant、Subscription 等）を変更。 | 影響する下位選択と表示をクリア。旧要求が遅延完了しても新対象の画面に混入させない。 |
| F1-S4 | 各階層の候補またはデプロイが 0 件。 | 該当一覧を取得。 | 0 件状態を明示（エラーや読込中と区別）。次候補がない場合は下位選択に進まない。 |
| F1-S5 | Azure CLI 未導入、未ログイン、または権限不足。 | 読取を実行。 | アプリを維持し、失敗要因と対処方法を表示。自動ログインや別スコープへの無断切替は行わない。 |
| F1-S6 | 一覧取得中に通信障害・API エラー・タイムアウトが発生。 | エラー確認後、復旧契機で再取得。 | 読取失敗要因と取得できた診断情報を表示。別 API への自動フォールバックは行わず、ユーザー操作でのみ再取得。 |
| F1-S7 | モック有効／無効を設定して起動。 | F1-S1〜S6 を確認。 | 有効時は架空データであることを画面明示し Azure 呼出を行わない。無効時は実接続のみを用い、失敗時も架空データへフォールバックしない。 |

仕様合意: **未合意**（[PLAN.md の U1](../PLAN.md)）。モック提示時に合意内容を記録します。

<a id="later-features"></a>
### 後続機能の範囲候補

各機能に着手する際に詳細シナリオと設計を追記します。

| 機能 ID | 範囲案 | 前提 |
| --- | --- | --- |
| <a id="f2"></a>F2 | 選択アカウントのモデル・バージョン・SKU 候補の参照。 | F1 の操作対象を利用。F3 との連携は F3 着手時に具体化。 |
| <a id="f3"></a>F3 | デプロイの作成、SKU・Capacity の変更、削除。 | F1・F2 の完了および U4 の解消。変更対象・費用の確認、結果不明時の扱いを定義。 |
| <a id="f4"></a>F4 | 選択アカウントのリージョンにおける Subscription クォータの参照。 | F1 の操作対象を利用。欠損値表示と比較対象を定義。 |

診断情報（ログ保存先、HTTP ステータス、Azure エラーコード、request-id）の表示範囲は、各機能のエラーシナリオ策定時に具体化します。

<a id="design"></a>
## 4. 実現方法と確認した事実

<a id="references"></a>
### 参照資料と確認範囲

確認日: 2026-09-13。仮実装コード（提供ローカルディレクトリー）の静的調査に基づく情報です。外部 API や依存の現行互換性は別途検証します。

| ID | 情報源 | 確認した内容 |
| --- | --- | --- |
| R1 | [仮実装 README](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/README.md) | 製品目的、機能一覧、対象・対象外、主対象 OS、操作・認証・変更時の制約。 |
| R2 | [仮実装アーキテクチャ](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/docs/architecture.md)、[仮実装 AGENTS](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/AGENTS.md) | 単一 Go モジュール構成、画面・サービス・AzureGo の境界定義、要求単位でのスコープ指定、フォールバック禁止方針。 |
| R3 | [仮実装の検証記録](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/docs/verification.md) | 依存不要テストの成功記録、および依存取得・全体ビルド・バインディング生成・UI起動・実Azure操作が未検証であることの記録。 |
| R4 | [mise.toml](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/mise.toml)、[go.mod](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/go.mod)、[package.json](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/package.json) | Go、Wails v3、Azure SDK for Go、React、TypeScript、Vite、mise による技術構成定義。 |
| R5 | [ScopePicker](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/features/scope/ScopePicker.tsx)、[一覧取得サービス](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/internal/service/scope.go)、[画面向けの型](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/internal/service/types.go)、[useQuery](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/lib/useQuery.ts)、[App](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/App.tsx) | 操作対象の段階選択、対象 kind の絞り込み、要求スコープと一覧表示項目、非同期応答の制御処理。 |

外部の仮実装は設計検討の参考として扱い、本リポジトリの実行時依存には含めません。正式仕様や手順は本書に集約します。

### 技術構成の候補と理由

仮実装と同様に Go + Wails v3、React + TypeScript + Vite、Azure SDK for Go、mise を起点とします。採用バージョンは互換性確認（U2）を経て確定します。

F1 では単一 Go モジュールと画面向けサービスを基本構成とし、現時点で不要な HTTP サーバー、DB、Repository 層、DI コンテナー、汎用状態管理ライブラリは導入しません。

### F1 の責務と境界の案

| 担当・境界 | 責務と結果の確定点 | 障害時の扱い・対応シナリオ |
| --- | --- | --- |
| 画面 | 操作対象と状態（読込中・一覧・空・エラー）を管理。選択変更時は下位の選択・表示を解除し、最新要求の結果のみを反映。 | 旧要求の応答を破棄。読取エラーを表示し、再取得操作を受け付ける（F1-S1〜S6）。 |
| 画面向けサービス | 要求ごとの Tenant・Subscription・Resource Group・Account を受け取り入力を検証。画面用データへの変換を担当し、SDK モデルやトークンは直接公開しない。 | 要求時の対象を固定し、不正入力や外部エラーを呼出し元へ返却（F1-S1、S3、S5、S6）。 |
| AzureGo と Azure SDK | 指定対象のリソース一覧を取得。認証は Azure CLI に委譲。AzureGo に Wails 依存を持ち込まない。 | 自動ログインや別対象への切替を行わずエラーを返却（F1-S2、S5、S6）。 |
| 保存と変更 | F1 は読取専用とし、リソース変更や認証情報の永続化は行わない（画面選択はメモリ内保持）。 | F3 の変更処理における結果確定・競合制御は U4 で定義。 |

### F1 のモック設計の案

本番用の画面および入出力型定義を共用し、Azure 呼出境界のみを固定データに置き換えます。画面の別系統作成は行わず、F1-S1〜S7 の検証に必要なデータと状態遷移（対象切替、遅延応答、0 件、認証・通信エラー）を再現可能にします。モックの配置や起動手順は作成時に確定します。

<a id="commands"></a>
## 5. 実行・切り替え・検証手順

### 現時点での文書確認

本リポジトリのルートで PowerShell を用いて以下を実行し、文書の存在と整合性を確認します。

```powershell
Get-Content -Raw AGENTS.md
Get-Content -Raw docs/document-policy.md
Get-Content -Raw PLAN.md
Get-Content -Raw docs/project.md
git status --short
git diff --check
```

### 実装時に整備する手順

F1 のモック作成および実処理接続に伴い、以下の実行手順を確定・記録します。

- **モック環境**: 依存取得、コード生成、起動・終了、および F1-S1〜S7 の状態再現手順
- **実接続環境**: 対象リソースの指定、読取結果の照合、テスト・型検査・ビルド、Windows でのネイティブ起動、および通信・認証エラー時の挙動確認

<a id="verification"></a>
## 6. 検証結果

### 文書整備の検証

対象: 初期文書一式 / 環境: Windows (PowerShell) / 確認日: 2026-09-13

| 確認内容 | 方法・根拠 | 結果 |
| --- | --- | --- |
| 配布文書の配置 | テンプレート配下とのファイル一覧比較。 | 対象 Markdown 7 ファイルの配置を確認。 |
| 標準・行動指針・ライセンスの保持 | 配布元とのハッシュ（SHA-256）比較。 | AGENTS.md、標準2文書、LICENSE が配布元と一致。 |
| リンクと記入欄 | Markdown リンク抽出によるパスおよびアンカー存在確認。テンプレート記入欄の残存確認。 | ファイル参照・アンカーがすべて有効。未記入欄なし。 |
| 文書差分の空白エラー | `git diff --check` による空白エラー検査。 | 7 ファイルすべてで空白エラーなし。 |
| 仕様・進捗の整合 | 第1〜4節と PLAN.md の照合。 | 機能 ID（F1〜F4）と課題 ID（U1〜U4）が整合し、未完了項目が正確に管理されていることを確認。 |

### アプリケーションの検証

正式版のコードおよびモックは未作成のため、F1-S1〜S7 の実行、依存取得、ビルド、実 Azure 接続等は未検証です。実装後に検証結果と証跡を本節に記録します。残る確認事項は [PLAN.md](../PLAN.md) を参照してください。
