-- Likes/dislikes and per-item policy toggles.
CREATE TABLE IF NOT EXISTS video_reactions (
    video_id   uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    value      int NOT NULL CHECK (value IN (1, -1)),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (video_id, user_id)
);

ALTER TABLE videos ADD COLUMN IF NOT EXISTS allow_downloads boolean NOT NULL DEFAULT true;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS allow_comments boolean NOT NULL DEFAULT true;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS allow_reports boolean NOT NULL DEFAULT true;
