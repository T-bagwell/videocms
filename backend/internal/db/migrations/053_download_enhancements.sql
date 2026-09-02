-- yt-dlp enhancements: proxy/cookies/login credentials and channel/playlist
-- bulk downloads with a per-job download archive.
ALTER TABLE downloads ADD COLUMN IF NOT EXISTS proxy text NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN IF NOT EXISTS cookies_path text NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN IF NOT EXISTS username text NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN IF NOT EXISTS password text NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'video';
