CREATE TABLE IF NOT EXISTS blocked_titles (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_blocked_titles_title ON blocked_titles(lower(title));
