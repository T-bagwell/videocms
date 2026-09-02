-- Per-user notification preferences (foundation for event delivery).
CREATE TABLE IF NOT EXISTS user_notification_prefs (
    user_id    uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled    boolean NOT NULL DEFAULT true,
    events     text[] NOT NULL DEFAULT
        '{scan,download,comment,favorite,rating,subscription,new_episode}',
    updated_at timestamptz NOT NULL DEFAULT now()
);
