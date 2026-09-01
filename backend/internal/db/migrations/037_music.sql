-- Music library: audio files are scanned with their artist/album tags and
-- grouped into albums (with embedded cover art extracted as the album cover).
CREATE TABLE IF NOT EXISTS albums (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id  uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name        text NOT NULL,
    artist      text NOT NULL DEFAULT '',
    year        int NOT NULL DEFAULT 0,
    cover_path  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, name, artist)
);

CREATE INDEX IF NOT EXISTS idx_albums_library ON albums(library_id);

ALTER TABLE videos ADD COLUMN IF NOT EXISTS artist text NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS album text NOT NULL DEFAULT '';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS album_id uuid REFERENCES albums(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_videos_album ON videos(album_id);
