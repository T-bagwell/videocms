# VideoCMS へのコントリビュート

VideoCMS への貢献に興味を持っていただきありがとうございます！ VideoCMS は
Go・React・PostgreSQL で構築されたセルフホスト型のビデオリソース管理システム
です。バグの報告、新機能の提案、ドキュメントや翻訳への協力など、このガイド
には始めるために必要なことがすべて記載されています。

## 目次

- [貢献の方法](#貢献の方法)
- [開発環境のセットアップ](#開発環境のセットアップ)
- [プロジェクト構成](#プロジェクト構成)
- [リポジトリの規約](#リポジトリの規約)
- [テスト](#テスト)
- [継続的インテグレーション](#継続的インテグレーション)
- [Pull request の流れ](#pull-request-の流れ)
- [ドキュメントとローカライズ](#ドキュメントとローカライズ)
- [トラブルシューティング](#トラブルシューティング)
- [サポート](#サポート)

## 貢献の方法

### バグの報告

- 重複を避けるため、まず[既存の issue](https://github.com/T-bagwell/videocms/issues)
  を検索してください。
- issue には以下を含めてください：VideoCMS のバージョン/コミット、OS と
  ブラウザ、PostgreSQL のバージョン、再現手順、期待される動作と実際の動作、
  関連ログ（スキャン/HLS の問題には特にバックエンドのログが役立ちます）。

### 機能のリクエスト

- UI のスケッチだけでなく、解決したい問題と具体的なユースケースを説明して
  ください。
- 小さく焦点を絞った機能リクエストのほうが、議論や実装がしやすくなります。

### ドキュメントと翻訳の改善

- 言語構成とドキュメント変更時のルールは
  [ドキュメントとローカライズ](#ドキュメントとローカライズ) を参照して
  ください。

### コードの提出

- タイポ修正などを除き、まず issue を開いてメンテナーがコメントできるように
  してから作業を始めてください。
- Pull request は小さく焦点を絞ってください（
  [Pull request の流れ](#pull-request-の流れ) を参照）。

## 開発環境のセットアップ

### 前提条件

- Go — バージョンは `backend/go.mod` に固定
- Node.js 18+、20+、または 22+（CI では 3 バージョンすべてを実行）
- PostgreSQL 14+
- ffmpeg/ffprobe（MKV/HEVC のトランスコードには libx265 が必要）

### 初期セットアップ

```bash
# 1. クローンしてリポジトリに入る
git clone git@github.com:T-bagwell/videocms.git
cd videocms

# 2. データベースを作成（冪等）
createdb videocms
# または: make db

# 3. （任意）デモメディアを生成
make demo

# 4. バックエンドを起動
# macOS の開発機では Go 環境が汚染されていることがあります（GOPATH の誤り、
# プロキシの問題）。必ずリポジトリのラッパーを使います：
./.codex/skills/videocms/scripts/goenv.sh --in backend go run ./cmd/server
# または一度だけ source する: source ./.codex/skills/videocms/scripts/goenv.sh

# 5. フロントエンドの開発サーバーを起動（/api は :8080 へプロキシ）
cd frontend
npm install
npm run dev
```

http://localhost:5173 を開き、初期管理者 **admin / admin123** でログインして
ください。

### よく使うコマンド

| コマンド | 内容 |
| --- | --- |
| `make db` | `videocms` データベースを作成（再実行可能） |
| `make server` | http://localhost:8080 でバックエンドを実行 |
| `make frontend` | :5173 で Vite 開発サーバーを実行（`/api` をプロキシ） |
| `make demo` | `demo-media/` と `demo-series/` のサンプルファイルを生成 |
| `make build` | バックエンド bin とフロントエンド `dist` をビルド |
| `make serve` | シングルポートの本番モード（バックエンドが SPA を配信） |

### 環境に関する注意

- このリポジトリでは `go` を直接実行せず、
  `./.codex/skills/videocms/scripts/goenv.sh --in backend ...` を使って
  ください（モジュールは `backend/` にあるため `--in backend` が必要）。
- macOS の開発機では、バックエンドは Homebrew の ffmpeg
  （`/usr/local/opt/ffmpeg/bin/`）を自動的に使用します。システムの ffmpeg は
  libx265 でクラッシュすることがあります。
- PostgreSQL が起動している必要があります。マイグレーションはバックエンド
  起動時に自動で適用されます。

## プロジェクト構成

```
backend/
  cmd/server/     entrypoint
  internal/
    api/          HTTP ハンドラーとルート
    auth/         JWT + ロールミドルウェア
    media/        スキャナー、スクレイパー、HLS、ストリーミング
    db/           pool + SQL マイグレーション（埋め込み）
    models/       ドメイン型
frontend/
  src/
    pages/        ルートコンポーネント
    components/   共有 UI
    i18n/         ロケール JSON（en/zh/fr/ja/de）
docs/             製品、アーキテクチャ、スクリーンショット
```

ディレクトリ構成、API ルート、データモデル、主要フロー、セキュリティ、
拡張ポイントの詳細は [architecture.md](architecture.md) を参照して
ください。

## リポジトリの規約

### バックエンド

- Go 標準ライブラリの `net/http` + `pgx/v5` のみ。Web フレームワークは
  使わない。
- 変更は `gofmt` と `go vet` を通す。
- DB スキーマの変更は新しい番号付きマイグレーション
  （`backend/internal/db/migrations/NNN_*.sql`）として追加する。マイグレー
  ションは起動時に自動適用される。適用済みのマイグレーションは絶対に編集せず、
  新しいものを追加する。
- 動画を表示するすべての一覧は `visibleEpisodes($N)`
  （`backend/internal/api/handlers_videos.go` で定義）を通すこと。ユーザーごと
  の非表示パス、管理者のタイトルブロック、ライブラリブロックが尊重される：
  ホーム、続きから視聴、お気に入り、プレイリスト、シリーズ詳細、シリーズ一覧。
- 管理者のコンテンツブロック：`blocked_titles` はタイトルを大文字小文字を
  区別しない部分文字列でマッチし、`visibleEpisodes` に組み込まれる。
  ライブラリ単位のブロックは `libraries.blocked` にあり、`visiblePaths` により
  `libraries` を JOIN しないサブクエリ内でも機能する。
- メディアエンドポイント（`/stream`、`/download`、`/poster`、`/hls/*`）は
  `?token=` を受け付け続ける。`<video>`/`<img>` タグがヘッダーなしで
  機能するようにするため。
- API・エラーメッセージは英語のまま。ローカライズされるのは Web UI のみ。

### フロントエンド

- UI テキストのハードコード禁止。ユーザーに見える文字列はすべて
  `useTranslation()` を通し、5 つのロケールファイルすべてに入れる
  （`frontend/src/i18n/locales/{en,zh,fr,ja,de}.json`）。
- エピソード切り替え時に `<video>` 要素を再マウントしてはいけない：
  PlayerPage は `activeId` 状態を保持し、`switchEpisode(nextId)` で同じ要素の
  HLS ソースを差し替える。
- 連続再生：`onEnded` は `queue[idx + 1]` を選んで切り替える。ナビゲートや
  プレイヤーの再構築はしない。

### シリーズとメディア

- エピソード検出は `backend/internal/media/episode.go`（`parseEpisode`）。
  対応マーカー：`S01E01`、`EP1`、`E01`、`第N集`、`ShowName01Title`、末尾の
  `(NN)` / `  NN`。
- （シリーズ名, シーズン）あたり 2 話以上でグループ化。スキャンごとに
  `rebuildSeries` がグループを再構築する。解析ルールを変更したら
  `episode_test.go` も更新する。
- シリーズ一覧の並び順：最新のエピソード取り込み順
  （`max(v.created_at)` DESC）、次に名前、次にシーズン。

### HLS（壊れやすいので退行させない）

- `-hls_playlist_type vod` を使わない：マニフェストがトランスコード完了まで
  バッファリングし、長い動画が再生開始に失敗する。ライブ成長型マニフェストは
  意図的なもの。`#EXT-X-ENDLIST` は ffmpeg プロセス完了時にサーバー側で追加
  される。
- `-hls_flags temp_file` を使わない。`-hls_list_size 0` を維持する。
- 安定した 6 秒セグメントのため、`expr:gte(t,n_forced*6)` でキーフレームを
  強制する。
- セッションは 15 分でアイドルアウトする。シークすると指定オフセットから
  トランスコードを再開する。

## テスト

```bash
# バックエンド（必ずラッパー経由で）
./.codex/skills/videocms/scripts/goenv.sh --in backend go test ./...
./.codex/skills/videocms/scripts/goenv.sh --in backend go vet ./...

# フロントエンド
cd frontend && npm run build
```

- 解析・スキャンロジックには単体テストを併記する
  （例：`internal/media/episode_test.go`）。
- 統合テスト（`internal/api/integration_test.go`）は PostgreSQL に接続できない
  場合は自動でスキップされる。実行するには `TEST_PG_DSN` を設定する。
- ネットワーク依存のスクレイパーテストは `NETWORK_TEST=1` が設定されていない
  限りスキップされる。

## 継続的インテグレーション

GitHub Actions は `main` へのプッシュと pull request で 2 つのワークフローを
実行します：

| ワークフロー | ファイル | 実行内容 |
| --- | --- | --- |
| Backend CI | `.github/workflows/backend.yml` | `backend/` で `go build`、`go vet`、`go test`（Go バージョンは `go.mod` から） |
| Frontend Build | `.github/workflows/webpack.yml` | `frontend/` で `npm ci` + `npm run build`（Node 18/20/22） |

レビューを依頼する前に両方をグリーンにしてください。

## Pull request の流れ

1. 自明でない変更は、まず issue を開いて方針を議論する。
2. `main` から短く分かりやすい名前のブランチを切る（`fix/`、`feat/`、
   `docs/`、`refactor/` など）。
3. コミットは焦点を絞る。1 つの PR で 1 つの論理的な変更に。
4. PR を開く前に：
   - `gofmt`、`go vet`、`go test ./...` が通る
   - `npm run build` が通る
   - UI の変更は PR の説明にスクリーンショットを添える
5. PR で何を・なぜ変更したかを説明し、修正する issue を参照する
   （`Closes #123`）。
6. ユーザーに見える変更は [changelog.md](changelog.md) を更新する。
7. ドキュメントに触れた場合は、すべての既存言語を更新する（下記参照）。

## ドキュメントとローカライズ

リポジトリは複数のドキュメントセットを管理しています。ドキュメントに触れる
ときは既存のすべての言語を更新し、コードフェンスは対で閉じるようにして
ください。

| ドキュメントセット | 言語 |
| --- | --- |
| README | en、zh-CN、ja |
| 製品ドキュメント（`docs/product.*.md`） | en、zh-CN、fr、ja、de |
| アーキテクチャドキュメント（`docs/architecture.*.md`） | en、zh-CN、ja |
| コントリビュートガイド | en、zh-CN、ja |

- ドキュメントファイルを追加・リネームしたら `INDEX.md` の索引も同期
  する。
- Web UI は 5 言語にローカライズされている。デフォルトは英語で、キーが
  欠落している場合は英語にフォールバックする。

### UI に言語を追加する

1. `frontend/src/i18n/locales/<code>.json` を追加する（英語ファイルをコピー
   して翻訳）。
2. `frontend/src/i18n/index.js` に登録する（`SUPPORTED_LANGS` +
   `resources`）。
3. 英語へのフォールバックが機能するよう、キー構造を同一に保つ。
4. 対応言語を一覧している README とドキュメントを更新する。

## トラブルシューティング

- `backend/` の外で `go run`/`go build` を実行すると失敗する。必ず
  `--in backend` を使う。
- クラッシュ後に `scanning` 状態のまま残ったスキャンは、起動時に `error` に
  リセットされる。管理ページから再スキャンする。
- スキャナーは macOS の `._*` ファイルと `.m3u8` ストリームディレクトリを
  スキップする。この挙動を維持する。
- `make serve` は :8080 にバインドして SPA を配信する。開発時は `make server`
  と `make frontend` の両方を使う（Vite が `/api` を :8080 にプロキシ）。

## サポート

- バグや機能リクエストは GitHub
  [issues](https://github.com/T-bagwell/videocms/issues) へ。
- リポジトリにはプロジェクトレベルの Codex スキル
  （`.codex/skills/videocms/`）が同梱されており、上記の環境・コマンド・規約が
  記録されています。このリポジトリで作業する Codex エージェントはそれを読み
  込んでください。
