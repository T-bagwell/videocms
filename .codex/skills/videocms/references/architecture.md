# VideoCMS Architecture Reference

## Repository layout

```
backend/
  cmd/server/            entrypoint (config load, db connect, migrate, seed admin)
  internal/
    api/                 HTTP handlers + router (net/http, Go 1.22+ patterns)
    auth/                JWT issue/verify, RequireAuth / RequireAdmin middleware
    config/              env-based config (PORT, DATABASE_URL, JWT_SECRET, ...)
    db/                  pgxpool connect, migrations (embedded), seed admin
    media/               scanner, scraper, episode parsing, HLS, streaming
    models/              domain structs (Video, Series, Library, Playlist, ...)
frontend/
  src/
    pages/               route components (Admin, Browse, Player, Series*, ...)
    components/          Navbar, Poster, PathPicker, PathFilterModal, ...
    i18n/                index.js + locales/{en,zh,fr,ja,de}.json
docs/                    multilingual product + architecture docs
scripts/                 make-demo-media.sh and similar
```

## Request flow

1. Auth: `POST /api/auth/login` returns a JWT; every other API call sends
   `Authorization: Bearer <jwt>`.
2. Media endpoints used by `<video>`/`<img>`/HLS accept `?token=<jwt>` instead
   (browser tags cannot send headers).
3. Listing endpoints inject `visibleEpisodes($N)` so the current user's hidden
   paths and unavailable files are excluded.
4. Playback: H.264/WebM served via HTTP Range (`/stream`); MKV/HEVC go through
   the HLS transcode pipeline (`/hls/playlist.m3u8`), which starts an ffmpeg
   session and returns a live-growing manifest.

## API routes

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | /api/auth/register | open | create account |
| POST | /api/auth/login | open | JWT login |
| GET | /api/auth/me | user | current profile |
| GET | /api/libraries | user | list libraries |
| POST | /api/libraries | admin | add library by server path |
| POST | /api/libraries/{id}/scan | admin | start background scan |
| POST | /api/libraries/{id}/scan/cancel | admin | cancel running scan |
| POST | /api/libraries/{id}/open | admin | open library folder on the server (open / xdg-open / explorer) |
| DELETE | /api/libraries/{id} | admin | remove library |
| GET | /api/videos | user | paginated list, `?library_id=`, `?q=`, hidden-path filtered |
| GET | /api/videos/{id} | user | video detail |
| PATCH | /api/videos/{id} | admin | edit title/year/synopsis/genres |
| POST | /api/videos/{id}/poster | admin | upload poster |
| GET | /api/videos/{id}/stream | user | HTTP Range stream |
| GET | /api/videos/{id}/download | user | download |
| GET | /api/videos/{id}/subtitles | user | SRT/WebVTT |
| GET | /api/videos/{id}/poster | user | poster image |
| GET | /api/videos/{id}/hls/{file...} | user | HLS manifest/segments |
| POST | /api/videos/{id}/scrape | admin | TMDB metadata scrape |
| GET | /api/series | user | series list, newest import first, `?library_id=` |
| GET | /api/series/{id} | user | series detail + episodes |
| GET | /api/series/{id}/poster | user | series poster |
| POST/DELETE | /api/series/{id}/favorite | user | series favorite |
| GET | /api/users/me/series-favorites | user | favorite series |
| PUT | /api/users/me/progress | user | save watch progress |
| GET | /api/users/me/continue | user | continue watching |
| POST/DELETE | /api/users/me/favorites | user | video favorites |
| GET | /api/users/me/hidden-paths | user | hidden path filters |
| POST | /api/users/me/hidden-paths | user | add filter |
| DELETE | /api/users/me/hidden-paths/{id} | user | remove filter |
| GET/POST | /api/admin/blocked-titles | admin | list/add title blocks |
| DELETE | /api/admin/blocked-titles/{id} | admin | remove title block |
| GET/POST/PATCH/DELETE | /api/playlists[...] | user | playlists + items |
| GET | /api/admin/users | admin | user management |
| GET | /api/admin/paths | admin | server folder picker |
| GET | /api/admin/stats | admin | stats |
| GET | /api/healthz | open | health check |

## Key flows

### Scan
`POST /api/libraries/{id}/scan` sets `scan_status='scanning'` and returns
immediately; the scanner walks the library path with 4 workers
(`SCAN_WORKERS`, max 16), probes each file with ffprobe, upserts
`videos`, generates a poster with ffmpeg when missing, then calls
`rebuildSeries`. Scan status lives in `libraries.scan_status` and is polled by
the admin UI.

### Series rebuild
`rebuildSeries` reads all available videos in a library, groups them by
`lower(seriesName) + season` using `parseEpisode`, requires >=2 members,
upserts `series`, assigns `videos.series_id/season/episode`, and deletes
series with <2 available episodes.

### HLS
`HLSManager` keys sessions by video UUID, transcodes to `data/hls/<uuid>/`
with libx264/aac, 6s segments, key frames every 6s, live manifest without
`-hls_playlist_type vod`, appends `#EXT-X-ENDLIST` when ffmpeg finishes, and
reaps idle sessions after 15 minutes.

### Hidden paths
`hidden_paths` stores per-user prefixes. `visibleEpisodes($N)` returns
`v.available AND NOT EXISTS (... v.file_path = hp.path OR starts_with(v.file_path, hp.path || '/'))`
and is concatenated into every video listing.

### Content blocking
`blocked_titles` stores admin title rules; `visibleEpisodes` also applies
`NOT EXISTS (... position(lower(bt.title) in lower(v.title)) > 0)`. Blocked
videos stay on disk and return instantly when the rule is removed. Admins can
list them with `GET /api/videos?include_blocked=1` (blocked_id included);
regular users cannot bypass the filter.
