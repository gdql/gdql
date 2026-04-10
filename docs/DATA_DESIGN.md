# GDQL Data Design

## The Core Question

Where does the data come from, and how do we make it fast to query?

---

## Data Sources

### Primary: setlist.fm API
- **Coverage**: Comprehensive setlist data for GD and related bands
- **API**: Public REST API (requires free API key)
- **Quality**: Community-maintained, generally accurate
- **Limitations**: Rate limited, no jam lengths, no lyrics

### Secondary: Archive.org (Live Music Archive)
- **Coverage**: 14,000+ GD recordings with metadata
- **API**: Public, no auth required
- **Quality**: Detailed metadata including recording info
- **Bonus**: Could link directly to streamable audio

### Supplementary Sources
| Source | Data | Notes |
|--------|------|-------|
| Jerrybase | Song histories, bust-out tracking | Would need scraping |
| Whitegum.com | Setlist data | Older, static data |
| Dead.net | Official info | Limited API access |
| Relisten API | Streaming metadata | Wraps archive.org |

### Lyrics
- Lyrics aren't in setlist databases
- Options: GDAO, custom scrape, or ship a curated lyrics file
- This could be a separate data file that ships with gdql

### Song title normalization
- **Titles containing ">"**: Sources sometimes give one item like "Scarlet Begonias > Fire on the Mountain". We **split** at ETL into two performances; canonical song names in the DB never contain `>`.
- **Jams/segments**: Names like "Jam", "Drums", "Space", "Phil's Jam", "Drums & Space" are normalized to a small set of canonical segment "songs" (Jam, Drums, Space, Drums & Space) via aliases.
- **Spelling/punctuation**: One canonical form per song; variants (St. vs St, US vs U.S.) handled via `song_aliases`.

See **[SONG_NORMALIZATION.md](SONG_NORMALIZATION.md)** for the full strategy.

**Broader authority issues**: When many uploaders contribute setlists with no single standard, we also face duplicate songs, set-order conflicts, set numbering (encore/soundcheck), venue/date disambiguation, and more. See **[DATA_AUTHORITY_PROBLEMS.md](DATA_AUTHORITY_PROBLEMS.md)** for a full catalog and handling strategies.

---

## Storage Strategy: Embedded SQLite

**Recommendation: Ship a pre-built SQLite database with the binary.**

### Why SQLite?

1. **Zero setup** - Users don't configure anything
2. **Single file** - Easy to distribute and update
3. **Fast** - Handles complex queries well with proper indexes
4. **Portable** - Works everywhere Go works
5. **Proven** - Battle-tested, won't corrupt your data
6. **Tooling** - Users can inspect/extend with standard SQLite tools

### Distribution Model

```
gdql                    # Binary (~10MB)
~/.gdql/
├── shows.db            # SQLite database (~50-100MB)
├── lyrics.db           # Lyrics database (optional, ~5MB)  
└── config.toml         # User preferences
```

**First run:**
```bash
$ gdql
No database found. Downloading latest show data...
Downloaded shows.db (87MB) to ~/.gdql/
Ready! Try: SHOWS FROM 1977 LIMIT 5;
```

**Updates:**
```bash
$ gdql update
Checking for updates...
Downloaded 47 new shows since last sync.
Database updated.
```

---

## Schema Design

### Core Tables

