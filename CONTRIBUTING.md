# Contributing to VideoCMS

Thanks for your interest! This document covers how to set up a development
environment and what to keep in mind when contributing.

## Development setup

```bash
# 1. Database (PostgreSQL 14+)
createdb videocms

# 2. Backend
# This machine's ambient Go env is polluted (bad GOPATH entry, wrong proxy);
# always go through the repo wrapper:
./.codex/skills/videocms/scripts/goenv.sh --in backend go run ./cmd/server
# or source it once:  source ./.codex/skills/videocms/scripts/goenv.sh
# (module lives in backend/, hence --in backend; wrapper runs from repo root)

# 3. Frontend
cd frontend
npm install
npm run dev                  # http://localhost:5173 (proxies /api to :8080)
```

Login with the initial admin **admin / admin123**.

> The repository ships a **project-level Codex skill** at
> `.codex/skills/videocms/` that encodes the environment, commands and
> conventions below. Codex agents working in this repo should load it.

## Project layout

```
backend/internal/
  api/        HTTP handlers & routes
  auth/       JWT + role middleware
  media/      scanner, scraper, HLS, streaming
  db/         pool + SQL migrations (embedded)
  models/     domain types
frontend/src/
  pages/      route components
  components/ shared UI
  i18n/       locale JSON (en/zh/fr/ja/de)
```

## Conventions

- Backend: Go standard library (`net/http`), `pgx/v5`; run `gofmt` and `go vet`
- Frontend: React 18 + Vite; keep components small and UI strings in i18n locale
  files — **no hardcoded UI text**
- DB schema changes go in a new numbered migration
  (`backend/internal/db/migrations/NNN_*.sql`)
- Media endpoints keep accepting `?token=` for `<video>`/`<img>` tags
- Every video listing must apply the shared SQL visibility conditions
  (`visibleEpisodes` / `visiblePaths` in `backend/internal/api/handlers_videos.go`)
  so per-user path filters, admin title blocks and blocked libraries are
  respected consistently (home, series, favorites, continue watching, playlists)
- HLS: keep the live-growing manifest — never add `-hls_playlist_type vod`;
  do not use `temp_file`; force key frames every 6s

## Adding a language

1. Add `frontend/src/i18n/locales/<code>.json` (copy the English file and translate)
2. Register it in `frontend/src/i18n/index.js` (`SUPPORTED_LANGS` + resources)
3. Keep the key structure identical so fallback to English works

## Tests

```bash
./.codex/skills/videocms/scripts/goenv.sh --in backend go test ./...
./.codex/skills/videocms/scripts/goenv.sh --in backend go vet ./...
```

Add unit tests alongside parsing/scanning logic
(e.g. `internal/media/episode_test.go`).

## Pull requests

- Keep changes focused; one logical change per PR
- Verify `go test ./...`, `go vet ./...`, and `npm run build` pass
- Update the [CHANGELOG.md](CHANGELOG.md) for user-visible changes
- Multi-language docs/UI: update or add the other languages when touching them
  (UI: en/zh/fr/ja/de; README: en/zh-CN/ja; architecture: en/zh-CN/ja)
- Keep docs/README.md in sync when adding or renaming documentation files
