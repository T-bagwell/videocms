-- Channels & subscriptions: per-user series subscriptions power follow buttons
-- and the "new uploads" feed.
CREATE TABLE IF NOT EXISTS channel_subscriptions (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id  uuid NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, series_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_subscriptions_series ON channel_subscriptions(series_id);
