-- IPTV: channels imported from M3U sources or generated from library videos,
-- plus an EPG programme index imported from XMLTV.
CREATE TABLE IF NOT EXISTS iptv_channels (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    tvg_id      text NOT NULL DEFAULT '',
    tvg_name    text NOT NULL DEFAULT '',
    logo        text NOT NULL DEFAULT '',
    group_title text NOT NULL DEFAULT '',
    source_url  text NOT NULL DEFAULT '',
    library_id  uuid REFERENCES libraries(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_iptv_channels_tvg ON iptv_channels(tvg_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iptv_channels_source ON iptv_channels(source_url) WHERE source_url <> '';

CREATE TABLE IF NOT EXISTS iptv_programmes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id  text NOT NULL DEFAULT '',
    start_utc   timestamptz NOT NULL,
    end_utc     timestamptz NOT NULL,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_iptv_programmes_channel ON iptv_programmes(channel_id, start_utc);
