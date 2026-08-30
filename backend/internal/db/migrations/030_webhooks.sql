CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    url        text NOT NULL,
    secret     text NOT NULL DEFAULT '',
    events     text[] NOT NULL DEFAULT '{}',
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_active ON webhook_subscriptions(active);
