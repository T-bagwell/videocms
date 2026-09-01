-- Quality profiles: an active profile biases multi-version movie selection
-- toward files inside a height range / codec preference. Scans re-score
-- versions so new releases automatically upgrade to the best in-range copy.
CREATE TABLE IF NOT EXISTS quality_profiles (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text NOT NULL UNIQUE,
    min_height      int NOT NULL DEFAULT 0,
    max_height      int NOT NULL DEFAULT 0,
    preferred_codec text NOT NULL DEFAULT '',
    active          boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
