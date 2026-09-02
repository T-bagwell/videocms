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
| GET | `/api/auth/sso` | 公開 | 設定済みの SSO プロバイダー |
| GET | `/api/auth/oidc/start` | 公開 | OIDC ログイン開始（IdP へリダイレクト） |
| GET | `/api/auth/saml/login` / `POST /api/auth/saml/acs` | 公開 | SAML ログインフロー |
| GET | `/api/auth/saml/metadata` | 公開 | IdP 設定用の SP メタデータ |
| GET | `/api/libraries` | ユーザー | ライブラリ一覧 |
| POST | `/api/libraries` | 管理者 | ライブラリ追加（サーバー絶対パス） |
| POST | `/api/libraries/{id}/scan` | 管理者 | スキャン開始 |
| POST | `/api/libraries/{id}/health` | 管理者 | ヘルスチェック（欠落/破損/重複） |
| POST | `/api/libraries/{id}/health/keep-best` | 管理者 | ベスト版を残し他をゴミ箱へ |
| POST | `/api/libraries/{id}/export-nfo` / `import-nfo` | 管理者 | Kodi 形式 NFO エクスポート/インポート |
| GET | `/api/videos` | ユーザー | 動画の検索/一覧 |
| GET | `/api/videos/{id}` | ユーザー | 動画詳細 |
| GET | `/api/videos/{id}/stream` | ユーザー | HTTP Range ストリーム |
| GET | `/api/videos/{id}/download` | ユーザー | 元ファイルのダウンロード |
| GET | `/api/videos/{id}/download/remux` | ユーザー | トラック選択付き MP4/MKV |
| GET | `/api/videos/{id}/tracks` | ユーザー | 音声/字幕トラック一覧 |
| GET/PUT/DELETE | `/api/videos/{id}/skip-interval(s)` | ユーザー | イントロ/クレジットスキップ区間 |
| POST | `/api/videos/{id}/transcribe` | 管理者 | Whisper 文字起こし |
| GET/POST/DELETE | `/api/videos/{id}/tags` | ユーザー | 動画タグ |
| POST | `/api/videos/{id}/analyze` | 管理者 | AI タガーを実行 |
| GET/POST | `/api/videos/{id}/comments`、`PUT …/rating` | ユーザー | コメントと評価 |
| GET | `/api/videos/{id}/similar` | ユーザー | 類似動画レコメンド |
| POST | `/api/uploads` | 管理者 | チャンクアップロードセッション作成 |
| PUT | `/api/uploads/{id}/chunk/{index}` | 管理者 | チャンクを 1 つアップロード |
| POST | `/api/uploads/{id}/complete` | 管理者 | アップロード完了 |
| POST | `/api/downloads` | 管理者 | yt-dlp ダウンロードをキューへ |
| GET | `/api/downloads` | 管理者 | ダウンロードジョブ一覧 |
| GET/PUT | `/api/watch/rooms/{id}` | ユーザー | 一緒に見るセッション状態 |
| GET/POST | `/api/live`、`/api/live/{id}/chat` | ユーザー/管理者 | ライブ配信とチャット |
| GET/POST | `/api/admin/blocked-titles` | 管理者 | タイトルブロック管理 |
| GET | `/api/admin/users` | 管理者 | ユーザー管理 |
| POST | `/api/admin/videos/batch` | 管理者 | 一括タグ/タグ解除/ゴミ箱へ |
| GET | `/api/admin/trash`、`POST …/restore` | 管理者 | ゴミ箱 |
| GET/POST/PATCH/DELETE | `/api/admin/storage-pools` | 管理者 | ローカル/S3/SFTP プール |
| GET | `/api/admin/jobs` / `system` | 管理者 | ジョブダッシュボード + ディスク使用量 |
| POST | `/api/admin/maintenance/run` | 管理者 | メンテナンスを即時実行 |
| GET | `/api/admin/backups[/{name}]` | 管理者 | バックアップ一覧/ダウンロード |
| GET/POST/PATCH/DELETE | `/api/admin/webhooks` | 管理者 | 署名付き Webhook 購読 |
| POST | `/api/admin/notify/test` | 管理者 | テスト通知を送信 |
| GET | `/api/openapi.json` | 公開 | API の OpenAPI 記述 |
| GET | `/api/healthz` | 公開 | ヘルスチェック |

