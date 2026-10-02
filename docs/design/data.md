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

一覧と選択、現在表示するモデルを一つの JSON として保存します。このファイルの項目と形式は Foundry の変更後も同じです。認証情報やトークンは保存しません。

Home画面で保存済みの Foundry 一覧・選択済み Foundry・モデル一覧が存在しない場合は、Azure から取得して保存します。保存済みファイルが存在して読み込みに成功した場合は、保存された一覧・選択・モデルを復元し、Azure からの取得と再保存は行いません。

### Foundry ごとのモデル

同じデータフォルダーの `foundry-models/` に、Foundry ごとの全モデルを UTF-8 の JSON 配列として保存します。ファイル名は Foundry のリソース ID の UTF-8 バイト列を SHA-256 でハッシュ化した小文字の16進数に `.json` を付けたものです。配列の各要素は上表の `deployments[]` と同じ `id`、`deploymentName`、`modelName`、`version` を持ちます。

初回取得では、選択先の全モデルを Foundry 別ファイルに保存した後、`foundry-state.json` を保存します。Foundry を変更する場合は、変更前のモデルを保持し、変更先のファイルが存在して読み込みに成功すればその内容を使用します。変更先のファイルが存在しない場合だけ全モデルを取得して保存します。保存済みの変更先モデルを使用する場合、そのモデルファイルは再保存しません。いずれの場合も、変更先の選択とモデルを `foundry-state.json` に保存します。現在と同じ Foundry を選んだ場合は保存しません。

各ファイルは同じフォルダーの一時ファイルへ書き込み、同期とクローズを完了した後に目的のファイルを置き換えます。複数ファイルは上記の順に個別に保存し、取得や必要な保存に失敗した場合は正常な閲覧結果を返しません。読み込みや JSON の復元に失敗した場合は取得へのフォールバックを行いません。
