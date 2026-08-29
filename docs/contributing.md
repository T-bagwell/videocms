# Contributing to VideoCMS

Thanks for your interest in contributing! VideoCMS is a self-hosted video
resource manager built with Go, React and PostgreSQL. Whether you found a bug,
want a new feature, or can help with docs and translations, this guide covers
everything you need to get started.

## Table of Contents

- [Ways to contribute](#ways-to-contribute)
- [Development setup](#development-setup)
- [Project layout](#project-layout)
- [Repository conventions](#repository-conventions)
- [Testing](#testing)
- [Continuous integration](#continuous-integration)
- [Pull request workflow](#pull-request-workflow)
- [Documentation & localization](#documentation--localization)
- [Troubleshooting](#troubleshooting)
- [Getting help](#getting-help)

## Ways to contribute

### Report a bug

- Search the [open issues](https://github.com/T-bagwell/videocms/issues) first
  to avoid duplicates.
- Open an issue with: VideoCMS version/commit, OS and browser, PostgreSQL
  version, steps to reproduce, expected vs. actual behavior, and relevant logs
  (backend logs are especially helpful for scanning/HLS problems).

### Request a feature

- Describe the problem you are trying to solve and a concrete use case rather
  than just a UI sketch.
- Small, focused feature requests are easier to discuss and implement.

### Improve documentation and translations

- See [Documentation & localization](#documentation--localization) for the
  language matrix and the rules for touching docs.

### Submit code

- For anything beyond a typo fix, open an issue first so maintainers can weigh
  in before you invest time.
- Keep pull requests small and focused (see
  [Pull request workflow](#pull-request-workflow)).

## Development setup

### Prerequisites

- Go — version pinned in `backend/go.mod`
- Node.js 20+, 22+ or 24+ (CI runs all three)
- PostgreSQL 14+
- ffmpeg/ffprobe (with libx265 for MKV/HEVC transcoding)

### One-time setup

```bash
# 1. Clone and enter the repo
git clone git@github.com:T-bagwell/videocms.git
cd videocms

# 2. Create the database (idempotent)
createdb videocms
# or: make db

# 3. (Optional) generate demo media
make demo

# 4. Start the backend
# The ambient Go environment on macOS dev machines can be polluted (bad GOPATH
# entry, wrong proxy); always go through the repo wrapper:
./.codex/skills/videocms/scripts/goenv.sh --in backend go run ./cmd/server
# or source it once: source ./.codex/skills/videocms/scripts/goenv.sh

# 5. Start the frontend dev server (proxies /api to :8080)
cd frontend
npm install
npm run dev
```

Open http://localhost:5173 and log in with the initial admin **admin /
admin123**.

### Daily commands

| Command | What it does |
| --- | --- |
| `make db` | Create the `videocms` database (safe to re-run) |
| `make server` | Run the backend on http://localhost:8080 |
| `make frontend` | Run the Vite dev server on :5173 (proxies `/api`) |
| `make demo` | Generate `demo-media/` and `demo-series/` sample files |
| `make build` | Build backend bin + frontend `dist` |
| `make serve` | Single-port production mode (backend serves the SPA) |

### Environment notes

- Never run `go` bare in this repo; use
  `./.codex/skills/videocms/scripts/goenv.sh --in backend ...` (the module
  lives in `backend/`, hence `--in backend`).
- On macOS dev machines the Homebrew ffmpeg at
  `/usr/local/opt/ffmpeg/bin/` is used automatically by the backend; the
  system ffmpeg can crash on libx265.
- PostgreSQL must be running; migrations apply automatically on backend
  startup.

## Project layout

```
backend/
  cmd/server/     entrypoint
  internal/
    api/          HTTP handlers & routes
    auth/         JWT + role middleware
    media/        scanner, scraper, HLS, streaming
    db/           pool + SQL migrations (embedded)
    models/       domain types
frontend/
  src/
    pages/        route components
    components/   shared UI
    i18n/         locale JSON (en/zh/fr/ja/de)
docs/             product, architecture, screenshots
```

See [architecture.md](architecture.md) for the full design: directory
map, API routes, data model, key flows, security and extension points.

## Repository conventions

### Backend

- Go standard library `net/http` + `pgx/v5` only — no web frameworks.
- Run `gofmt` and `go vet` on your changes.
- DB schema changes go into a new numbered migration
  (`backend/internal/db/migrations/NNN_*.sql`); migrations apply automatically
  on startup. Never edit an already-applied migration — add a new one.
- Every listing that shows videos must filter through `visibleEpisodes($N)`
  (defined in `backend/internal/api/handlers_videos.go`) so per-user hidden
  paths, admin title blocks and blocked libraries are respected: home,
  continue watching, favorites, playlists, series detail, and series list.
- Admin content blocking: `blocked_titles` matches titles as case-insensitive
  substrings and is folded into `visibleEpisodes`; library-level blocking lives
  on `libraries.blocked` and works even inside subqueries via `visiblePaths`.
- Media endpoints (`/stream`, `/download`, `/poster`, `/hls/*`) keep accepting
  `?token=` so `<video>`/`<img>` tags work without headers.
- API/error messages stay English; only the web UI is localized.

### Frontend

- No hardcoded UI text — every user-visible string goes through
  `useTranslation()` and into all five locale files
  (`frontend/src/i18n/locales/{en,zh,fr,ja,de}.json`).
- The player must never remount the `<video>` element when switching episodes:
  PlayerPage keeps an `activeId` state and calls `switchEpisode(nextId)`,
  swapping the HLS source on the same element.
- Continuous playback: `onEnded` picks `queue[idx + 1]` and switches; do not
  navigate or rebuild the player.

### Series & media

- Episode detection lives in `backend/internal/media/episode.go`
  (`parseEpisode`); supported markers: `S01E01`, `EP1`, `E01`, `第N集`,
  `ShowName01Title`, trailing `(NN)` / `  NN`.
- Grouping requires >= 2 episodes per (series name, season); groups are rebuilt
  by `rebuildSeries` after every scan. Changing parsing rules requires updating
  `episode_test.go`.
- Series list order: newest episode import first
  (`max(v.created_at)` DESC), then name, then season.

### HLS (fragile — do not regress)

- Do NOT use `-hls_playlist_type vod`: the manifest then buffers until the
  whole transcode finishes and long videos fail to start. The live-growing
  manifest is intentional; `#EXT-X-ENDLIST` is appended server-side when the
  ffmpeg process completes.
- Do not use `-hls_flags temp_file`; keep `-hls_list_size 0`.
- Key frames are forced with `expr:gte(t,n_forced*6)` for stable 6s segments.
- Sessions idle out after 15 minutes; seeking restarts the transcode at the
  requested offset.

## Testing

```bash
# Backend (always through the wrapper)
./.codex/skills/videocms/scripts/goenv.sh --in backend go test ./...
./.codex/skills/videocms/scripts/goenv.sh --in backend go vet ./...

# Frontend
cd frontend
npm run lint
npm run test
npm run build
```

- Add unit tests alongside parsing/scanning logic
  (e.g. `internal/media/episode_test.go`).
- Integration tests (`internal/api/integration_test.go`) skip automatically
  when PostgreSQL is not reachable; set `TEST_PG_DSN` to run them.
- Network-dependent scraper tests skip unless `NETWORK_TEST=1`.

## Continuous integration

GitHub Actions runs two workflows on push to `main` and on pull requests:

| Workflow | File | What it runs |
| --- | --- | --- |
| Backend CI | `.github/workflows/backend.yml` | `go build`, `go vet`, golangci-lint, and `go test` (unit + PostgreSQL integration) in `backend/` |
| Frontend CI | `.github/workflows/webpack.yml` | `npm ci`, ESLint, Vitest, and `npm run build` in `frontend/` (Node 20/22/24) |
| CodeQL | `.github/workflows/codeql.yml` | Security scanning for Go and JavaScript (push, PR, weekly) |
| Release | `.github/workflows/release.yml` | Cross-platform binaries + GitHub Release on `v*` tags |

Dependabot opens weekly dependency-update PRs (Go, npm, GitHub Actions); keep
their checks green too — comment `@dependabot rebase` to re-sync a PR with
`main`.

Keep both green before requesting review.

## Pull request workflow

1. For non-trivial changes, open an issue first and discuss the approach.
2. Create a branch off `main` with a short, descriptive name (`fix/`, `feat/`,
   `docs/`, `refactor/`...).
3. Make focused commits; one logical change per PR.
4. Before opening the PR:
   - `gofmt`, `go vet`, `golangci-lint run`, `go test ./...` pass
   - `npm run lint`, `npm run test`, `npm run build` pass
   - UI changes are covered by screenshots in the PR description
5. Describe what and why in the PR; reference the issue it fixes
   (`Closes #123`).
6. Update [changelog.md](changelog.md) for user-visible changes.
7. Update docs in every existing language when you touch them (see below).

## Documentation & localization

The repository maintains several documentation sets; when you touch a doc,
update every existing language and keep code fences paired.

| Document set | Languages |
| --- | --- |
| README | en, zh-CN, ja |
| Product docs (`docs/product.*.md`) | en, zh-CN, fr, ja, de |
| Architecture docs (`docs/architecture.*.md`) | en, zh-CN, ja |
| Contributing guide | en, zh-CN, ja |

- Keep `INDEX.md` in sync when adding or renaming documentation files.
- The web UI is localized in five languages; the default is English and missing
  keys fall back to English.

### Adding a language to the UI

1. Add `frontend/src/i18n/locales/<code>.json` (copy the English file and
   translate).
2. Register it in `frontend/src/i18n/index.js` (`SUPPORTED_LANGS` +
   `resources`).
3. Keep the key structure identical so fallback to English works.
4. Update the README and docs that list supported languages.

## Troubleshooting

- `go run`/`go build` outside `backend/` fails — always use `--in backend`.
- A scan left in `scanning` status after a crash is reset to `error` on
  startup; rescan from the admin page.
- The scanner skips macOS `._*` files and `.m3u8` stream directories — keep
  that behavior.
- `make serve` binds :8080 and serves the SPA; use both `make server` and
  `make frontend` in development (Vite proxies `/api` to :8080).

## Getting help

- GitHub [issues](https://github.com/T-bagwell/videocms/issues) for bugs and
  feature requests.
- The repository ships a project-level Codex skill
  (`.codex/skills/videocms/`) that encodes the environment, commands and
  conventions above; Codex agents working in this repo should load it.
