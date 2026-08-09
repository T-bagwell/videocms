# VideoCMS — Self-hosted Video Resource Management

> **Languages:** English | [中文](README.zh-CN.md) | [日本語](README.ja.md)

A self-hosted video library built with **Go + PostgreSQL + React**. Point it at any
folder on disk, scan it, and browse/play the videos from a web UI — with metadata,
posters, watch progress, favorites, playlists, and a fully localized interface.

## Architecture

```
┌──────────────────┐  HTTP/JSON + Range streaming   ┌───────────────────┐
│  React frontend   │ ─────────────────────────────▶ │  Go backend        │
│  (i18n: en/zh/    │                                │  (net/http, 8080)  │
│   fr/ja/de)       │                                └─────────┬─────────┘
└──────────────────┘                                          │ ffprobe/ffmpeg
        ▲                                                     ▼
        │  proxy /api                              Media library folders (disk)
        └───────────────────────────────────────────────────────┘
                                                            │
                       PostgreSQL ── metadata / progress / favorites / playlists
```

| Layer | Implementation |
| --- | --- |
| Media server (backend core) | Go service: folder scanning, metadata, playlists, JWT auth, Range streaming, HLS transcoding |
| Media library (storage) | Any folder on the server disk, added by absolute path or a server-side folder picker |
| Web player (frontend) | React SPA: browse, search, details, playback, resume, favorites, playlists, admin |
| Database | PostgreSQL: users, libraries, videos, watch progress, favorites, playlists |

## Features

- **Scanning**: recursive discovery of mp4/mkv/webm/avi/mov/ts…, parallel probing
  (4 workers, tunable via `SCAN_WORKERS`), live progress, cancel anytime; skips
  macOS `._` resource-fork files and `.m3u8` HLS stream folders
- **Metadata**: ffprobe extracts duration/resolution/codec; filename parsing for
  title/year; ffmpeg generates posters; optional **TMDB scraping** (zh/en)
- **TV series auto-grouping**: numbered files (S01E01, EP1, 第1集…) are grouped
  into series sorted by episode, with a separate “TV Shows” category
- **Playback**: HTTP Range streaming for H.264 MP4/WebM; **HLS transcoding** for
  MKV/HEVC via ffmpeg (seek + resume supported); subtitle auto-detection
- **Users**: register/login, JWT, admin/user roles; admin user management
- **Social**: favorites, playlists with sequential playback, continue watching
- **Admin**: stats, library management, metadata editing, poster upload, users
- **i18n**: web UI in **English (default), 中文, Français, 日本語, Deutsch**

## Quick Start

### 1. Database

Local PostgreSQL 14 (Homebrew):

```bash
createdb videocms
```

Or via Docker:

```bash
docker compose up -d db
# DATABASE_URL=postgres://videocms:videocms@localhost:5432/videocms?sslmode=disable
```

### 2. Demo videos (optional)

```bash
./scripts/make-demo-media.sh   # generates 3 synthetic videos in demo-media/
```

### 3. Backend

```bash
cd backend
go run ./cmd/server
```

First start creates tables and an initial admin **admin / admin123** (change it
after first login). Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Listen port |
| `DATABASE_URL` | `postgres://localhost:5432/videocms?sslmode=disable` | PostgreSQL DSN |
| `JWT_SECRET` | dev constant | **Set a strong value in production** |
| `DATA_DIR` | `data` | Generated files (posters, HLS) |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | admin / admin123 | Initial admin |
| `FFPROBE_BIN` / `FFMPEG_BIN` | auto-detected | ffprobe/ffmpeg binaries |
| `TMDB_API_KEY` | empty (scraping off) | [TMDB API key](https://www.themoviedb.org/settings/api) |
| `TMDB_LANGUAGE` | `zh-CN` | Scraping language |
| `SCAN_WORKERS` | `4` | Parallel probe workers (1-16) |
| `WEB_ROOT` | auto (`frontend/dist`) | Built frontend directory |

### 4. Frontend

```bash
cd frontend
npm install
npm run dev        # http://localhost:5173 (proxies /api to :8080)
```

### 5. Add a library

Log in → **Admin → Libraries** → enter a name and a server folder path (or use
**Browse…** to pick one) → **Scan**. Videos appear on the home page.

### 6. LAN / phone access

The Go server can serve the built frontend directly (one port for everything):

```bash
make serve
```

1. Find the Mac's LAN IP: `ipconfig getifaddr en0` (e.g. `192.168.3.19`)
2. Phone on the **same WiFi**, open `http://192.168.3.19:8080`, log in
3. If macOS asks about the firewall, allow incoming connections

This deployment is plain HTTP with a dev JWT secret — use only on trusted LANs;
for public access add an HTTPS reverse proxy and a strong `JWT_SECRET`.

## API Overview

Auth: `POST /api/auth/register` · `POST /api/auth/login` · `GET /api/auth/me`

Libraries: `GET/POST /api/libraries` · `POST /api/libraries/{id}/scan` ·
`POST /api/libraries/{id}/scan/cancel` · `DELETE /api/libraries/{id}` ·
`GET /api/admin/paths?path=/dir` (browse server directories)

Videos: `GET /api/videos` (paging/search/filter/sort) · `GET /api/videos/{id}` ·
`PATCH /api/videos/{id}` · `POST /api/videos/{id}/poster` ·
`GET /api/videos/{id}/stream|download|subtitles|poster` ·
`POST /api/videos/{id}/scrape` ·
`GET /api/videos/{id}/hls/{file}` (HLS transcode; `start` param supported)

User data: `PUT /api/users/me/progress` · `GET /api/users/me/continue` ·
`GET/POST /api/users/me/favorites` · `DELETE /api/users/me/favorites/{videoId}`

Playlists: `GET/POST /api/playlists` · `GET/PATCH/DELETE /api/playlists/{id}` ·
`POST /api/playlists/{id}/items` · `DELETE /api/playlists/{id}/items/{videoId}`

Admin: `GET /api/admin/stats` · `GET /api/admin/users` ·
`PATCH /api/admin/users/{id}` · `POST /api/admin/users/{id}/reset-password` ·
`DELETE /api/admin/users/{id}`

Media URLs (`<video>`/`<img>` cannot set headers) accept `?token=JWT`.

## Known Limitations

- Browsers play H.264 MP4/WebM directly; MKV/HEVC use single-bitrate HLS
  transcoding (idle sessions reaped after 15 min); “Transcode play” is offered as a fallback
- TMDB scraping needs access to api.themoviedb.org
- Full rescan is incremental (diff-based); a filesystem-watch mode could be added later

## License

[Apache License 2.0](LICENSE)

## Documentation

- [Product documentation](docs/product.md) (English | [中文](docs/product.zh-CN.md) | [Français](docs/product.fr.md) | [日本語](docs/product.ja.md) | [Deutsch](docs/product.de.md))
- [System architecture design](docs/architecture.md) (English | [中文](docs/architecture.zh-CN.md) | [日本語](docs/architecture.ja.md))
