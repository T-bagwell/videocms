ALTER TABLE share_tokens
    ADD COLUMN IF NOT EXISTS theme text NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS custom_title text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS hide_nav boolean NOT NULL DEFAULT false;