```sql
-- Venues
CREATE TABLE venues (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT,
    state TEXT,
    country TEXT,
    capacity INTEGER,
    latitude REAL,
    longitude REAL
);

-- Shows
CREATE TABLE shows (
    id INTEGER PRIMARY KEY,
    date DATE NOT NULL UNIQUE,
    venue_id INTEGER REFERENCES venues(id),
    tour TEXT,                    -- "Europe 72", "Fall 1977", etc.
    notes TEXT,
    soundboard BOOLEAN,           -- SBD recording exists?
    archive_id TEXT,              -- archive.org identifier
    rating REAL                   -- average community rating
);

-- Songs (the catalog, not performances)
CREATE TABLE songs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    short_name TEXT,              -- "FOTD" for "Friend of the Devil"
    writers TEXT,                 -- "Hunter/Garcia"
    first_played DATE,
    last_played DATE,
    times_played INTEGER,
    is_cover BOOLEAN,
    original_artist TEXT,         -- if cover
    tempo_bpm INTEGER,            -- rough tempo for "ballad" queries
    typical_length_seconds INTEGER
);

-- Performances (a specific song at a specific show)
CREATE TABLE performances (
    id INTEGER PRIMARY KEY,
    show_id INTEGER NOT NULL REFERENCES shows(id),
    song_id INTEGER NOT NULL REFERENCES songs(id),
    set_number INTEGER,           -- 1, 2, 3 (encore), 0 (soundcheck)
    position INTEGER NOT NULL,    -- order within the set
    
    -- Transition to NEXT song
    segue_type TEXT,              -- '>' (segue), '>>' (break), '~>' (tease)
    
    -- Performance details (when available)
    length_seconds INTEGER,
    is_opener BOOLEAN,            -- opened the set?
    is_closer BOOLEAN,            -- closed the set?
    guest TEXT,                   -- guest musician if any
    notes TEXT,                   -- "with extended jam", "acoustic", etc.
    
    UNIQUE(show_id, set_number, position)
);

-- Lyrics (separate table, possibly separate DB)
CREATE TABLE lyrics (
    song_id INTEGER PRIMARY KEY REFERENCES songs(id),
    lyrics TEXT,
    lyrics_fts TEXT               -- for full-text search
);

-- Musicians (for guest queries)
CREATE TABLE musicians (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    instrument TEXT,
    member_type TEXT              -- 'core', 'touring', 'guest'
);

CREATE TABLE show_musicians (
    show_id INTEGER REFERENCES shows(id),
    musician_id INTEGER REFERENCES musicians(id),
    PRIMARY KEY (show_id, musician_id)
);
```

### Indexes for Fast Queries

```sql
-- Date range queries: SHOWS FROM 1977-1980
CREATE INDEX idx_shows_date ON shows(date);

-- Song lookup: WHERE "Scarlet Begonias"
CREATE INDEX idx_songs_name ON songs(name);
CREATE INDEX idx_songs_short ON songs(short_name);

-- Finding performances of a song
CREATE INDEX idx_perf_song ON performances(song_id);
CREATE INDEX idx_perf_show ON performances(show_id);

-- Set position queries: SET2 OPENED "Samson"
CREATE INDEX idx_perf_position ON performances(show_id, set_number, position);

-- Segue queries (the tricky ones)
CREATE INDEX idx_perf_segue ON performances(show_id, set_number, position, song_id, segue_type);

-- Venue queries: SHOWS AT "Fillmore"
CREATE INDEX idx_venues_name ON venues(name);
CREATE INDEX idx_shows_venue ON shows(venue_id);

-- Full-text search on lyrics
CREATE VIRTUAL TABLE lyrics_fts USING fts5(lyrics, content=lyrics, content_rowid=song_id);
```

---

## The Segue Query Problem

The most interesting GDQL queries involve transitions:

```sql
SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";
SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";
```

### Solution: Self-Join with Position

```sql
-- Find shows where Scarlet > Fire
SELECT DISTINCT s.date, v.name
FROM performances p1
JOIN performances p2 ON p1.show_id = p2.show_id 
                    AND p1.set_number = p2.set_number
                    AND p1.position = p2.position - 1
JOIN songs s1 ON p1.song_id = s1.id
JOIN songs s2 ON p2.song_id = s2.id
JOIN shows s ON p1.show_id = s.id
JOIN venues v ON s.venue_id = v.id
WHERE s1.name = 'Scarlet Begonias'
  AND s2.name = 'Fire on the Mountain'
  AND p1.segue_type = '>';  -- actual segue, not just "followed by"
```

For 3+ song chains (Help > Slip > Frank), we chain more joins:

```sql
-- GDQL: "Help on the Way" > "Slipknot!" > "Franklin's Tower"
SELECT DISTINCT s.date
FROM performances p1
JOIN performances p2 ON p1.show_id = p2.show_id 
                    AND p1.set_number = p2.set_number
                    AND p1.position = p2.position - 1
JOIN performances p3 ON p2.show_id = p3.show_id 
                    AND p2.set_number = p3.set_number
                    AND p2.position = p3.position - 1
JOIN songs s1 ON p1.song_id = s1.id AND s1.name = 'Help on the Way'
JOIN songs s2 ON p2.song_id = s2.id AND s2.name = 'Slipknot!'
JOIN songs s3 ON p3.song_id = s3.id AND s3.name = "Franklin's Tower"
JOIN shows s ON p1.show_id = s.id
WHERE p1.segue_type = '>' AND p2.segue_type = '>';
```

