# GDQL - Grateful Dead Query Language

> A domain-specific query language for searching through Grateful Dead shows, setlists, and songs.

## Overview

GDQL is a SQL-inspired query language designed specifically for querying the Grateful Dead's extensive catalog of live performances. The language provides intuitive, music-centric syntax for exploring setlists, song transitions, lyrics, venues, and the many unique aspects of Dead culture.

---

## Language Design Philosophy

1. **SQL-familiar** - Use familiar keywords (`SELECT`, `FROM`, `WHERE`) where appropriate
2. **Domain-native** - Introduce Dead-specific constructs (`INTO`, `SEGUE`, `JAM`, `TEASE`)
3. **Human-readable** - Queries should read almost like English questions about shows
4. **Flexible date handling** - Support multiple date formats (77, 1977, 5/8/77, spring-77)

---

## Core Query Types

### Show Queries

```sql
-- Basic show search with date range
SHOWS FROM 1977-1980;

-- Shows with specific song
SHOWS FROM 77 WHERE PLAYED "Scarlet Begonias";

-- Shows where two songs were played together (segue/transition)
SHOWS FROM 77-80 WHERE "Dire Wolf" INTO "Friend of the Devil";
SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";

-- The legendary segue
SHOWS WHERE "Dark Star" > "St. Stephen" > "The Eleven";

-- Find sandwich jams (song A -> song B -> song A)
SHOWS WHERE "Playing in the Band" SANDWICH;
SHOWS WHERE "Slipknot!" SANDWICHED BY "Franklin's Tower";
```

**Try in Sandbox:** [1977-1980](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSAxOTc3LTE5ODA7&run=1) · [Scarlet Begonias](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSA3NyBXSEVSRSBQTEFZRUQgIlNjYXJsZXQgQmVnb25pYXMiOw&run=1) · [Scarlet→Fire](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSA3Ny04MCBXSEVSRSAiU2NhcmxldCBCZWdvbmlhcyIgPiAiRmlyZSBvbiB0aGUgTW91bnRhaW4iOw&run=1)

### Song Queries

```sql
-- Songs with lyric content
SONGS WITH LYRICS("train", "road");
SONGS WITH LYRICS("mama" OR "papa");

-- Songs by composition date
SONGS WRITTEN 1968-1970;
SONGS WRITTEN BY "Hunter/Garcia";

-- Songs by performance characteristics
SONGS WITH AVG_LENGTH > 15min;
SONGS WITH MAX_LENGTH > 30min;
SONGS NEVER_OPENED;  -- never played as opener
SONGS ONLY_ENCORE;   -- only ever played as encore
```

### Performance Queries

```sql
-- Find specific performances
PERFORMANCES OF "Dark Star" FROM 1968-1974 WITH LENGTH > 20min;

-- Find first/last performances
FIRST "Dark Star";
LAST "Dark Star";
FIRST "Dark Star" > "St. Stephen";  -- first time this segue happened

-- Bust-outs (songs returning after long absence)
BUSTOUTS > 100 SHOWS;  -- songs that returned after 100+ show gap
PERFORMANCES OF "Dark Star" AFTER BUSTOUT;
```

