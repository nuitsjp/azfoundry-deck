# インストーラーをReleasesへ発行する

## 主アクター
アプリのリリース担当者。

## 目的
指定したバージョンのWindows用インストーラーをGitHub Releasesへ公開し、利用者がダウンロードできるようにする。

## 前提
- 公開先は `nuitsjp/azfoundry-deck`。
- 既存のWindows CIとNSIS生成処理を使用する。
- 公開対象は製品側のWindows x64版。
- バージョンの正本は `build/app.json` とし、タグ・アプリ・インストーラーのバージョンを一致させる。

## 共通の受け入れ条件
- タグは `vX.Y.Z` 形式とする。
- バージョンは `X.Y.Z` 形式で、各要素は0〜65535の範囲とする。
- 検証とビルドが成功した場合だけ、通常のReleaseとして公開する。
- Releaseには `azfoundrydeck-X.Y.Z-amd64-setup.exe` を添付する。

## シナリオ
- [バージョンタグを付与してインストーラーを公開する](scenarios/バージョンタグを付与してインストーラーを公開する.md)

## 実現パターン
[UCP-2. ビルド処理とGitHub Actionsによるリリース](../../design/UCP-2.md)
