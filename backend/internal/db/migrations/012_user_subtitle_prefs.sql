CREATE TABLE IF NOT EXISTS user_subtitle_prefs (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id   uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    track_id   uuid NOT NULL REFERENCES subtitle_tracks(id) ON DELETE CASCADE,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_user_subtitle_prefs_track ON user_subtitle_prefs(track_id);
