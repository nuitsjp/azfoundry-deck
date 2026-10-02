# データ設計

保存形式と現在のテーブル設計の正本です。DBを使う場合は以下にER図とテーブル定義を記載し、使わない場合はその旨と実際の保存形式を記します。変更範囲と論点は [設計標準](../standards/design-and-documentation.md#architecture-method) に従って会話で提示します。

DB は使いません。アカウント識別情報は OS のクレデンシャルマネージャーに、トークンは Azure SDK の永続キャッシュに保存し、ログイン状態はメモリで保持します。

## Foundry とデプロイモデル

アプリのデータフォルダーに `foundry-state.json` を UTF-8 の JSON で保存します。既定のフォルダーは `%APPDATA%\AzFoundryDeck` です。`WAILS_DATA_DIR` を指定した場合はそのフォルダーを使います。

| 項目 | 型 | 内容 |
| --- | --- | --- |
| `foundries` | 配列 | 取得したすべての Foundry |
| `foundries[].id` | 文字列 | Foundry の Azure リソース ID |
| `foundries[].name` | 文字列 | Foundry の名称 |
| `foundries[].subscriptionName` | 文字列 | 所属サブスクリプション名 |
| `foundries[].resourceGroupName` | 文字列 | 所属リソースグループ名 |
| `selectedFoundryId` | 文字列 | `foundries[].id` のいずれかを参照する選択済み Foundry |
| `deployments` | 配列 | 選択された Foundry のすべてのデプロイ済みモデル |
| `deployments[].id` | 文字列 | デプロイの Azure リソース ID |
| `deployments[].deploymentName` | 文字列 | デプロイ名 |
| `deployments[].modelName` | 文字列 | モデル名 |
| `deployments[].version` | 文字列 | モデルのバージョン |

一覧と選択、モデルを一つの JSON として保存します。全取得成功後に同じフォルダーの一時ファイルへ書き込み、書き込みの完了後に `foundry-state.json` を置き換えます。取得や保存が失敗した場合は正常な初回閲覧結果を返しません。認証情報やトークンはこのファイルに含めません。

Home画面で保存済みの Foundry 一覧・選択済み Foundry・モデル一覧が存在しない場合は、Azure から取得して保存します。保存済みファイルが存在して読み込みに成功した場合は、保存された一覧・選択・モデルを復元し、Azure からの取得と再保存は行いません。
