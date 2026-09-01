-- Multi-version movies: files of the same film (1080p / 4K / extended cut)
-- are grouped automatically by a normalized version_key; version_label is a
-- human-readable marker and version_rank picks the best copy for playback.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS version_key text NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS version_label text NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS version_rank int NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_videos_version_key ON videos(version_key) WHERE version_key <> '';
