# Data sources and import strategy

## setlist.fm: how many requests?

- API returns **20 setlists per page** (fixed; cannot be increased).
- Grateful Dead have ~2,300 setlists → **~115–120 API requests** for a full import.
- Free tier: **1,440 requests/day**.

So **one full GD import fits in a single day’s quota** (you’d use ~120 of 1,440). If you hit 429, it’s usually because the key was already used elsewhere that day (browser, another tool, or a previous run). Options: wait until the next UTC day, or request the upgraded limit (50k/day) at [setlist.fm/settings/api](https://www.setlist.fm/settings/api).

---

## Alternative sources (APIs and scraped)

| Source | Data | Access | Notes |
|--------|------|--------|--------|
| **setlist.fm** | Setlists, venues, dates, segues | REST API (key, rate limited) | Current `gdql import setlistfm`. |
| **Archive.org** | Recordings + metadata (track titles, dates, venues) | Public APIs, no key | Metadata/track lists can imply setlists. No dedicated “setlist” endpoint; need to parse item metadata. |
| **Relisten** | Setlists + streaming metadata | Open API (relistenapi.alecgorge.com) | Wraps Archive.org and others. Open source; could be a second importer. |
| **Jerrybase** | Song histories, setlists | Web | No official API; setlist data on show pages. |
| **Whitegum / dead.net** | Setlist data | Scraping / limited | Older or limited; more work for uncertain gain. |

**Recommendation for “better source” right now:**

- **Keep setlist.fm** as the primary, legal, defined pipeline. One full import is ~120 requests; with the upgraded limit it’s trivial.
- **Add Relisten or Archive.org** as an alternative or supplement later (e.g. `gdql import relisten` or `gdql import archive`) if you want a second source or extra metadata (e.g. links to recordings). Both are usable from Go.

---

## Go vs Python for import

**Recommendation: keep import in Go.**

- **Single binary**: `gdql init`, `gdql import setlistfm`, `gdql "SHOWS FROM 1977"` — no extra runtime or scripts.
- **One language**: Parsing, planning, SQL gen, and data pipeline all in one place; easier to refactor and test.
- **Scraping in Go** is doable (e.g. `colly`, `goquery`, or `net/http` + a parser). Slightly more verbose than Python, but for “fetch API → map to schema → write SQLite” we’re already doing that in Go with setlist.fm.

**When Python (or a separate script) might make sense:**

- A site has **no API** and is heavily JS-rendered or anti-bot; a small Python (or Node) script that **outputs JSON or CSV** for the Go importer to consume keeps the main tool in Go and avoids a dependency in the shipped binary.
- One-off data fixes or experiments where you want to iterate quickly; the “authoritative” import path can still be Go.

**Canonical format:** Any source (API or scrape) can produce `[]canonical.Show` and call `canonical.WriteShows` (see **docs/CANONICAL_IMPORT.md** and `internal/import/canonical`). That doc also has the JSON shape for scrapers or a future `gdql import json`.

**Summary:** Use Go for the official import pipeline. Add other **API** sources (Relisten, Archive.org) in Go when needed. Use a separate scraper (Python or other) only if we need a source with no API, and have it feed data into the same SQLite schema (e.g. via a small Go import that reads JSON/CSV).
