-- Books & comics: EPUB and CBZ files are indexed as books (no media probe)
-- and served by dedicated reader endpoints.
CREATE TABLE IF NOT EXISTS books (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id      uuid NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title           text NOT NULL,
    author          text NOT NULL DEFAULT '',
    format          text NOT NULL DEFAULT '',
    file_path       text NOT NULL UNIQUE,
    size_bytes      bigint NOT NULL DEFAULT 0,
    available       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_scanned_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_books_library ON books(library_id, available);
