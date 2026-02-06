# GDQL - Grateful Dead Query Language

A novelty query language for searching through Grateful Dead shows, setlists, and songs.

> **Development Philosophy: Test-First Driven Design (TDD)**  
> Every feature in GDQL is built test-first. We write failing tests before implementation.  
> See [TESTING_STRATEGY.md](TESTING_STRATEGY.md) for our comprehensive testing approach.

```sql
SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";

SONGS WITH LYRICS("train", "road", "rose") WRITTEN 1968-1970;

SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";

PERFORMANCES OF "Dark Star" FROM 1972 WITH LENGTH > 20min;
```

**Try in Sandbox:** [Scarlet→Fire](https://sandbox.gdql.dev?example=scarlet-fire&run=1) · [SONGS LYRICS](https://sandbox.gdql.dev?example=songs-lyrics&run=1) · [Help→Slip→Frank](https://sandbox.gdql.dev?example=help-slip-frank&run=1) · [Dark Star](https://sandbox.gdql.dev?example=dark-star&run=1)

## What is this?

GDQL is a SQL-inspired domain-specific language designed for querying the Grateful Dead's live performance history. It provides intuitive, music-centric syntax for exploring:

- **Setlists** - What songs were played at which shows
- **Segues** - Song transitions (the famous Scarlet > Fire, Help > Slip > Frank)
- **Lyrics** - Search songs by lyrical content
- **Jams** - Find extended improvisations
- **Venues** - Search by location
- **Eras** - Query by band periods (Pigpen era, Brent era, etc.)

## Quick Examples

```sql
-- Find shows with the Scarlet > Fire combo
SHOWS FROM 77-79 WHERE "Scarlet Begonias" > "Fire on the Mountain";

-- Songs about trains
SONGS WITH LYRICS("train", "railroad", "engineer");

-- The longest Dark Stars
PERFORMANCES OF "Dark Star" ORDER BY LENGTH DESC LIMIT 10;

-- Shows at the Fillmore
SHOWS AT "Fillmore West" FROM 1969;

-- What did they play at Cornell '77?
SETLIST FOR 5/8/77;
```

**Try in Sandbox:** [Scarlet→Fire](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSA3Ny03OSBXSEVSRSAiU2NhcmxldCBCZWdvbmlhcyIgPiAiRmlyZSBvbiB0aGUgTW91bnRhaW4iOw&run=1) · [SONGS LYRICS](https://sandbox.gdql.dev?q=U09OR1MgV0lUSCBMWVJJQ1MoInRyYWluIiwgInJhaWxyb2FkIiwgImVuZ2luZWVyIik7&run=1) · [Dark Star by length](https://sandbox.gdql.dev?q=UEVSRk9STUFOQ0VTIE9GICJEYXJrIFN0YXIiIE9SREVSIEJZIExFTkdUSCBERVNDIExJTUlUIDEwOw&run=1) · [Cornell setlist](https://sandbox.gdql.dev?q=U0VUTElTVCBGT1IgNS84Lzc3Ow&run=1)

## What gets built

**`go build` produces a single binary that includes a default database.**

- The **default DB** (schema + seed: Cornell ’77, Scarlet > Fire, a few songs) is **embedded** in the binary (`cmd/gdql/embeddb/default.db`). When the user runs `gdql` without `-db`, that file is unpacked to the config dir (e.g. `~/.config/gdql/shows.db`) and used. Use `-db <path>` to point at a different database.
- **`gdql init [path]`** still creates a fresh DB from embedded schema+seed at the given path. To **regenerate** the embedded default DB after changing schema or seed, run from repo root: `go run ./cmd/build_embed_db`, then rebuild.

## Installation

### Download a release (recommended)

Pre-built binaries and a pre-built **shows.db** are published on GitHub Releases. No build or import required.

- **[Releases](https://github.com/gdql/gdql/releases)** — download `gdql` (or `gdql.exe` on Windows) and optionally `shows.db`. Put the binary on your PATH; use `-db shows.db` or `GDQL_DB` if the database is not in the current directory.

### Where to put files when you install the binary

| What | Where | Notes |
|------|--------|------|
| **Binary** | Anywhere on your PATH (e.g. `/usr/local/bin`, `~/bin`, or `C:\tools`) | Only file required; see below. |
| **Database** | Any path you like | Default: embedded DB (unpacked to config dir, e.g. `~/.config/gdql/shows.db`). Use `-db <path>` to override. |
| **Alias file** | Any path | Optional. Pass path when running: `gdql import aliases <path/to/aliases.json>`. Example: [data/song_aliases.json](data/song_aliases.json). |
| **Query files** | Any path | Pass with `-f`: `gdql -db shows.db -f query.gdql`. |

### Packaging for easy install (one binary)

**The default database is embedded in the binary.** When someone runs `gdql` without `-db`, the program uses the embedded DB, unpacking it to the config directory (e.g. `~/.config/gdql/shows.db`) on first use. So:

- **Package = single binary.** Install the `gdql` (or `gdql.exe`) binary to a directory on PATH. No separate DB file is required. First run unpacks the embedded default DB to the config dir; the user can run queries immediately.
- **To change the embedded default DB:** from repo root run `go run ./cmd/build_embed_db`, then rebuild. Use `go run ./cmd/build_embed_db --from full.db` to embed a DB you built (e.g. after `gdql import json shows.json -db full.db`); see [scripts/README.md](scripts/README.md) for the full flow. The file `cmd/gdql/embeddb/default.db` is committed so normal `go build` works.
- **Optional:** To ship a *larger* pre-filled database without embedding, build one (e.g. `gdql import json shows.json -db full.db`), then install it as e.g. `/usr/share/gdql/shows.db` and set `GDQL_DB` or document `-db /usr/share/gdql/shows.db`.

### Build from source (requires Go 1.21+)

```bash
cd /path/to/gdql
go mod tidy
go install ./cmd/...
```

This installs the `gdql` binary to `$GOBIN` (default `$GOPATH/bin`). Ensure that directory is on your `PATH`.

To build without installing:

```bash
go build -o gdql ./cmd/gdql
```

On **Windows**, the binary is `gdql.exe`. Run it explicitly so the shell doesn't prompt "open with":

```powershell
go build -o gdql.exe ./cmd/gdql
.\gdql.exe -f query.gdql
```

**Data:** Use `shows.db` from [Releases](https://github.com/gdql/gdql/releases), or run `gdql init` for a minimal DB, or `gdql import setlistfm` (with `SETLISTFM_API_KEY`) to import from setlist.fm. To add song name variants (e.g. so `"Scarlet Begonias"` matches sources that store `"Scarlet Begonias-"`), use **`gdql import aliases <file.json>`** — see [data/song_aliases.json](data/song_aliases.json) and [SONG_NORMALIZATION.md](SONG_NORMALIZATION.md).

## Usage

```bash
# Run a query (needs a database; default path: shows.db or GDQL_DB env)
gdql -db shows.db "SHOWS FROM 1977 LIMIT 5"

# From a file. Save as UTF-8 without BOM.
gdql -db shows.db -f query.gdql

# From stdin
echo 'SHOWS FROM 1977;' | gdql -db shows.db -
```

**On Windows PowerShell** the current directory is not on `PATH`. Run the executable explicitly:

```powershell
.\gdql.exe -db shows.db "SHOWS FROM 1977 LIMIT 5"
.\gdql.exe -db shows.db -f query.gdql
```

**Queries with song names** are often mangled by PowerShell (quotes stripped or `>` treated as redirection). Use one of:

- **`-f` file** (recommended): put the query in `query.gdql` and run `.\gdql.exe -f query.gdql`.
- **Whole query in single quotes**, double quotes for song names:  
  `.\gdql.exe 'SHOWS FROM 1969 WHERE PLAYED "St Stephen" > "The Eleven";'`
- **Backtick-escape** the inner double quotes:  
  `.\gdql.exe "SHOWS WHERE \`"Scarlet Begonias\`" > \`"Fire on the Mountain\`""`

*Planned: interactive REPL, -e flag.*

## Documentation

- **[docs/LANGUAGE.md](docs/LANGUAGE.md)** — Language reference (living doc; we update it as we add features).
- **[docs/INSTALL_GO_WSL.md](docs/INSTALL_GO_WSL.md)** — How to install Go on WSL.
- **[DESIGN.md](DESIGN.md)** — Full language design and ideas.
- **[SPEC.md](SPEC.md)** — Implementation spec and grammar.

**Go API docs:** From the repo root, run `go doc ./...` to see package and symbol docs. Add `// Comment` above exported types and functions to build those docs as you go.

### Running tests

```bash
go test ./...                    # all tests
go test -v ./test/acceptance/    # example / docs-style E2E tests only
go test ./test/acceptance/ -run TestE2E_SetlistForDate   # one example test
```

The **acceptance** tests run the same kinds of queries as in the README and docs (e.g. SHOWS FROM 1977, Scarlet > Fire, SETLIST FOR 5/8/77, SONGS WITH LYRICS, PERFORMANCES OF "Dark Star") against a fixture DB.

## Status

✅ **Functional** — Parse, plan, execute against SQLite. Supports SHOWS, SONGS, PERFORMANCES, SETLIST with date ranges, segue chains (e.g. Scarlet > Fire), position/played/guest conditions, and table/JSON/CSV/setlist output. Run `gdql init` to create a sample DB; query with no `-db` for the embedded default or `-db <path>` to use another DB.

## License

MIT

---

*"What a long, strange trip it's been."*
