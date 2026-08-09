# Contributing to VideoCMS

Thanks for your interest! This document covers how to set up a development
environment and what to keep in mind when contributing.

## Development setup

```bash
# 1. Database (PostgreSQL 14+)
createdb videocms

# 2. Backend
cd backend
go run ./cmd/server          # http://localhost:8080

# 3. Frontend
cd frontend
npm install
npm run dev                  # http://localhost:5173 (proxies /api to :8080)
```

Login with the initial admin **admin / admin123**.

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

## Adding a language

1. Add `frontend/src/i18n/locales/<code>.json` (copy the English file and translate)
2. Register it in `frontend/src/i18n/index.js` (`SUPPORTED_LANGS` + resources)
3. Keep the key structure identical so fallback to English works

## Tests

```bash
cd backend
go test ./...
```

Add unit tests alongside parsing/scanning logic
(e.g. `internal/media/episode_test.go`).

## Pull requests

- Keep changes focused; one logical change per PR
- Verify `go test ./...`, `go vet ./...`, and `npm run build` pass
- Update the [CHANGELOG.md](CHANGELOG.md) for user-visible changes
- Multi-language docs/UI: update or add the other languages when touching them

