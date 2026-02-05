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

## Installation

### Download a release (recommended)

Pre-built binaries and a pre-built **shows.db** are published on GitHub Releases. No build or import required.

- **[Releases](https://github.com/gdql/gdql/releases)** — download `gdql` (or `gdql.exe` on Windows) and optionally `shows.db`. Put the binary on your PATH; use `-db shows.db` or `GDQL_DB` if the database is not in the current directory.

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

**Data:** Use `shows.db` from [Releases](https://github.com/gdql/gdql/releases), or run `gdql init` for a minimal DB, or `gdql import setlistfm` (with `SETLISTFM_API_KEY`) to import from setlist.fm.

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

**Queries with song names** (double quotes) are often mangled by PowerShell. Use either:

- **`-f` file** (recommended): put the query in `query.gdql` and run `.\gdql.exe -f query.gdql`.
- **Backtick-escape** the inner double quotes:  
  `.\gdql.exe "SHOWS WHERE \`"Scarlet Begonias\`" > \`"Fire on the Mountain\`""`

*Planned: interactive REPL, -e flag.*

## Documentation

- **[docs/LANGUAGE.md](docs/LANGUAGE.md)** — Language reference (living doc; we update it as we add features).
- **[docs/INSTALL_GO_WSL.md](docs/INSTALL_GO_WSL.md)** — How to install Go on WSL.
- **[DESIGN.md](DESIGN.md)** — Full language design and ideas.
- **[SPEC.md](SPEC.md)** — Implementation spec and grammar.

**Go API docs:** From the repo root, run `go doc ./...` to see package and symbol docs. Add `// Comment` above exported types and functions to build those docs as you go.

## Status

✅ **Functional** — Parse, plan, execute against SQLite. Supports SHOWS, SONGS, PERFORMANCES, SETLIST with date ranges, segue chains (e.g. Scarlet > Fire), position/played/guest conditions, and table/JSON/CSV/setlist output. Run `gdql init` to create a sample DB, then query with `-db shows.db`.

## License

MIT

---

*"What a long, strange trip it's been."*
