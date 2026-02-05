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

### Source

- **Relisten API** — Public API for live music setlists (no key required). Uses `GET /api/v2/artists/grateful-dead/years/{year}` and per-show details for setlists.

If the API base URL changes, edit `RELISTEN_BASE` in the script (e.g. `https://relistenapi.alecgorge.com/api/v2`).
