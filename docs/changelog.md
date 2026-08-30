# Changelog

All notable changes to VideoCMS are documented here.

## [Unreleased]

### Changed

- Frontend toolchain upgraded: React 19 / react-dom 19, Vite 8 +
  @vitejs/plugin-react 6, react-router-dom 7, hls.js 1.7.1, i18next 26.4,
  react-i18next 17.0.12, jsdom 27.3; Node.js 20+ is now required for frontend
  development
- Backend dependencies updated (golang.org/x/crypto 0.55, fsnotify 1.10.1)
- Documentation restructured: all docs moved into `docs/` (READMEs,
  contributing guides, changelog, security policy); the docs index is now
  `docs/INDEX.md` and the root README is a short landing page
- Roadmap expanded with planned capabilities (uploads/downloads, playback &
  subtitles, metadata & AI, organization & search, sharing & social, storage &
  operations), benchmarked against similar self-hosted video projects

### Added

- Local speech transcription: admins run whisper.cpp on a video
  (`WHISPER_BIN` / `WHISPER_MODEL`) to generate a WebVTT transcript that is
  searchable and selectable as a subtitle track
- Live streaming: admins create RTMP streams (`RTMP_INGEST_URL` + per-stream
  key), the server pulls the ingest into a rolling HLS playlist, and a live
  watch page includes a built-in polling chat
- Watch together: create or join a synchronized watch room (shared id +
  token) and keep play/pause/position in sync across viewers via polling; a
  Cast / AirPlay button opens the native casting UI where supported
- VAAPI hardware encoding (`HLS_HW_ACCEL=vaapi`, device via `HLS_VAAPI_DEVICE`)
  and HDR→SDR tone mapping (`HLS_TONE_MAP=1`) round out hardware-accelerated
  HLS transcoding
- Automatic subtitle download & matching: admins search online subtitle
  providers (OpenSubtitles.com) by title + year, optionally filtered by
  language, and download a candidate straight into the video's subtitle
  tracks (config: `SUBTITLE_OS_USERNAME` / `SUBTITLE_OS_PASSWORD` /
  `SUBTITLE_OS_API_KEY`)
- Styled (ASS) soft subtitles: ASS/SSA tracks are rendered with a libass WASM
  overlay (jassub) preserving fonts, colors, positioning and effects; the
  renderer follows the per-user subtitle offset
- Trick-play previews: hovering the seek strip shows the thumbnail frame
  nearest the pointer (one 160×90 frame every 10 seconds, generated on demand
  per video) and clicking seeks to that time
- Subtitle sync: players can nudge subtitles by ±0.5s (saved per user per
  video), and the subtitle endpoint serves shifted WebVTT for direct playback
- Hardware-accelerated HLS transcoding: `HLS_HW_ACCEL=videotoolbox|nvenc|qsv`
  switches the HLS video encoder from software x264 to a GPU encoder
- Multi-audio HLS playback: videos with several audio streams get a separate
  HLS audio track per stream (`EXT-X-MEDIA` AUDIO group), and the player shows
  an audio track selector so you can switch without restarting playback
- Frontend API base URL is configurable for separate deployments: build with
  `VITE_API_BASE_URL` or inject `window.__VIDEOCMS_API_BASE__` at runtime
- Configurable CORS: `CORS_ORIGINS` (comma-separated) restricts which browser
  origins may call the API for separate frontend deployments; empty defaults
  to `*` (token auth). Range streaming now also exposes `Accept-Ranges`,
  `Content-Range` and `Content-Length` headers cross-origin
- yt-dlp integration: admins can queue video/playlist/channel URLs from a new
  Downloads tab, choose a target folder and format, repeat downloads on a
  schedule, and cancel or retry jobs; progress is live and finished files are
  indexed automatically when the target is inside a library
- Configurable downloads: videos can be downloaded as **MP4 or MKV** with a
  chosen audio track plus any embedded or uploaded subtitles — remuxed on the
  fly with no re-encoding from the video page's Download dialog
