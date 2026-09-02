# Scraper SDK

VideoCMS lets admins install external scrapers that implement a small JSON
contract. A registered scraper is selectable per video from the detail page
(`?provider=<name>` on `POST /api/videos/{id}/scrape`) exactly like the
built-in TMDB/TVMaze/AniList/Wikipedia/OMDb providers.

## Contract

The scraper receives a title (and optional year) and must answer with JSON:

```json
{
  "title": "External Title",
  "year": 2024,
  "synopsis": "Description text",
  "genres": ["Sci-Fi", "Drama"],
  "poster_url": "https://…/poster.jpg",
  "backdrop_url": "https://…/backdrop.jpg",
  "trailer_url": "https://www.youtube.com/watch?v=…",
  "trailer_title": "Official Trailer"
}
```

Only `title` is required; every other field is optional. An empty `title`
means "no match".

## Modes

Two kinds of installable scrapers are supported:

- **URL** (`kind: "url"`): the server POSTs `{"title": "...", "year": N}` with
  `Content-Type: application/json` to the configured URL (a `%s` placeholder is
  replaced with the URL-escaped title). Non-2xx responses and invalid JSON fail
  the scrape.
- **Command** (`kind: "command"`): the server executes the configured command
  with the title and year as arguments (`<command> "<title>" 2024`) and parses
  the contract JSON from stdout.

## Registration API

```bash
curl -X POST /api/admin/scrapers \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name": "my-scraper", "kind": "url", "url": "https://api.example.com/scrape/%s"}'
```

`PATCH /api/admin/scrapers/{id}` with `{"enabled": false}` disables a scraper
without deleting it; `DELETE /api/admin/scrapers/{id}` removes it. The admin
console's **Scrapers** tab manages the same operations. A disabled or unknown
provider name makes the scrape fail with `502`.

Scrapers run server-side: only install scrapers you trust, and keep
credentials out of command arguments that appear in process listings.
