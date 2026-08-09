CREATE TABLE IF NOT EXISTS series_favorites (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id  uuid NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, series_id)
);

CREATE INDEX IF NOT EXISTS idx_series_favorites_user ON series_favorites(user_id, created_at DESC);