**Try in Sandbox:** [Dark Star](https://sandbox.gdql.dev?q=UEVSRk9STUFOQ0VTIE9GICJEYXJrIFN0YXIiIEZST00gMTk2OC0xOTc0IFdJVEggTEVOR1RIID4gMjBtaW47&run=1)

### Venue Queries

```sql
-- Shows at specific venues
SHOWS AT "Fillmore West";
SHOWS AT VENUE LIKE "Fillmore%";
SHOWS IN "San Francisco" FROM 1969;
SHOWS IN STATE "California" FROM 1970;

-- Venue statistics
VENUES WITH SHOWS > 20;
```

---

## Transition Operators

One of GDQL's key features is expressing song transitions/segues:

| Operator | Meaning | Example |
|----------|---------|---------|
| `>` | Segued into (no break) | `"Scarlet" > "Fire"` |
| `INTO` | Same as `>` | `"Scarlet" INTO "Fire"` |
| `>>` | Followed by (with break) | `"Bertha" >> "Mama Tried"` |
| `THEN` | Same as `>>` | `"Bertha" THEN "Mama Tried"` |
| `~>` | Teased into | `"Dark Star" ~> "The Other One"` |
| `TEASE` | Contained a tease | `"Dark Star" TEASE "The Other One"` |

---

## Special Constructs

### Set Position

```sql
-- Positional queries
SHOWS WHERE SET1 OPENED "Jack Straw";
SHOWS WHERE SET2 CLOSED "Sugar Magnolia";
SHOWS WHERE ENCORE = "U.S. Blues";
SHOWS WHERE SET2 OPENED "Samson and Delilah" FROM 1977;
```

**Try in Sandbox:** [PRIMAL](https://sandbox.gdql.dev?q=U0hPV1MgRlJPTSBQUklNQUw7&run=1)

### Jam Characteristics

```sql
-- Jam-focused queries
SHOWS WHERE "Playing in the Band" JAM > 25min;
JAMS > 20min FROM 1972;
SHOWS WITH DRUMS > 15min;
SHOWS WITH SPACE > 10min;
SHOWS WHERE DRUMS > SPACE;  -- drums longer than space
```

### Guest Appearances

```sql
-- Shows with guests
SHOWS WITH GUEST "Branford Marsalis";
SHOWS WITH HORN_SECTION;
SHOWS WITH GUESTS FROM 1970;
```

### Era Shortcuts

```sql
-- Built-in era aliases
SHOWS FROM PRIMAL;       -- 1965-1969
SHOWS FROM EUROPE72;     -- Spring 1972 Europe tour
SHOWS FROM WALLOFOUND;   -- 1974 (Wall of Sound era)
SHOWS FROM HIATUS;       -- 1975
SHOWS FROM DEAD_ERA;     -- 1965-1995
SHOWS FROM BRENT_ERA;    -- 1979-1990
SHOWS FROM VINCE_ERA;    -- 1990-1995
```

---

## Example Queries (The Fun Ones)

```sql
-- "What shows had that monster Scarlet > Fire from the late 70s?"
SHOWS FROM 77-79 
WHERE "Scarlet Begonias" > "Fire on the Mountain" 
  AND LENGTH("Fire on the Mountain") > 12min;

-- "When did they play Morning Dew as an encore?"
SHOWS WHERE ENCORE = "Morning Dew";

-- "Find shows where Dark Star went into something weird"
SHOWS WHERE "Dark Star" > NOT "St. Stephen";

-- "What songs mention trains?"
SONGS WITH LYRICS("train", "railroad", "locomotive", "engineer");

-- "Cornell '77 setlist" (the famous show)
SETLIST FOR 5/8/77;
SETLIST FOR "Cornell 1977";  -- natural language alias

-- "Shows where they opened with a ballad"
SHOWS WHERE SET1 OPENED TEMPO < 100;

-- "Find the longest Estimated Prophet"
PERFORMANCES OF "Estimated Prophet" ORDER BY LENGTH DESC LIMIT 1;

-- "What songs did Pigpen sing lead on?"
SONGS WHERE LEAD_VOCAL = "Pigpen";
SONGS WITH VOCAL("Pigpen");

-- "Shows with unusual openers"
SHOWS WHERE SET1 OPENED RARITY > 0.8;  -- rarely-played openers

-- "Find the 'Help > Slip > Frank' combos"
SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";

-- "When did they bust out Attics of My Life?"
PERFORMANCES OF "Attics of My Life" AFTER GAP > 50;

-- "West Coast 1972"
SHOWS FROM 1972 IN REGION "West Coast";

-- "Acoustic sets"
SHOWS WITH ACOUSTIC_SET FROM 1970;
```

---

## Query Modifiers

```sql
-- Sorting
SHOWS FROM 1977 ORDER BY DATE;
SHOWS FROM 1977 ORDER BY RATING DESC;
PERFORMANCES OF "Dark Star" ORDER BY LENGTH DESC;

-- Limiting
SHOWS FROM 1972 LIMIT 10;

-- Counting
COUNT SHOWS FROM 1977;
COUNT PERFORMANCES OF "Eyes of the World";

-- Distinct
DISTINCT SONGS FROM 5/8/77;
DISTINCT VENUES FROM 1969;
```

---

## Output Formats

```sql
-- Default: structured output
SHOWS FROM 5/8/77;

-- Specific output formats
SHOWS FROM 5/8/77 AS SETLIST;    -- formatted setlist
SHOWS FROM 5/8/77 AS JSON;       -- JSON output
SHOWS FROM 5/8/77 AS CSV;        -- CSV output
SHOWS FROM 1977 AS CALENDAR;     -- calendar view
```

---

## Implementation Language Comparison

### Go ⭐ Recommended

**Pros:**
- Excellent for building parsers (hand-rolled recursive descent or tools like `participle`, `goyacc`)
- Single binary distribution - perfect for CLI tool
- Fast execution
- Good concurrency for potential web API
- Strong standard library

**Cons:**
- More verbose than some alternatives
- Error handling can be repetitive

**Libraries:** `participle` (parser combinator), `goyacc`, `antlr4-go`

### Rust

**Pros:**
- Excellent parsing libraries (`nom`, `pest`, `lalrpop`)
- Memory safe, very fast
- Great error messages possible with careful design
- Pattern matching is perfect for AST manipulation

**Cons:**
- Steeper learning curve
- Longer development time
- Overkill for a novelty project

**Libraries:** `pest` (PEG parser), `nom` (parser combinator), `lalrpop`

### Python

**Pros:**
- Fastest to prototype
- Great parsing libraries (`lark`, `pyparsing`, `ply`)
- Easy string manipulation for lyrics search
- Large ecosystem for web APIs (FastAPI, Flask)

**Cons:**
- Slower runtime (though probably fine for this use case)
- Distribution is messier (need Python installed or bundle)
- Type hints help but not enforced

**Libraries:** `lark` (EBNF grammar), `pyparsing`, `textX`

### TypeScript

**Pros:**
- Natural fit if you want a web-based REPL
- Good parser libraries (`nearley`, `chevrotain`, `ohm-js`)
- Easy to build interactive web UI
- Could run directly in browser

**Cons:**
- Less suited for CLI distribution
- Runtime dependency on Node.js

**Libraries:** `nearley` (Earley parser), `chevrotain` (parser building toolkit), `ohm-js`

### Recommendation

**Go** is the sweet spot for this project:

1. **CLI-first**: Single binary you can distribute easily
2. **Parser tooling**: `participle` makes building DSLs quite pleasant
3. **Performance**: Fast enough that queries feel instant
4. **Extensibility**: Easy to add a web API later
5. **Community**: Good for open-source CLI tools

If you wanted a web-based REPL as the primary interface, TypeScript would be worth considering. If you wanted to prototype the grammar quickly first, Python with `lark` is excellent.

---

## Data Sources

Potential data sources for the query engine:

- **setlist.fm** - Comprehensive setlist database
- **Dead.net** - Official archives
- **Archive.org** - Live Music Archive (show recordings, metadata)
- **GDAO** - Grateful Dead Archive Online
- **Custom scraping** - Build a local database from multiple sources

---

## Project Structure (Go)

```
gdql/
├── cmd/
│   └── gdql/
│       └── main.go          # CLI entry point
├── internal/
│   ├── lexer/
│   │   └── lexer.go         # Tokenization
│   ├── parser/
│   │   └── parser.go        # AST construction
│   ├── ast/
│   │   └── ast.go           # Abstract Syntax Tree types
│   ├── eval/
│   │   └── evaluator.go     # Query execution
│   └── data/
│       └── source.go        # Data source interfaces
├── pkg/
│   └── gdql/
│       └── gdql.go          # Public API
├── grammar/
│   └── gdql.ebnf            # Formal grammar spec
├── testdata/
│   └── shows.json           # Test data
├── go.mod
├── go.sum
├── README.md
└── DESIGN.md
```

---

## Future Ideas

- **Interactive REPL** with tab completion
- **Web interface** with syntax highlighting
- **Integration with Archive.org** for direct show playback
- **"Did You Mean?"** suggestions for misspelled song names
- **Natural language mode**: "show me dark stars from 72"
- **Playlist export** - Export query results to Spotify/Apple Music
- **Statistical queries** - "most common opener", "average show length by year"

---

## Grammar (EBNF Draft)

```ebnf
query       = show_query | song_query | perf_query | setlist_query ;

show_query  = "SHOWS" [from_clause] [where_clause] [modifiers] ;
song_query  = "SONGS" [with_clause] [written_clause] [modifiers] ;
perf_query  = "PERFORMANCES" "OF" song_ref [from_clause] [with_clause] [modifiers] ;

from_clause = "FROM" date_range ;
date_range  = date ["-" date] | era_alias ;
date        = year | month "/" day "/" year | season "-" year ;
year        = digit digit [digit digit] ;
era_alias   = "PRIMAL" | "EUROPE72" | "WALLOFOUND" | ... ;

where_clause = "WHERE" condition { ("AND" | "OR") condition } ;
condition    = song_condition | position_condition | guest_condition | ... ;

song_condition = song_ref [transition_op song_ref] ;
transition_op  = ">" | ">>" | "INTO" | "THEN" | "~>" | "TEASE" ;
song_ref       = string_literal | "NOT" song_ref ;

with_clause = "WITH" with_condition { "," with_condition } ;
with_condition = "LYRICS" "(" string_list ")" 
               | "LENGTH" comp_op duration
               | "GUEST" string_literal
               | ... ;

modifiers   = [order_clause] [limit_clause] [output_clause] ;
order_clause = "ORDER" "BY" field ["ASC" | "DESC"] ;
limit_clause = "LIMIT" number ;
output_clause = "AS" ("JSON" | "CSV" | "SETLIST" | "CALENDAR") ;
```

---

## Contributing

This is a passion project. PRs welcome for:
- Grammar improvements
- New query types
- Data source integrations
- Bug fixes
- Documentation

---

*"Once in a while you get shown the light, in the strangest of places if you look at it right."*
