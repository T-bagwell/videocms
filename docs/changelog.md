# Changelog

All notable changes to VideoCMS are documented here.

## [Unreleased]

### Changed

- Video detail page layout polished: rating and comments now sit side by side
  in compact panels on desktop (stacked on mobile), and the action buttons
  were regrouped into a consistent toolbar with admin tools (scrape,
  transcription, subtitles, metadata editing) in a dedicated panel
- Video action buttons now use a consistent inline SVG icon set (play,
  favorite, playlist, download, share, offline) instead of mixed unicode
  symbols; button labels were cleaned up across all five locales
- The player page gained a Download button in its toolbar that opens the
  same track-picker dialog as the detail page (container, audio, subtitles,
  original file), so media can be downloaded without leaving playback
- Product manual screenshots refreshed with the current UI: icon-based action
  buttons, side-by-side rating/comments, the player download button, and
  clean posters for the demo media (previously some demo posters were frames
  of ffmpeg test patterns with large red regions)
- Fixed two CSS layout bugs on video cards: the generic progress-bar style
  was overriding the card progress bar (rendering a full-height red block on
  partially-watched posters) and the detail-page poster was being stretched
  by `height: 100%` instead of keeping its 16:9 ratio
- Product manual screenshots refreshed again: the video detail page now shows
  a richer demo poster (blurred video-frame backdrop with title), synopsis,
  tags, ratings and comments, and the demo library metadata was filled in for
  the featured demo title
- Roadmap expanded with a new backlog organized by capability area (playback &
  subtitles, media types & libraries, live TV & IPTV, discovery & automation,
  users & analytics, extensibility & operations): multi-version movies,
  trailers/featurettes, theme songs, chapters, AV1/VP9 and HDR passthrough,
  music/audiobook/book/photo libraries, IPTV channels with EPG, request
  workflows, quality-profile automation, watch-history sync, statistics
  dashboards, moderation tooling, plugin/scraper systems, OpenAPI docs,
  metrics, Docker/Helm packaging and distributed transcoding (README in
  en/zh/ja)
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
- Documentation overhaul: architecture docs (en/zh-CN/ja) refreshed with the
  full system diagram, ER diagram, schema listing, configuration table and
  extension points; product user manuals (5 languages) gained offline saving,
  parental PIN, smart collections and FAQ entries; the deployment guide now
  documents the expanded REST API surface plus DLNA, SAML and SMTP setups; the
  security policy covers DLNA allowlists, SAML keys, SMTP TLS and webhook
  signatures. The product manuals now ship real UI screenshots (browse, series,
  detail, player, share page, admin console) instead of placeholders

### Added

- Theme songs: admins attach an audio theme song per movie/TV entry (stored
  under `DATA_DIR/theme-songs/`); the detail page shows a preview player with
  the title and stream link for every user
- Trailers & featurettes: metadata scraping stores the official YouTube trailer
  on the video, the detail page plays it in an embedded modal, and admins can
  attach self-hosted featurette files (uploaded under `DATA_DIR/featurettes`)
  that are listed, streamed and removable from the same page
