CREATE TABLE IF NOT EXISTS subtitle_tracks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id     uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position     int NOT NULL DEFAULT 0,
    lang         text NOT NULL DEFAULT '',
    title        text NOT NULL DEFAULT '',
    path         text NOT NULL DEFAULT '',
    kind         text NOT NULL DEFAULT 'sidecar' CHECK (kind IN ('sidecar', 'embedded', 'upload')),
    source_key   text NOT NULL DEFAULT '',
    stream_index int NOT NULL DEFAULT -1,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_subtitle_tracks_video_source
    ON subtitle_tracks(video_id, source_key) WHERE source_key <> '';

CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_video ON subtitle_tracks(video_id, position);

-- Backfill a track row for every existing active subtitle so the new UI can
-- list and switch them right away. Kind defaults to sidecar; the next scan
-- rebuilds sidecar/embedded tracks with accurate metadata.
INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
SELECT id, 0, '', '', subtitle_path, 'sidecar', 'legacy:' || subtitle_path, -1
FROM videos
WHERE subtitle_path <> '';
