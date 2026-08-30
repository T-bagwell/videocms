CREATE TABLE IF NOT EXISTS downloads (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    url           text NOT NULL,
    title         text NOT NULL DEFAULT '',
    target_path   text NOT NULL,
    format        text NOT NULL DEFAULT 'bv*+ba/b',
    status        text NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'downloading', 'completed', 'failed', 'canceled')),
    progress      double precision NOT NULL DEFAULT 0,
    error         text NOT NULL DEFAULT '',
    interval_secs bigint NOT NULL DEFAULT 0,
    last_run_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status, created_at);
