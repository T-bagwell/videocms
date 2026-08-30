CREATE TABLE IF NOT EXISTS storage_pools (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL UNIQUE,
    type       text NOT NULL CHECK (type IN ('local', 's3', 'sftp')),
    mount_path text NOT NULL DEFAULT '',
    config     jsonb NOT NULL DEFAULT '{}',
    readonly   boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
