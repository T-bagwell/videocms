-- Backdrops/fanart: a wide hero image per video, fetched during scraping or
-- uploaded by admins, shown as the detail-page banner.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS backdrop_path text NOT NULL DEFAULT '';
