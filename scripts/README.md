# Scripts

## populate_canonical_json.py

Fetches Grateful Dead setlist data from the **Relisten API** and writes canonical JSON for `gdql-import json`.

```bash
pip install requests
python scripts/populate_canonical_json.py -o shows.json
```

Note: The primary data source is the **Deadlists crawler** (`gdql-import deadlists`), which provides proper set/encore structure. This script is an alternative source.

## scrape_lyrics.go

Fetches lyrics for songs in the database.

## verify/main.go

Runs hand-curated query assertions against the database.
