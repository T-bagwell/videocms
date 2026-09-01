-- Remembered playback speed per user/video (audiobook friendly): stored on
-- watch_progress so switching devices keeps the chosen rate.
ALTER TABLE watch_progress ADD COLUMN IF NOT EXISTS playback_rate double precision NOT NULL DEFAULT 1;