With proper indexes, these queries are fast (< 50ms on the full dataset).

### Alternative: Denormalized Setlist String

For simpler pattern matching, we could also store a denormalized setlist:

```sql
CREATE TABLE show_setlists (
    show_id INTEGER PRIMARY KEY,
    set1 TEXT,  -- "Jack Straw > Tennessee Jed >> Cassidy > ..."
    set2 TEXT,
    encore TEXT
);

-- Now we can do pattern matching:
SELECT * FROM show_setlists WHERE set2 LIKE '%Scarlet Begonias > Fire on the Mountain%';
```

**Recommendation**: Use both. Normalized for precise queries, denormalized for pattern matching and display.

---

## Data Size Estimates

| Entity | Count | Notes |
|--------|-------|-------|
| Shows | ~2,300 | 1965-1995 |
| Songs | ~450 | Including covers |
| Performances | ~40,000 | ~17 songs per show avg |
| Venues | ~500 | Unique venues |
| Lyrics | ~300 | Original songs only |

**Estimated DB size**: 50-100MB uncompressed, ~20MB compressed

This is small enough to:
- Download quickly on first run
- Ship embedded in the binary if desired
- Keep entirely in memory for fastest queries

---

## Query Execution Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│  GDQL Query                                                     │
│  SHOWS FROM 77-80 WHERE "Scarlet" > "Fire" ORDER BY DATE;       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Lexer/Parser → AST                                             │
│  {type: "show_query", from: {start: 1977, end: 1980},           │
│   where: {type: "segue", songs: ["Scarlet", "Fire"], op: ">"},  │
│   order: {field: "date", dir: "asc"}}                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Query Planner                                                  │
│  - Resolve song names ("Scarlet" → "Scarlet Begonias")          │
│  - Expand date ranges (77-80 → 1977-01-01 to 1980-12-31)        │
│  - Generate SQL query                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  SQL Execution (SQLite)                                         │
│  SELECT s.date, v.name, ... FROM performances p1 ...            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Result Formatter                                               │
│  - Default: table                                               │
│  - AS JSON: {"shows": [...]}                                    │
│  - AS SETLIST: formatted setlist text                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Data Pipeline (Build Process)

```bash
# One-time data build (run by maintainers, not users)
$ cd gdql/data

# Fetch from setlist.fm
$ go run ./cmd/fetch-setlistfm --output raw/setlistfm.json

# Fetch from archive.org  
$ go run ./cmd/fetch-archive --output raw/archive.json

# Merge and normalize
$ go run ./cmd/build-db --input raw/ --output shows.db

# Validate
$ go run ./cmd/validate --db shows.db

# Publish
$ gh release upload v0.1.0 shows.db
```

Users just download the pre-built `shows.db`.

---

## Offline-First, Update Optional

The tool should work completely offline after initial setup:

```bash
# Works offline
$ gdql 'SHOWS FROM 1977'

# Optional: check for data updates
$ gdql update --check
New data available: 12 shows added since 2025-01-15

# Optional: download updates
$ gdql update
```

---

## Open Questions

1. **Lyrics licensing** - Can we ship lyrics, or just song titles?
2. **Jam lengths** - setlist.fm doesn't have these; archive.org metadata might
3. **Song name aliases** - How fuzzy? "Scarlet" → "Scarlet Begonias"?
4. **Tease tracking** - Is there a good source for this data?
5. **Related bands** - Include JGB, Phil & Friends, Dead & Co?

---

## Recommendation Summary

| Aspect | Decision |
|--------|----------|
| Storage | SQLite, single file |
| Location | `~/.gdql/shows.db` |
| Distribution | Download on first run |
| Updates | Manual `gdql update` command |
| Primary source | setlist.fm API |
| Lyrics | Separate optional file |
| Query engine | Parse GDQL → Generate SQL → Execute |

This gives us:
- Zero-config user experience
- Fast queries (SQLite + indexes)
- Offline capability
- Easy updates
- Standard tooling (users can sqlite3 the db directly)
