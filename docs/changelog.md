# Changelog

All notable changes to VideoCMS are documented here.

## [Unreleased]

### Changed

- Documentation restructured: all docs moved into `docs/` (READMEs,
  contributing guides, changelog, security policy); the docs index is now
  `docs/INDEX.md` and the root README is a short landing page

### Added

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
