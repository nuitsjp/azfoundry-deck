# Azureへログインする

## 主アクター
Azure 上の Microsoft Foundry を管理する運用者（Azure アカウントを持つ個人）

## 目的
以降の Foundry とモデルデプロイの管理操作に使う Azure の認証済み状態をアプリ内で確立し、次回起動以降も再ログインなしで引き継ぐ。

## 前提
- 利用者は Microsoft Entra ID のアカウントを持ち、Azure サブスクリプションへのアクセス権がある。
- 端末に既定のブラウザーがあり、インターネットに接続できる。
- アプリは Windows デスクトップ版として動作する。

## 共通の受け入れ条件
- 認証は Azure SDK for Go（`azidentity` の `InteractiveBrowserCredential`）で行う。
- ログイン情報は OS のクレデンシャルマネージャーにのみ保存する。パスワードやトークンを画面・ログ・設定ファイルへ平文で出力しない。
- ログイン失敗時に、ダミーの成功状態へ切り替えない。

## シナリオ
- [ブラウザーでAzureにサインインする](scenarios/ブラウザーでAzureにサインインする.md)

## 実現パターン
[UCP-1. 画面操作から Go サービス経由で Azure SDK を呼ぶ](../../design/UCP-1.md)
