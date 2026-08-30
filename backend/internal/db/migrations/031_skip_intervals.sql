CREATE TABLE IF NOT EXISTS skip_intervals (
    video_id  uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    kind      text NOT NULL CHECK (kind IN ('intro', 'credits')),
    start_sec double precision NOT NULL DEFAULT 0,
    end_sec   double precision NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (video_id, kind)
);
