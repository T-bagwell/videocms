---
name: videocms
description: Develop, debug, or extend the VideoCMS media server (Go backend in backend/, React+Vite frontend in frontend/, PostgreSQL). Use for builds, tests, migrations, media scanning, series grouping, HLS playback, i18n, docs, or Git/GitHub workflows in this repository.
license: Apache-2.0
---

# VideoCMS Development Skill

VideoCMS is a self-hosted video resource manager: a Go media server
(`backend/`) that scans server folders, groups numbered files into TV series,
transcodes to HLS, and serves a React SPA (`frontend/`) with per-user
favorites, playlists, watch progress, and hidden-path filters. Metadata lives
in PostgreSQL. Admins can also upload files into server folders in resumable
chunks, download videos as MKV/MP4 with selectable tracks, and queue yt-dlp
downloads (with optional schedules). Repository is public, Apache-2.0, on
GitHub (`T-bagwell/videocms`).

## Development environment (this machine)

The ambient Go environment on this machine is polluted (GOPATH contains a
shell metacharacter, GOARCH/GOPROXY may be wrong). Never run `go` bare.
Always go through the wrapper:

```bash
# fixed env for one command (module lives in backend/, so use --in backend)
./.codex/skills/videocms/scripts/goenv.sh --in backend go test ./...
./.codex/skills/videocms/scripts/goenv.sh --in backend go vet ./...
# or source it to fix the current shell
source ./.codex/skills/videocms/scripts/goenv.sh
```

The wrapper pins `GOPATH=$HOME/go`, `GOARCH=amd64`, and
`GOPROXY=https://goproxy.cn,direct` (the default proxy times out here).

Required services: PostgreSQL 14+ (Homebrew) with a `videocms` database, and
the Homebrew ffmpeg/ffprobe at `/usr/local/opt/ffmpeg/bin/` (the system copy
crashes on libx265; the backend probes the Homebrew path automatically, so do
not rely on `PATH`).

## Day-to-day commands

```bash
createdb videocms          # once; idempotent in Makefile
make db                    # same, safe to re-run
make server                # backend only, http://localhost:8080
make frontend              # Vite dev server :5173, /api proxied to :8080
make demo                  # generate demo-media/ and demo-series/ sample files
make build                 # backend bin + frontend dist
make serve                 # single-port production mode (backend serves dist)
```

Initial admin login: `admin / admin123`.

Run backend tests with the wrapper (see above). Frontend build:

```bash
cd frontend && npm run lint && npm run test && npm run build
```

## Conventions (do not violate)

Backend
- Go standard library `net/http` + `pgx/v5` only; no web frameworks.
- New DB schema changes go into a new numbered migration
  `backend/internal/db/migrations/NNN_*.sql`; migrations apply automatically
  on startup. Never edit an already-applied migration; add a new one.
- Every listing that shows videos must filter through
  `visibleEpisodes($N)` (defined in `backend/internal/api/handlers_videos.go`)
  so per-user hidden paths and unavailable files are respected: home,
  continue watching, favorites, playlists, series detail, and series list.
- Admin content blocking: `blocked_titles` (migration 007) matches titles as
  case-insensitive substrings. The condition is folded into `visibleEpisodes`,
  so blocked media disappears from every listing without deleting files.
  Admin API: `GET|POST /api/admin/blocked-titles`,
  `DELETE /api/admin/blocked-titles/{id}`. Admin video lists may pass
  `include_blocked=1` to inspect/restore blocked items (ignored for non-admins).
- Library-level blocking: `libraries.blocked` (migration 008) hides an entire
  library; `visiblePaths` evaluates `NOT EXISTS (... lb.blocked)` so it works
  even inside subqueries that do not join `libraries`. Admin API:
  `PATCH /api/libraries/{id}` with `{"blocked": true|false}`.
- Media endpoints (`/stream`, `/download`, `/poster`, `/hls/*`) keep accepting
  `?token=` so `<video>`/`<img>` tags work without headers.
