CREATE TABLE IF NOT EXISTS series (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id    uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name          text NOT NULL,
    season        int NOT NULL DEFAULT 0,
    episode_count int NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, name, season)
);

CREATE INDEX IF NOT EXISTS idx_series_library ON series(library_id);

ALTER TABLE videos ADD COLUMN IF NOT EXISTS series_id uuid REFERENCES series(id) ON DELETE SET NULL;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS season int NOT NULL DEFAULT 0;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS episode int NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_videos_series ON videos(series_id);

