-- Scraper SDK: installable external scrapers registered by admins. A URL
-- scraper receives POST {"title","year"} and returns scraper JSON; a command
-- scraper is executed with <title> <year> args and prints the same JSON.
CREATE TABLE IF NOT EXISTS scrapers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL UNIQUE,
    kind       text NOT NULL DEFAULT 'command' CHECK (kind IN ('command', 'url')),
    command    text NOT NULL DEFAULT '',
    url        text NOT NULL DEFAULT '',
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
