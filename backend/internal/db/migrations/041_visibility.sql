-- Per-item visibility: public (anonymous access), private (login, default),
-- unlisted (link only, hidden from listings) and password-protected (bcrypt
-- hash checked by anonymous unlock tokens).
ALTER TABLE videos ADD COLUMN IF NOT EXISTS visibility text NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private', 'public', 'unlisted', 'password'));
ALTER TABLE videos ADD COLUMN IF NOT EXISTS access_password_hash text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_videos_visibility ON videos(visibility) WHERE visibility <> 'private';
