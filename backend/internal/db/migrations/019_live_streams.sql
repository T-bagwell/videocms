CREATE TABLE IF NOT EXISTS live_streams (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title       text NOT NULL,
    stream_key  text NOT NULL UNIQUE,
    status      text NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'starting', 'live', 'offline')),
    error       text NOT NULL DEFAULT '',
    created_by  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_live_streams_status ON live_streams(status);

CREATE TABLE IF NOT EXISTS chat_messages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    live_id     uuid NOT NULL REFERENCES live_streams(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username    text NOT NULL DEFAULT '',
    body        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_live ON chat_messages(live_id, created_at);
