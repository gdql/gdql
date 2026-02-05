# GDQL Testing Strategy - Test-First Driven Development

> A comprehensive testing strategy for GDQL that prioritizes TDD and ensures robust, maintainable code.

---

## 1. Test Categories

### 1.1 Unit Tests
**Purpose**: Test individual components in isolation with fast feedback loops.

#### Components to Test:

- **Lexer (`internal/lexer/`)**:
  - Token recognition (keywords, operators, literals)
  - Edge cases (quoted strings with punctuation, date formats)
  - Error handling (invalid tokens, unterminated strings)
  - Position tracking for error messages

- **Parser (`internal/parser/`)**:
  - AST construction correctness
  - Grammar rule coverage
  - Error recovery and reporting
  - Operator precedence

- **AST (`internal/ast/`)**:
  - Node type definitions
  - Visitor pattern implementation
  - Serialization/deserialization

- **Song Resolver (`internal/planner/`)**:
  - Exact name matching
  - Short name resolution ("Scarlet" → "Scarlet Begonias")
  - Fuzzy matching (typos, punctuation variations)
  - Abbreviation expansion ("FOTD" → "Friend of the Devil")

- **Date Expander (`internal/planner/`)**:
  - Date format parsing (77, 1977, 5/8/77, spring-77)
  - Era alias expansion (PRIMAL, EUROPE72, etc.)
  - Date range validation

- **SQL Generator (`internal/planner/sqlgen/`)**:
  - Query type translation (SHOWS → SELECT from shows)
  - Segue chain SQL generation (2-song, 3-song, N-song)
  - Set position queries (SET1 OPENED, ENCORE)
  - Aggregations (COUNT, AVG)
  - ORDER BY, LIMIT clauses

- **Result Formatter (`internal/formatter/`)**:
  - JSON output
  - CSV output
  - SETLIST formatted output
  - Default table output

**Test Location**: `internal/*/*_test.go`

**Tools**: Standard Go `testing` package, `testify/assert` for assertions

---

### 1.2 Integration Tests
**Purpose**: Test component interactions without full database.

#### Integration Points:

- **Parser → Planner → SQL Generator**:
  - End-to-end query parsing to SQL generation
  - Verify IR intermediate representation correctness
  - SQL correctness without execution

- **Song Resolver + SQL Generator**:
  - Resolved song IDs appear correctly in SQL
  - Fuzzy matching affects query construction

- **Date Expander + SQL Generator**:
  - Date ranges correctly translated to SQL BETWEEN clauses
  - Era aliases expand to correct date ranges

**Test Location**: `internal/integration_test.go` or `internal/*/integration_test.go`

**Approach**: Mock data sources, verify SQL output matches expectations

---

### 1.3 Acceptance Tests (End-to-End)
**Purpose**: Test full query execution against test database.

#### Test Database Setup:

- Minimal SQLite database with curated test data
- Covers edge cases: segues, set positions, date ranges
- Fast to set up and tear down (< 100ms)

**Test Queries**: Use examples from DESIGN.md

**Test Location**: `test/acceptance/acceptance_test.go`

**Tools**: Test database fixture, SQLite in-memory for speed

---

### 1.4 Golden Tests (Snapshot Tests)
**Purpose**: Capture expected outputs for regression testing.

#### What to Capture:

- **Parser Output**: AST JSON snapshots for complex queries
- **SQL Output**: Generated SQL for various query patterns
- **Query Results**: JSON output for acceptance test queries

**Benefits**:
- Catch unintended changes in output format
- Document expected behavior
- Easy to update when behavior intentionally changes

**Test Location**: `test/golden/` directory with `.golden` files

**Tools**: Custom test helper that compares actual vs golden files

---

## 2. Parser Test Structure

### 2.1 Minimal Test Data Approach

Parser tests should be **pure** - they don't need any database or external data.

#### Test Structure:

