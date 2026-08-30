# Deployment

VideoCMS supports two topologies:

1. **Single service (default)** — the backend serves the built React app and
   the REST API on one port (`make serve`, or run the backend binary with
   `frontend/dist` present).
2. **Separate frontend & backend** — the backend runs as an API-only service
   and the frontend is deployed as static files behind nginx or any web
   server. Everything the UI does is exposed as a RESTful API, so you can also
   drive the system programmatically.

## Backend as an API-only service

Start the backend binary without `WEB_ROOT` and without a `frontend/dist`
directory next to it — only the `/api` routes are then mounted.

```bash
export PORT=8080
export DATABASE_URL=postgres://videocms:videocms@localhost:5432/videocms?sslmode=disable
export JWT_SECRET="$(openssl rand -hex 32)"
export CORS_ORIGINS=https://media.example.com   # optional; default = *
./videocms-server
```

Health check: `GET /api/healthz` → `{"status":"ok"}`.

Other variables: `ADMIN_USERNAME`/`ADMIN_PASSWORD`, `DATA_DIR`, `FFMPEG_BIN`/
`FFPROBE_BIN`, `YTDLP_PATH`, `TMDB_API_KEY`, `SCAN_WORKERS`,
`WATCH_INTERVAL` (see the README configuration table).

## Frontend as static files

```bash
cd frontend
npm ci
npm run build        # outputs frontend/dist
```

Point the SPA at the API in one of two ways:

- **Same-origin reverse proxy** — serve `frontend/dist` and proxy `/api` to
  the backend (see [deploy/nginx.conf.example](../deploy/nginx.conf.example)).
  No extra configuration is needed.
- **Cross-origin** — build with `VITE_API_BASE_URL=https://api.example.com`,
  or inject the base URL at runtime before the app boots:

  ```html
  <script>
    window.__VIDEOCMS_API_BASE__ = 'https://api.example.com';
  </script>
  ```

  The runtime override wins over the build-time variable. When frontend and
  backend are on different origins, set `CORS_ORIGINS` on the backend to the
  frontend origin (or leave it empty to allow any origin; requests are
  authenticated with a bearer token, not cookies).

## Using the REST API

All endpoints live under `/api` and return JSON. Errors use
`{"error": "message"}` with an appropriate status code.

1. **Authenticate**:

   ```bash
   curl -s -X POST https://api.example.com/api/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"admin123"}'
   # → {"token":"<jwt>","user":{...}}
   ```

2. **Call the API** with the bearer token:

   ```bash
   curl -s https://api.example.com/api/libraries \
     -H 'Authorization: Bearer <jwt>'
   ```

   Media endpoints (`/stream`, `/download`, `/poster`, `/hls/*`,
   `/subtitles/*`) also accept the token as a query parameter
   (`?token=<jwt>`) so `<video>`/`<img>` tags work without headers.

