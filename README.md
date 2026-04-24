# GDQL - Grateful Dead Query Language

[![CI](https://github.com/gdql/gdql/actions/workflows/ci.yml/badge.svg)](https://github.com/gdql/gdql/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gdql/gdql.svg)](https://pkg.go.dev/github.com/gdql/gdql)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

SQL for Deadheads. Query every show, setlist, and song from 30 years of the Grateful Dead. Because someone had to bring structure to the most beautifully unstructured band in history.

**[Documentation](https://docs.gdql.dev)** | **[Try it in the Sandbox](https://sandbox.gdql.dev)** | **[Releases](https://github.com/gdql/gdql/releases)**

<p align="center">
  <img src="https://raw.githubusercontent.com/gdql/.github/main/demo.gif" alt="GDQL demo" width="720">
</p>

```sql
SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";

SONGS WITH LYRICS("train", "road", "rose") WRITTEN 1968-1970;

SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";

PERFORMANCES OF "Dark Star" FROM 1972 WITH LENGTH > 20min;
```

**Try:** [Scarlet→Fire](https://sandbox.gdql.dev?example=scarlet-fire&run=1) · [SONGS LYRICS](https://sandbox.gdql.dev?example=songs-lyrics&run=1) · [Help→Slip→Frank](https://sandbox.gdql.dev?example=help-slip-frank&run=1) · [Dark Star](https://sandbox.gdql.dev?example=dark-star&run=1)

## What is this?

You know that argument about whether the 5/8/77 or 2/13/70 Dark Star is better? GDQL won't settle it, but it'll tell you every show where Dark Star appeared, what it segued into, and how long the jam lasted. It's a query language for people who think setlist.fm doesn't have enough operators.

- **Segues** - The `>` operator finds song transitions. `"Scarlet Begonias" > "Fire on the Mountain"` does what you think it does.
- **Lyrics** - Search the catalog by words. Find every song about trains, or roses, or whatever Garcia was thinking about.
- **Eras** - `FROM EUROPE72` or `FROM BRENT_ERA`. Because the band in '69 and the band in '89 were basically different species.
- **Venues** - Every Fillmore, Winterland, and college gymnasium they ever played.
- **Setlists** - `SETLIST FOR 5/8/77` gives you Cornell. You already knew that date by heart.

## Quick Examples

```sql
-- Every Scarlet > Fire from the golden era
SHOWS FROM 77-79 WHERE "Scarlet Begonias" > "Fire on the Mountain";

-- Songs about trains (there are more than you'd think)
SONGS WITH LYRICS("train", "railroad", "engineer");

-- The longest Dark Stars — for the truly committed
PERFORMANCES OF "Dark Star" ORDER BY LENGTH DESC LIMIT 10;

-- What happened at the Fillmore in '69?
SHOWS AT "Fillmore West" FROM 1969;

-- The setlist everyone argues about
SETLIST FOR 5/8/77;
```

**Try in Sandbox:** [Scarlet→Fire](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSA3Ny03OSBXSEVSRSAiU2NhcmxldCBCZWdvbmlhcyIgPiAiRmlyZSBvbiB0aGUgTW91bnRhaW4iOw&run=1) · [SONGS LYRICS](https://sandbox.gdql.dev?q=U09OR1MgV0lUSCBMWVJJQ1MoInRyYWluIiwgInJhaWxyb2FkIiwgImVuZ2luZWVyIik7&run=1) · [Dark Star by length](https://sandbox.gdql.dev?q=UEVSRk9STUFOQ0VTIE9GICJEYXJrIFN0YXIiIE9SREVSIEJZIExFTkdUSCBERVNDIExJTUlUIDEwOw&run=1) · [Cornell setlist](https://sandbox.gdql.dev?q=U0VUTElTVCBGT1IgNS84Lzc3Ow&run=1)

## Installation

### Download a release (recommended)

1. Download `gdql` from **[Releases](https://github.com/gdql/gdql/releases/latest)** (`gdql.exe` on Windows)
2. Put it on your PATH
3. Run it

```bash
gdql "SHOWS FROM 1977 LIMIT 5"
```

The database is baked into the binary. No separate files to download.

### Build from source (requires Go 1.24+)

```bash
git clone https://github.com/gdql/gdql
cd gdql
go build -o gdql ./cmd/gdql
```

## Usage

```bash
gdql "SHOWS FROM 1977 LIMIT 5"
gdql -f query.gdql
echo ‘SHOWS FROM 1977;’ | gdql -
```

Use `-db <path>` to query a custom database instead of the embedded one.

**PowerShell:** queries with `>` or quotes can get mangled. Use `-f query.gdql` or wrap in single quotes:

```powershell
gdql ‘SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";’
```
- **Backtick-escape** the inner double quotes:  
  `.\gdql.exe "SHOWS WHERE \`"Scarlet Begonias\`" > \`"Fire on the Mountain\`""`

*Planned: interactive REPL, -e flag.*

## Documentation

- **[docs.gdql.dev](https://docs.gdql.dev)** — Hosted docs site (cookbook, cheat sheet, language reference, data pipelines).
- **[docs/LANGUAGE.md](docs/LANGUAGE.md)** — Language reference (living doc; we update it as we add features).
- **[docs/INSTALL_GO_WSL.md](docs/INSTALL_GO_WSL.md)** — How to install Go on WSL.
- **[DESIGN.md](DESIGN.md)** — Full language design and ideas.
- **[SPEC.md](SPEC.md)** — Implementation spec and grammar.

## Data pipelines

The embedded database is enriched by several scripts that land in
`data/` alongside the DB. See the [Data Pipelines doc](https://docs.gdql.dev/data-pipelines/)
for the full overview; short summary:

| Script | Produces | Purpose |
|---|---|---|
| `scripts/scrape_lyrics.go` | `lyrics.json` | Genius scrape for songs with ≥N plays |
| `scripts/geocode_venues.py` | `data/venues_geo.json` | Nominatim lat/lon for every venue |
| `scripts/fetch_weather.py` | `data/weather.json` | Open-Meteo historical daily for every show |

### `gdql-import` subcommands

```bash
gdql-import [-db <path>] setlistfm                      # import shows from setlist.fm API
gdql-import [-db <path>] json <file>                    # import from canonical JSON
gdql-import [-db <path>] lyrics <file.json>             # lyrics JSON (from scrape_lyrics)
gdql-import [-db <path>] aliases <file.json>            # setlist-text → canonical song
gdql-import [-db <path>] relations <file.json>          # song-to-song cross-refs
gdql-import [-db <path>] merge-songs <file.json>        # apply kind=merge_into destructively
gdql-import [-db <path>] fix-sets                       # re-infer set numbers
```

### CI automation

- **`.github/workflows/enrich-data.yml`** — path-filtered jobs that re-run the three
  enrichment scripts when their inputs change, then open a PR per change.
  Also runs weekly as a drift safety net.
- **`.github/workflows/release.yml`** — on `v*` tag push, builds release binaries
  and dispatches `repository_dispatch` events to downstream consumers
  (`gdql/sandbox`, `samburba/deaddaily-timeline` aka *Dead Daily Explore*,
  `samburba/deaddaily-listen` aka *Dead Daily Listen*).

**Go API docs:** From the repo root, run `go doc ./...` to see package and symbol docs. Add `// Comment` above exported types and functions to build those docs as you go.

### Running tests

```bash
go test ./...                    # all tests
go test -v ./test/acceptance/    # example / docs-style E2E tests only
go test ./test/acceptance/ -run TestE2E_SetlistForDate   # one example test
```

The **acceptance** tests run the same kinds of queries as in the README and docs (e.g. SHOWS FROM 1977, Scarlet > Fire, SETLIST FOR 5/8/77, SONGS WITH LYRICS, PERFORMANCES OF "Dark Star") against a fixture DB.

## Status

Fully functional. Parses, plans, and executes against SQLite. Supports SHOWS, SONGS, PERFORMANCES, SETLIST with date ranges, segue chains, position/played/guest conditions, and table/JSON/CSV output. The whole setlist database ships inside the binary — just run it.

## Data sources

The embedded database is built from these sources:

- **[Deadlists (setlists.net)](http://www.setlists.net/)** — show dates, venues, setlists with proper set/encore structure
- **[Relisten](https://relisten.net/) / [archive.org](https://archive.org/details/GratefulDead)** — track durations from the Live Music Archive recordings
- **[Relisten](https://relisten.net/)** — lyrics data

The `>` operator in queries means "next song in the setlist" — not necessarily a musical segue. Real segue data is hard to source at scale, so GDQL uses position as a proxy and marks a curated list of known segue pairs (Scarlet > Fire, China Cat > Rider, etc.).

If you spot missing or incorrect data, [open an issue](https://github.com/gdql/gdql/issues).

## License

MIT

---

*"Once in a while you get shown the light, in the strangest of places if you look at it right."*
