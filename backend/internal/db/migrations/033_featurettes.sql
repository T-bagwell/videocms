-- Trailers & featurettes: trailer links come from metadata scraping and are
-- stored on the video row; self-hosted featurette files live in a dedicated
-- table, one row per attached file.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS trailer_url text NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS trailer_title text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS featurettes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id     uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    title        text NOT NULL,
    file_path    text NOT NULL UNIQUE,
    size_bytes   bigint NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_featurettes_video ON featurettes(video_id, created_at);
