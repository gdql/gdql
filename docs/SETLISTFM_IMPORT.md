# Importing data from setlist.fm

GDQL can populate its database from the [setlist.fm API](https://api.setlist.fm/docs/1.0/index.html).

## Get an API key

1. **Register** at [setlist.fm](https://www.setlist.fm/signup) (free).
2. **Request an API key** at **[https://www.setlist.fm/settings/api](https://www.setlist.fm/settings/api)** (logged-in users only).  
   Your key is generated immediately.
3. The API is **free for non-commercial use**. For commercial use, [contact setlist.fm](https://www.setlist.fm/contact).

## Rate limits

- **Max 2.0 requests/second** and **max 1440 requests/day** (free tier).
- The importer throttles requests to stay within these limits. You can request an upgrade on the [API settings page](https://www.setlist.fm/settings/api) if needed.

## Usage

Set your key via the environment (do not commit the key or put it in scripts).

**Bash / zsh:**
```bash
export SETLISTFM_API_KEY="your-key-here"
gdql import setlistfm
# or with a specific DB:
gdql -db ~/.gdql/shows.db import setlistfm
```

**PowerShell:**
```powershell
$env:SETLISTFM_API_KEY = "your-key-here"
.\gdql.exe import setlistfm
# or: gdql -db shows.db import setlistfm
```

The importer fetches Grateful Dead setlists (by MusicBrainz ID), maps them to the GDQL schema, and inserts venues, shows, songs, and performances. Because it fetches each setlist by ID for full song data, a full run uses ~2,450 requests (over the free 1,440/day).

### If you hit 429 (Too Many Requests)

- **Do not delete `shows.db`.** Run the same command again after your daily limit resets (e.g. next day). The importer skips shows already in the DB and continues with the rest.
- Optionally request a higher limit (50k/day) at [setlist.fm/settings/api](https://www.setlist.fm/settings/api) so a full import finishes in one run.

## API reference

- **Docs:** [https://api.setlist.fm/docs/1.0/](https://api.setlist.fm/docs/1.0/)
- **API key:** [https://www.setlist.fm/settings/api](https://www.setlist.fm/settings/api)
- **Terms:** [https://www.setlist.fm/help/api-terms](https://www.setlist.fm/help/api-terms)
