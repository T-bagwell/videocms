# Changelog

All notable changes to VideoCMS are documented here.

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

- ffprobe/ffmpeg resolution preferring the Homebrew build when the system copy
  is broken (missing libx265)
- Scan cancellation no longer marks untouched videos as missing
- Stale “scanning” status reset after a server restart
- Series name parsing for mid-title episode numbers; unicode-letter guard
- HLS manifest buffering with VOD playlists (long files could not start)
- NULL series fields breaking movie queries
- Fullscreen lost between auto-advanced episodes