```go
// internal/parser/parser_test.go

func TestParseShowQuery(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantAST *ast.ShowQuery
        wantErr error
    }{
        {
            name:  "basic show query",
            input: "SHOWS;",
            wantAST: &ast.ShowQuery{
                Type: ast.QueryTypeShows,
            },
        },
        {
            name:  "show query with date range",
            input: "SHOWS FROM 1977-1980;",
            wantAST: &ast.ShowQuery{
                Type: ast.QueryTypeShows,
                From: &ast.FromClause{
                    DateRange: &ast.DateRange{
                        Start: mustParseDate("1977-01-01"),
                        End:   mustParseDate("1980-12-31"),
                    },
                },
            },
        },
        {
            name:  "show query with segue",
            input: `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";`,
            wantAST: &ast.ShowQuery{
                Type: ast.QueryTypeShows,
                Where: &ast.WhereClause{
                    Conditions: []ast.Condition{
                        &ast.SegueCondition{
                            Songs: []string{"Scarlet Begonias", "Fire on the Mountain"},
                            Operator: ast.SegueOpSegue, // '>'
                        },
                    },
                },
            },
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parser := NewParser(strings.NewReader(tt.input))
            gotAST, err := parser.Parse()
            
            if tt.wantErr != nil {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.wantErr.Error())
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.wantAST, gotAST)
            }
        })
    }
}
```

#### Test Data Requirements:

- **No external data needed** - parser only validates syntax
- **String literals** - Song names, dates, venue names as quoted strings
- **Edge cases**: 
  - Punctuation in song names ("Slipknot!", "Friend of the Devil")
  - Various date formats (77, 1977, 5/8/77)
  - Complex segue chains (3+ songs)
  - Nested conditions (AND/OR)

#### Minimal Fixture Data:

Even for parser tests, create a small constants file:

```go
// internal/parser/testdata.go

const (
    // Common song names for test inputs
    TestScarletBegonias = `"Scarlet Begonias"`
    TestFireOnMountain  = `"Fire on the Mountain"`
    TestDarkStar        = `"Dark Star"`
    TestHelpOnTheWay    = `"Help on the Way"`
    TestSlipknot        = `"Slipknot!"`
    TestFranklinsTower  = `"Franklin's Tower"`
)

// Common date strings
const (
    Date1977    = "1977"
    Date1977_80 = "1977-1980"
    DateCornell = "5/8/77"
)
```

---

## 3. Testing SQL Generation Without Full Database

### 3.1 SQL Verification Strategy

Test SQL generation separately from execution using **SQL validation and comparison**.

#### Approach 1: SQL String Comparison

```go
// internal/planner/sqlgen/sqlgen_test.go

func TestGenerateShowQueryWithSegue(t *testing.T) {
    ir := &ir.QueryIR{
        Type: ir.QueryTypeShows,
        SegueChain: &ir.SegueChainIR{
            Songs: []int{123, 456}, // song_ids resolved
            Operators: []ir.SegueOp{ir.SegueOpSegue},
        },
    }
    
    gen := NewSQLGenerator()
    sql, args := gen.Generate(ir)
    
    expectedSQL := `
        SELECT DISTINCT s.date, s.id, v.name
        FROM performances p1
        JOIN performances p2 ON p1.show_id = p2.show_id
                            AND p1.set_number = p2.set_number
                            AND p1.position = p2.position - 1
        JOIN songs s1 ON p1.song_id = s1.id
        JOIN songs s2 ON p2.song_id = s2.id
        JOIN shows s ON p1.show_id = s.id
        JOIN venues v ON s.venue_id = v.id
        WHERE s1.id = ? AND s2.id = ?
        AND p1.segue_type = '>'
    `
    
    assertSQLEqual(t, normalizeSQL(expectedSQL), normalizeSQL(sql))
    assert.Equal(t, []interface{}{123, 456}, args)
}

// Helper to normalize SQL for comparison (remove extra whitespace, lowercase keywords)
func normalizeSQL(sql string) string {
    // Implementation that normalizes whitespace and case
}
```

#### Approach 2: SQL Parsing and AST Comparison

Parse generated SQL and verify structure:

```go
import (
    "github.com/xwb1989/sqlparser" // SQL parser
)

