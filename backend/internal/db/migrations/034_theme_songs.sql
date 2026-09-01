-- Theme songs: one optional audio preview per video, stored under
-- DATA_DIR/theme-songs/<video_id>/ and served to the detail page player.
CREATE TABLE IF NOT EXISTS theme_songs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id    uuid NOT NULL UNIQUE REFERENCES videos(id) ON DELETE CASCADE,
    title       text NOT NULL,
    file_path   text NOT NULL UNIQUE,
    size_bytes  bigint NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);
