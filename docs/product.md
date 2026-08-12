# VideoCMS — Product Documentation

> **Languages:** English | [中文](product.zh-CN.md) | [Français](product.fr.md) | [日本語](product.ja.md) | [Deutsch](product.de.md)

## 1. What is VideoCMS?

VideoCMS is a self-hosted video resource management system. Point it at folders on
your server disk, scan them once, and every video becomes part of a browsable,
searchable media library — with posters, metadata, watch progress, favorites,
playlists and an optional “TV Shows” category for numbered episodes.

It is built with **Go + React + PostgreSQL** and runs entirely on your own
hardware. Your videos never leave your network unless you choose to share them.

## 2. Features

| Area | What you get |
| --- | --- |
| Media libraries | Any server folder; add by path or with the built-in folder picker |
| Scanning | Automatic discovery of mp4/mkv/webm/avi/mov/ts… with live progress and cancel |
| Metadata | Codec/resolution/duration from ffprobe, posters generated from the video, editable title/year/synopsis/genres |
| TMDB scraping | Optional online enrichment: localized titles, synopsis, genres and posters |
| Playback | In-browser H.264/WebM playback; automatic HLS transcoding for MKV/HEVC |
| TV Shows | Numbered files (S01E01, EP1, 第1集…) auto-grouped into series sorted by episode |
| Personal | Continue watching, favorites, playlists with sequential playback |
| Users | Register/login, admin roles, admin user management |
| Content blocking | Admins block media by title — hidden for everyone, files and records kept, unblock anytime |
| Interface | 5 languages: English (default), 中文, Français, 日本語, Deutsch |

## Screenshots

> *Coming soon — run `make serve` and open `http://<server-ip>:8080` to see the UI
> in action. The web player, series pages and admin console are all covered in
> the [product tour](product.md).*

## 3. Quick Start

### Requirements

- A server (macOS/Linux/Windows) with Go 1.22+ to build, or a prebuilt binary
- PostgreSQL 14+
- ffmpeg/ffprobe (for metadata, posters and transcoding)
- Node.js 18+ (only for frontend development; the built UI is served by the backend)

### Install

```bash
createdb videocms                       # or: docker compose up -d db
cd backend && go run ./cmd/server       # first start creates tables + admin/admin123
cd frontend && npm install && npm run dev   # http://localhost:5173
```

For LAN/phone access, build once and use a single port:

```bash
make serve                              # http://<LAN-IP>:8080
```

### First login

Open the web UI and sign in with the initial administrator **admin / admin123**
(change the password right away — Admin → Users → Reset password).

### Add your first library

1. Go to **Admin → Libraries**
2. Enter a name, then type a server folder path or click **Browse…** to pick one
3. Click **Scan** — the count updates live; videos appear on the home page

## 4. User Guide

### 4.1 Browsing & search

- The home page shows Continue Watching, TV Shows and the video grid
- Search matches title, synopsis and filename
- Filter by library and type (All / Movies / TV Shows), sort by title, year,
  duration, date added or popularity
- “Load more” paginates the grid

### 4.2 Playing videos

- Click any video for the detail page (poster, metadata, synopsis)
- **Play** starts playback; if you have progress it resumes automatically
- H.264 MP4 / WebM play natively; MKV/HEVC are transcoded on the fly
  (first playback takes a few seconds; “Transcode play” is offered as a fallback)
- Download is available for offline/local playback
- Progress is saved every 5 seconds and on pause/end — visible in Continue Watching

### 4.3 Favorites & playlists

- ☆ Favorite on the detail page; manage them under **My Favorites**
- Create playlists from the **Playlists** page or from any video (“Add to playlist”)
- A playlist can be played in order (▶ Play all), with a visible queue

### 4.4 TV Shows

Videos with episode markers in their filenames are grouped automatically after a
scan. Supported patterns: `S01E01`, `E01`, `EP1`, `第1集`, trailing numbers such
as `1 (4)` or `-535`. A group needs at least 2 episodes to become a series.

- The **TV Shows** page lists all series with posters and episode counts
- A series page shows episodes sorted by season → episode
- Series re-group on every scan; a series with fewer than 2 available episodes
  is cleaned up automatically

### 4.5 Playback tips

- The player uses the browser's native video controls — **Space** to play/pause,
  **F** for fullscreen, **←/→** to seek, **↑/↓** for volume, **M** to mute
