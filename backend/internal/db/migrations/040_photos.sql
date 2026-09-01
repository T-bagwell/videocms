-- Photo library: images are indexed with EXIF metadata (taken-at, camera)
-- and grouped into folder-based albums with a cover photo.
CREATE TABLE IF NOT EXISTS photo_albums (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id  uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name        text NOT NULL,
    cover_path  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, name)
);

CREATE INDEX IF NOT EXISTS idx_photo_albums_library ON photo_albums(library_id);

CREATE TABLE IF NOT EXISTS photos (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id      uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    album_id        uuid REFERENCES photo_albums(id) ON DELETE SET NULL,
    title           text NOT NULL,
    file_path       text NOT NULL UNIQUE,
    size_bytes      bigint NOT NULL DEFAULT 0,
    width           int NOT NULL DEFAULT 0,
    height          int NOT NULL DEFAULT 0,
    taken_at        timestamptz,
    camera          text NOT NULL DEFAULT '',
    available       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_scanned_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_photos_library ON photos(library_id, available);
CREATE INDEX IF NOT EXISTS idx_photos_album ON photos(album_id);
