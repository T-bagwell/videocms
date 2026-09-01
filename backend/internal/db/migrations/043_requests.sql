-- Request workflow: users ask for titles, admins approve (optionally feeding
-- the yt-dlp download queue) or reject.
CREATE TABLE IF NOT EXISTS requests (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       text NOT NULL,
    year        int NOT NULL DEFAULT 0,
    media_type  text NOT NULL DEFAULT 'movie',
    notes       text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'downloading')),
    decided_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    decided_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_requests_user ON requests(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status, created_at DESC);
