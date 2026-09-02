-- Plugin/extension system: admin-installed plugins (webhook or command) that
-- receive matching server events.
CREATE TABLE IF NOT EXISTS plugins (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL UNIQUE,
    description  text NOT NULL DEFAULT '',
    install_url  text NOT NULL DEFAULT '',
    kind         text NOT NULL DEFAULT 'webhook' CHECK (kind IN ('webhook', 'command')),
    events       text[] NOT NULL DEFAULT '{}',
    enabled      boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);