- Multi-version movies: files of the same film (1080p / 4K / extended cut /
  director's cut) are grouped automatically during scans, the best-scored copy
  is the one shown in listings and playback, and the detail page lists every
  version with a one-click switch to the player
- Email notifications: an SMTP channel (`SMTP_HOST`/`SMTP_PORT`/`SMTP_USER`/
  `SMTP_PASSWORD`, `NOTIFY_EMAIL_FROM`/`NOTIFY_EMAIL_TO`) delivers the same
  scan/download/upload events as plain-text mail, with implicit TLS on 465 or
  STARTTLS otherwise
- SAML 2.0 single sign-on: `GET /api/auth/saml/login` starts an AuthnRequest
  flow, `POST /api/auth/saml/acs` verifies the signed response (crewjam/saml)
  and `/api/auth/saml/metadata` publishes SP metadata; users bind via
  `users.oauth_sub` with a `saml:` prefix and can be granted admin through a
  `roles` attribute (`SAML_IDP_METADATA_URL`, `SAML_SP_CERT`, `SAML_SP_KEY`,
  `SAML_SP_ENTITY_ID`, `SAML_ACS_URL`)
- Casting: the player can cast to Chromecast (Cast SDK sender + short-lived
  share token), and `DLNA_ENABLED=1` exposes a lightweight UPnP media server
  (SSDP discovery, DIDL-Lite browse via GET/SOAP, direct `/dlna/video/{id}/stream`
  URLs; `DLNA_ALLOWED_IPS` restricts clients by IP/CIDR)
- Intro/credits skip: mark the start and end of the intro or credits in the
  player (two clicks each), then skip them with one tap; intervals are stored
  per video (`skip_intervals`) and exposed via
  `GET|PUT|DELETE /api/videos/{id}/skip-interval(s)`
- PWA: installable web app (manifest + service worker caching the app shell),
  offline saving of videos to the Cache API from the detail page, and
  safe-area mobile layout
- Webhooks + API docs: admin-managed webhook subscriptions deliver signed
  events (`X-Videocms-Signature` HMAC) with per-subscription event filters;
  `GET /api/openapi.json` exposes a lightweight OpenAPI 3 document
- Scheduled maintenance: `MAINT_INTERVAL_HOURS` (default 24) runs JSON backups
  (`MAINT_BACKUP_RETENTION`, default 7) plus per-library health checks, with
  optional rescans (`MAINT_RESCAN=1`); admins can trigger it manually and
  list/download backups (restore via the existing import)
- Background-jobs dashboard: a unified admin Jobs tab aggregates scans,
  uploads, downloads and live streams with progress/errors and contextual
  cancel/retry/start/stop actions, plus free/total disk stats
- Storage pools: admins define local/S3/SFTP pools (name, type, mount path,
  config, read-only); upload and yt-dlp targets can use `pool://name[/sub]`,
  routed through the pool's local mount (e.g. s3fs/sshfs)
- Notifications: webhook (`NOTIFY_WEBHOOK_URL`) and Apprise
  (`NOTIFY_APPRISE_URL`) channels deliver scan/download/upload events; admins
  can send a test notification
- Share page customization: theme (default/dark), custom title and
  hide-navigation options; `?embed=1` renders a chrome-free player for iframe
  embedding
- Parental controls: users can set a PIN and unlock rated content for
  5 minutes; admins set an allowed-rating policy per user and a content rating
  per video; libraries support a storage quota enforced on uploads
- OIDC single sign-on: the login page offers SSO; `OIDC_ISSUER` /
  `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` / `OIDC_REDIRECT_URL` configure the
  discovery + authorization-code flow, and users are auto-provisioned or
  linked by `oauth_sub`
- Comments, 1-5 star ratings (with averages) and a recent-activity feed
  (comments + favorites) on the browse page
- NFO metadata import/export: per-library export writes Kodi-style movie NFO
  files next to videos, and import applies title/year/plot/genres back into
  the database (Plex/Jellyfin/Kodi compatible)
- Batch edit/organize: admins select videos in the admin list to bulk-tag,
  clear tags or move to trash; the recycle bin lists trashed files (with
  original paths) and supports one-click restore
- Fuzzy full-text search: pg_trgm GIN indexes accelerate substring matching,
  and a new `sort=fuzzy` mode filters and ranks by trigram similarity so typos
  still find videos (title / synopsis / filename)
- User-defined tags (manual + AI), smart collections (named saved filter
  sets) and saved browse filters; collections and filters are stored per user
  and replay the same `/videos` filter parameters
- Similar-video recommendations (shared genres, year, series and tags) on the
  detail page, plus a tag cloud with `?tag=` filtering on the browse page
- Media health checks: admins run a per-library check that reports
  missing/corrupt files and duplicate candidates, then keep the best version
  while moving the rest into the server trash (`DATA_DIR/trash`)
- AI tagging: an optional external tagger (`AI_TAG_BIN`, one tag per stdout
  line) can label videos; tags live in the new `tags`/`video_tags` tables, are
  shown on the detail page, filter search via `?tag=`, and can be managed
  manually
- Pluggable metadata sources: `SCRAPE_CUSTOM_URL` provides a custom JSON
  scraper (with a `%s` title placeholder) selectable per video alongside TMDB,
  and `POST /api/videos/{id}/scrape` supports `?provider=` and `?force=` for
  per-item override
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
