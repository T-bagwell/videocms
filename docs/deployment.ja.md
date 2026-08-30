# デプロイ

VideoCMS は 2 種類の構成に対応しています。

1. **単一サービス（デフォルト）** — バックエンドがビルド済み React アプリと
   REST API を 1 つのポートで配信（`make serve`、またはバックエンドバイナリの
   隣に `frontend/dist` がある場合）。
2. **フロントエンドとバックエンドの分離** — バックエンドは API 専用サービス
   として実行し、フロントエンドは nginx など任意の Web サーバーで静的ファイル
   として配信。UI の操作はすべて RESTful API として公開されるため、システムを
   プログラムから操作することもできます。

## バックエンドを API 専用で運用

`WEB_ROOT` を設定せず、隣に `frontend/dist` を置かずにバックエンドバイナリを
起動すると、`/api` ルートだけがマウントされます。

```bash
export PORT=8080
export DATABASE_URL=postgres://videocms:videocms@localhost:5432/videocms?sslmode=disable
export JWT_SECRET="$(openssl rand -hex 32)"
export CORS_ORIGINS=https://media.example.com   # 任意。デフォルトは *
./videocms-server
```

ヘルスチェック：`GET /api/healthz` → `{"status":"ok"}`。

その他の変数：`ADMIN_USERNAME`/`ADMIN_PASSWORD`、`DATA_DIR`、`FFMPEG_BIN`/
`FFPROBE_BIN`、`YTDLP_PATH`、`TMDB_API_KEY`、`SCAN_WORKERS`、
`WATCH_INTERVAL`（設定一覧は README 参照）。

## フロントエンドを静的ファイルで配信

```bash
cd frontend
npm ci
npm run build        # frontend/dist が出力される
```

SPA から API を指す方法は 2 通りあります。

- **同一オリジンのリバースプロキシ** — `frontend/dist` を配信し、`/api` を
  バックエンドへプロキシ（[deploy/nginx.conf.example](../deploy/nginx.conf.example)
  参照）。追加設定は不要です。
- **クロスオリジン** — ビルド時に `VITE_API_BASE_URL=https://api.example.com`
  を指定するか、起動前にランタイムでベース URL を注入します。

  ```html
  <script>
    window.__VIDEOCMS_API_BASE__ = 'https://api.example.com';
  </script>
  ```

  ランタイム注入はビルド時変数より優先されます。フロントエンドとバックエンドの
  オリジンが異なる場合は、バックエンドの `CORS_ORIGINS` にフロントエンドの
  オリジンを設定します（空のままなら任意のオリジンを許可。認証は Bearer
  Token で行い、Cookie には依存しません）。

## REST API の利用

すべてのエンドポイントは `/api` 配下にあり JSON を返します。エラーは
`{"error": "message"}` と適切なステータスコードで返ります。

1. **認証**：

   ```bash
   curl -s -X POST https://api.example.com/api/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"admin123"}'
   # → {"token":"<jwt>","user":{...}}
   ```

2. **Bearer Token で API を呼ぶ**：

   ```bash
   curl -s https://api.example.com/api/libraries \
     -H 'Authorization: Bearer <jwt>'
   ```

   メディア系エンドポイント（`/stream`、`/download`、`/poster`、`/hls/*`、
   `/subtitles/*`）はトークンをクエリパラメータ（`?token=<jwt>`）でも受け付け、
   `<video>`/`<img>` タグがヘッダーなしで動作します。

主なエンドポイント（管理系は管理者アカウントが必要）：

| メソッド | パス | 権限 | 用途 |
| --- | --- | --- | --- |
| POST | `/api/auth/login` | 公開 | JWT を取得 |
| GET | `/api/libraries` | ユーザー | ライブラリ一覧 |
| POST | `/api/libraries` | 管理者 | ライブラリ追加（サーバー絶対パス） |
| POST | `/api/libraries/{id}/scan` | 管理者 | スキャン開始 |
| GET | `/api/videos` | ユーザー | 動画の検索/一覧 |
| GET | `/api/videos/{id}` | ユーザー | 動画詳細 |
| GET | `/api/videos/{id}/stream` | ユーザー | HTTP Range ストリーム |
| GET | `/api/videos/{id}/download` | ユーザー | 元ファイルのダウンロード |
| GET | `/api/videos/{id}/download/remux` | ユーザー | トラック選択付き MP4/MKV |
| GET | `/api/videos/{id}/tracks` | ユーザー | 音声/字幕トラック一覧 |
| POST | `/api/uploads` | 管理者 | チャンクアップロードセッション作成 |
| PUT | `/api/uploads/{id}/chunk/{index}` | 管理者 | チャンクを 1 つアップロード |
| POST | `/api/uploads/{id}/complete` | 管理者 | アップロード完了 |
| POST | `/api/downloads` | 管理者 | yt-dlp ダウンロードをキューへ |
| GET | `/api/downloads` | 管理者 | ダウンロードジョブ一覧 |
| GET/POST | `/api/admin/blocked-titles` | 管理者 | タイトルブロック管理 |
| GET | `/api/admin/users` | 管理者 | ユーザー管理 |
| GET | `/api/healthz` | 公開 | ヘルスチェック |

完全なルート一覧とデータフローは [architecture.ja.md](architecture.ja.md) を参照。