完全なルート一覧とデータフローは [architecture.ja.md](architecture.ja.md) を参照。

## Docker と Kubernetes

公式コンテナイメージはリポジトリルートの `Dockerfile` でビルドされます（マルチステージ：
Node 22 でフロントエンド、Go でバックエンドをビルドし、`alpine` ランタイムに ffmpeg・
SPA・API を同梱、ポート 8080）。`docker-compose.yml` で PostgreSQL 16 + VideoCMS を
データ用ボリューム 2 つ（DB とメディア）とともに起動できます：

```bash
JWT_SECRET="$(openssl rand -hex 32)" docker compose up -d --build
```

Kubernetes では `videocms-helm/` の Helm chart を使用します：

```bash
helm install videocms ./videocms-helm \
  --set env.JWT_SECRET="$(openssl rand -hex 32)" \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=media.example.com
```

チャートは水平スケーリング（`autoscaling.enabled=true`、`replicaCount>1`）に対応しており、
共有メディアボリューム（NFS/SMB/CephFS などの `ReadWriteMany` ストレージクラス）が必要です。
トランスコードセッションとファイルウォッチャーはレプリカごとに独立しているため、
複数レプリカでは `DATA_DIR` を共有ストレージに設定し、定期スキャンは単一レプリカで
実行してください（重複作業の防止）。

## 任意の連携

### DLNA / Chromecast

LAN 内の UPnP/DLNA 対応テレビやプレイヤーにライブラリを公開する場合：

```bash
export DLNA_ENABLED=1
export DLNA_FRIENDLY_NAME="Home Media"        # 任意
export DLNA_ALLOWED_IPS="192.168.3.0/24"      # 任意。空 = LAN 全体
```

サーバーは UDP 1900 の SSDP に応答し、`/dlna/device.xml`、
`/dlna/content/{id}`（DIDL-Lite）、`/dlna/video/{id}/stream` を提供します。
Cast 対応ブラウザではプレイヤーに「テレビにキャスト」（Chromecast）ボタンが
表示され、短期共有リンク経由で配信します。Chromecast からサーバーに
到達できる必要があります。

### SAML 2.0 シングルサインオン

SP 鍵ペア（CN は公開ドメイン）を生成し、バックエンドを IdP メタデータに
向けます：

```bash
openssl req -x509 -newkey rsa:2048 -keyout sp.key -out sp.crt \
  -days 3650 -nodes -subj "/CN=videocms"
export SAML_IDP_METADATA_URL=https://idp.example.com/metadata
export SAML_SP_CERT=/etc/videocms/sp.crt
export SAML_SP_KEY=/etc/videocms/sp.key
export SAML_ACS_URL=https://media.example.com/api/auth/saml/acs
export SAML_SP_ENTITY_ID=https://media.example.com/api/auth/saml/acs
```

`https://media.example.com/api/auth/saml/metadata` を取得して IdP に登録します。
ユーザーは `saml:` プレフィックス付きの `users.oauth_sub` に紐付き、
`roles` 属性に "admin" が含まれると初回ログインで管理者権限を付与します。

### メール通知（SMTP）

```bash
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587                 # 465 = 暗黙 TLS
export SMTP_USER=videocms@example.com
export SMTP_PASSWORD='secret'
export NOTIFY_EMAIL_FROM=videocms@example.com
export NOTIFY_EMAIL_TO=ops@example.com,admin@example.com
```

スキャン・アップロード・ダウンロードのイベントがプレーンテキストメールで
配信されます（587/25 は STARTTLS、465 は暗黙 TLS）。管理概要ページのボタンか
`POST /api/admin/notify/test` でテストできます。
