CREATE TABLE IF NOT EXISTS share_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id   uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    token      text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_share_tokens_video ON share_tokens(video_id);
CREATE INDEX IF NOT EXISTS idx_share_tokens_expires ON share_tokens(expires_at);