func TestSQLStructure(t *testing.T) {
    gen := NewSQLGenerator()
    sql, _ := gen.Generate(ir)
    
    stmt, err := sqlparser.Parse(sql)
    require.NoError(t, err)
    
    selectStmt := stmt.(*sqlparser.Select)
    
    // Verify table joins
    assert.Len(t, selectStmt.From, 5) // p1, p2, s1, s2, s
    
    // Verify WHERE conditions
    assert.Contains(t, sql, "p1.show_id = p2.show_id")
    assert.Contains(t, sql, "p1.position = p2.position - 1")
}
```

#### Approach 3: In-Memory SQLite Validation

Use SQLite's EXPLAIN to verify SQL is valid:

```go
func TestSQLValidity(t *testing.T) {
    db, _ := sql.Open("sqlite3", ":memory:")
    defer db.Close()
    
    // Create minimal schema (just for validation)
    db.Exec(`CREATE TABLE performances (show_id INT, set_number INT, position INT, song_id INT, segue_type TEXT)`)
    db.Exec(`CREATE TABLE songs (id INT, name TEXT)`)
    db.Exec(`CREATE TABLE shows (id INT, date DATE)`)
    db.Exec(`CREATE TABLE venues (id INT, name TEXT)`)
    
    gen := NewSQLGenerator()
    sql, args := gen.Generate(ir)
    
    // Use EXPLAIN to validate SQL structure
    rows, err := db.Query(fmt.Sprintf("EXPLAIN QUERY PLAN %s", sql), args...)
    require.NoError(t, err, "Generated SQL should be valid")
    defer rows.Close()
    
    // Verify query plan is reasonable (uses indexes, etc.)
}
```

### 3.2 Parameterized Query Testing

Test that SQL uses parameters correctly to prevent SQL injection:

```go
func TestSQLParameterization(t *testing.T) {
    ir := &ir.QueryIR{
        SegueChain: &ir.SegueChainIR{
            Songs: []int{123, 456},
        },
    }
    
    gen := NewSQLGenerator()
    sql, args := gen.Generate(ir)
    
    // Verify no string interpolation in SQL
    assert.NotContains(t, sql, "123")
    assert.NotContains(t, sql, "456")
    assert.Contains(t, sql, "?")
    
    // Verify args match placeholders
    placeholderCount := strings.Count(sql, "?")
    assert.Equal(t, placeholderCount, len(args))
}
```

---

## 4. Fixture Data for Realistic Tests

### 4.1 Minimal Test Database Fixture

Create a test database with **minimal but realistic** data that covers all query patterns.

#### Test Database Schema:

Same as production schema, but with curated test data.

#### Test Data Requirements:

**1 Show (Cornell 5/8/77)** - The legendary show:
- 1 venue (Barton Hall, Cornell University)
- ~20 songs across Set 1 and Set 2
- Famous segue: "Scarlet Begonias" > "Fire on the Mountain"
- Encore: "U.S. Blues"

**2-3 Additional Shows** (1977, 1972, 1990):
- Different eras for date range testing
- Different segue patterns
- Set position variations

**~10 Songs**:
- Core songs: "Scarlet Begonias", "Fire on the Mountain", "Dark Star", "St. Stephen", "Playing in the Band"
- Cover songs for testing `is_cover` flag
- Songs with lyrics for FTS testing

**Test Database Size**: < 100KB, in-memory SQLite

#### Fixture Structure:

```
test/fixtures/
├── schema.sql              # Database schema
├── minimal_data.sql        # INSERT statements for test data
├── setup_testdb.go         # Helper to create test database
└── testdata/
    ├── shows.json          # Raw show data (optional, for ETL testing)
    └── lyrics.txt          # Sample lyrics (optional)
```

#### Fixture Helper:

```go
// test/fixtures/setup_testdb.go

package fixtures

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
    "os"
    "path/filepath"
)

// SetupTestDB creates an in-memory SQLite database with test data
func SetupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    // Create schema
    schema, err := os.ReadFile("test/fixtures/schema.sql")
    require.NoError(t, err)
    _, err = db.Exec(string(schema))
    require.NoError(t, err)
    
    // Insert test data
    data, err := os.ReadFile("test/fixtures/minimal_data.sql")
    require.NoError(t, err)
    _, err = db.Exec(string(data))
    require.NoError(t, err)
    
    return db
}
```

#### Minimal Test Data (`minimal_data.sql`):

```sql
-- Venues
INSERT INTO venues (id, name, city, state) VALUES
(1, 'Barton Hall', 'Ithaca', 'New York'),
(2, 'Fillmore West', 'San Francisco', 'California'),
(3, 'Red Rocks Amphitheatre', 'Morrison', 'Colorado');

