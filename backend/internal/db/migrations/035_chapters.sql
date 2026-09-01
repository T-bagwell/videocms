-- Chapters: parsed from ffprobe during scans and shown on the player
-- timeline. The chapters API doubles as a generic media-segment index.
CREATE TABLE IF NOT EXISTS chapters (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id  uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position  int NOT NULL DEFAULT 0,
    start_sec double precision NOT NULL,
    end_sec   double precision NOT NULL,
    title     text NOT NULL DEFAULT '',
    UNIQUE (video_id, position)
);

CREATE INDEX IF NOT EXISTS idx_chapters_video ON chapters(video_id, start_sec);
