ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS allowed_domains text[] NOT NULL DEFAULT '{}';