-- Shows
INSERT INTO shows (id, date, venue_id, notes) VALUES
(1, '1977-05-08', 1, 'Cornell 1977 - Legendary show'),
(2, '1972-08-27', 2, 'Veneta, Oregon - Sunshine Daydream'),
(3, '1990-03-29', 3, 'Branford Marsalis guest appearance');

-- Songs
INSERT INTO songs (id, name, short_name, writers, tempo_bpm, typical_length_seconds) VALUES
(1, 'Scarlet Begonias', 'Scarlet', 'Hunter/Garcia', 120, 480),
(2, 'Fire on the Mountain', 'Fire', 'Hunter/Hart', 110, 420),
(3, 'Dark Star', NULL, 'Garcia/Hunter/Lesh', 80, 1200),
(4, 'St. Stephen', NULL, 'Hunter/Garcia', 130, 360),
(5, 'Playing in the Band', 'Playin', 'Hunter/Weir', 125, 900),
(6, 'Help on the Way', NULL, 'Hunter/Garcia', 100, 180),
(7, 'Slipknot!', NULL, 'Garcia/Lesh/Weir/Kreutzmann', 140, 240),
(8, "Franklin's Tower", 'Franklin', 'Hunter/Garcia/Kreutzmann', 115, 300),
(9, 'U.S. Blues', NULL, 'Hunter/Garcia', 135, 240),
(10, 'Morning Dew', NULL, 'Dobson/Rose', 90, 600);

-- Performances (Cornell 1977 - Set 1)
INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, is_opener, is_closer) VALUES
(1, 1, 5, 1, 1, '>>', true, false),   -- Playing in the Band (opener)
(2, 1, 10, 1, 2, '>', false, false),  -- > Morning Dew
(3, 1, 3, 1, 3, '>>', false, false),  -- >> Dark Star
-- ... more Set 1 songs
(10, 1, 1, 2, 5, '>', false, false),  -- Scarlet Begonias
(11, 1, 2, 2, 6, NULL, false, false), -- Fire on the Mountain (segue from Scarlet)
(12, 1, 9, 3, 1, NULL, false, true);   -- U.S. Blues (encore)

-- Critical segue: Scarlet > Fire
UPDATE performances SET segue_type = '>' WHERE id = 10;

