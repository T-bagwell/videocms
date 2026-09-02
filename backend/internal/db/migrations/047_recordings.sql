-- Tuner-based live TV: scheduled recordings capture any channel's stream
-- (HDHomeRun tuner URL or IPTV source) into DATA_DIR/recordings/.
CREATE TABLE IF NOT EXISTS recordings (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id  uuid NOT NULL REFERENCES iptv_channels(id) ON DELETE CASCADE,
    title       text NOT NULL,
    start_utc   timestamptz NOT NULL,
    end_utc     timestamptz NOT NULL,
    status      text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'recording', 'done', 'failed')),
    file_path   text NOT NULL DEFAULT '',
    error       text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recordings_start ON recordings(status, start_utc);
