# Changelog

All notable changes to VideoCMS are documented here.

## [Unreleased]

### Added

- Filesystem watching for incremental indexing: new, changed, and removed
  files are picked up automatically (default every 30s, `WATCH_INTERVAL`),
  without a full rescan
- Admin content blocking by title: block media without deleting files/records,
  hidden for everyone, unblock anytime (`blocked_titles` table)
- Library-level blocking: block/unblock an entire library from the admin panel,
  hidden for everyone, nothing deleted (`libraries.blocked` column)
- Open a library folder on the server with the system file manager (one click)
- TV Shows: files named with only an episode number (e.g. `01.mkv`) are grouped
  under the containing directory name (or the library name for root-level
  files), so a series folder scans directly into the series list
- Project-level Codex skill (`.codex/skills/videocms/`) with environment
  wrapper, commands and conventions

## [0.1.0] — 2026-08-09

### Added

- Media library management: add/delete libraries by server path, built-in server
  folder picker, background scanning with live progress and cancel
- Parallel scanning (4 workers, `SCAN_WORKERS`); skips macOS `._` files and
  `.m3u8` stream folders; probe failures still index the file
- Metadata: ffprobe extraction, ffmpeg posters, editable title/year/synopsis/
  genres, SRT→WebVTT subtitles
- Optional TMDB scraping (search + details + poster download, rate-limited)
- Playback: HTTP Range streaming for H.264/WebM; HLS transcoding for MKV/HEVC
  (live-growing playlist, ~1s start, stable 6s segments, seek via restart)
- TV Shows: auto-grouping of numbered files (`S01E01`, `EP1`, `第1集`,
  `Show01Title`), seasons, play-all with continuous playback that keeps fullscreen
- Personal features: continue watching, video favorites, series favorites,
  playlists with sequential playback
- Users: JWT auth, admin/user roles, admin user management with safety guards
- Per-user hidden-path filters applied across all listings
- Series list ordered by newest import; library filter (like the home page)
- i18n: en / 中文 / Français / 日本語 / Deutsch (English default)
- Production mode: backend serves the built frontend on a single port (`make serve`)

### Fixed

- Sidecar subtitle selection is now deterministic when several subtitle files
  exist next to a video (`.srt` preferred) instead of a random one each scan
- ffprobe/ffmpeg resolution preferring the Homebrew build when the system copy
  is broken (missing libx265)
- Scan cancellation no longer marks untouched videos as missing
- Stale “scanning” status reset after a server restart
- Series name parsing for mid-title episode numbers; unicode-letter guard
- HLS manifest buffering with VOD playlists (long files could not start)
- NULL series fields breaking movie queries
- Fullscreen lost between auto-advanced episodes