- Anything that writes to server folders (library creation, uploads, yt-dlp
  downloads) must require an existing absolute directory and reject `..`;
  uploaded filenames are reduced with `filepath.Base`. Upload/yt-dlp tests use
  fake binaries (`Downloader.SetBin`) so they never hit the network.
- API/error messages stay English; only the web UI is localized.

Frontend
- No hardcoded UI text. Every user-visible string goes through
  `useTranslation()` and into all five locale files:
  `frontend/src/i18n/locales/{en,zh,fr,ja,de}.json`.
- Adding a language = new locale JSON + registration in
  `frontend/src/i18n/index.js` (`SUPPORTED_LANGS` + resources) + docs.
  Default language is English; missing keys fall back to `en`.
- Player must never remount the `<video>` element when switching episodes.
  PlayerPage keeps an `activeId` state and calls `switchEpisode(nextId)`,
  which swaps the HLS source on the same element, preserving fullscreen.
- Continuous playback: `onEnded` picks `queue[idx + 1]` and switches; do not
  navigate or rebuild the player.

Series/media
- Episode detection lives in `backend/internal/media/episode.go`
  (`parseEpisode`); supported markers: `S01E01`, `EP1`, `E01`, `第N集`,
  `ShowName01Title`, trailing `(NN)` / `  NN`. Grouping requires >=2 episodes
  per (series name, season); groups are rebuilt by `rebuildSeries` after every
  scan. Changing parsing rules requires updating `episode_test.go`.
- Series list order: newest episode import first (`max(v.created_at)` DESC),
  then name, then season. `?library_id=` filters the series list just like the
  home page, and must apply the same `visibleEpisodes` conditions.

HLS (fragile - do not regress)
- Do NOT use `-hls_playlist_type vod`: the manifest then buffers until the
  whole transcode finishes and long videos fail to start. The live-growing
  manifest is intentional; `#EXT-X-ENDLIST` is appended server-side when the
  ffmpeg process completes.
- Do not use `-hls_flags temp_file`; keep `-hls_list_size 0`.
- Key frames are forced with `expr:gte(t,n_forced*6)` for stable 6s segments.
- Sessions idle out after 15 minutes; seeking restarts the transcode at the
  requested offset.

## Docs, Git, GitHub

- The English README lives at the repo root (`README.md`); the zh-CN / ja
  READMEs and all other docs live in `docs/` (contributing guides, changelog,
  security policy); `docs/INDEX.md` indexes everything. Product docs exist in
  five languages (en, zh-CN, fr, ja, de); architecture in three (en, zh-CN, ja).
  When touching docs, update every existing language and keep code fences paired.
- Update docs/changelog.md for user-visible changes.
- Commits: `git commit` locally, push via:
  ```bash
  export GH_TOKEN='<personal access token>'
  git -c http.extraHeader="Authorization: Bearer $GH_TOKEN" push origin main
  ```
  Never write the token into files, commit messages, or scripts. Origin is an
  SSH URL in this clone; authenticate with a PAT on the push command only.

CI (`.github/workflows/`): Backend CI runs build/vet/golangci-lint and tests
against a PostgreSQL service container; Frontend CI runs ESLint, Vitest and the
Vite build on Node 20/22/24; CodeQL scans Go + JavaScript; Release builds
cross-platform binaries on `v*` tags. Dependabot opens weekly dependency PRs.

## References (load only when needed)

- `references/architecture.md` - directory map, API route table, and key
  data flows (scan, series rebuild, HLS, auth).
- `references/schema.md` - PostgreSQL tables, columns, and indexes.
- `references/episode-parsing.md` - exact `parseEpisode` matching rules with
  examples and edge cases.

## Known pitfalls

- `go run`/`go build` outside `backend/` fails ("directory prefix . does not
  contain main module") - always `--in backend`.
- A scan left in `scanning` status after a crash is reset to `error` on
  startup; rescan from the admin page.
- macOS writes `._*` files and media folders may contain `.m3u8` stream dirs;
  the scanner skips both - keep that behavior.
- `make serve` binds `:8080` and serves the SPA; Vite dev (`make frontend`)
  proxies `/api` to `:8080`, so both are needed in development.
