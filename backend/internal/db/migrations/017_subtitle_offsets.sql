CREATE TABLE IF NOT EXISTS subtitle_offsets (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id   uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    offset_ms  bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, video_id)
);
