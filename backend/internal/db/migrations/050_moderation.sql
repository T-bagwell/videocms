-- Moderation toolset: content reports, account muting and global blocking.
CREATE TABLE IF NOT EXISTS reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id    uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id     uuid REFERENCES users(id) ON DELETE SET NULL,
    reason      text NOT NULL DEFAULT '',
    details     text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'reviewed', 'dismissed')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    decided_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    decided_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status, created_at);

ALTER TABLE users ADD COLUMN IF NOT EXISTS muted boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS global_blocked boolean NOT NULL DEFAULT false;
