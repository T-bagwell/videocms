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
| GET | `/api/libraries` | user | list libraries |
| POST | `/api/libraries` | admin | add library (absolute server path) |
| POST | `/api/libraries/{id}/scan` | admin | start a scan |
| GET | `/api/videos` | user | search/browse videos |
| GET | `/api/videos/{id}` | user | video detail |
| GET | `/api/videos/{id}/stream` | user | HTTP Range stream |
| GET | `/api/videos/{id}/download` | user | download original file |
| GET | `/api/videos/{id}/download/remux` | user | MP4/MKV with selected tracks |
| GET | `/api/videos/{id}/tracks` | user | audio/subtitle track list |
| POST | `/api/uploads` | admin | create a chunked upload session |
| PUT | `/api/uploads/{id}/chunk/{index}` | admin | upload one chunk |
| POST | `/api/uploads/{id}/complete` | admin | finish the upload |
| POST | `/api/downloads` | admin | queue a yt-dlp download |
| GET | `/api/downloads` | admin | list download jobs |
| GET/POST | `/api/admin/blocked-titles` | admin | manage title blocks |
| GET | `/api/admin/users` | admin | manage users |
| GET | `/api/healthz` | open | health check |

See [architecture.md](architecture.md) for the full route table and data
flows.
