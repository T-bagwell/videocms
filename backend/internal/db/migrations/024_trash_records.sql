CREATE TABLE IF NOT EXISTS trash_records (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    original_path text NOT NULL,
    trash_path    text NOT NULL,
    video_id      uuid REFERENCES videos(id) ON DELETE SET NULL,
    moved_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_trash_records_moved ON trash_records(moved_at DESC);
