# 🎬 VideoCMS

> **Self-hosted video resource management** — Go · React · PostgreSQL

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14-4169E1?logo=postgresql&logoColor=white)
![i18n](https://img.shields.io/badge/i18n-5%20languages-8A2BE2)

**Languages:** English · [中文](docs/README.zh-CN.md) · [日本語](docs/README.ja.md)

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
| ▶️ Playback | H.264/WebM play natively (HTTP Range); **MKV/HEVC transcoded to adaptive multi-quality HLS on the fly** (quality selector); subtitles auto-detected (SRT→WebVTT), embedded-subtitle extraction, upload, **multi-language switching** and per-user subtitle preference; **download as MKV/MP4 with a chosen audio track and subtitles (remuxed, no re-encode)** |
| ⬆️ Uploads & downloads | Admin **Uploads** tab: chunked, resumable uploads into any server folder (auto-indexed inside libraries); **yt-dlp** download queue with optional scheduled repeats |
| 🔗 Sharing | Short-lived public share links for **videos, TV shows and playlists** (signed, expiring, revocable, optional password and domain allow-list) — anyone with the link can watch without an account; content blocking is respected |
| 👤 Personal | Continue watching, favorites (videos **and** series), playlists with sequential playback |
| 🔐 Users | Register/login with JWT; admin/user roles; admin user management with safety guards |
| 🚫 Content blocking | Admins block media by title in the admin panel — hidden for everyone, files and records kept, unblock anytime |
| 🚫 Library blocking | Block an entire library from the admin panel — all its media vanishes for everyone, nothing is deleted |
| 🚫 Path filters | Hide any server path per user — excluded everywhere (home, series, favorites, continue watching, playlists) |
| 🌐 Interface | i18n: **English (default), 中文, Français, 日本語, Deutsch** |

## Content Controls

Three independent layers decide who sees what — files and records are never
touched by any of them:

| Layer | Who manages | Scope | Where it applies |
| --- | --- | --- | --- |
| 🏷️ Title blocking | Admin | Videos whose title contains the blocked text | Every listing for everyone |
| 📚 Library blocking | Admin | An entire library (all its videos) | Every listing for everyone |
| 🛤️ Path filters | Each user | Any server path the user chooses | Every listing for that user only |

All three are evaluated in SQL on every listing (home, TV shows, favorites,
continue watching, playlists), so a blocked item disappears everywhere at once
and reappears immediately when unblocked.

## Screenshots

![Home](docs/screenshots/home.png)
![TV Shows](docs/screenshots/series.png)
![Video detail](docs/screenshots/detail.png)
![Player](docs/screenshots/player.png)

## Documentation

All documentation is multi-language. Start at the **[docs index](docs/INDEX.md)**:

| Doc | Languages | Audience |
| --- | --- | --- |
| [Product documentation](docs/product.md) | EN · 中文 · FR · JA · DE | End users |
| [System architecture](docs/architecture.md) | EN · 中文 · JA | Developers |
| [Deployment](docs/deployment.md) | EN · 中文 · JA | Operators |
| [README](README.md) / [中文](docs/README.zh-CN.md) / [日本語](docs/README.ja.md) | EN · 中文 · 日本語 | Everyone |

## Quick Start

### Requirements

- Go 1.26+ (to build) or a prebuilt binary
- PostgreSQL 14+
- ffmpeg + ffprobe (metadata, posters, transcoding)
- yt-dlp (optional — powers the admin Downloads queue)
- Node.js 20+ (frontend development only — the built UI is served by the backend)

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

To run the backend as an API-only service and host the frontend separately
(nginx or any static server), see [deployment.md](docs/deployment.md).

Log in with the initial admin **admin / admin123** and change the password
immediately (Admin → Users → Reset password). Then add your first library under
**Admin → Libraries → Scan** (the path must be an absolute server path, e.g.
`/media/movies`).

You can also drop files straight into a server folder from **Admin → Uploads**
(chunked, resumable — files inside a library are indexed automatically), or
queue videos from the web with **Admin → Downloads** (yt-dlp, optional
scheduled repeats).

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
| `YTDLP_PATH` | `yt-dlp` on PATH | yt-dlp binary used by the Downloads queue |
| `TMDB_API_KEY` / `TMDB_LANGUAGE` | empty / zh-CN | Metadata scraping; without a key the free TVMaze, AniList and Wikipedia APIs are used |
| `TVMAZE_ENABLED` | `1` | Set `0` to disable the keyless TVMaze metadata fallback |
| `ANILIST_ENABLED` | `1` | Set `0` to disable the keyless AniList metadata fallback |
| `WIKIPEDIA_LANG` / `WIKIPEDIA_ENABLED` | `en` / `1` | Language edition and switch for the keyless Wikipedia fallback |
| `SCAN_WORKERS` | `4` | Parallel scan workers (1–16) |
| `WATCH_INTERVAL` | `30` | Fallback interval for incremental scans (fsnotify events index immediately); `0` disables watching |
| `HLS_HW_ACCEL` | empty (software x264) | HLS video encoder: `videotoolbox`, `nvenc`, `qsv` or `vaapi`; empty uses libx264 |
| `HLS_VAAPI_DEVICE` | `/dev/dri/renderD128` | VAAPI render device (used with `HLS_HW_ACCEL=vaapi`) |
| `HLS_TONE_MAP` | `0` | Set `1` to enable HDR→SDR tone mapping in HLS transcoding |
| `SUBTITLE_OS_USERNAME` / `SUBTITLE_OS_PASSWORD` / `SUBTITLE_OS_API_KEY` | empty | OpenSubtitles credentials for the online subtitle search |
| `RTMP_INGEST_URL` | `rtmp://localhost:1935/live` | Base RTMP ingest URL (nginx-rtmp or equivalent); streams append their key |
| `WHISPER_BIN` / `WHISPER_MODEL` | empty | whisper.cpp CLI and model path for speech transcription |
| `SCRAPE_CUSTOM_URL` | empty | Custom JSON scraper endpoint; `%s` is replaced with the URL-escaped title |
| `AI_TAG_BIN` | empty | External AI tagging tool; receives the media path and prints one tag per line |
| `WEB_ROOT` | auto (`frontend/dist`) | Built frontend for single-service mode; leave unset for API-only deployment |
| `CORS_ORIGINS` | empty (`*`) | Comma-separated browser origins allowed to call the API (separate frontend deployments) |
| `VITE_API_BASE_URL` | empty | Frontend build-time API base URL for cross-origin deployments (runtime override: `window.__VIDEOCMS_API_BASE__`) |

## Project Structure

```
backend/                 Go server (net/http + pgx)
  cmd/server/            entry point
  internal/api/          HTTP handlers, routes, middleware
  internal/auth/         JWT + role middleware
  internal/media/        scanner, TMDB scraper, HLS manager, streaming
  internal/db/           pool + embedded SQL migrations
  internal/models/       domain types
frontend/                React 19 SPA (Vite)
  src/i18n/locales/      en / zh / fr / ja / de
  src/pages/             browse, player, series, playlists, admin…
docs/                    all documentation (multi-language)
scripts/                 demo media generator
```

## Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go (net/http, pgx/v5), JWT (HS256), bcrypt |
| Frontend | React 19, Vite 8, react-router, i18next, hls.js |
| Database | PostgreSQL 14 (embedded SQL migrations) |
| Media | ffprobe (metadata), ffmpeg (posters, HLS transcoding) |
| Docs | Markdown + Mermaid (GitHub-rendered) |
| Quality | ESLint 10, Vitest 4, golangci-lint (linting & tests in CI) |

## Security

- All mutations are admin-only; media URLs require the user's JWT (header or `?token=`)
- Passwords hashed with bcrypt; roles reloaded from the DB on every request
- HLS segment names are validated and confined to the session directory
- SQL is parameterized throughout
- **Production**: set a strong `JWT_SECRET`, run behind HTTPS (reverse proxy),
  and use `ADMIN_USERNAME/ADMIN_PASSWORD` for the initial account

See also [security.md](docs/security.md).

## Roadmap

Planned work is informed by feature sets of similar self-hosted video projects
(Jellyfin, MediaCMS, Stash, Kirari04/videocms, yt-dlp tools, …).

### Done

- [x] Library scanning (parallel, cancelable, live progress) + event-driven
  filesystem watching
- [x] Metadata + posters: TMDB scraping with keyless TVMaze fallback
- [x] Native playback + adaptive-bitrate HLS transcoding
- [x] TV series auto-grouping (S01E01, EP1, 第N集, number-only filenames…)
- [x] Favorites (videos & series), playlists, continue watching
- [x] Content controls: title blocking, library blocking, per-user path filters
- [x] Subtitles: embedded extraction, upload, multi-language tracks, per-user
  preference
- [x] Public sharing: signed short-lived links for videos, series and
  playlists, with optional password and domain allow-list
- [x] i18n (en/zh/fr/ja/de)
- [x] Admin: data export/backup, open library folder, server directory picker

### Planned

**Upload & download**

- [x] Chunked, resumable browser uploads with a queue-based upload manager
- [x] Download as MP4/MKV with selectable audio/subtitle tracks (no re-encode)
- [x] yt-dlp integration: pull videos/channels from online sites on a schedule

**Playback & subtitles**

- [x] Multiple audio tracks with in-player switching (separate HLS audio tracks)
- [x] Styled (ASS) soft subtitles
- [x] Subtitle sync / offset controls (direct playback)
- [x] Automatic subtitle download & matching
- [x] Hardware-accelerated transcoding (VAAPI/NVENC/QSV) + HDR tone mapping
- [x] Trick-play thumbnails / preview timeline
- [ ] Intro & credits skip
- [x] Watch together (synchronized sessions)
- [x] Casting (Web AirPlay)
- [ ] Casting (Chromecast / DLNA)
- [x] Live streaming ingest (RTMP) with built-in chat

**Metadata & AI**

- [x] Local speech transcription (Whisper) → searchable transcripts and captions
- [x] Pluggable metadata sources / custom scrapers with per-item override
- [x] AI tagging, scene detection and image analysis for smarter search
- [ ] Media health checks: duplicate detection, corrupt-file checks,
  keep-best-version cleanup
- [ ] Similar-video recommendations and tag cloud

**Organization & search**

- [ ] User-defined tags, smart collections, saved filters
- [ ] Full-text/fuzzy search over titles, synopsis, transcripts and tags
- [ ] Batch edit/organize (move, rename, re-tag) with recycle bin
- [ ] NFO metadata import/export for Plex/Jellyfin/Kodi compatibility

**Users, sharing & social**

- [ ] Comments, ratings and activity feeds
- [ ] OIDC/SAML single sign-on
- [ ] Parental controls (PIN / content ratings) and per-user quotas
- [ ] Share page customization and embeddable players
- [ ] Notifications (email / webhook / Apprise) for scan, upload and transcode events

**Storage & operations**

- [ ] Storage pools: local, S3-compatible and SFTP, routed from the admin UI
- [ ] Background-jobs dashboard (monitor / cancel / retry) + richer system stats
- [ ] Scheduled maintenance: re-scan, health checks, metadata backup/restore
- [ ] Webhooks + mature public REST API for third-party automation
- [ ] PWA with offline downloads and polished mobile UX

## Contributing

Contributions are welcome! See the contribution guide:

- English: [contributing.md](docs/contributing.md)
- 中文: [contributing.zh-CN.md](docs/contributing.zh-CN.md)
- 日本語: [contributing.ja.md](docs/contributing.ja.md)

## License

[Apache License 2.0](LICENSE) — see [LICENSE](LICENSE).
