CREATE TABLE IF NOT EXISTS watch_rooms (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token         text NOT NULL UNIQUE,
    video_id      uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    owner_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playing       boolean NOT NULL DEFAULT false,
    position_sec  double precision NOT NULL DEFAULT 0,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_watch_rooms_token ON watch_rooms(token);
