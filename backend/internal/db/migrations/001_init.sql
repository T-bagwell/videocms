CREATE TABLE IF NOT EXISTS users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    role          text NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS libraries (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text NOT NULL,
    path             text NOT NULL UNIQUE,
    scan_status      text NOT NULL DEFAULT 'idle' CHECK (scan_status IN ('idle', 'scanning', 'error')),
    scan_error       text NOT NULL DEFAULT '',
    scan_started_at  timestamptz,
    scan_finished_at timestamptz,
    video_count      bigint NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS videos (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id      uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title           text NOT NULL,
    filename        text NOT NULL,
    file_path       text NOT NULL UNIQUE,
    size_bytes      bigint NOT NULL DEFAULT 0,
    duration_sec    double precision NOT NULL DEFAULT 0,
    width           int NOT NULL DEFAULT 0,
    height          int NOT NULL DEFAULT 0,
    video_codec     text NOT NULL DEFAULT '',
    container       text NOT NULL DEFAULT '',
    year            int NOT NULL DEFAULT 0,
    synopsis        text NOT NULL DEFAULT '',
    genres          text[] NOT NULL DEFAULT '{}',
    poster_path     text NOT NULL DEFAULT '',
    subtitle_path   text NOT NULL DEFAULT '',
    available       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_scanned_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_videos_library ON videos(library_id);
CREATE INDEX IF NOT EXISTS idx_videos_title ON videos(lower(title));
CREATE INDEX IF NOT EXISTS idx_videos_available ON videos(available) WHERE available;
CREATE INDEX IF NOT EXISTS idx_videos_year ON videos(year DESC);

CREATE TABLE IF NOT EXISTS watch_progress (
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id     uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position_sec double precision NOT NULL DEFAULT 0,
    duration_sec double precision NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_watch_progress_user_updated ON watch_progress(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS favorites (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id   uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_user ON favorites(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS playlists (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_playlists_user ON playlists(user_id);

CREATE TABLE IF NOT EXISTS playlist_items (
    playlist_id uuid NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    video_id    uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position    int NOT NULL DEFAULT 0,
    added_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist ON playlist_items(playlist_id, position);

