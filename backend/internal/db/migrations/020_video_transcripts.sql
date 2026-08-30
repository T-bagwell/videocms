CREATE TABLE IF NOT EXISTS video_transcripts (
    video_id   uuid PRIMARY KEY REFERENCES videos(id) ON DELETE CASCADE,
    lang       text NOT NULL DEFAULT '',
    path       text NOT NULL DEFAULT '',
    text       text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
