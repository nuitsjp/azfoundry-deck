# AzFoundry Deck プロジェクト定義

本書は、プロジェクト固有の要件・仕様・設計・検証内容を管理する正本です。作業進捗と未決事項は [PLAN.md](../PLAN.md)、文書運用ルールは [文書方針](document-policy.md) を参照してください。

- **合意事項**: 仮実装の正式実装に向け、`aidd-project-template` に従って開発する。加えて、[F1 のヒアリング結果](#f1-agreement) と [モック用技術構成](#mock-stack) を合意済み（判断者: リポジトリ所有者、日付: 2026-09-13）。
- **仕様の位置づけ**: 合意済みの要件と技術構成を除き、仮実装由来の制約・後続機能および詳細設計は案です。F1 のシナリオはヒアリングを反映したモック確認用の案であり、動作するモックによる仕様合意（U1）は未完了です。未決事項は [PLAN.md（U1〜U4）](../PLAN.md) で追跡します。

<a id="requirements"></a>
## 1. 目的と範囲

| 項目 | 内容 |
| --- | --- |
| 目的 | Microsoft Foundry の操作対象を明示し、モデル候補・デプロイ・クォータをデスクトップ画面で確認・管理可能にする。 |
| 利用者・利用場面（想定） | Azure CLI でサインインし、対象リソースのアクセス権を持つ開発者・運用担当者によるデプロイの確認・管理。 |
| 対象リソース候補 | Azure Public Cloud の既存 `Microsoft.CognitiveServices/accounts`（`kind` が `AIServices` または `OpenAI`）。管理プレーンのデプロイ、モデル・SKU 候補、およびアカウントのリージョンに対応する Subscription クォータ。 |
| 初期スコープ（F1） | 複数テナントにまたがる全対象アカウントのデプロイ名とレート制限を一覧で確認し、属性で絞り込む読取フロー。対象範囲と表示要件は [F1 のヒアリング結果](#f1-agreement) を正本とする。 |
| 対象外候補 | Foundry プロジェクト管理、Hub・Azure ML エンドポイント、アカウント新規作成、推論、ファインチューニング、課金取得、クォータ引き上げ申請、Sovereign Cloud。 |
| プラットフォーム | Windows 11 を主対象とする（Linux・macOS への対応は現時点では対象外）。 |

## 2. 制約・品質要求・受け入れ条件

仮実装から引き継いだ制約の案です。今回合意した F1 の要件は第3節に記録し、残る制約の正式採用は仕様合意（U1）で決定します。

| 観点 | 制約・期待する振る舞い | 合否の確認方法 |
| --- | --- | --- |
| 認証と操作対象 | 認証はログイン済み Azure CLI に委譲し、アプリから自動ログインや `az account set` を行わない。各要求に操作対象を明示して渡す。 | F1 の実接続呼出しを検査し、選択対象との一致、およびログインや既定 Subscription の変更がないことを確認する。 |
| データの取扱い | トークンや API キーを画面・ログに表示・保存しない。欠損した数量を推測で 0 に置き換えない。 | 応答表示とログを検査し、欠損値を含むデータが正しく表示されることを確認する。 |
| エラーと再取得 | 認証・権限・通信のエラーを適切に表示する。別 API・スコープ・デモへ自動切替せず、旧取得処理の応答で新しい結果を上書きしない。部分失敗時の表示は第3節の合意に従う。 | F1-S3〜S6 のモック再現、および実接続での認証・権限・通信エラー発生時の挙動を検証する。 |
| モックと実処理 | モックは明示的に有効化し画面で識別可能とする。本番構成では無効化する。 | F1-S7 を確認し、モック無効時に実際のリソースを参照することを検証する。 |
| 変更操作（F3） | 対象名の完全一致確認を必須とし、作成および SKU・Capacity 変更時は費用確認を要求する。既存デプロイは上書きしない。 | F3 着手時に、未確認・既存名・競合・結果不明時の受け入れ条件を定める（U4）。 |
| 開発・配布 | タスクは `mise run ...` に統一し、Windows では WebView2 を利用する。 | 正式版のコード生成、型検査、テスト、ビルド、および Windows でのネイティブ起動を検証する（U2）。 |

<a id="features"></a>
## 3. 機能仕様と合意

<a id="f1"></a>
### F1. アカウント横断のデプロイ一覧と絞り込み

目的: 複数アカウントにデプロイされたモデルのデプロイ名とレート制限の設定値を、一つの一覧で確認・比較可能にする。

<a id="f1-agreement"></a>
#### ヒアリングで合意した要件

判断者: リポジトリ所有者。日付: 2026-09-13。根拠: 本タスクで一問ずつ確認した回答。仮実装の画面・操作を引き継ぐ前提は取り下げ、ゼロから検討する。確認済みの利用状況は、テナントをまたいで普段約10アカウントを利用すること。

| 観点 | 合意内容 |
| --- | --- |
| 対象と初期表示 | アクセス権のある全 Foundry アカウントのデプロイを、テナントをまたいで一つの一覧に表示する。普段使う約10アカウントだけには限定しない。 |
| 性能による例外 | 全件表示が重すぎる場合は、既定の表示対象を単一アカウントとすることを許容する。実際の取得時間を確認して判断する。許容時間と既定範囲は未確定（U3）。エラー時に対象を自動縮小する意味ではない。 |
| 主な確認項目 | デプロイ名と設定 TPM（1分あたりのトークン数）を必須の表示項目とし、設定 RPM（1分あたりのリクエスト数）も取得できる場合に表示する。 |
| 絞り込み | テナント、サブスクリプション、リージョン、アカウント、モデル、デプロイ名の6属性を対象にする。 |
| 更新契機 | 起動時に取得し、その後は更新ボタンで手動取得する。定期的な自動更新は行わない。 |
| 部分失敗 | 成功したデプロイを一覧に表示する。取得に失敗したアカウントと理由は、同一画面内の別領域に表示する。失敗が残る間は一覧が全件を網羅していないことを明示する。 |

一覧の行はデプロイ単位、取得エラーはアカウント単位で扱う。異なるアカウントに同名デプロイが存在しても区別できるよう、各行に所属テナント・サブスクリプション・アカウントとリソース識別子を保持する。TPM を SKU の Capacity で代用しない。値を取得できない場合の表示と、各モデル・SKU で設定値を正しく取得できるかは、モック確認および実接続検証で確かめる（U1、U3）。

#### モックで確認するシナリオ案

旧案の段階選択と単一テナント表示を、上記の合意に合わせて更新した。シナリオ ID は維持する。絞り込みの操作部品・一致条件、列の配置、取得中の表示はモックで具体化する。

| シナリオ ID | 初期条件・入力 | 操作・契機 | 期待結果・観測内容 |
| --- | --- | --- | --- |
| F1-S1 | 起動直後。複数テナント・サブスクリプションに約10アカウントがあり、同名デプロイも存在する。 | 起動時の取得。 | 全対象アカウントを探索し、デプロイを一つの一覧に表示する。取得中を示し、所属の違いを識別できる。対象探索の網羅性は実接続時に確認する（U3）。 |
| F1-S2 | デプロイあり。TPM・RPM が得られる行と、値が欠損する行を含む。 | 初回取得結果の表示、または更新ボタンの押下。 | デプロイ名と設定 TPM を確認でき、RPM は取得可能なら表示する。欠損は不明と示し、0 や Capacity に置き換えない。定期更新を行わない。 |
| F1-S3 | 一覧表示済み、または手動更新中。 | 6属性の絞り込み条件を変更・解除する。 | 条件に合うデプロイのみ表示し、解除で取得済みの全対象に戻る。旧取得処理の遅延応答で最新結果を上書きせず、最新の絞り込み条件を適用する。 |
| F1-S4 | デプロイが0件、絞り込み結果が0件、または取得失敗により表示できる行がない。 | 一覧取得または絞り込み。 | 取得完了後の0件、条件に一致する行の不在、読込中、取得失敗を区別する。失敗により全件を確認できていない状態を、取得成功の0件として扱わない。 |
| F1-S5 | Azure CLI 未導入、未ログイン、または対象探索時の認証・権限エラー。 | 起動時または手動で読取を実行。 | アプリを維持し、失敗要因と対処方法を表示する。アカウント特定前の探索失敗は、判明しているテナント等の対象をエラー領域に示し、全件確認済みとしない。自動ログインや別スコープへの無断切替は行わない。 |
| F1-S6 | 複数アカウントの取得中、一部に権限不足・通信障害・API エラー・タイムアウトが発生する。 | 成功・失敗結果の表示後、復旧して更新ボタンを押す。 | 成功したデプロイの一覧と、失敗アカウント・理由の領域を同一画面に表示する。一覧が不完全であることを明示し、全件取得が成功した更新で失敗表示を解消する。別 API へ自動切替しない。 |
| F1-S7 | モック有効／無効を設定して起動。 | F1-S1〜S6 を確認。 | 有効時は架空データであることを画面明示し Azure 呼出を行わない。無効時は実接続のみを用い、失敗時も架空データへフォールバックしない。 |

仕様合意: **要件のヒアリングは合意済み。動作するモックによる仕様合意は未完了**（[PLAN.md の U1](../PLAN.md)）。モック提示時に対象版・シナリオと動作合意を記録する。

<a id="later-features"></a>
### 後続機能の範囲候補

各機能に着手する際に詳細シナリオと設計を追記します。

| 機能 ID | 範囲案 | 前提 |
| --- | --- | --- |
| <a id="f2"></a>F2 | 選択アカウントのモデル・バージョン・SKU 候補の参照。 | F1 で保持する所属アカウント情報を利用。F2 への操作入口は着手時に具体化し、F3 との連携は F3 着手時に具体化。 |
| <a id="f3"></a>F3 | デプロイの作成、SKU・Capacity の変更、削除。 | F1・F2 の完了および U4 の解消。変更対象・費用の確認、変更直前の候補の再検証、結果不明時の扱いを定義。 |
| <a id="f4"></a>F4 | 選択アカウントのリージョンにおける Subscription クォータの参照。 | F1 で保持する所属アカウント情報を利用。操作入口、欠損値表示と比較対象は着手時に定義。 |

診断情報（ログ保存先、HTTP ステータス、Azure エラーコード、request-id）の表示範囲は、各機能のエラーシナリオ策定時に具体化します。

<a id="design"></a>
## 4. 実現方法と確認した事実

<a id="references"></a>
### 参照資料と確認範囲

確認日: 2026-09-13。提供された仮実装の静的調査であり、版・コミットは未特定、動作の再検証は未実施です。外部 API の現行仕様、依存の入手性・互換性は別途検証します。

仮実装由来の出典は目的・範囲と後続機能が R1、制約が R1〜R3、当初の F1 案が R1・R2・R5 です。現在の F1 の要件は [ヒアリング結果](#f1-agreement) に基づきます。開発者・運用担当者という利用者像は機能と認証方式からの想定です。

| ID | 情報源 | 確認した内容 |
| --- | --- | --- |
| R1 | [仮実装 README](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/README.md) | 製品目的、機能一覧、対象・対象外、主対象 OS、操作・認証・変更時の制約。 |
| R2 | [仮実装アーキテクチャ](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/docs/architecture.md)、[仮実装 AGENTS](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/AGENTS.md) | 単一 Go モジュール構成、画面・サービス・AzureGo の境界定義、要求単位でのスコープ指定、フォールバック禁止方針。 |
| R3 | [仮実装の検証記録](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/docs/verification.md) | 依存不要テストの成功記録、および依存取得・全体ビルド・バインディング生成・UI起動・実Azure操作が未検証であることの記録。 |
| R4 | [mise.toml](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/mise.toml)、[go.mod](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/go.mod)、[package.json](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/package.json) | Go、Wails v3、Azure SDK for Go、React、TypeScript、Vite、mise による技術構成定義。 |
| R5 | [ScopePicker](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/features/scope/ScopePicker.tsx)、[一覧取得サービス](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/internal/service/scope.go)、[画面向けの型](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/internal/service/types.go)、[useQuery](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/lib/useQuery.ts)、[App](C:/Users/atsus/Downloads/AzFoundry-Deck-mise/azfoundry-deck/frontend/src/App.tsx) | 操作対象の段階選択、対象 kind の絞り込み、要求スコープと一覧表示項目、非同期応答の制御処理。 |

外部の仮実装は設計検討の参考として扱い、本リポジトリの実行時依存には含めません。正式仕様や手順は本書に集約します。

<a id="mock-stack"></a>
### 技術構成と選定理由

既存の画面と処理境界を検討材料にできるため、仮実装と同じ Go + Wails v3、React + TypeScript + Vite、Azure SDK for Go、mise を起点とします。2026-09-13、U2 のモック用構成として次を選定し、依存取得・コード生成・型検査・Windows ビルドを確認しました。

モック用構成の採用合意: リポジトリ所有者、2026-09-13。本タスクで Wails v3 がベータ版であることと検証範囲を提示し、「OK」の回答を得た。画面・操作は [F1 の要件](#f1-agreement) に従ってゼロから検討する。

| 対象 | 固定バージョン | 定義先 |
| --- | --- | --- |
| Go / Node.js | 1.26.8 / 24.20.0 | [mise.toml](../mise.toml) |
| Wails 本体 / CLI | v3.0.0-beta.6 | [go.mod](../go.mod)、[mise.toml](../mise.toml) |
| Wails フロントエンドランタイム | 3.0.0-beta.5 | [package.json](../frontend/package.json) |
| React / React DOM、および各型定義 | 19.2.0 | [package.json](../frontend/package.json) |
| TypeScript / Vite | 5.9.3 / 8.0.16 | [package.json](../frontend/package.json) |

Go と Node.js はインストール済みの版を使用し、Wails 本体・CLI は同一版に固定しました。ランタイムの beta.5 は [Wails beta.6 内の公式定義](https://github.com/wailsapp/wails/blob/v3.0.0-beta.6/v3/internal/runtime/desktop/%40wailsio/runtime/package.json) と一致します。Wails はベータ版のため、自動更新せず、変更時にコード生成とビルドを再検証します。

Vite は仮実装候補の 8.0.13 で `npm audit` が高深刻度1件を報告したため、[公式の修正版 8.0.16](https://github.com/vitejs/vite/security/advisories/GHSA-fx2h-pf6j-xcff) に更新しました。新しい画面ライブラリやモックツールは追加していません。現時点ではホットリロードを必要としないため、React 用 Vite プラグインも追加せず、Vite 標準の TSX 変換を使用します。

Azure SDK は今回の固定データ検証には不要なため未導入です。実接続用の採用版と互換性は U2 の残件とし、API・認証・権限の動作検証は U3 で扱います。

F1 では単一 Go モジュールと画面向けサービスを基本構成とし、現時点で不要な HTTP サーバー、DB、Repository 層、DI コンテナー、汎用状態管理ライブラリは導入しません。

<a id="rate-limit-source"></a>
### レート制限の外部仕様の確認

2026-09-13、Microsoft の [Deployments List（API 2025-06-01）](https://learn.microsoft.com/en-us/rest/api/microsoftfoundry/accountmanagement/deployments/list?view=rest-microsoftfoundry-accountmanagement-2025-06-01) を確認しました。応答には `sku.capacity`、`properties.rateLimits`、`properties.callRateLimit` が定義されていますが、直接 `tpm` / `rpm` という名前の項目はありません。汎用の制限情報をどの設定値として扱うかは、このスキーマだけでは確定できません。

[クォータの公式資料](https://learn.microsoft.com/en-us/azure/foundry/openai/how-to/quota) は、容量単位と TPM/RPM の対応がモデルにより異なると説明しています。また、[Provisioned の容量](https://learn.microsoft.com/en-us/azure/foundry/openai/concepts/provisioned-throughput) は PTU で扱われます。このため、Capacity の生値を TPM として表示したり、全モデル・SKU に一律の換算を適用したりしません。採用 API・SDK の確定と、各モデル・SKU の取得値を Portal の設定値に照合する検証は U2・U3 の残件です。今回の確認は文書調査のみで、Azure への接続や値の取得は未実施です。

### F1 の責務と境界の案

| 担当・境界 | 責務と結果の確定点 | 障害時の扱い・対応シナリオ |
| --- | --- | --- |
| 画面 | アカウント横断のデプロイ一覧、6属性の絞り込み、取得状況、独立したエラー領域を管理する。最新取得結果に現在の絞り込み条件を適用する。 | 旧取得処理の応答を破棄し、部分失敗時も成功データを表示する。一覧の不完全性を示して手動更新を受け付ける（F1-S1〜S6）。 |
| 画面向けサービス | 対象探索とアカウントごとの取得結果を取りまとめ、デプロイ行と取得失敗情報を分けて返す。画面用データへ変換し、SDK モデルやトークンは直接公開しない。 | 各取得要求の所属テナント・サブスクリプション・アカウントを固定し、失敗範囲を保持する。アカウント特定前の探索失敗も欠落させない（F1-S1、S5、S6）。 |
| AzureGo と Azure SDK | テナント・サブスクリプション・対象アカウントを探索し、各アカウントのデプロイを全ページ取得する。各要求に操作対象を明示し、認証は Azure CLI の資格情報取得機構に委譲する。AzureGo に Wails 依存を持ち込まない。 | 列挙の網羅性、レート制限値の取得可否と全件取得の所要時間は U3 で検証する。自動ログインや失敗時の別対象への切替を行わない（F1-S1、S2、S5、S6）。 |
| 保存と変更 | F1 は読取専用とし、リソース変更や認証情報の永続化は行わない（絞り込み状態はメモリ内保持の案）。 | F3 の変更処理における結果確定・競合制御は U4 で定義。 |

### F1 のモック設計の案

本番用の画面・入出力処理・型定義を共用し、Azure 呼出境界のみを固定データに置き換えます。仮実装の見た目・操作は引き継ぐ前提にせず、F1-S1〜S7 に必要な複数テナント・約10アカウントのデータ、同名デプロイ、設定 TPM/RPM と欠損値、6属性の絞り込み、手動更新、遅延応答、0件、全体失敗・部分失敗を再現可能にします。実画面はモックから実処理への接続後も共用し、モックの配置や起動手順は作成時に確定します。モックの表示速度を実 Azure の取得性能の証拠として扱いません。

U2 のひな形は、[Go サービス](../internal/service/compatibility.go) の固定データを、[生成した TypeScript](../frontend/bindings/github.com/nuitsjp/azfoundry-deck/internal/service/compatibilityservice.ts) を通じて [React 画面](../frontend/src/App.tsx) へ渡す最小構成です。数量欠損は Go のポインターから `number | null` として生成します。データと画面には架空データであることを明示し、起動には `AZFOUNDRY_MOCK=1` が必要です。このひな形は F1 の画面作成の起点とし、互換性確認専用のサービス・データは F1 の契約確定時に置き換えます。F1-S1〜S7 のモック完成や仕様合意を意味しません。

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
git diff HEAD --check
```

リンク先のファイル・アンカーと、機能・課題 ID の対応も確認します。`git diff HEAD --check` はステージ済み・未ステージの変更を検査しますが、未追跡ファイルは別途確認が必要です。

### モック用構成の準備とビルド（確認済み）

Windows 11 / PowerShell 7 / mise 2026.8.6 で確認しました。リポジトリのルートで実行します。

```powershell
mise trust
mise install
mise run setup
mise run build
```

`setup` は [go.sum](../go.sum) と [package-lock.json](../frontend/package-lock.json) に基づき依存を取得します。`build` は `bindings`、`check` の順に実行した後、フロントエンドと Windows 実行ファイルを生成します。`bindings` と `check` は個別に `mise run ...` でも実行できます。初回の Go 埋め込み処理に必要な空ファイルは `bindings` が生成し、Vite のビルドで置き換えます。

出力先は `bin/azfoundry-deck.exe` です。生成したバインディングは保存し、手作業で編集しません。`frontend/dist`、`frontend/node_modules`、`bin` は生成物として Git 管理対象外です。

### 互換性確認用画面の操作手順（画面操作は未検証）

```powershell
mise run mock
```

`mock` はビルド後に `AZFOUNDRY_MOCK=1` を設定して起動します。「固定データを取得」を押すと架空のデプロイ一件と、数量欠損を表す「不明」を表示する設計です。終了はウィンドウを閉じます。実接続は未実装であり、モック指定なしではエラー終了します。

F1 全シナリオの起動・状態再現手順はモック作成時に整備します。実処理接続時には、モック無効化、対象リソース指定、読取結果照合、実接続用のテスト・ビルド、Windows ネイティブ起動、およびエラー時にモックへ切り替わらないことを検証します。

<a id="verification"></a>
## 6. 検証結果

### 文書整備の検証

対象: 簡潔化前の初期文書（コミット `133ca0e`） / 環境: Windows (PowerShell) / 確認日: 2026-09-13。以下は導入時の記録であり、変更後の文書と配布元の一致を示すものではありません。

| 確認内容 | 方法・根拠 | 結果 |
| --- | --- | --- |
| 配布文書の配置 | テンプレート配下とのファイル一覧比較。 | 対象 Markdown 7 ファイルの配置を確認。 |
| 標準・行動指針・ライセンスの保持 | 配布元とのハッシュ（SHA-256）比較。 | AGENTS.md、標準2文書、LICENSE が配布元と一致。 |
| リンクと記入欄 | Markdown リンク抽出によるパスおよびアンカー存在確認。テンプレート記入欄の残存確認。 | ファイル参照・アンカーがすべて有効。未記入欄なし。 |
| 文書差分の空白エラー | 配布元と採用後の各ファイルに `git -c core.autocrlf=false diff --no-index --check` を実行。 | 7 ファイルすべてで空白エラーなし。 |
| 仕様・進捗の整合 | 第1〜4節と PLAN.md の照合。 | 機能 ID（F1〜F4）と課題 ID（U1〜U4）が整合し、未完了項目が正確に管理されていることを確認。 |

簡潔化後の復元確認（2026-09-13、Windows / PowerShell）: 版 `a7cd666` からの文書復元差分を確認。Markdown 7 文書のファイル参照67件（アンカー指定29件）、機能・課題 ID、未記入欄、LICENSE の一致、および `git -c core.autocrlf=false diff HEAD --check` は合格。標準・行動指針の本文には、文書方針に記録した表現上の差分があります。

ヒアリング反映の確認（2026-09-13、Windows / PowerShell）: 要件・シナリオ・合意状態を更新した作業ツリーについて、Markdown 7 文書のファイル参照95件、明示アンカー38件、F1-S1〜S7 各1行の存在、および `git -c core.autocrlf=false diff HEAD --check` は合格。U2 検証時のソース17ファイルは保存済み SHA-256 と一致し、今回の文書更新ではアプリケーションを変更していません。アプリケーションのテスト・ビルドは再実行していません。

### アプリケーションの検証

対象: U2 互換性確認用ひな形（入力ファイルの SHA-256 は [対象ファイル記録](verification/u2-inputs.sha256)）。確認日: 2026-09-13。環境: Windows 11 Pro 10.0.26200 / amd64、PowerShell 7.6.5、mise 2026.8.6、技術構成は第4節の固定版。Azure 呼出を含まない構成の検証です。

| 確認内容 | 結果と証跡 |
| --- | --- |
| 依存取得・固定 | `mise install`、`mise run setup` が成功。Go の版とチェックサム、npm の lockfile を保存。`go mod verify` は `all modules verified`。[実行ログ](verification/u2-build.txt) |
| バインディング生成 | Wails beta.6 が1サービス・1メソッド・1モデルを生成。数量欠損が `number \| null` になることを生成ファイルで確認。[実行ログ](verification/u2-build.txt) |
| Go・TypeScript 検査とビルド | `go vet ./...`、`tsc --noEmit`、Vite build、Windows向け `go build -tags production` がすべて合格。[実行ログ](verification/u2-build.txt) |
| 生成物がない状態からの再現 | 別ディレクトリへソースと依存定義のみをコピーし、バインディング・dist・node_modules・bin がない状態から `setup` と `build` が合格。インストール済みツールと依存キャッシュは共用。[再現ログ](verification/u2-clean-build.txt) |
| npm 依存監査 | Vite 8.0.13 では高深刻度1件で終了コード1。8.0.16 へ更新後は0件で終了コード0。[更新前](verification/u2-audit-before.json)、[更新後](verification/u2-audit-after.json) |
| ネイティブ起動の前提 | `wails3 doctor` が WebView2 153.0.4234.32 を検出し、`No issues found`。ウィンドウ起動や画面操作の成功を示すものではない。 |

初期の環境確認では Go shim の使用版未設定と、PowerShell の `mise` ラッパーによる `exec` の引数エラーが発生しました。前者は本リポジトリの `mise.toml` による版固定、後者は診断時に `mise.exe exec` を直接使用して解消しました。通常の `mise run ...` の手順とは分けて扱います。

F1-S1〜S7 の動作テスト、Windows のネイティブ起動と画面からの固定データ取得、Azure SDK の互換性、実 Azure 接続は未検証です。今回は依存・生成コード・ビルドの技術検証に限定し、機能テストは未作成・未実施です。仮実装の過去の結果を正式版の実績として転記しません。残件は [PLAN.md](../PLAN.md) の U1〜U4 で管理します。
