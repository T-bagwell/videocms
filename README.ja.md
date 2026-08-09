# VideoCMS — ビデオリソース管理システム

> **言語:** [English](README.md) | [中文](README.zh-CN.md) | 日本語

**Go + PostgreSQL + React** によるセルフホスト型ビデオライブラリです。
ディスク上のフォルダをスキャンしてメタデータ（タイトル・年・解像度・コーデック・再生時間）と
ポスターを自動抽出し、Web 上で閲覧・再生できます。視聴履歴・お気に入り・プレイリストに対応。

## 主な機能

- ライブラリ管理：サーバー上のフォルダを追加/削除、バックグラウンドスキャン
- スキャン性能：並列プローブ（デフォルト 4 worker、`SCAN_WORKERS` で調整）、
  リアルタイム進捗、いつでもキャンセル可能。macOS の `._` ファイルと `.m3u8` HLS フォルダは自動スキップ
- メタデータ：ffprobe で抽出、ファイル名からタイトル/年を解析、ffmpeg でポスター生成、
  **TMDB スクレイピング**も可能（`TMDB_API_KEY`）
- **ドラマ自動グループ化**：連番ファイル（S01E01、EP1、第1集…）をエピソード順に
  ドラマとして自動グループ化し、「ドラマ」カテゴリで個別表示
- 再生：H.264 MP4/WebM は Range ストリーミング、MKV/HEVC は **HLS トランスコード**（シーク・再開対応）
- ユーザー：登録/ログイン、JWT、管理者/一般ユーザー、管理者によるユーザー管理
- お気に入り・プレイリスト（連続再生）・「続きを見る」
- 管理画面：統計、ライブラリ管理（サーバー上のフォルダ選択機能付き）、メタデータ編集、ポスターアップロード
- **多言語 UI**：デフォルト英語、中文 / English / Français / 日本語 / Deutsch に切替可能

## クイックスタート

```bash
# 1. データベース
createdb videocms                       # または docker compose up -d db

# 2.（任意）デモ動画を生成
./scripts/make-demo-media.sh

# 3. バックエンド（初回起動でテーブル作成 + admin/admin123 作成）
cd backend && go run ./cmd/server

# 4. フロントエンド
cd frontend && npm install && npm run dev   # http://localhost:5173

# 5. スマホ / LAN アクセス（フロントを Go サーバーが配信）
make serve                                  # http://<LAN IP>:8080
```

環境変数と API 一覧は [README.md](README.md) を参照してください。

## 既知の制限

- ブラウザで直接再生できるのは H.264 MP4 / WebM のみ。MKV / HEVC は単一ビットレートの
  HLS トランスコード（15 分アイドルで回収）を利用
- TMDB スクレイピングには api.themoviedb.org へのアクセスが必要
- スキャンは差分更新方式の全量再スキャン。ファイル監視モードは将来の拡張候補

## ドキュメント

- [システムアーキテクチャ設計](docs/architecture.ja.md)（[English](docs/architecture.md) | [中文](docs/architecture.zh-CN.md)）
