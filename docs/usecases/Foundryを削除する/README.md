# Foundryを削除する

## 主アクター

Azure にログイン済みの Foundry 運用者。

## 目的

選択中の Foundry を Azure 上から削除する。その Foundry が属するリソースグループに Foundry 関連のリソースしかなければ、リソースグループごと削除する。他のリソースもあれば、Foundry 関連のリソースだけを削除する。

## 前提

- 利用対象のテナントが選択されている。
- Home画面で削除対象の Foundry が選択されている。
- 削除対象のリソースグループとリソースを削除する権限がある。
- Foundry 関連のリソースは、`Microsoft.CognitiveServices/accounts`（AIServices、OpenAI）とその子リソース（プロジェクト、デプロイ）とする。ストレージや Key Vault などの接続先リソースは Foundry 関連に含めず、他のリソースとして扱う。

## シナリオ

- [Foundry関連のみのリソースグループごと削除する](scenarios/Foundry関連のみのリソースグループごと削除する.md)

## 実現パターン

[UCP-1. 画面操作から Go サービス経由で Azure SDK を呼ぶ](../../design/UCP-1.md#foundryを削除する)
