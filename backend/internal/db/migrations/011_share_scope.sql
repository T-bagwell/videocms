ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS scope text NOT NULL DEFAULT 'video'
        CHECK (scope IN ('video', 'series', 'playlist'));
ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS series_id uuid REFERENCES series(id) ON DELETE CASCADE;
ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS playlist_id uuid REFERENCES playlists(id) ON DELETE CASCADE;
ALTER TABLE share_tokens
    ALTER COLUMN video_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_share_tokens_series ON share_tokens(series_id);
CREATE INDEX IF NOT EXISTS idx_share_tokens_playlist ON share_tokens(playlist_id);
