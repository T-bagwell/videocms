CREATE TABLE IF NOT EXISTS uploads (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    filename    text NOT NULL,
    target_path text NOT NULL,
    total_size  bigint NOT NULL DEFAULT 0,
    chunk_size  bigint NOT NULL DEFAULT 8388608,
    status      text NOT NULL DEFAULT 'uploading'
                CHECK (status IN ('uploading', 'completed', 'failed')),
    error       text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_uploads_status ON uploads(status);
