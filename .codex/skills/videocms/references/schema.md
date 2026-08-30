# VideoCMS Database Schema

All migrations in `backend/internal/db/migrations/` are applied in numeric
order on startup. `CREATE TABLE IF NOT EXISTS` + `ADD COLUMN IF NOT EXISTS`
make them idempotent.

## users

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | `gen_random_uuid()` |
| username | text UNIQUE | |
| password_hash | text | bcrypt |
| display_name | text | |
| role | text | `user` or `admin` |
| created_at | timestamptz | |

## libraries

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | |
| name | text | |
| path | text UNIQUE | server folder |
| scan_status | text | idle / scanning / error / cancelled |
| scan_error | text | |
| scan_started_at / scan_finished_at | timestamptz | |
| video_count | bigint | |
| blocked | boolean | (migration 008) library-wide content block |
| created_at | timestamptz | |

## videos

Base columns (001): id, library_id FK, title, filename, file_path UNIQUE,
size_bytes, duration_sec, width, height, video_codec, container, year,
synopsis, genres text[], poster_path, subtitle_path, available bool,
created_at, updated_at, last_scanned_at.

Added later: series_id (FK series, 004), season int, episode int (004),
scraped_at timestamptz (002), tmdb_id int (002).

## watch_progress

PK (user_id, video_id); position_sec, duration_sec, updated_at.

## favorites

PK (user_id, video_id); created_at. Indexed by (user_id, created_at DESC).

## playlists / playlist_items

`playlists`: PK id, user_id FK, name, description, timestamps.
`playlist_items`: PK (playlist_id, video_id), position int, added_at.

## series

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | |
| library_id | uuid FK | ON DELETE CASCADE |
| name | text | cleaned series name |
| season | int | 0 = no season marker |
| episode_count | int | |
| created_at / updated_at | timestamptz | |
| UNIQUE | (library_id, name, season) | |

## series_favorites

PK (user_id, series_id); created_at.

## hidden_paths

PK id, user_id FK, path text, created_at; UNIQUE (user_id, path).
Used by `visibleEpisodes()`.

## blocked_titles (migration 007)

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | |
| title | text | case-insensitive substring matched against video titles |
| created_at | timestamptz | |

Every query selecting `videoColumns` joins `blockedLateral` (longest matching
rule) and surfaces `blocked_id`; `visibleEpisodes` = `visiblePaths AND
blockedTitlesCondition`.

`visiblePaths` also excludes videos whose library is blocked via
`NOT EXISTS (SELECT 1 FROM libraries lb WHERE lb.id = v.library_id AND lb.blocked)`
— written against `v.library_id` so it works in subqueries without a
`libraries` join.

## uploads (migration 015)

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | upload session id |
| filename | text | sanitized with `filepath.Base` at creation |
| target_path | text | existing absolute server directory (no `..`) |
| total_size | bigint | 0 = unknown (chunk sequence must be contiguous) |
| chunk_size | bigint | default 8 MiB |
| status | text | uploading / completed / failed |
| error | text | |
| created_at / updated_at | timestamptz | |

Chunks live on disk at `DATA_DIR/uploads/<id>/<n>.part` and are removed after
`complete` assembles the final file into `target_path`.

## downloads (migration 016)

| column | type | notes |
| --- | --- | --- |
| id | uuid PK | |
| url | text | http(s) video/playlist/channel URL |
| title | text | filled by yt-dlp, empty for queued jobs |
| target_path | text | existing absolute server directory |
| format | text | yt-dlp format selector, default `bv*+ba/b` |
| status | text | queued / downloading / completed / failed / canceled |
| progress | double precision | parsed from `[download] NN%` lines |
| error | text | failure message |
| interval_secs | bigint | 0 = once; >0 = re-run on schedule after completion |
| last_run_at | timestamptz | used to decide when a scheduled job is due |
| created_at / updated_at | timestamptz | |

## Notes

- Never modify an applied migration; append `NNN_<name>.sql` and use
  `IF NOT EXISTS` guards.
- `uploads` and `downloads` are admin-only operational tables; they are not
  exposed to regular users and hold no media metadata.
- Series rows are fully derived from videos during `rebuildSeries`; do not
  hand-insert them outside the rebuild path.
