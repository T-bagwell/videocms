-- Trakt watch-history sync: audit log of server-side sync runs.
CREATE TABLE IF NOT EXISTS trakt_sync_log (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    synced_at  timestamptz NOT NULL DEFAULT now(),
    item_count int NOT NULL DEFAULT 0,
    error      text NOT NULL DEFAULT ''
);
