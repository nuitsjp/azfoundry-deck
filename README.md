# Wails テンプレート

新規プロジェクトは配布元ルートの `mise run init:wails <新しい出力先> --name Company.Product` で生成します。`--name` は英字で始まる ASCII 英数字とアンダースコアをドットで区切った製品名です。生成先ルートには Wails・React・Go の実行可能なアプリ一式が入り、`build/app.json` のアプリ ID・表示名・実行ファイル名、Go モジュール名、npm パッケージ名が製品名に合わせられます。メモ編集・CSV取り込み・アプリ内更新の参照実装は `reference/` に残します。製品の現行仕様はルートの `docs/` に記述し、参照実装のユースケース・シナリオと設計は `reference/docs/` に保持します。

生成先ルートで `mise trust`、`mise run setup`、`mise run setup:browser` を実行し、`mise run dev` で製品アプリを起動するか `mise run verify` で検証します。サンプルは `reference/` へ移動して同じコマンドを実行します。製品と参照アプリはアプリ ID が異なるため、保存データと多重起動の判定を共有しません。`.github/workflows/windows.yml` は変更パスに応じて製品と `reference/` を選び、検証とビルドを `ci` タスクでまとめて実行します。共通ファイルの変更時と手動実行時は両方を実行し、インストーラーを artifact として保存します。

サンプルの構成、実行手順、配布・更新の設定と、製品固有の実装へ置き換える箇所は [reference/README.md](reference/README.md) から参照してください。共通の `AGENTS.md`、標準、文書検査スクリプトは生成時に `template/` からルートへ配置します。生成時に依存取得やビルドは行わず、既存の出力先は上書きしません。`wails-template/` は単体では実行せず、生成先で開発・検証します。
