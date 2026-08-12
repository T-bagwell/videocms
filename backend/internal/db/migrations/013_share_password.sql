ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS password_hash text NOT NULL DEFAULT '';