- Watch progress resumes automatically; "Continue Watching" on the home page
  picks up where you left off
- MKV/HEVC files are transcoded to HLS — the first playback takes a few seconds,
  then seeking and next-episode autoplay work as usual
- Transcoded playback uses an adaptive multi-quality ladder — use the quality
  selector above the player to switch (Auto by default)
- Subtitles (sidecar, embedded or uploaded) can be toggled in the player
- On a phone, rotate to landscape for the best playback experience; use the
  queue below the player to jump between episodes

### 4.6 Sharing

- Open a video and press **Share** to create a link (default 7 days, valid from
  1 hour up to 1 year)
- Anyone with the link can watch the video on a public page — no account needed
- Links expire automatically and can be revoked anytime from the same dialog
- Shared content still respects admin controls: blocked titles and blocked
  libraries never show up in share links

## 5. Admin Guide

### 5.1 Overview

Stats: videos, libraries, users, playlists, favorites, series and storage used.

### 5.2 Libraries

- Add libraries with a name and a server path (folder picker included)
- **Scan** indexes new/changed files; **Stop scan** cancels; progress is live
- Deleting a library removes its video records — files on disk are kept
- **Open folder** opens the library directory on the server with the system
  file manager, so you can inspect or manage the actual media files
- **Block library** hides the whole library for every user (home, series,
  favorites, continue watching, playlists) without deleting anything;
  unblock restores it immediately

### 5.3 Videos

- Search any video and edit title/year/genres/synopsis
- Upload a custom poster
- **Scrape** fetches metadata from TMDB (requires `TMDB_API_KEY` and network
  access to api.themoviedb.org)

### 5.4 Users

- Change roles (user/admin), reset passwords, delete accounts
- Safeguards: you cannot delete your own account, and the last admin cannot be
  deleted or demoted

### 5.5 Blocked content

- Block any media by title (e.g. a show name) without deleting files or records
- Blocked titles are hidden for every user across home, series, favorites,
  continue watching and playlists — unblocking restores them immediately
- Use the search box to preview which media a title would block before adding it

## 6. Configuration

All settings are environment variables (see the README for the full table). The
most important ones:

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Listen port |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | Database connection |
| `JWT_SECRET` | dev value | **Set a strong secret in production** |
| `TMDB_API_KEY` | empty | Enables metadata scraping |
| `SCAN_WORKERS` | `4` | Parallel scanning workers |
| `WATCH_INTERVAL` | `30` | Seconds between automatic incremental scans; `0` disables |

## 7. FAQ

**Why does my library show “Scanning” for a long time?**
The first scan of a large folder takes a while (ffprobe + poster generation per
file). Progress is live, and you can Stop scan at any time. Rescans only touch
changed files and finish much faster. macOS `._` files and `.m3u8` segment
folders are skipped automatically.

**Why can’t my browser play an MKV/HEVC file?**
Browsers only decode H.264/VP9. VideoCMS transcodes MKV/HEVC to HLS on the fly,
or you can download the file and play it locally.

**How are TV Shows detected?**
Filenames with `S01E01`, `E01`, `EP1`, `第1集` or trailing numbers, grouped by a
common name prefix. At least 2 episodes per group. If your naming scheme is not
recognized, adjust the filenames or ask the maintainer for a new pattern.

**TMDB scraping fails.**
Check that `TMDB_API_KEY` is set and that the server can reach
api.themoviedb.org. On restricted networks scraping will not work; everything
else is unaffected.

**How do I access VideoCMS from my phone?**
Both devices must be on the same network. Use `make serve`, find the server IP
with `ipconfig getifaddr en0`, and open `http://<ip>:8080`. Allow the macOS
firewall prompt on first run.

**Is it safe for public access?**
The default deployment is plain HTTP with a development JWT secret. For anything
exposed beyond a trusted LAN, use an HTTPS reverse proxy and set `JWT_SECRET`.

**How do I hide content without deleting files?**
Admins can block a media title (**Admin → Blocked**) or an entire library
(**Admin → Libraries → Block library**); blocked content vanishes for every
user everywhere and returns immediately when unblocked. Regular users can also
hide any server path for themselves via the path filter.

## 8. Technology & License

Go + React + PostgreSQL, with ffmpeg for media processing. The interface is fully
localized (en/zh/fr/ja/de). Licensed under the **Apache License 2.0**.

For internals, see the [system architecture](architecture.md).
