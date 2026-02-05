# GDQL Implementation Specification

> **For AI Implementation** - This specification is designed to be read by an AI assistant to implement GDQL from scratch using test-first driven design in Go.

**Version**: 1.0  
**Date**: February 4, 2026  
**Status**: Ready for Implementation

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Development Approach](#2-development-approach)
3. [Project Structure](#3-project-structure)
4. [Core Interfaces](#4-core-interfaces)
5. [Grammar Specification](#5-grammar-specification)
6. [Data Schema](#6-data-schema)
7. [Implementation Phases](#7-implementation-phases)
8. [Critical Implementation Notes](#8-critical-implementation-notes)
9. [Testing Requirements](#9-testing-requirements)
10. [Known Edge Cases](#10-known-edge-cases)

---

## 1. Project Overview

### What is GDQL?

GDQL (Grateful Dead Query Language) is a SQL-inspired domain-specific language for querying Grateful Dead live performance data. It provides intuitive, music-centric syntax for exploring setlists, song transitions (segues), lyrics, venues, and performance history.

### Example Queries

```sql
-- Basic show query with date range
SHOWS FROM 1977-1980;

-- Find shows with famous Scarlet > Fire segue
SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";

-- Three-song chain (Help > Slip > Frank)
SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";

-- Songs with specific lyrics
SONGS WITH LYRICS("train", "road");

-- Performance length queries
PERFORMANCES OF "Dark Star" FROM 1972 WITH LENGTH > 20min;

-- Set position queries
SHOWS WHERE SET2 OPENED "Samson and Delilah";

-- Get setlist for specific date
SETLIST FOR 5/8/77;
```

### Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go 1.21+ | Single binary, good parser tooling, fast |
| Database | SQLite (via `modernc.org/sqlite`) | Pure Go, no CGO, embedded |
| Parser | `github.com/alecthomas/participle` | Declarative grammar, good errors |
| CLI | `github.com/spf13/cobra` | Standard Go CLI framework |
| Testing | `testing` + `testify/assert` | Standard Go testing |

---

## 2. Development Approach

### Test-First Driven Design (TDD)

**CRITICAL**: Every feature MUST be implemented test-first.

```
1. Write failing test → 2. Implement minimal code → 3. Pass test → 4. Refactor → 5. Repeat
```

### TDD Workflow Example

```go
// Step 1: Write failing test FIRST
func TestParseDateRange(t *testing.T) {
    input := "SHOWS FROM 1977-1980;"
    parser := NewParser(strings.NewReader(input))
    
    ast, err := parser.Parse()
    
    require.NoError(t, err)
    assert.Equal(t, 1977, ast.From.Start.Year)
    assert.Equal(t, 1980, ast.From.End.Year)
}

// Step 2: Run test → FAILS (parser doesn't exist)

// Step 3: Implement minimal code to pass

// Step 4: Refactor

// Step 5: Add edge case tests
```

### Test Categories

| Category | Purpose | Location |
|----------|---------|----------|
| **Unit Tests** | Isolated component tests | `internal/*/*_test.go` |
| **Integration Tests** | Component interactions | `internal/integration_test.go` |
| **Acceptance Tests** | End-to-end with test DB | `test/acceptance/` |
| **Golden Tests** | Snapshot regression | `test/golden/` |

### Coverage Goals

- Lexer/Parser: 95%+
- SQL Generator: 90%+
- Song Resolver: 85%+
- Overall: 80%+

---

## 3. Project Structure

```
gdql/
├── cmd/
│   └── gdql/
│       ├── main.go              # Entry point
│       └── cli.go               # Cobra command setup
├── internal/
│   ├── token/
│   │   └── token.go             # Token types
│   ├── lexer/
│   │   ├── lexer.go             # Tokenization
│   │   └── lexer_test.go
│   ├── parser/
│   │   ├── parser.go            # AST construction
│   │   ├── grammar.go           # Participle grammar tags
│   │   └── parser_test.go
│   ├── ast/
│   │   ├── ast.go               # AST node types
│   │   └── visitor.go           # Visitor pattern
│   ├── ir/
│   │   └── ir.go                # Intermediate representation
│   ├── planner/
│   │   ├── planner.go           # AST → IR transformation
│   │   ├── planner_test.go
│   │   ├── resolver/
│   │   │   ├── song.go          # Song name resolution
│   │   │   └── song_test.go
│   │   ├── expander/
│   │   │   ├── date.go          # Date/era expansion
│   │   │   └── date_test.go
│   │   └── sqlgen/
│   │       ├── generator.go     # IR → SQL generation
│   │       ├── segue.go         # Segue-specific SQL
│   │       └── generator_test.go
│   ├── executor/
│   │   ├── engine.go            # Query coordination
│   │   ├── cache.go             # Query result caching
│   │   └── engine_test.go
│   ├── formatter/
│   │   ├── formatter.go         # Formatter interface
│   │   ├── json.go
│   │   ├── csv.go
│   │   ├── setlist.go
│   │   └── table.go
│   ├── data/
│   │   ├── source.go            # DataSource interface
│   │   ├── sqlite/
│   │   │   ├── db.go            # SQLite implementation
│   │   │   ├── migrations.go    # Schema migrations
│   │   │   └── db_test.go
│   │   └── mock/
│   │       └── mock.go          # Mock for testing
│   └── errors/
│       └── errors.go            # Error types
├── pkg/
│   └── gdql/
│       └── gdql.go              # Public API
├── test/
│   ├── acceptance/
│   │   └── acceptance_test.go
│   ├── golden/
│   │   ├── parser_output.golden
│   │   └── sql_output.golden
│   └── fixtures/
│       ├── schema.sql           # Test database schema
│       ├── minimal_data.sql     # Test data
│       └── testdb.go            # Test DB helper
├── grammar/
│   └── gdql.ebnf                # Formal grammar
├── go.mod
├── go.sum
├── README.md
├── DESIGN.md
├── DATA_DESIGN.md
├── TESTING_STRATEGY.md
├── ARCHITECTURE_REVIEW.md
├── PERFORMANCE_ANALYSIS.md
└── SPEC.md                      # This file
```

---

## 4. Core Interfaces

### Interface Contracts

These interfaces MUST be defined and implemented. Dependencies flow left-to-right only.

```
Lexer → Parser → Planner → SQLGenerator → DataSource → Executor → Formatter
```

### 4.1 Token Types

```go
// internal/token/token.go

type TokenType int

const (
    EOF TokenType = iota
    ILLEGAL
    
    // Keywords
    SHOWS
    SONGS
    PERFORMANCES
    SETLIST
    FROM
    WHERE
    WITH
    WRITTEN
    ORDER
    BY
    LIMIT
    AS
    AND
    OR
    NOT
    INTO
    THEN
    TEASE
    SET1
    SET2
    ENCORE
    OPENED
    CLOSED
    LYRICS
    LENGTH
    FIRST
    LAST
    COUNT
    DISTINCT
    
    // Literals
    STRING    // "Scarlet Begonias"
    NUMBER    // 1977
    DURATION  // 20min
    
    // Operators
    GT        // >   (segue)
    GTGT      // >>  (followed by)
    TILDE_GT  // ~>  (tease)
    EQ        // =
    GTEQ      // >=
    LTEQ      // <=
    
    // Delimiters
    LPAREN
    RPAREN
    COMMA
    SEMICOLON
)

type Token struct {
    Type     TokenType
    Literal  string
    Position Position
}

type Position struct {
    Line   int
    Column int
    Offset int
}
```

### 4.2 Lexer Interface

```go
// internal/lexer/lexer.go

type Lexer interface {
    NextToken() Token
    PeekToken() Token
    Position() Position
}

type lexer struct {
    input   []rune
    pos     int
    readPos int
    ch      rune
    line    int
    col     int
}

func New(input string) Lexer
```

### 4.3 AST Types

```go
// internal/ast/ast.go

type Query interface {
    queryNode()
}

// ShowQuery represents: SHOWS [FROM date_range] [WHERE conditions] [modifiers]
type ShowQuery struct {
    From      *DateRange
    Where     *WhereClause
    OrderBy   *OrderClause
    Limit     *int
    OutputFmt OutputFormat
}

// SongQuery represents: SONGS [WITH clause] [WRITTEN clause] [modifiers]
type SongQuery struct {
    With    *WithClause
    Written *DateRange
    OrderBy *OrderClause
    Limit   *int
}

// PerformanceQuery represents: PERFORMANCES OF song [FROM range] [WITH clause]
type PerformanceQuery struct {
    Song    *SongRef
    From    *DateRange
    With    *WithClause
    OrderBy *OrderClause
    Limit   *int
}

// SetlistQuery represents: SETLIST FOR date
type SetlistQuery struct {
    Date *Date
}

// DateRange represents date ranges: 1977, 1977-1980, 5/8/77, spring-77
type DateRange struct {
    Start *Date
    End   *Date
    Era   *EraAlias // PRIMAL, EUROPE72, etc.
}

type Date struct {
    Year   int
    Month  int // 0 if not specified
    Day    int // 0 if not specified
    Season string // "spring", "summer", "fall", "winter"
}

// WhereClause represents WHERE conditions
type WhereClause struct {
    Conditions []Condition
    Operators  []LogicOp // AND, OR between conditions
}

type LogicOp int
const (
    OpAnd LogicOp = iota
    OpOr
)

// Condition is the interface for all condition types
type Condition interface {
    conditionNode()
}

// SegueCondition represents: "Song A" > "Song B" > "Song C"
type SegueCondition struct {
    Songs     []*SongRef
    Operators []SegueOp // Between consecutive songs
}

type SegueOp int
const (
    SegueOpSegue SegueOp = iota // >  (direct segue)
    SegueOpBreak                 // >> (followed by with break)
    SegueOpTease                 // ~> (tease)
)

// PositionCondition represents: SET1 OPENED "Song", ENCORE = "Song"
type PositionCondition struct {
    Set      SetPosition
    Operator PositionOp
    Song     *SongRef
}

type SetPosition int
const (
    SetAny SetPosition = iota
    Set1
    Set2
    Set3
    Encore
)

type PositionOp int
const (
    PosOpened PositionOp = iota
    PosClosed
    PosEquals
)

// SongRef represents a song reference
type SongRef struct {
    Name    string
    Negated bool // For NOT "Song"
}

// WithClause represents WITH conditions
type WithClause struct {
    Conditions []WithCondition
}

type WithCondition interface {
    withConditionNode()
}

// LyricsCondition represents: LYRICS("word1", "word2")
type LyricsCondition struct {
    Words    []string
    Operator LogicOp // AND or OR between words
}

// LengthCondition represents: LENGTH > 20min
type LengthCondition struct {
    Operator CompOp
    Duration time.Duration
}

type CompOp int
const (
    CompGT CompOp = iota
    CompLT
    CompEQ
    CompGTE
    CompLTE
)

// OrderClause represents ORDER BY field [ASC|DESC]
type OrderClause struct {
    Field string
    Desc  bool
}

type OutputFormat int
const (
    OutputDefault OutputFormat = iota
    OutputJSON
    OutputCSV
    OutputSetlist
    OutputCalendar
)

// Era aliases
type EraAlias int
const (
    EraPrimal    EraAlias = iota // 1965-1969
    EraEurope72                   // Spring 1972
    EraWallOfSound                // 1974
    EraHiatus                     // 1975
    EraBrent                      // 1979-1990
    EraVince                      // 1990-1995
)
```

### 4.4 Parser Interface

```go
// internal/parser/parser.go

type Parser interface {
    Parse() (ast.Query, error)
}

type parser struct {
    lexer  lexer.Lexer
    curTok token.Token
    peekTok token.Token
    errors []ParseError
}

func New(l lexer.Lexer) Parser

type ParseError struct {
    Position token.Position
    Message  string
    Expected string
}
```

### 4.5 Intermediate Representation (IR)

```go
// internal/ir/ir.go

// QueryIR is the resolved, expanded representation ready for SQL generation
type QueryIR struct {
    Type       QueryType
    DateRange  *ResolvedDateRange
    SegueChain *SegueChainIR
    Conditions []ConditionIR
    OrderBy    *OrderByIR
    Limit      *int
    OutputFmt  OutputFormat
}

type QueryType int
const (
    QueryTypeShows QueryType = iota
    QueryTypeSongs
    QueryTypePerformances
    QueryTypeSetlist
)

// ResolvedDateRange has actual dates (not eras)
type ResolvedDateRange struct {
    Start time.Time
    End   time.Time
}

// SegueChainIR has resolved song IDs (not names)
type SegueChainIR struct {
    SongIDs   []int      // Resolved song IDs
    Operators []SegueOp
}

// ConditionIR is a resolved condition
type ConditionIR interface {
    conditionIRNode()
}

type PositionConditionIR struct {
    Set      SetPosition
    Operator PositionOp
    SongID   int
}

type LyricsConditionIR struct {
    Words    []string
    Operator LogicOp
}

type LengthConditionIR struct {
    Operator  CompOp
    Seconds   int
}
```

### 4.6 Planner Interface

```go
// internal/planner/planner.go

type Planner interface {
    Plan(ast.Query) (*ir.QueryIR, error)
}

type planner struct {
    songResolver SongResolver
    dateExpander DateExpander
}

func New(sr SongResolver, de DateExpander) Planner

// internal/planner/resolver/song.go

type SongResolver interface {
    Resolve(name string) (int, error)            // Returns song ID
    ResolveFuzzy(name string) ([]SongMatch, error)
    Suggest(name string) []string                 // For "did you mean?"
}

type SongMatch struct {
    ID         int
    Name       string
    Score      float64 // 1.0 = exact match
}

// internal/planner/expander/date.go

type DateExpander interface {
    Expand(*ast.DateRange) (*ir.ResolvedDateRange, error)
    ExpandEra(ast.EraAlias) (*ir.ResolvedDateRange, error)
}
```

### 4.7 SQL Generator Interface

```go
// internal/planner/sqlgen/generator.go

type SQLGenerator interface {
    Generate(*ir.QueryIR) (*SQLQuery, error)
}

type SQLQuery struct {
    SQL  string
    Args []interface{}
}

type sqlGenerator struct{}

func New() SQLGenerator
```

### 4.8 DataSource Interface

```go
// internal/data/source.go

type DataSource interface {
    ExecuteQuery(ctx context.Context, q *SQLQuery) (*ResultSet, error)
    GetSong(ctx context.Context, name string) (*Song, error)
    GetSongByID(ctx context.Context, id int) (*Song, error)
    SearchSongs(ctx context.Context, pattern string) ([]*Song, error)
    Close() error
}

type ResultSet struct {
    Columns []string
    Rows    []Row
}

type Row []interface{}

// Domain types
type Show struct {
    ID       int
    Date     time.Time
    VenueID  int
    Venue    string
    City     string
    State    string
    Notes    string
    Rating   float64
}

type Song struct {
    ID           int
    Name         string
    ShortName    string
    Writers      string
    FirstPlayed  time.Time
    LastPlayed   time.Time
    TimesPlayed  int
}

type Performance struct {
    ID            int
    ShowID        int
    SongID        int
    SetNumber     int
    Position      int
    SegueType     string
    LengthSeconds int
}
```

### 4.9 Executor Interface

```go
// internal/executor/engine.go

type Executor interface {
    Execute(ctx context.Context, query string) (*Result, error)
    ExecuteAST(ctx context.Context, q ast.Query) (*Result, error)
}

type executor struct {
    parser    parser.Parser
    planner   planner.Planner
    sqlGen    sqlgen.SQLGenerator
    dataSource data.DataSource
    formatter formatter.Formatter
    cache     Cache
}

func New(ds data.DataSource) Executor

type Result struct {
    Type      ResultType
    Shows     []*Show
    Songs     []*Song
    Performances []*Performance
    Setlist   *Setlist
    SQL       string        // For debugging
    Duration  time.Duration // Query time
}

type ResultType int
const (
    ResultShows ResultType = iota
    ResultSongs
    ResultPerformances
    ResultSetlist
)
```

### 4.10 Formatter Interface

```go
// internal/formatter/formatter.go

type Formatter interface {
    Format(*Result, OutputFormat) (string, error)
}

type formatter struct {
    json     *JSONFormatter
    csv      *CSVFormatter
    setlist  *SetlistFormatter
    table    *TableFormatter
}

func New() Formatter
```

---

## 5. Grammar Specification

### EBNF Grammar (Complete)

```ebnf
(* GDQL Grammar - Complete Specification *)

query = show_query | song_query | perf_query | setlist_query 
      | first_query | last_query | count_query ;

(* Query Types *)
show_query = "SHOWS" [from_clause] [where_clause] [modifiers] ;
song_query = "SONGS" [with_clause] [written_clause] [modifiers] ;
perf_query = "PERFORMANCES" "OF" song_ref [from_clause] [with_clause] [modifiers] ;
setlist_query = "SETLIST" "FOR" (date | string_literal) ;
first_query = "FIRST" song_ref [segue_chain] ;
last_query = "LAST" song_ref [segue_chain] ;
count_query = "COUNT" (show_query | song_query | perf_query) ;

(* Clauses *)
from_clause = "FROM" date_range ;
where_clause = "WHERE" condition { logic_op condition } ;
with_clause = "WITH" with_condition { "," with_condition } ;
written_clause = "WRITTEN" date_range ;
modifiers = [order_clause] [limit_clause] [output_clause] ;

(* Date Handling *)
date_range = date ["-" date] | era_alias ;
date = year | full_date | season_date ;
year = digit digit [digit digit] ;           (* 77 or 1977 *)
full_date = digit digit "/" digit digit "/" year ; (* 5/8/77 *)
season_date = season "-" year ;              (* spring-77 *)
season = "spring" | "summer" | "fall" | "winter" ;
era_alias = "PRIMAL" | "EUROPE72" | "WALLOFOUND" | "HIATUS" 
          | "BRENT_ERA" | "VINCE_ERA" ;

(* Conditions *)
condition = segue_condition | position_condition | played_condition 
          | length_condition | guest_condition ;
          
segue_condition = song_ref { segue_op song_ref } ;
segue_op = ">" | ">>" | "~>" | "INTO" | "THEN" | "TEASE" ;

position_condition = set_position position_op song_ref ;
set_position = "SET1" | "SET2" | "SET3" | "ENCORE" ;
position_op = "OPENED" | "CLOSED" | "=" ;

played_condition = "PLAYED" song_ref ;

length_condition = "LENGTH" "(" song_ref ")" comp_op duration ;

guest_condition = "GUEST" string_literal ;

logic_op = "AND" | "OR" ;

(* With Conditions *)
with_condition = lyrics_condition | length_with | guest_with ;
lyrics_condition = "LYRICS" "(" string_list ")" ;
length_with = "LENGTH" comp_op duration ;
guest_with = "GUEST" string_literal ;

(* Modifiers *)
order_clause = "ORDER" "BY" field ["ASC" | "DESC"] ;
limit_clause = "LIMIT" number ;
output_clause = "AS" output_format ;
output_format = "JSON" | "CSV" | "SETLIST" | "CALENDAR" | "TABLE" ;

(* Comparisons *)
comp_op = ">" | "<" | "=" | ">=" | "<=" | "!=" ;
duration = number duration_unit ;
duration_unit = "min" | "minute" | "minutes" | "sec" | "second" | "seconds" ;

(* Song References *)
song_ref = string_literal | "NOT" song_ref ;

(* Literals *)
string_literal = '"' { character - '"' | escape } '"' ;
string_list = string_literal { "," string_literal } ;
number = digit { digit } ;
digit = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" ;
field = "DATE" | "LENGTH" | "RATING" | "NAME" | "TIMES_PLAYED" ;
escape = "\" ( '"' | "\" | "n" | "t" ) ;

(* Comments *)
comment = "--" { character - newline } newline ;
```

### Operator Precedence

| Precedence | Operator | Description |
|------------|----------|-------------|
| 1 (highest) | `NOT` | Negation |
| 2 | `>`, `>>`, `~>` | Segue operators |
| 3 | `AND` | Logical AND |
| 4 (lowest) | `OR` | Logical OR |

### Segue Operator Semantics

| Operator | Alias | Meaning | SQL Translation |
|----------|-------|---------|-----------------|
| `>` | `INTO` | Direct segue (no break) | `segue_type = '>'` |
| `>>` | `THEN` | Followed by (with break) | `segue_type = '>>'` |
| `~>` | `TEASE` | Teased into | `segue_type = '~>'` |

---

## 6. Data Schema

### SQLite Schema

```sql
-- Enable foreign keys
PRAGMA foreign_keys = ON;

-- Venues
CREATE TABLE venues (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT,
    state TEXT,
    country TEXT DEFAULT 'USA',
    latitude REAL,
    longitude REAL,
    UNIQUE(name, city)
);

-- Shows
CREATE TABLE shows (
    id INTEGER PRIMARY KEY,
    date DATE NOT NULL UNIQUE,
    venue_id INTEGER REFERENCES venues(id),
    tour TEXT,
    notes TEXT,
    soundboard BOOLEAN DEFAULT false,
    archive_id TEXT,
    rating REAL CHECK(rating >= 0 AND rating <= 5)
);

-- Songs (the catalog). Canonical names never contain ">".
CREATE TABLE songs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    short_name TEXT,
    writers TEXT,
    first_played DATE,
    last_played DATE,
    times_played INTEGER DEFAULT 0,
    is_cover BOOLEAN DEFAULT false,
    original_artist TEXT,
    tempo_bpm INTEGER,
    typical_length_seconds INTEGER,
    segment_type TEXT CHECK(segment_type IS NULL OR segment_type IN ('jam', 'drums', 'space', 'drums_space'))  -- For Jam, Drums, Space segments
);

-- Song aliases for fuzzy matching
CREATE TABLE song_aliases (
    id INTEGER PRIMARY KEY,
    song_id INTEGER NOT NULL REFERENCES songs(id),
    alias TEXT NOT NULL UNIQUE
);

-- Performances (song at a specific show)
CREATE TABLE performances (
    id INTEGER PRIMARY KEY,
    show_id INTEGER NOT NULL REFERENCES shows(id),
    song_id INTEGER NOT NULL REFERENCES songs(id),
    set_number INTEGER NOT NULL CHECK(set_number >= 0),
    position INTEGER NOT NULL CHECK(position > 0),
    segue_type TEXT CHECK(segue_type IN ('>', '>>', '~>', NULL)),
    length_seconds INTEGER,
    guest TEXT,
    notes TEXT,
    UNIQUE(show_id, set_number, position)
);

-- Denormalized setlists for pattern matching (CRITICAL for performance)
CREATE TABLE show_setlists (
    show_id INTEGER PRIMARY KEY REFERENCES shows(id),
    set1 TEXT,  -- "Jack Straw > Tennessee Jed >> Cassidy"
    set2 TEXT,
    set3 TEXT,
    encore TEXT,
    full_setlist TEXT  -- All sets concatenated
);

-- Lyrics (separate, optional)
CREATE TABLE lyrics (
    song_id INTEGER PRIMARY KEY REFERENCES songs(id),
    lyrics TEXT NOT NULL
);

-- FTS5 for lyrics search
CREATE VIRTUAL TABLE lyrics_fts USING fts5(
    lyrics,
    content=lyrics,
    content_rowid=song_id
);

-- Metadata
CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Required Indexes

```sql
-- Date range queries
CREATE INDEX idx_shows_date ON shows(date);

-- Song lookup
CREATE INDEX idx_songs_name ON songs(name);
CREATE INDEX idx_songs_short ON songs(short_name);
CREATE INDEX idx_aliases_alias ON song_aliases(alias);

-- Performance queries
CREATE INDEX idx_perf_show ON performances(show_id);
CREATE INDEX idx_perf_song ON performances(song_id);
CREATE INDEX idx_perf_position ON performances(show_id, set_number, position);
CREATE INDEX idx_perf_length ON performances(song_id, length_seconds);

-- Optimized segue query index
CREATE INDEX idx_perf_segue ON performances(show_id, set_number, position, song_id);

-- Venue queries
CREATE INDEX idx_venues_name ON venues(name);
CREATE INDEX idx_shows_venue ON shows(venue_id);
```

---

## 7. Implementation Phases

### Phase 1: Foundation (Week 1)

**Deliverables**: Lexer, Parser, AST

**Test-First Implementation Order**:

1. **Token types** (`internal/token/token.go`)
   - Define all token types first
   - No tests needed (just type definitions)

2. **Lexer** (`internal/lexer/`)
   - Test: Single keyword tokenization
   - Test: String literal with quotes
   - Test: Numbers and dates
   - Test: Operators (`>`, `>>`, `~>`)
   - Test: Full query tokenization
   - Test: Error handling (unclosed quotes)

3. **AST types** (`internal/ast/`)
   - Define all AST node types
   - No tests needed (just type definitions)

4. **Parser** (`internal/parser/`)
   - Test: Parse `SHOWS;`
   - Test: Parse `SHOWS FROM 1977;`
   - Test: Parse `SHOWS FROM 1977-1980;`
   - Test: Parse `SHOWS WHERE "Song";`
   - Test: Parse segue condition `"A" > "B"`
   - Test: Parse 3-song chain `"A" > "B" > "C"`
   - Test: Parse position condition `SET1 OPENED "Song"`
   - Test: Parse with ORDER BY and LIMIT
   - Test: Error handling (malformed queries)

### Phase 2: Planning Layer (Week 2)

**Deliverables**: Song Resolver, Date Expander, IR

**Test-First Implementation Order**:

1. **IR types** (`internal/ir/`)
   - Define all IR node types

2. **Date Expander** (`internal/planner/expander/`)
   - Test: Expand `77` → `1977-01-01 to 1977-12-31`
   - Test: Expand `1977-1980` → full range
   - Test: Expand `5/8/77` → single day
   - Test: Expand `spring-77` → Mar 20 - Jun 20
   - Test: Expand era aliases (PRIMAL, EUROPE72)
   - Test: Invalid date handling

3. **Song Resolver** (`internal/planner/resolver/`)
   - Test: Exact name match
   - Test: Short name resolution (`"Scarlet"` → ID)
   - Test: Case-insensitive matching
   - Test: Unknown song returns error with suggestions
   - Test: Fuzzy matching for typos

4. **Planner** (`internal/planner/`)
   - Test: Plan simple show query
   - Test: Plan query with date range
   - Test: Plan query with segue condition
   - Test: Song names resolved to IDs in IR

### Phase 3: SQL Generation (Week 3)

**Deliverables**: SQL Generator

**Test-First Implementation Order**:

1. **Basic SQL** (`internal/planner/sqlgen/`)
   - Test: Generate `SELECT * FROM shows` for `SHOWS;`
   - Test: Generate date filter `WHERE date BETWEEN ? AND ?`
   - Test: Parameterized queries (no string interpolation!)
   - Test: SQL is valid (use EXPLAIN)

2. **Segue SQL** (`internal/planner/sqlgen/segue.go`)
   - Test: 2-song segue query
   - Test: 3-song segue query
   - Test: Mixed operators (`>` and `>>`)
   - Test: Position condition SQL

3. **Modifier SQL**
   - Test: ORDER BY clause
   - Test: LIMIT clause
   - Test: COUNT queries

### Phase 4: Data Layer (Week 4)

**Deliverables**: SQLite DataSource, Test Fixtures

**Test-First Implementation Order**:

1. **DataSource interface** (`internal/data/`)
   - Define interface
   - Create mock implementation for testing

2. **SQLite implementation** (`internal/data/sqlite/`)
   - Test: Open/close database
   - Test: Execute simple query
   - Test: Execute parameterized query
   - Test: GetSong by name
   - Test: SearchSongs with pattern

3. **Test fixtures** (`test/fixtures/`)
   - Create schema.sql
   - Create minimal_data.sql (Cornell 5/8/77 + a few shows)
   - Create testdb.go helper

### Phase 5: Execution & Output (Week 5)

**Deliverables**: Executor, Formatters

**Test-First Implementation Order**:

1. **Executor** (`internal/executor/`)
   - Test: Execute string query end-to-end
   - Test: Execute AST directly
   - Test: Error propagation
   - Test: Query caching

2. **Formatters** (`internal/formatter/`)
   - Test: JSON output
   - Test: CSV output
   - Test: Table output (default)
   - Test: Setlist formatted output

### Phase 6: CLI (Week 6)

**Deliverables**: CLI tool

**Test-First Implementation Order**:

1. **CLI commands** (`cmd/gdql/`)
   - Test: `gdql -e "SHOWS FROM 1977"`
   - Test: `gdql` (REPL mode)
   - Test: `gdql --help`
   - Test: `gdql update`

2. **REPL** (if time permits)
   - Readline support
   - History
   - Tab completion

---

## 8. Critical Implementation Notes

### 8.1 Segue Query SQL Pattern

This is the most complex query. For `"A" > "B" > "C"`:

```sql
SELECT DISTINCT s.date, v.name
FROM performances p1
JOIN performances p2 ON p1.show_id = p2.show_id 
                    AND p1.set_number = p2.set_number
                    AND p1.position = p2.position - 1
JOIN performances p3 ON p2.show_id = p3.show_id 
                    AND p2.set_number = p3.set_number
                    AND p2.position = p3.position - 1
JOIN songs s1 ON p1.song_id = s1.id
JOIN songs s2 ON p2.song_id = s2.id
JOIN songs s3 ON p3.song_id = s3.id
JOIN shows s ON p1.show_id = s.id
JOIN venues v ON s.venue_id = v.id
WHERE s1.id = ?
  AND s2.id = ?
  AND s3.id = ?
  AND p1.segue_type = '>'
  AND p2.segue_type = '>'
```

### 8.2 SQL Injection Prevention

**CRITICAL**: Always use parameterized queries.

```go
// WRONG - SQL injection vulnerability
sql := fmt.Sprintf("SELECT * FROM songs WHERE name = '%s'", name)

// CORRECT - Parameterized
sql := "SELECT * FROM songs WHERE name = ?"
args := []interface{}{name}
```

### 8.3 Song Name Resolution Strategy

1. Exact match (case-insensitive)
2. Short name match (from `songs.short_name`)
3. Alias match (from `song_aliases` table)
4. Jam/segment mapping (e.g. "Phil's Jam" → canonical "Jam") — see SONG_NORMALIZATION.md
5. Fuzzy match (Levenshtein distance ≤ 2)
6. Return error with suggestions if no match

**If the user's string contains ">"**: Do not resolve as a single song. Suggest: "Did you mean a segue? Try: \"Song A\" > \"Song B\"."

### 8.4 Song and jam normalization (ETL)

- **Canonical names never contain ">"**. If the source has "A > B" as one item, split into two performances and set `segue_type = '>'` between them.
- **Jams/segments**: Map all jam-like names (Drums, Space, Improv, Phil's Jam, etc.) to canonical segment songs: Jam, Drums, Space, Drums & Space. Use `song_aliases` and/or a segment mapping table.
- Full rules: **[SONG_NORMALIZATION.md](SONG_NORMALIZATION.md)**.
- Broader issues (duplicate songs/shows, set order, venue/date disambiguation, placeholders, etc.): **[DATA_AUTHORITY_PROBLEMS.md](DATA_AUTHORITY_PROBLEMS.md)**.

### 8.5 Date Parsing Priorities

1. Full date: `5/8/77` → May 8, 1977
2. Year range: `1977-1980`
3. Single year: `1977` or `77`
4. Season: `spring-77` → Mar 20 - Jun 20, 1977
5. Era alias: `PRIMAL` → 1965-1969

### 8.6 Error Messages

Format errors with position and suggestions:

```
Error at position 15: Unknown song "Scarlet Begonia"

  SHOWS WHERE "Scarlet Begonia" > "Fire on the Mountain"
              ^^^^^^^^^^^^^^^^^

Did you mean:
  - "Scarlet Begonias"
```

### 8.7 Performance Targets

| Query Type | Target P95 | Max Acceptable |
|------------|------------|----------------|
| Simple queries | <10ms | <50ms |
| 2-song segue | <50ms | <200ms |
| 3-song chain | <100ms | <500ms |
| 4-song chain | <500ms | <2s |

---

## 9. Testing Requirements

### 9.1 Required Test Files

Every implementation file MUST have a corresponding `_test.go`:

```
internal/lexer/lexer.go      → internal/lexer/lexer_test.go
internal/parser/parser.go    → internal/parser/parser_test.go
internal/planner/planner.go  → internal/planner/planner_test.go
...
```

### 9.2 Test Data

Minimal test database with:

- **1 legendary show**: Cornell 5/8/77 (has Scarlet > Fire)
- **2-3 additional shows**: Different eras (1972, 1990)
- **10 core songs**: Scarlet, Fire, Dark Star, Help, Slipknot, Franklin's, etc.
- **Segue examples**: At least one of each type (`>`, `>>`, `~>`)

### 9.3 Golden Tests

Store expected outputs in `test/golden/`:

```
test/golden/
├── parser/
│   ├── shows_basic.golden           # AST for "SHOWS;"
│   ├── shows_date_range.golden      # AST for "SHOWS FROM 1977-1980;"
│   └── shows_segue.golden           # AST for segue query
├── sqlgen/
│   ├── shows_basic.golden           # SQL for basic show query
│   └── segue_2song.golden           # SQL for 2-song segue
└── results/
    ├── cornell.golden               # Result for "SETLIST FOR 5/8/77"
    └── scarlet_fire.golden          # Result for Scarlet > Fire query
```

### 9.4 Acceptance Test Examples

```go
func TestQueryScarletFire(t *testing.T) {
    db := fixtures.SetupTestDB(t)
    engine := executor.New(db)
    
    result, err := engine.Execute(ctx, 
        `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";`)
    
    require.NoError(t, err)
    require.Len(t, result.Shows, 1)
    assert.Equal(t, "1977-05-08", result.Shows[0].Date.Format("2006-01-02"))
}

func TestQueryHelpSlipFrank(t *testing.T) {
    db := fixtures.SetupTestDB(t)
    engine := executor.New(db)
    
    result, err := engine.Execute(ctx,
        `SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";`)
    
    require.NoError(t, err)
    // Verify results match expected shows
}
```

---

## 10. Known Edge Cases

### Must Handle

| Edge Case | Example | Expected Behavior |
|-----------|---------|-------------------|
| Song with apostrophe | `"Friend of the Devil"` | Parse correctly |
| Song with exclamation | `"Slipknot!"` | Parse correctly |
| Song name typo | `"Scarlet Begonia"` | Error with suggestion |
| Unknown song | `"Nonexistent Song"` | Error with "not found" |
| Invalid date | `13/32/77` | Parse error |
| Future date | `SHOWS FROM 2025` | Empty result (no error) |
| Empty result | Valid query, no matches | Return empty array |
| Very long chain | 5+ song segue | Warning about performance |
| Unclosed quote | `"Scarlet Begonias` | Parse error |
| SQL characters | `"Song'; DROP TABLE--"` | Parameterized (safe) |
| Case variation | `"scarlet begonias"` | Match case-insensitively |
| Short name | `"Scarlet"` | Resolve to full name |

### May Defer

- Cross-set segues (Set 1 closer → Set 2 opener)
- SANDWICH queries (complex pattern matching)
- Natural language dates (`"Cornell 1977"`)
- Comments (`-- comment`)

---

## Appendix: Quick Reference

### A. Go Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/parser/

# Run specific test
go test -run TestParseSegue ./internal/parser/

# Update golden files
UPDATE_GOLDEN=1 go test ./test/golden/

# Build binary
go build -o gdql ./cmd/gdql/

# Install
go install ./cmd/gdql/
```

### B. Key Dependencies

```go
// go.mod
module github.com/username/gdql

go 1.21

require (
    github.com/alecthomas/participle/v2 v2.1.1
    github.com/spf13/cobra v1.8.0
    github.com/stretchr/testify v1.8.4
    modernc.org/sqlite v1.28.0
)
```

### C. Example Queries Reference

```sql
-- Basic queries
SHOWS;
SHOWS FROM 1977;
SHOWS FROM 1977-1980;
SHOWS FROM 5/8/77;

-- Segue queries
SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";
SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";
SHOWS WHERE "Dark Star" >> "Morning Dew";
SHOWS WHERE "Dark Star" ~> "The Other One";

-- Position queries
SHOWS WHERE SET1 OPENED "Jack Straw";
SHOWS WHERE SET2 CLOSED "Sugar Magnolia";
SHOWS WHERE ENCORE = "U.S. Blues";

-- Song queries
SONGS WITH LYRICS("train", "road");
SONGS WRITTEN 1968-1970;

-- Performance queries
PERFORMANCES OF "Dark Star" FROM 1972-1974;
PERFORMANCES OF "Dark Star" WITH LENGTH > 20min;

-- Modifiers
SHOWS FROM 1977 ORDER BY DATE DESC LIMIT 10;
SHOWS FROM 1977 AS JSON;

-- Special
SETLIST FOR 5/8/77;
FIRST "Dark Star" > "St. Stephen";
COUNT SHOWS FROM 1977;
```

---

*"The test is the specification. Write it first."*
