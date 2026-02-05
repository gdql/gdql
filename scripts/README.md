# Scripts

## populate_canonical_json.py

Fetches Grateful Dead setlist data from the **Relisten API** and writes a JSON file in the [canonical format](../docs/CANONICAL_IMPORT.md) for `gdql import json`.

### Setup

```bash
pip install requests
```

### Usage

```bash
# Default: 1965–1995, output shows.json
python scripts/populate_canonical_json.py -o shows.json

# Specific years
python scripts/populate_canonical_json.py -o shows.json --years 1972 1977 1978

# Custom year range
python scripts/populate_canonical_json.py -o shows.json --first-year 1970 --last-year 1980

# Slower (more polite to the API)
python scripts/populate_canonical_json.py -o shows.json --delay 0.5
```

### Import into GDQL

```bash
gdql init
gdql import json shows.json
gdql "SHOWS FROM 1977 LIMIT 5"
```

### Embed full dataset in the binary (optional)

To ship the gdql binary with **all** your imported shows (so users get the full DB by default):

1. **Import into a DB file** (use a dedicated path so you don’t overwrite your working DB):

   ```bash
   gdql import json shows.json -db full.db
   ```

2. **Copy that DB into the embed location** and rebuild:

   ```bash
   go run ./cmd/build_embed_db --from full.db
   go build -o gdql ./cmd/gdql
   ```

   `build_embed_db --from full.db` copies `full.db` to `cmd/gdql/embeddb/default.db`, which is embedded in the binary. The next `go build` produces a binary that unpacks this full DB when run with the default path and no existing DB.

3. **Optional:** Load song aliases so query names match Relisten variants (e.g. `"Scarlet Begonias"` matches `"Scarlet Begonias-"`):

   ```bash
   gdql -db full.db import aliases data/song_aliases.json
   go run ./cmd/build_embed_db --from full.db
   go build -o gdql ./cmd/gdql
   ```

**Note:** The embedded DB can get large (many MB) if you have thousands of shows. Use a smaller JSON range (e.g. `--years 1972 1977 1978`) if you want a slimmer binary.

### Source

- **Relisten API** — Public API for live music setlists (no key required). Uses `GET /api/v2/artists/grateful-dead/years/{year}` and per-show details for setlists.

If the API base URL changes, edit `RELISTEN_BASE` in the script (e.g. `https://relistenapi.alecgorge.com/api/v2`).
