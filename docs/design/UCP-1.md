# UCP-1. 画面操作から Go サービス経由で Azure SDK を呼ぶ

適用条件と関与コンテナは [アーキテクチャの一覧](../architecture.md#patterns) を参照します。パターンからの逸脱は対象 UC ごとに本書へ記録します。図は主成功系列を役割名で示します。

| 役割 | 責務 | 実装パス（段階4完了時に記入） |
| --- | --- | --- |
| 画面 | ボタンと状態（未ログイン・サインイン待ち・ログイン済み・失敗）の表示 | `frontend/src/usecases/azure-login/AzureLogin.tsx`、`frontend/src/features/auth/queries.ts` |
| Go サービス | ログインの実行、ログイン状態の保持、アカウント識別情報の資格情報マネージャーへの保存 | `internal/azauth/service.go`、`internal/azauth/credential_manager.go` |
| Azure SDK | `azidentity` によるブラウザー認証、トークン取得と永続キャッシュへの保存、ARM からのテナント名取得 | `internal/azauth/browser.go` |

```mermaid
sequenceDiagram
  participant U as 画面
  participant S as Go サービス
  participant K as Azure SDK
  U->>S: ログインを要求
  S->>K: ブラウザー認証とトークン取得
  K->>K: トークンを永続キャッシュに保存
  K-->>S: アカウント識別情報とテナント名
  S->>S: アカウント識別情報を OS に保存
  S-->>U: アカウント名とテナント名
```

- 整合性: 状態更新の主体 Go サービス / 結果確定点 トークン取得（永続キャッシュへの保存を含む）、テナント名取得、アカウント識別情報の保存のすべての成功時 / 障害時の停止・継続 いずれかが失敗すれば未ログインのままにし、理由を画面へ返す
- モックに置き換える境界と合成点: なし。E2E 用ビルド（`e2e` タグ）に限り、`internal/azauth/e2e.go` と `record_store_e2e.go` で Azure SDK 側のサインインと保存先を差し替える
