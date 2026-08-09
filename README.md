# 🎬 VideoCMS

> **Self-hosted video resource management** — Go · React · PostgreSQL

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14-4169E1?logo=postgresql&logoColor=white)
![i18n](https://img.shields.io/badge/i18n-5%20languages-8A2BE2)

**Languages:** English · [中文](README.zh-CN.md) · [日本語](README.ja.md)

VideoCMS turns folders on your server disk into a browsable, searchable video
library. Scan once, and every video gets posters, metadata, watch progress,
favorites, playlists — and numbered files automatically group into TV Shows.

---

## Table of Contents

- [Features](#features)
- [Screenshots](#screenshots)
- [Documentation](#documentation)
- [Quick Start](#quick-start)
- [LAN / Phone Access](#lan--phone-access)
- [Configuration](#configuration)
- [Project Structure](#project-structure)
- [Tech Stack](#tech-stack)
- [Security](#security)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

## Features

| Area | Highlights |
| --- | --- |
| 📂 Media libraries | Any server folder; add by path or with the built-in **server folder picker** |
| 🔍 Scanning | Recursive discovery of mp4/mkv/webm/avi/mov/ts…; parallel probing (4 workers, `SCAN_WORKERS`); live progress; **cancel anytime**; skips macOS `._` files and `.m3u8` stream folders |
| 🏷️ Metadata | ffprobe extracts codec/resolution/duration; posters generated from the video; editable title/year/synopsis/genres; optional **TMDB scraping** |
| 📺 TV Shows | Numbered files (`S01E01`, `EP1`, `第1集`, `Show01Title`…) auto-group into series sorted by episode; season-aware; play-all with continuous playback |
| ▶️ Playback | H.264/WebM play natively (HTTP Range); **MKV/HEVC transcoded to HLS on the fly**; subtitles auto-detected (SRT→WebVTT); download for offline |
| 👤 Personal | Continue watching, favorites (videos **and** series), playlists with sequential playback |
| 🔐 Users | Register/login with JWT; admin/user roles; admin user management with safety guards |
| 🚫 Path filters | Hide any server path per user — excluded everywhere (home, series, favorites, continue watching, playlists) |
| 🌐 Interface | i18n: **English (default), 中文, Français, 日本語, Deutsch** |

## Screenshots

> *Coming soon — the UI is live at `http://<your-server>:8080` after `make serve`.*

## Documentation

All documentation is multi-language. Start at the **[docs index](docs/README.md)**:

| Doc | Languages | Audience |
| --- | --- | --- |
| [Product documentation](docs/product.md) | EN · 中文 · FR · JA · DE | End users |
| [System architecture](docs/architecture.md) | EN · 中文 · JA | Developers |
| [README](README.zh-CN.md) / [README.ja.md](README.ja.md) | 中文 · 日本語 | Everyone |

## Quick Start

### Requirements

- Go 1.22+ (to build) or a prebuilt binary
- PostgreSQL 14+
- ffmpeg + ffprobe (metadata, posters, transcoding)
- Node.js 18+ (frontend development only — the built UI is served by the backend)

### Install

```bash
# 1. Database
createdb videocms                          # or: docker compose up -d db

# 2. (optional) demo videos
./scripts/make-demo-media.sh

# 3. Backend — first start creates tables + admin/admin123
cd backend && go run ./cmd/server

# 4. Frontend (dev mode, hot reload)
cd frontend && npm install && npm run dev  # http://localhost:5173
```

For production-style single-port serving:

```bash
make serve                                 # builds UI + serves everything on :8080
```

Log in with the initial admin **admin / admin123** and change the password
immediately (Admin → Users → Reset password). Then add your first library under
**Admin → Libraries → Scan**.

## LAN / Phone Access

1. Find the server IP: `ipconfig getifaddr en0` (e.g. `192.168.3.19`)
2. Phone on the **same network** → open `http://192.168.3.19:8080`
3. Allow the macOS firewall prompt on first run

> Plain HTTP + dev JWT is fine on a trusted LAN. For anything public, see
> [Security](#security).

## Configuration

All settings are environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Listen address |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | PostgreSQL DSN |
| `JWT_SECRET` | dev constant | Token signing key — **set a strong value in production** |
| `DATA_DIR` | `data` | Posters + HLS segments |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | admin / admin123 | Initial admin |
| `FFPROBE_BIN` / `FFMPEG_BIN` | auto-detect | Tool paths (Homebrew fallback) |
| `TMDB_API_KEY` / `TMDB_LANGUAGE` | empty / zh-CN | Metadata scraping |
| `SCAN_WORKERS` | `4` | Parallel scan workers (1–16) |
| `WEB_ROOT` | auto (`frontend/dist`) | Built frontend for production mode |

## Project Structure

```
backend/                 Go server (net/http + pgx)
  cmd/server/            entry point
  internal/api/          HTTP handlers, routes, middleware
  internal/auth/         JWT + role middleware
  internal/media/        scanner, TMDB scraper, HLS manager, streaming
  internal/db/           pool + embedded SQL migrations
  internal/models/       domain types
frontend/                React 18 SPA (Vite)
  src/i18n/locales/      en / zh / fr / ja / de
  src/pages/             browse, player, series, playlists, admin…
docs/                    product + architecture docs (multi-language)
scripts/                 demo media generator
```

## Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go (net/http, pgx/v5), JWT (HS256), bcrypt |
| Frontend | React 18, Vite, react-router, i18next, hls.js |
| Database | PostgreSQL 14 (embedded SQL migrations) |
| Media | ffprobe (metadata), ffmpeg (posters, HLS transcoding) |
| Docs | Markdown + Mermaid (GitHub-rendered) |

## Security

- All mutations are admin-only; media URLs require the user's JWT (header or `?token=`)
- Passwords hashed with bcrypt; roles reloaded from the DB on every request
- HLS segment names are validated and confined to the session directory
- SQL is parameterized throughout
- **Production**: set a strong `JWT_SECRET`, run behind HTTPS (reverse proxy),
  and use `ADMIN_USERNAME/ADMIN_PASSWORD` for the initial account

See also [SECURITY.md](SECURITY.md).

## Roadmap

- [x] Library scanning (parallel, cancelable, live progress)
- [x] Metadata + posters + TMDB scraping
- [x] Native playback + HLS transcoding
- [x] TV series auto-grouping (multiple naming patterns)
- [x] Favorites (videos & series), playlists, continue watching
- [x] i18n (en/zh/fr/ja/de)
- [ ] Filesystem watching for incremental indexing
- [ ] Adaptive-bitrate (multi-quality) HLS
- [ ] Embedded subtitle extraction / upload
- [ ] Public sharing with signed short-lived URLs

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE) — see [LICENSE](LICENSE).
