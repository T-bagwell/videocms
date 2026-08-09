# 🎬 VideoCMS

> **セルフホスト型ビデオリソース管理システム** — Go · React · PostgreSQL

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14-4169E1?logo=postgresql&logoColor=white)
![i18n](https://img.shields.io/badge/i18n-5%20languages-8A2BE2)

**言語:** [English](README.md) | [中文](README.zh-CN.md) | 日本語

サーバーのディスク上のフォルダを、閲覧・検索可能なビデオライブラリに変えるシステムです。
一度スキャンすれば、すべての動画にポスター・メタデータ・視聴履歴・お気に入り・プレイリストが付き、
連番ファイルは自動でドラマとしてグループ化されます。

---

## 目次

- [機能](#機能)
- [スクリーンショット](#スクリーンショット)
- [ドキュメント](#ドキュメント)
- [クイックスタート](#クイックスタート)
- [LAN / スマホアクセス](#lan--スマホアクセス)
- [設定](#設定)
- [プロジェクト構成](#プロジェクト構成)
- [技術スタック](#技術スタック)
- [セキュリティ](#セキュリティ)
- [ロードマップ](#ロードマップ)
- [コントリビュート](#コントリビュート)
- [ライセンス](#ライセンス)

## 機能

| 分野 | ハイライト |
| --- | --- |
| 📂 メディアライブラリ | サーバー上の任意フォルダ。パス入力または内蔵**フォルダ選択 UI** |
| 🔍 スキャン | mp4/mkv/webm/avi/mov/ts… を再帰検出。並列プローブ（4 worker、`SCAN_WORKERS`）、リアルタイム進捗、**いつでもキャンセル**。macOS の `._` ファイルと `.m3u8` フォルダは自動スキップ |
| 🏷️ メタデータ | ffprobe でコーデック/解像度/再生時間、動画からポスター自動生成、タイトル/年/あらすじ/ジャンル編集可、任意の **TMDB スクレイピング** |
| 📺 ドラマ | 連番ファイル（`S01E01`、`EP1`、`第1集`、`タイトル01話名`…）を自動グループ化、シーズン対応、リスト連続再生 |
| ▶️ 再生 | H.264/WebM はネイティブ再生（HTTP Range）、**MKV/HEVC はリアルタイム HLS トランスコード**、字幕自動検出（SRT→WebVTT）、ダウンロード対応 |
| 👤 パーソナル | 続きを見る、お気に入り（動画・ドラマ）、連続再生できるプレイリスト |
| 🔐 ユーザー | JWT で登録/ログイン、管理者/一般ユーザー、ガード付きユーザー管理 |
| 🚫 コンテンツブロック | 管理者がタイトル単位でブロック — 全ユーザーに非表示、ファイルとレコードは保持、いつでも解除可能 |
| 🚫 ライブラリブロック | 管理画面からライブラリ全体をブロック — メディアは全ユーザーに非表示、何も削除されません |
| 🚫 パスフィルター | サーバーパスをユーザーごとに非表示化 — ホーム・ドラマ・お気に入り・続きを見る・プレイリストすべてに反映 |
| 🌐 インターフェース | i18n：**English（デフォルト）、中文、Français、日本語、Deutsch** |

## コンテンツ管理

誰に何が見えるかを決める独立した 3 層の仕組みです。いずれもファイルや
レコードを削除しません：

| 層 | 管理 | 対象 | 適用範囲 |
| --- | --- | --- | --- |
| 🏷️ タイトルブロック | 管理者 | タイトルに指定テキストを含む動画 | 全一覧・全ユーザー |
| 📚 ライブラリブロック | 管理者 | ライブラリ全体（すべての動画） | 全一覧・全ユーザー |
| 🛤️ パスフィルター | 各ユーザー | ユーザーが選んだサーバーパス | 全一覧・そのユーザーのみ |

3 層ともすべての一覧（ホーム・ドラマ・お気に入り・続きを見る・プレイリスト）で
SQL 評価され、ブロックされた項目は一斉に消え、解除すると即座に復元されます。

## スクリーンショット

> *近日公開 — `make serve` 後、`http://<サーバーIP>:8080` で UI を確認できます。*

## ドキュメント

すべてのドキュメントは多言語です。**[ドキュメント索引](docs/README.md)** から始めてください：

| ドキュメント | 言語 | 対象 |
| --- | --- | --- |
| [製品ドキュメント](docs/product.ja.md) | EN · 中文 · FR · JA · DE | エンドユーザー |
| [システムアーキテクチャ](docs/architecture.ja.md) | EN · 中文 · JA | 開発者 |
| [README](README.md) / [中文](README.zh-CN.md) | English · 中文 | すべて |

## クイックスタート

### 必要な環境

- Go 1.26+（ビルド用、またはビルド済みバイナリ）
- PostgreSQL 14+
- ffmpeg + ffprobe（メタデータ、ポスター、トランスコード）
- Node.js 18+（フロントエンド開発のみ。本番 UI はバックエンドが配信）

### インストール

```bash
# 1. データベース
createdb videocms                          # または: docker compose up -d db

# 2.（任意）デモ動画を生成
./scripts/make-demo-media.sh

# 3. バックエンド — 初回起動でテーブル作成 + admin/admin123
cd backend && go run ./cmd/server

# 4. フロントエンド（開発モード、ホットリロード）
cd frontend && npm install && npm run dev  # http://localhost:5173
```

本番相当のシングルポート配信：

```bash
make serve                                 # UI をビルドし :8080 で一括配信
```

初期管理者 **admin / admin123** でログインし、すぐにパスワードを変更してください
（管理 → ユーザー管理 → パスワード再設定）。その後、管理 → ライブラリ → スキャン で
最初のライブラリを追加します。

## LAN / スマホアクセス

1. サーバーの IP を確認：`ipconfig getifaddr en0`（例 `192.168.3.19`）
2. スマホを**同じネットワーク**に接続 → `http://192.168.3.19:8080` を開く
3. 初回は macOS のファイアウォール許可が必要な場合があります

> 平文 HTTP + 開発用 JWT は信頼できる LAN のみ推奨。公開アクセスは[セキュリティ](#セキュリティ)を参照。

## 設定

すべて環境変数で設定します：

| 変数 | デフォルト | 用途 |
| --- | --- | --- |
| `PORT` | `8080` | リッスンアドレス |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | PostgreSQL DSN |
| `JWT_SECRET` | 開発用定数 | トークン署名鍵 — **本番では強力な値に** |
| `DATA_DIR` | `data` | ポスター + HLS セグメント |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | admin / admin123 | 初期管理者 |
| `FFPROBE_BIN` / `FFMPEG_BIN` | 自動検出 | ツールパス（Homebrew フォールバック） |
| `TMDB_API_KEY` / `TMDB_LANGUAGE` | 空 / zh-CN | メタデータスクレイピング |
| `SCAN_WORKERS` | `4` | 並列スキャンワーカー数（1-16） |
| `WEB_ROOT` | 自動（`frontend/dist`） | 本番モードのフロントエンドディレクトリ |

## プロジェクト構成

```
backend/                 Go サーバー（net/http + pgx）
  cmd/server/            エントリポイント
  internal/api/          HTTP ハンドラ、ルーティング、ミドルウェア
  internal/auth/         JWT + ロールミドルウェア
  internal/media/        スキャナー、TMDB スクレイパー、HLS 管理、ストリーミング
  internal/db/           プール + 埋め込み SQL マイグレーション
  internal/models/       ドメイン型
frontend/                React 18 SPA（Vite）
  src/i18n/locales/      en / zh / fr / ja / de
  src/pages/             ブラウズ、プレイヤー、ドラマ、プレイリスト、管理…
docs/                    製品 + アーキテクチャ（多言語）
scripts/                 デモ素材ジェネレーター
```

## 技術スタック

| 層 | 技術 |
| --- | --- |
| バックエンド | Go（net/http、pgx/v5）、JWT（HS256）、bcrypt |
| フロントエンド | React 18、Vite、react-router、i18next、hls.js |
| データベース | PostgreSQL 14（埋め込み SQL マイグレーション） |
| メディア | ffprobe（メタデータ）、ffmpeg（ポスター、HLS トランスコード） |
| ドキュメント | Markdown + Mermaid（GitHub レンダリング） |

## セキュリティ

- 変更系操作はすべて管理者限定。メディア URL はユーザー JWT 必須（ヘッダーまたは `?token=`）
- パスワードは bcrypt、ロールは毎リクエスト DB から再取得
- HLS セグメント名は検証されセッションディレクトリ内に制限
- SQL はすべてパラメータ化
- **本番**：強力な `JWT_SECRET`、HTTPS リバースプロキシ、
  `ADMIN_USERNAME/ADMIN_PASSWORD` で初期アカウントを指定

[SECURITY.md](SECURITY.md) も参照してください。

## ロードマップ

- [x] ライブラリスキャン（並列、キャンセル可能、リアルタイム進捗）
- [x] メタデータ + ポスター + TMDB スクレイピング
- [x] ネイティブ再生 + HLS トランスコード
- [x] ドラマ自動グループ化（複数の命名規則）
- [x] お気に入り（動画・ドラマ）、プレイリスト、続きを見る
- [x] コンテンツ管理：タイトルブロック、ライブラリブロック、ユーザー別パスフィルター
- [x] i18n（en/zh/fr/ja/de）
- [ ] ファイル監視による増分取り込み
- [ ] アダプティブビットレート（多段階 HLS）
- [ ] 内蔵字幕の抽出 / アップロード
- [ ] 署名付き短時間 URL による公開共有

## コントリビュート

[CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。

## ライセンス

[Apache License 2.0](LICENSE)
