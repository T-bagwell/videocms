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

- A server (macOS/Linux/Windows) with Go 1.26+ to build, or a prebuilt binary
- PostgreSQL 14+
- ffmpeg/ffprobe (for metadata, posters and transcoding)
- Node.js 20+ (only for frontend development; the built UI is served by the backend)

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
2. Enter a name, then type an **absolute** server folder path (e.g.
   `/media/movies`) or click **Browse…** to pick one — relative paths are rejected
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
- Multiple audio tracks can be switched during HLS playback (audio selector
  in the player)
- Subtitles can be nudged ±0.5s to fix sync during direct playback; the offset
  is saved per user and video
- Hovering the seek strip shows thumbnail previews while scrubbing, and
  clicking the strip jumps to that position
- ASS/SSA subtitles render with their original styling (fonts, colors,
  positioning and effects)
- Admins can search and download subtitles from online providers (e.g.
  OpenSubtitles) per video
- **Watch together**: create or join a room to keep playback synchronized with
  friends; a **Cast / AirPlay** button is available where the browser supports it
- **Live streaming**: admins create RTMP streams (OBS-compatible ingest URL)
  and viewers watch with a built-in chat
- Admins can run **speech transcription** (Whisper) on a video; the transcript
  is searchable and selectable as a subtitle track
- **Download** a remuxed copy as MKV or MP4 with a chosen audio track and
  subtitles (no re-encode), or grab the original file for offline playback
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
- Subtitles (sidecar, embedded or uploaded) can be toggled in the player,
  switched between languages when several tracks exist, and each user can set
  their own default per video (admins can set the global default)
- On a phone, rotate to landscape for the best playback experience; use the
  queue below the player to jump between episodes

### 4.6 Sharing

- Open a video, TV show or playlist and press **Share** to create a link
  (default 7 days, valid from 1 hour up to 1 year)
- Anyone with the link can watch on a public page — no account needed; TV show
  and playlist links play through the whole queue
- Links expire automatically and can be revoked anytime from the same dialog
- Links can be password-protected — viewers are asked for the password on the
  public page before anything loads
- Links can be restricted to an allow-list of domains — requests from other
  hosts are rejected
- Shared content still respects admin controls: blocked titles and blocked
  libraries never show up in share links

## 5. Admin Guide

### 5.1 Overview

Stats: videos, libraries, users, playlists, favorites, series and storage used.

- **Export / import backup**: download a full JSON backup of the server
  metadata, or restore one (libraries and videos are upserted by path; personal
  data is restored for existing users)

### 5.2 Libraries

- Add libraries with a name and an absolute server path (folder picker included)
- **Uploads** tab: chunked, resumable uploads into any server folder (e.g. a
  library folder) — finished files are indexed automatically
- **Scan** indexes new/changed files; **Stop scan** cancels; progress is live
- Deleting a library removes its video records — files on disk are kept
- **Open folder** opens the library directory on the server with the system
  file manager, so you can inspect or manage the actual media files
- **Health check** reports missing/corrupt files and duplicates; **Keep best**
  moves the rest into the server trash
- **Export NFO / Import NFO** reads and writes Kodi-style metadata files next
  to the videos (Plex/Jellyfin/Kodi compatible)
- **Block library** hides the whole library for every user (home, series,
  favorites, continue watching, playlists) without deleting anything;
  unblock restores it immediately

### 5.3 Videos

- Search any video and edit title/year/genres/synopsis
- Upload a custom poster
- **Download** opens a dialog to save the video as MKV or MP4 with a chosen
  audio track and embedded/uploaded subtitles (remuxed without re-encoding)
- **Scrape** fetches metadata from TMDB (requires `TMDB_API_KEY` and network
  access to api.themoviedb.org)
- Admins can pick **TMDB or a custom provider** for scraping and force an
  overwrite of existing metadata
- Videos can carry **tags** (manual or from an optional AI analysis tool);
  tags are shown on the detail page and can filter search