-- Lyrics (for FTS testing)
INSERT INTO lyrics (song_id, lyrics) VALUES
(1, 'As I was walkin' round Grosvenor Square...'),
(2, 'Long distance runner, what you standin' there for?...'),
-- ... more lyrics
```

### 4.2 Test Data Patterns

Cover these scenarios:

- **Segue Types**: `>`, `>>`, `~>` (tease)
- **Set Positions**: Opener, closer, encore, middle
- **Date Ranges**: Single year, year range, era aliases
- **Song Variations**: Full name, short name, with punctuation
- **Multi-song Chains**: 2-song, 3-song segue chains
- **Edge Cases**: Song appears twice in show, cross-set segues

---

## 5. Test-First Development Workflow

### 5.1 TDD Cycle

For each feature, follow this cycle:

#### Step 1: Write Failing Test

```go
// internal/parser/parser_test.go

func TestParseEraAlias(t *testing.T) {
    input := "SHOWS FROM PRIMAL;"
    parser := NewParser(strings.NewReader(input))
    
    ast, err := parser.Parse()
    
    require.NoError(t, err)
    assert.Equal(t, ast.QueryTypeShows, ast.Type)
    assert.NotNil(t, ast.From)
    assert.Equal(t, ast.EraAliasPrimal, ast.From.DateRange.Era)
}
```

**Run**: `go test ./internal/parser/` → **FAILS** (parser doesn't support era aliases yet)

#### Step 2: Implement Minimal Code

```go
// internal/parser/parser.go

func (p *Parser) parseFromClause() (*ast.FromClause, error) {
    // ... existing code ...
    
    if p.peekToken().Type == token.EraAlias {
        era := p.nextToken()
        return &ast.FromClause{
            DateRange: &ast.DateRange{
                Era: parseEraAlias(era.Value),
            },
        }, nil
    }
    
    // ... existing date parsing ...
}
```

**Run**: `go test ./internal/parser/` → **PASSES**

#### Step 3: Refactor

Clean up code, extract common patterns, improve error messages.

#### Step 4: Add More Tests

Add edge cases:
- Multiple era aliases
- Era alias with WHERE clause
- Invalid era alias

### 5.2 Feature Implementation Order (TDD)

#### Phase 1: Foundation (Week 1)

1. **Lexer** (test-first)
   - Start with simplest tokens (keywords, operators)
   - Add string literals, numbers, dates
   - Test error handling

2. **Parser** (test-first)
   - Basic query types (SHOWS, SONGS)
   - Date parsing (single year)
   - Simple WHERE conditions

3. **AST** (test-first)
   - Node type definitions
   - Visitor pattern

#### Phase 2: Core Features (Week 2-3)

4. **Song Resolver** (test-first)
   - Exact matching
   - Short name resolution
   - Error handling for unknown songs

5. **Date Expander** (test-first)
   - Date format parsing
   - Era alias expansion
   - Date range validation

6. **SQL Generator - Basic Queries** (test-first)
   - Simple SHOWS queries
   - Date filtering
   - Song filtering

#### Phase 3: Advanced Features (Week 4-5)

7. **SQL Generator - Segues** (test-first)
   - 2-song segues
   - 3-song chains
   - Various segue operators

8. **SQL Generator - Set Positions** (test-first)
   - SET1 OPENED
   - ENCORE queries

9. **Result Formatting** (test-first)
   - JSON output
   - SETLIST format

#### Phase 4: Integration (Week 6)

10. **End-to-End** (acceptance tests)
    - Full query execution
    - Error handling
    - Performance testing

### 5.3 Test Organization

```
gdql/
├── internal/
│   ├── lexer/
│   │   ├── lexer.go
│   │   └── lexer_test.go       # Unit tests
│   ├── parser/
│   │   ├── parser.go
│   │   ├── parser_test.go      # Unit tests
│   │   └── testdata.go         # Test constants
│   ├── planner/
│   │   ├── planner.go
│   │   ├── planner_test.go     # Unit tests
│   │   ├── sqlgen/
│   │   │   ├── sqlgen.go
│   │   │   └── sqlgen_test.go  # SQL generation tests
│   │   └── resolver/
│   │       ├── resolver.go
│   │       └── resolver_test.go
│   └── formatter/
│       ├── formatter.go
│       └── formatter_test.go
├── test/
│   ├── acceptance/
│   │   └── acceptance_test.go  # End-to-end tests
│   ├── golden/
│   │   ├── parser_output.golden # AST snapshots
│   │   └── sql_output.golden   # SQL snapshots
│   └── fixtures/
│       ├── schema.sql
│       ├── minimal_data.sql
│       └── setup_testdb.go
└── go.mod
```

### 5.4 Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestParseShowQuery ./internal/parser/

# Run acceptance tests
go test ./test/acceptance/

# Update golden files
UPDATE_GOLDEN=1 go test ./test/golden/
```

---

## 6. Test Examples

### 6.1 Lexer Test Example

```go
func TestLexerTokens(t *testing.T) {
    tests := []struct {
        input string
        want  []token.Token
    }{
        {
            input: "SHOWS FROM 1977;",
            want: []token.Token{
                {Type: token.SHOWS, Value: "SHOWS"},
                {Type: token.FROM, Value: "FROM"},
                {Type: token.Year, Value: "1977"},
                {Type: token.Semicolon, Value: ";"},
            },
        },
        {
            input: `"Scarlet Begonias" > "Fire on the Mountain"`,
            want: []token.Token{
                {Type: token.String, Value: "Scarlet Begonias"},
                {Type: token.GreaterThan, Value: ">"},
                {Type: token.String, Value: "Fire on the Mountain"},
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            lexer := NewLexer(strings.NewReader(tt.input))
            var got []token.Token
            for {
                tok := lexer.NextToken()
                if tok.Type == token.EOF {
                    break
                }
                got = append(got, tok)
            }
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### 6.2 Parser Test Example

```go
func TestParseSegueChain(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  *ast.SegueCondition
    }{
        {
            name:  "two-song segue",
            input: `"Scarlet Begonias" > "Fire on the Mountain"`,
            want: &ast.SegueCondition{
                Songs: []string{"Scarlet Begonias", "Fire on the Mountain"},
                Operator: ast.SegueOpSegue,
            },
        },
        {
            name:  "three-song chain",
            input: `"Help on the Way" > "Slipknot!" > "Franklin's Tower"`,
            want: &ast.SegueCondition{
                Songs: []string{"Help on the Way", "Slipknot!", "Franklin's Tower"},
                Operators: []ast.SegueOp{ast.SegueOpSegue, ast.SegueOpSegue},
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parser := NewParser(strings.NewReader(tt.input))
            // Parse just the condition part
            cond, err := parser.parseCondition()
            require.NoError(t, err)
            assert.Equal(t, tt.want, cond)
        })
    }
}
```

### 6.3 SQL Generation Test Example

```go
func TestGenerateSegueQuery(t *testing.T) {
    ir := &ir.QueryIR{
        Type: ir.QueryTypeShows,
        SegueChain: &ir.SegueChainIR{
            Songs: []int{1, 2}, // Scarlet Begonias, Fire on the Mountain
            Operators: []ir.SegueOp{ir.SegueOpSegue},
        },
    }
    
    gen := NewSQLGenerator()
    sql, args := gen.Generate(ir)
    
    // Verify structure
    assert.Contains(t, sql, "JOIN performances p1")
    assert.Contains(t, sql, "JOIN performances p2")
    assert.Contains(t, sql, "p1.position = p2.position - 1")
    assert.Contains(t, sql, "p1.segue_type = '>'")
    
    // Verify parameters
    assert.Equal(t, []interface{}{1, 2}, args)
    
    // Verify SQL is valid (using EXPLAIN)
    db := fixtures.SetupTestDB(t)
    _, err := db.Query(fmt.Sprintf("EXPLAIN QUERY PLAN %s", sql), args...)
    assert.NoError(t, err)
}
```

### 6.4 Acceptance Test Example

```go
func TestShowQueryWithSegue(t *testing.T) {
    db := fixtures.SetupTestDB(t)
    engine := NewQueryEngine(db)
    
    query := `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";`
    
    result, err := engine.Execute(query)
    require.NoError(t, err)
    
    assert.Len(t, result.Shows, 1)
    assert.Equal(t, "1977-05-08", result.Shows[0].Date.Format("2006-01-02"))
    assert.Equal(t, "Barton Hall", result.Shows[0].VenueName)
}
```

---

## 7. Continuous Testing

### 7.1 Pre-Commit Hooks

```bash
#!/bin/sh
# .git/hooks/pre-commit

go test ./...
if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi
```

### 7.2 CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - run: go test -v -cover ./...
      - run: go test ./test/acceptance/
```

---

## 8. Metrics and Goals

### Test Coverage Goals:

- **Lexer/Parser**: 95%+ coverage (these are critical, well-defined components)
- **SQL Generator**: 90%+ coverage (complex logic, must be correct)
- **Song Resolver**: 85%+ coverage (fuzzy matching is hard to test exhaustively)
- **Overall**: 80%+ coverage

### Performance Goals:

- Unit tests: < 100ms per package
- Integration tests: < 500ms total
- Acceptance tests: < 1s total

### Quality Goals:

- All tests must pass before merging PR
- Golden files updated when behavior changes intentionally
- No test flakiness (all tests must be deterministic)

---

## Summary: TDD Checklist

For each feature:

- [ ] Write failing test first
- [ ] Implement minimal code to pass
- [ ] Refactor
- [ ] Add edge case tests
- [ ] Write acceptance test
- [ ] Update golden files if output changed
- [ ] Document in test comments

**Test Categories Summary**:
1. ✅ **Unit Tests** - Fast, isolated component tests
2. ✅ **Integration Tests** - Component interaction without database
3. ✅ **Acceptance Tests** - Full execution against test database
4. ✅ **Golden Tests** - Snapshot regression tests

**Key Principles**:
- **Parser tests need NO database** - pure syntax validation
- **SQL generation tests validate SQL structure** - no execution needed
- **Acceptance tests use minimal fixture database** - fast, realistic
- **Write tests FIRST** - TDD drives design

---

*"The test is the specification."*
