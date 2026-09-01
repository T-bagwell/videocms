-- HDR flag: detected from ffprobe color/transfer metadata during scans so the
-- HLS transcoder can pass through Dolby Vision / HDR10+ content or tone map
-- it depending on the chosen encoder.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS hdr boolean NOT NULL DEFAULT false;