- Chunked, resumable uploads: admins can upload files into any server folder
  from a new Uploads tab with a queue-based manager; large files are split into
  chunks that survive pauses and network errors, and finished files inside a
  library are indexed automatically by the file watcher
- CI: backend tests now run against PostgreSQL (integration tests), plus
  golangci-lint, frontend ESLint + Vitest tests, CodeQL scanning, Dependabot,
  and a release workflow for cross-platform binaries
- Expanded contributing guide (English, 中文, 日本語) covering development
  setup, repository conventions, testing, CI and the pull request workflow
- Filesystem watching for incremental indexing: new, changed, and removed
  files are picked up automatically (default every 30s, `WATCH_INTERVAL`),
  without a full rescan
- Adaptive-bitrate HLS: transcoded playback now generates a multi-quality
  ladder (up to 1280px, capped by source resolution) with a master playlist
  and a quality selector in the player
- Embedded subtitle extraction: text subtitle streams inside MKV/MP4 are
  extracted to WebVTT automatically during scanning (and on demand from the
  video page); image-based tracks (PGS, VobSub…) are skipped
- Subtitle upload: admins can upload `.srt/.vtt/.ass/.ssa` subtitles for any
  video, replace or remove them
- Player subtitles: sidecar, embedded and uploaded subtitles are now displayed
  in the player (native `<track>` for direct playback, HLS subtitle group for
  transcoded playback, with a subtitle toggle)
- Public sharing: any user can create short-lived, revocable share links
  (default 7 days, 1 hour–1 year) that play in a public page without an
  account; share links respect admin title/library blocking and expire server-side
- Sharing extended to **TV shows and playlists**: share links for a series
  play every episode in order, playlist links play the playlist sequence
- Multi-language subtitle tracks: all sidecar subtitle files and every embedded
  text track are now listed, extractable and switchable in the player (native
  `track` menu or an HLS subtitle group with per-track playlists)
- Filesystem watching is now event-driven (fsnotify, recursive per library):
  new, changed, removed and same-size-modified files are indexed within
  seconds, with the periodic diff-based pass kept as a fallback
- Data export/backup: admins can download a full JSON backup (users, libraries,
  videos, series, playlists, personal data, content controls) and every user
  can export their own favorites, progress, hidden paths and playlists
- Keyless metadata fallback: when `TMDB_API_KEY` is empty, scraping uses the
  free TVMaze API (`TVMAZE_ENABLED=0` disables it)
- README screenshots replaced the "coming soon" placeholder
- Per-user subtitle preference: each user can pick their own subtitle track per
  video (saved in `user_subtitle_prefs`); admins can also set the global default
- Share links can be password-protected: an optional password is bcrypt-hashed
  at creation time and required on every public endpoint (header or `?pw=`)
- Share links can be restricted to allowed domains: the request host (or
  Origin) must match the list set at creation time
- Expired share tokens are deleted automatically every hour
- Third keyless metadata provider: AniList (anime/animation) is tried after
  TVMaze when `TMDB_API_KEY` is empty (`ANILIST_ENABLED=0` disables it)
- Fourth keyless metadata provider: Wikipedia (generic last-resort fallback,
  `WIKIPEDIA_LANG` selects the language edition, `WIKIPEDIA_ENABLED=0` disables)
- Watcher fix: subtitle sidecar changes now resync the sibling video's subtitle
  tracks instead of being probed as video files
- Backup restore: admins can import a backup exported by the admin export
  endpoint (libraries/videos/series upserted by path/name; personal data
  restored for existing users)
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

### Fixed

- Hardened upload target validation: the admin upload/download endpoints now
  guard server paths with a CodeQL-recognized absolute-path check before any
  filesystem call (path-injection scan clean)
- Cancelling a yt-dlp job between queue claim and process start no longer
  lets it run to completion
- The initial admin password is no longer printed in plain text in server logs
- Creating a library now requires an absolute server path; relative paths are
  rejected with a clear error instead of being resolved against the working
  directory
- The admin directory browser normalizes user-supplied paths to a clean
  absolute path, so `..` segments and relative input can never escape the
  filesystem root (path-injection hardening)

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
