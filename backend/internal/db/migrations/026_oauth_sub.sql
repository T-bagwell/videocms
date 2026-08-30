ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_sub text;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_oauth_sub
    ON users(oauth_sub) WHERE oauth_sub <> '';
