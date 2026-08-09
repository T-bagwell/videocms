CREATE TABLE IF NOT EXISTS hidden_paths (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, path)
);

CREATE INDEX IF NOT EXISTS idx_hidden_paths_user ON hidden_paths(user_id);