Key endpoints (all admin endpoints require an admin account):

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | `/api/auth/login` | open | get a JWT |
| GET | `/api/auth/sso` | open | which SSO providers are configured |
| GET | `/api/auth/oidc/start` | open | start OIDC login (redirect to IdP) |
| GET | `/api/auth/saml/login` / `POST /api/auth/saml/acs` | open | SAML login flow |
| GET | `/api/auth/saml/metadata` | open | SP metadata for configuring the IdP |
| GET | `/api/libraries` | user | list libraries |
| POST | `/api/libraries` | admin | add library (absolute server path) |
| POST | `/api/libraries/{id}/scan` | admin | start a scan |
| POST | `/api/libraries/{id}/health` | admin | health check (missing/corrupt/duplicates) |
| POST | `/api/libraries/{id}/health/keep-best` | admin | move duplicates to trash, keep best |
| POST | `/api/libraries/{id}/export-nfo` / `import-nfo` | admin | Kodi-style NFO export/import |
| GET | `/api/videos` | user | search/browse videos |
| GET | `/api/videos/{id}` | user | video detail |
| GET | `/api/videos/{id}/stream` | user | HTTP Range stream |
| GET | `/api/videos/{id}/download` | user | download original file |
| GET | `/api/videos/{id}/download/remux` | user | MP4/MKV with selected tracks |
| GET | `/api/videos/{id}/tracks` | user | audio/subtitle track list |
| GET/PUT/DELETE | `/api/videos/{id}/skip-interval(s)` | user | intro/credits skip intervals |
| POST | `/api/videos/{id}/transcribe` | admin | Whisper transcription |
| GET/POST/DELETE | `/api/videos/{id}/tags` | user | video tags |
| POST | `/api/videos/{id}/analyze` | admin | run the AI tagger |
| GET/POST | `/api/videos/{id}/comments`, `PUT …/rating` | user | comments and ratings |
| GET | `/api/videos/{id}/similar` | user | similar-video recommendations |
| POST | `/api/uploads` | admin | create a chunked upload session |
| PUT | `/api/uploads/{id}/chunk/{index}` | admin | upload one chunk |
| POST | `/api/uploads/{id}/complete` | admin | finish the upload |
| POST | `/api/downloads` | admin | queue a yt-dlp download |
| GET | `/api/downloads` | admin | list download jobs |
| GET/PUT | `/api/watch/rooms/{id}` | user | watch-together session state |
| GET/POST | `/api/live`, `/api/live/{id}/chat` | user/admin | live streams and chat |
| GET/POST | `/api/admin/blocked-titles` | admin | manage title blocks |
| GET | `/api/admin/users` | admin | manage users |
| POST | `/api/admin/videos/batch` | admin | bulk tag / clear tags / move to trash |
| GET | `/api/admin/trash`, `POST …/restore` | admin | recycle bin |
| GET/POST/PATCH/DELETE | `/api/admin/storage-pools` | admin | local/S3/SFTP pools |
| GET | `/api/admin/jobs` / `system` | admin | jobs dashboard + disk usage |
| POST | `/api/admin/maintenance/run` | admin | run maintenance now |
| GET | `/api/admin/backups[/{name}]` | admin | list/download backups |
| GET/POST/PATCH/DELETE | `/api/admin/webhooks` | admin | signed webhook subscriptions |
| POST | `/api/admin/notify/test` | admin | send a test notification |
| GET | `/api/openapi.json` | open | OpenAPI description of the API |
| GET | `/api/healthz` | open | health check |

See [architecture.md](architecture.md) for the full route table and data
flows.

## Optional integrations

### DLNA / Chromecast

To expose the library to UPnP/DLNA TVs and players on your LAN:

```bash
export DLNA_ENABLED=1
export DLNA_FRIENDLY_NAME="Home Media"        # optional
export DLNA_ALLOWED_IPS="192.168.3.0/24"      # optional; empty = whole LAN
```

The server answers SSDP on UDP 1900 and serves `/dlna/device.xml`,
`/dlna/content/{id}` (DIDL-Lite) and `/dlna/video/{id}/stream`. The web player
shows a **Cast to TV** button (Chromecast) in Chromecast-enabled browsers; it
streams through a short-lived share link, so the Chromecast must be able to
reach the server.

### SAML 2.0 single sign-on

Generate an SP key pair (valid for the domain you expose), then point the
backend at the IdP metadata:

```bash
openssl req -x509 -newkey rsa:2048 -keyout sp.key -out sp.crt \
  -days 3650 -nodes -subj "/CN=videocms"
export SAML_IDP_METADATA_URL=https://idp.example.com/metadata
export SAML_SP_CERT=/etc/videocms/sp.crt
export SAML_SP_KEY=/etc/videocms/sp.key
export SAML_ACS_URL=https://media.example.com/api/auth/saml/acs
export SAML_SP_ENTITY_ID=https://media.example.com/api/auth/saml/acs
```

Fetch `https://media.example.com/api/auth/saml/metadata` and register it in the
IdP. Users bind to `users.oauth_sub` with a `saml:` prefix; a `roles` attribute
containing "admin" grants admin on first login.

### Email notifications (SMTP)

```bash
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587                 # 465 = implicit TLS
export SMTP_USER=videocms@example.com
export SMTP_PASSWORD='secret'
export NOTIFY_EMAIL_FROM=videocms@example.com
export NOTIFY_EMAIL_TO=ops@example.com,admin@example.com
```

Scan, upload and download events are delivered as plain-text email (STARTTLS on
587/25, implicit TLS on 465). Test with the admin overview button or
`POST /api/admin/notify/test`.
