CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_videos_title_trgm
    ON videos USING gin (lower(title) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_videos_synopsis_trgm
    ON videos USING gin (lower(synopsis) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_videos_filename_trgm
    ON videos USING gin (lower(filename) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_transcripts_text_trgm
    ON video_transcripts USING gin (lower(text) gin_trgm_ops);