- The detail page shows **similar videos**; the browse page has a **tag cloud**
  for one-click filtering
- The browse page can **save filters**, re-apply them, and create named
  **smart collections** from the current filter set
- Search offers a **fuzzy relevance sort** that tolerates typos
  (title / synopsis / filename)
- The admin video list supports **batch actions** (tag, clear tags, move to
  trash) and a **recycle bin** with one-click restore
- Users can **comment and rate** videos (1-5 stars); the home page shows a
  recent-activity feed of comments and favorites
- **Single sign-on** (OIDC): the login page offers an SSO button when the
  server is configured with an identity provider
- **Parental controls**: admins set an allowed-rating policy per user and a
  content rating per video; users can lock with a PIN and unlock rated content
  for 5 minutes. Libraries can also carry a **storage quota** enforced on
  uploads
- **Share customization**: pick a theme, a custom title and hide navigation;
  appending `?embed=1` embeds a chrome-free player in any page
- **Notifications**: webhook or Apprise channels can receive scan, upload and
  download events; admins can send a test notification from the overview
- **Storage pools**: define local, S3-compatible or SFTP pools with a local
  mount path; uploads and downloads can target `pool://name[/sub]`
- **Jobs dashboard**: one place to watch scans, uploads, downloads and live
  streams (progress, errors, cancel/retry/start/stop) plus disk usage
- **Scheduled maintenance**: automatic JSON backups and health checks
  (interval and retention configurable); admins can run it now and download
  backups

### 5.4 Users

- Change roles (user/admin), reset passwords, delete accounts
- Safeguards: you cannot delete your own account, and the last admin cannot be
  deleted or demoted

### 5.5 Blocked content

- Block any media by title (e.g. a show name) without deleting files or records
- Blocked titles are hidden for every user across home, series, favorites,
  continue watching and playlists — unblocking restores them immediately
- Use the search box to preview which media a title would block before adding it

### 5.6 Downloads (yt-dlp)

- Queue any video/playlist/channel URL; pick a target folder, a yt-dlp format
  and an optional repeat interval (hours) — the server downloads via yt-dlp
- Live progress, cancel, and retry failed jobs
- Finished files land in the chosen folder and are indexed automatically if
  it is inside a library

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
| `YTDLP_PATH` | `yt-dlp` on PATH | yt-dlp binary used by the Downloads queue |
| `HLS_HW_ACCEL` | empty (software x264) | HLS video encoder: videotoolbox, nvenc, qsv or vaapi; empty uses libx264 |
| `HLS_VAAPI_DEVICE` | `/dev/dri/renderD128` | VAAPI render device (with `HLS_HW_ACCEL=vaapi`) |
| `HLS_TONE_MAP` | `0` | `1` enables HDR→SDR tone mapping in HLS transcoding |
| `SUBTITLE_OS_USERNAME` / `SUBTITLE_OS_PASSWORD` / `SUBTITLE_OS_API_KEY` | empty | OpenSubtitles credentials for the online subtitle search |
| `RTMP_INGEST_URL` | `rtmp://localhost:1935/live` | Base RTMP ingest URL (nginx-rtmp or equivalent) |
| `WHISPER_BIN` / `WHISPER_MODEL` | empty | whisper.cpp CLI and model for speech transcription |
| `SCRAPE_CUSTOM_URL` | empty | Custom JSON scraper endpoint; `%s` is replaced with the URL-escaped title |
| `AI_TAG_BIN` | empty | External AI tagging tool (media path argument, one tag per line) |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` / `OIDC_REDIRECT_URL` | empty | OIDC single sign-on settings |
| `NOTIFY_WEBHOOK_URL` / `NOTIFY_APPRISE_URL` | empty | Notification channels (JSON webhook, Apprise API) |
| `MAINT_INTERVAL_HOURS` / `MAINT_BACKUP_RETENTION` / `MAINT_RESCAN` | `24` / `7` / `0` | Maintenance schedule, backup retention, rescan flag |

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
