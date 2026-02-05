# Canonical import format

Any data source (API, scrape, JSON file) can feed GDQL by producing shows in a **canonical format** and calling the shared writer. Duplicate shows (same date + venue) are skipped.

## How to do it

1. **Create or use a DB** (optional if you already have one):
   ```bash
   gdql init
   ```
   Or use an existing `shows.db`.

2. **Save your data as JSON** in the shape below (one array of shows). Scrape a site, call an API, or hand-write a file — as long as the JSON matches the format.

3. **Import the file:**
   ```bash
   gdql -db shows.db import json shows.json
   ```
   Or: `gdql import json -f shows.json` (uses default DB).

4. **Query:** `gdql -db shows.db "SHOWS FROM 1977"`, etc.

## Package

`internal/import/canonical` defines:

- **Types:** `Show`, `Venue`, `Set`, `SongInSet` (date, venue, sets of songs, segue flags).
- **Writer:** `canonical.WriteShows(ctx, db, shows)` — inserts into the existing SQLite DB, creating venues/songs as needed.

Use this from a new importer (e.g. `gdql import relisten`, `gdql import json`) or from a scraper that outputs in this shape.

## JSON shape (for scrapers or `gdql import json`)

Your scraper or a JSON file can use this structure. The writer accepts `[]canonical.Show`; you can unmarshal from JSON that looks like:

```json
[
  {
    "date": "1977-05-08",
    "venue": {
      "name": "Barton Hall",
      "city": "Ithaca",
      "state": "NY",
      "country": "USA"
    },
    "tour": "",
    "notes": "",
    "sets": [
      {
        "songs": [
          { "name": "Mississippi Half-Step", "segue_before": false },
          { "name": "Jack Straw", "segue_before": true }
        ]
      },
      {
        "songs": [
          { "name": "Scarlet Begonias", "segue_before": false },
          { "name": "Fire on the Mountain", "segue_before": true }
        ]
      }
    ]
  }
]
```

- **date:** `YYYY-MM-DD` or `DD-MM-YYYY` (writer normalizes).
- **venue:** `name` required; `city`, `state`, `country` optional.
- **sets:** Array of sets (Set 1, Set 2, Encore). Each set has `songs`: array of `{ "name": "...", "segue_before": true|false }`.
- **segue_before:** `true` = this song was segued into from the previous (`>`).
- Song names must **not** contain `" > "`. Split into two songs and set `segue_before: true` on the second.

## Alternative data sources

See **docs/DATA_SOURCES_IMPORT.md** for a table of sources (setlist.fm, Internet Archive, Relisten, Jerrybase, etc.). For scraped data: produce the canonical JSON shape above and run `gdql import json <file>`.
