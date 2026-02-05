# GDQL Performance Analysis

> Performance engineering review with specific recommendations for query optimization, indexing, caching, and benchmarking.

## Executive Summary

**Database Size**: ~50-100MB (small enough for in-memory operation)  
**Performance Target**: <100ms for 95% of queries, <500ms for complex queries  
**Critical Path**: Segue queries, especially long chains (3+ songs) and SANDWICH patterns

---

## 1. Slow Queries & Worst-Case Complexities

### 🔴 Critical Performance Issues

#### 1.1 Long Segue Chains (N ≥ 4 songs)

**Query Example:**
```sql
SHOWS WHERE "A" > "B" > "C" > "D" > "E";
```

**Complexity Analysis:**
- **2-song chain**: O(n) with index → **~10-50ms** ✅
- **3-song chain**: O(n²) with index → **~50-200ms** ⚠️
- **4-song chain**: O(n³) with index → **~200-1000ms** 🔴
- **5+ song chain**: O(n⁴+) → **>2 seconds** 🔴

**Root Cause**: Each additional song adds another self-join on `performances` table:
```sql
-- 4-song chain requires 3 self-joins
FROM performances p1
JOIN performances p2 ON p1.show_id=p2.show_id AND p2.position=p1.position+1
JOIN performances p3 ON p2.show_id=p3.show_id AND p3.position=p2.position+1
JOIN performances p4 ON p3.show_id=p4.show_id AND p4.position=p3.position+1
```

**Recommendation**: 
- **N ≤ 3**: Use normalized self-join (current approach)
- **N ≥ 4**: Switch to denormalized `show_setlists` pattern matching:
  ```sql
  SELECT * FROM show_setlists 
  WHERE set1 LIKE '%A > B > C > D%' OR set2 LIKE '%A > B > C > D%'
  ```
- **Hybrid**: Use denormalized for discovery, verify with normalized for precision

#### 1.2 SANDWICH Queries

**Query Example:**
```sql
SHOWS WHERE "Playing in the Band" SANDWICH;
```

**Complexity**: **O(n³)** - Triple self-join with no efficient index  
**Estimated Time**: **>5 seconds** on full dataset (40K performances)

**Current Approach** (from QUERY_EXECUTION_ANALYSIS.md):
```sql
SELECT DISTINCT s.* FROM shows s
WHERE EXISTS (
  SELECT 1 FROM performances p1, performances p2, performances p3
  WHERE p1.show_id=s.id AND p2.show_id=s.id AND p3.show_id=s.id
    AND p1.song_id=p3.song_id  -- same song
    AND p1.song_id != p2.song_id  -- different middle song
    AND p1.position < p2.position AND p2.position < p3.position
)
```

**Recommendation**: 
- **Pre-compute sandwich patterns** during ETL into `segue_patterns` table
- **Fallback**: Use denormalized setlist string with regex pattern matching
- **Cache results** for common songs (Playing in the Band, Dark Star, etc.)

#### 1.3 Rarity Calculations

**Query Example:**
```sql
SHOWS WHERE SET1 OPENED RARITY > 0.8;
```

**Complexity**: **O(n²)** - Subquery per row  
**Estimated Time**: **~500ms-2s** depending on selectivity

**Current Approach**:
```sql
-- Requires computing opener frequency for each song
SELECT s.* FROM shows s 
JOIN performances p ON s.id=p.show_id 
WHERE p.set_number=1 AND p.position=1
  AND (SELECT COUNT(*) FROM performances p2 
       WHERE p2.song_id=p.song_id AND p2.set_number=1 AND p2.position=1)::FLOAT /
      (SELECT COUNT(*) FROM performances p3 WHERE p3.song_id=p.song_id)::FLOAT < 0.2
```

**Recommendation**: 
- **Pre-compute** `opener_rarity_score REAL` in `songs` table during ETL
- **Materialized view**: `song_opener_stats` with pre-calculated frequencies

#### 1.4 Bust-out Detection

**Query Example:**
```sql
PERFORMANCES OF "Attics" AFTER GAP > 50;
```

**Complexity**: **O(n log n)** - Window function over all performances  
**Estimated Time**: **~200-800ms**

**Current Approach**:
```sql
SELECT p.*, s.date FROM performances p
JOIN shows s ON p.show_id=s.id
JOIN (
  SELECT song_id, date, 
         LAG(date) OVER (PARTITION BY song_id ORDER BY date) AS prev_date
  FROM performances p2 JOIN shows s2 ON p2.show_id=s2.id
) gaps ON p.song_id=gaps.song_id AND s.date=gaps.date
WHERE gaps.date - gaps.prev_date > 50
```

**Recommendation**:
- **Pre-compute** `days_since_last_played INTEGER` in `performances` table during ETL
- **Index** on `(song_id, days_since_last_played)` for fast filtering

#### 1.5 Statistical Aggregations

**Query Example:**
```sql
SONGS WITH AVG_LENGTH > 15min;
```

**Complexity**: **O(n)** - Full table scan with GROUP BY  
**Estimated Time**: **~100-300ms** (acceptable but can be optimized)

**Recommendation**:
- **Materialized view**: `song_stats` table with pre-computed:
  - `avg_length_seconds`
  - `max_length_seconds`
  - `min_length_seconds`
  - `times_played`
  - `times_opened`
  - `times_encore`
- **Refresh strategy**: Update during ETL, not on-demand

#### 1.6 Negative Segue Queries

**Query Example:**
```sql
SHOWS WHERE "Dark Star" > NOT "St. Stephen";
```

**Complexity**: **O(n²)** - Must check all segues from Dark Star  
**Estimated Time**: **~200-500ms**

**Recommendation**: Acceptable performance, but consider:
- **Early filtering**: Start with most selective song (Dark Star)
- **Index optimization**: Ensure `idx_perf_segue` covers this pattern

---

## 2. Should Database Be Loaded Into Memory?

### ✅ **YES - Strong Recommendation**

**Rationale:**

1. **Size**: 50-100MB fits comfortably in modern RAM
2. **Access Pattern**: Read-heavy workload (queries, no writes during execution)
3. **Performance Gain**: 10-100x faster than disk I/O
4. **SQLite Support**: Built-in memory-mapped I/O and WAL mode

### Implementation Options

#### Option 1: SQLite Memory-Mapped I/O (Recommended)

```go
// Enable memory-mapped I/O for read-only access
db.Exec("PRAGMA mmap_size=268435456")  // 256MB
db.Exec("PRAGMA journal_mode=WAL")     // Write-Ahead Logging
```

**Pros:**
- OS handles caching automatically
- No code changes needed
- Works with existing SQLite queries

**Cons:**
- Still touches disk initially (OS cache)
- Not truly "in-memory"

#### Option 2: Full In-Memory Database

```go
// Load entire database into memory
db, err := sql.Open("sqlite3", "file:shows.db?mode=memory&cache=shared")
// Then copy schema + data
```

**Pros:**
- Fastest possible access
- No disk I/O at all

**Cons:**
- Requires copying data on startup (~100-200ms)
- Uses more RAM
- More complex implementation

#### Option 3: Hybrid Approach (Best for CLI)

```go
// On first query, check if DB is "hot" (recently accessed)
// If cold, load into memory-mapped mode
// If hot, use direct file access
```

**Recommendation**: **Option 1 (Memory-Mapped I/O)**

- Simple to implement
- Good performance (OS cache is excellent)
- No startup penalty
- Works well for CLI tool usage patterns

### Memory Usage Estimate

| Component | Size | Notes |
|-----------|------|-------|
| Database file | 50-100MB | Compressed on disk |
| Uncompressed in RAM | 80-150MB | SQLite overhead |
| Indexes | 20-30MB | Additional index space |
| **Total** | **~100-180MB** | Acceptable for modern systems |

---

## 3. Are Proposed Indexes Sufficient?

### Current Indexes (from DATA_DESIGN.md)

```sql
CREATE INDEX idx_shows_date ON shows(date);
CREATE INDEX idx_songs_name ON songs(name);
CREATE INDEX idx_songs_short ON songs(short_name);
CREATE INDEX idx_perf_song ON performances(song_id);
CREATE INDEX idx_perf_show ON performances(show_id);
CREATE INDEX idx_perf_position ON performances(show_id, set_number, position);
CREATE INDEX idx_perf_segue ON performances(show_id, set_number, position, song_id, segue_type);
CREATE INDEX idx_venues_name ON venues(name);
CREATE INDEX idx_shows_venue ON shows(venue_id);
CREATE VIRTUAL TABLE lyrics_fts USING fts5(...);
```

### ⚠️ Missing Critical Indexes

#### 3.1 Composite Index for Segue Queries

**Problem**: `idx_perf_segue` may not be optimal for position-based joins

**Current**:
```sql
CREATE INDEX idx_perf_segue ON performances(show_id, set_number, position, song_id, segue_type);
```

**Issue**: Column order matters. For `p1.position = p2.position - 1` joins, we need:
```sql
-- Better: position first for range scans
CREATE INDEX idx_perf_segue_opt ON performances(show_id, set_number, position, segue_type, song_id);
```

**Recommendation**: Add optimized index:
```sql
-- For segue queries: WHERE show_id=X AND set_number=Y AND position BETWEEN Z-1 AND Z+1
CREATE INDEX idx_perf_segue_opt ON performances(show_id, set_number, position, segue_type);
-- Keep song_id separate for filtering
CREATE INDEX idx_perf_song_lookup ON performances(song_id, show_id, set_number, position);
```

#### 3.2 Covering Index for Common Queries

**Problem**: Queries often need `show_id`, `set_number`, `position`, and `song_id` together

**Recommendation**: Add covering index to avoid table lookups:
```sql
CREATE INDEX idx_perf_covering ON performances(show_id, set_number, position, song_id) 
INCLUDE (length_seconds, segue_type, is_opener, is_closer);
```

**Note**: SQLite doesn't support `INCLUDE`, so use:
```sql
CREATE INDEX idx_perf_covering ON performances(show_id, set_number, position, song_id, length_seconds, segue_type);
```

#### 3.3 Date Range + Venue Queries

**Problem**: `SHOWS FROM 1977 AT "Fillmore"` requires two lookups

**Recommendation**: Add composite index:
```sql
CREATE INDEX idx_shows_date_venue ON shows(date, venue_id);
```

#### 3.4 Song Performance Lookups

**Problem**: `PERFORMANCES OF "Dark Star" ORDER BY LENGTH DESC` needs sorting

**Recommendation**: Add index for common sort patterns:
```sql
CREATE INDEX idx_perf_song_length ON performances(song_id, length_seconds DESC);
CREATE INDEX idx_perf_song_date ON performances(song_id, show_id) 
-- (show_id links to date via shows table)
```

**Better**: Denormalize date into performances for faster sorting:
```sql
ALTER TABLE performances ADD COLUMN show_date DATE;
CREATE INDEX idx_perf_song_date ON performances(song_id, show_date DESC);
```

#### 3.5 Full-Text Search Optimization

**Current**: FTS5 virtual table (good)

**Enhancement**: Add trigram index for fuzzy song name matching:
```sql
-- For "Scarlet" → "Scarlet Begonias" matching
-- SQLite doesn't have native trigrams, but can use extension
-- Or implement in application layer with Levenshtein distance
```

**Recommendation**: Application-layer fuzzy matching with cached results

### Index Maintenance

**Recommendation**: 
- **ANALYZE** after data loads: `ANALYZE performances;`
- **REINDEX** periodically if performance degrades
- **Monitor index usage**: SQLite's `sqlite_stat1` table

### Final Index Recommendations

```sql
-- Core indexes (keep existing)
CREATE INDEX idx_shows_date ON shows(date);
CREATE INDEX idx_songs_name ON songs(name);
CREATE INDEX idx_perf_song ON performances(song_id);
CREATE INDEX idx_perf_show ON performances(show_id);

-- Optimized segue index
CREATE INDEX idx_perf_segue_opt ON performances(show_id, set_number, position, segue_type);
CREATE INDEX idx_perf_song_lookup ON performances(song_id, show_id, set_number, position);

-- Covering index for common patterns
CREATE INDEX idx_perf_covering ON performances(show_id, set_number, position, song_id, length_seconds);

-- Date + venue composite
CREATE INDEX idx_shows_date_venue ON shows(date, venue_id);

-- Song performance sorting
CREATE INDEX idx_perf_song_length ON performances(song_id, length_seconds DESC);

-- Denormalized date (if added)
CREATE INDEX idx_perf_song_date ON performances(song_id, show_date DESC);
```

**Index Size Estimate**: ~20-30MB additional storage (acceptable)

---

## 4. Caching Strategies

### 4.1 Query Result Caching

**Target**: Frequently executed queries with stable results

**Cacheable Queries:**
- `SHOWS FROM 1977` (date ranges)
- `SONGS WITH LYRICS("train")` (lyrics searches)
- `FIRST "Dark Star"` (first/last performances)
- `COUNT SHOWS FROM 1977` (aggregations)

**Non-Cacheable Queries:**
- Queries with `ORDER BY RATING` (if ratings update)
- Queries with `LIMIT` (unless cached with full result set)

**Implementation**:
```go
type QueryCache struct {
    cache map[string]CachedResult
    ttl   time.Duration
}

type CachedResult struct {
    Result    interface{}
    Timestamp time.Time
    QueryHash string
}

// Cache key: hash of normalized query AST
func (qc *QueryCache) Get(query *AST) (*CachedResult, bool) {
    key := qc.hashQuery(query)
    cached, ok := qc.cache[key]
    if !ok || time.Since(cached.Timestamp) > qc.ttl {
        return nil, false
    }
    return &cached, true
}
```

**Recommendation**: 
- **TTL**: 24 hours for show/song queries
- **Size limit**: 100MB cache (LRU eviction)
- **Cache key**: Normalized query AST hash (ignore whitespace, case)

### 4.2 Pre-computed Aggregations

**Target**: Expensive calculations that don't change frequently

**Candidates**:
1. **Song statistics** (avg_length, max_length, times_played)
2. **Common segue patterns** (Help>Slip>Frank, Scarlet>Fire)
3. **Sandwich patterns** (Playing>X>Playing)
4. **Rarity scores** (opener_rarity, encore_rarity)
5. **Bust-out gaps** (days_since_last_played)

**Implementation**: Materialized tables (see Section 5)

### 4.3 Song Name Resolution Cache

**Problem**: "Scarlet" → "Scarlet Begonias" lookup happens frequently

**Recommendation**: In-memory map:
```go
type SongResolver struct {
    exactMap   map[string]int64      // "Scarlet Begonias" → song_id
    aliasMap   map[string][]int64    // "Scarlet" → [song_id, ...]
    fuzzyCache map[string][]int64    // Levenshtein matches
}
```

**Cache Size**: ~1-2MB (450 songs × ~50 bytes)

### 4.4 Date Range Expansion Cache

**Problem**: "77-80" → "1977-01-01 to 1980-12-31" conversion

**Recommendation**: Simple map cache:
```go
var dateRangeCache = map[string]DateRange{
    "77-80": {Start: time.Date(1977, 1, 1), End: time.Date(1980, 12, 31)},
    "PRIMAL": {Start: time.Date(1965, 1, 1), End: time.Date(1969, 12, 31)},
    // ...
}
```

### 4.5 Denormalized Setlist Cache

**Problem**: Reconstructing setlist strings for pattern matching

**Recommendation**: Pre-compute during ETL:
```sql
CREATE TABLE show_setlists (
    show_id INTEGER PRIMARY KEY,
    set1 TEXT,  -- "Jack Straw > Tennessee Jed >> Cassidy > ..."
    set2 TEXT,
    encore TEXT,
    -- Full-text search index
    setlist_fts TEXT  -- Concatenated for FTS
);

CREATE VIRTUAL TABLE setlist_fts USING fts5(set1, set2, encore, content=show_setlists);
```

**Update Strategy**: Rebuild during ETL, not on-demand

### Cache Invalidation Strategy

**When to invalidate:**
- Database update (`gdql update` command)
- Manual cache clear (`gdql cache --clear`)

**Recommendation**: 
- **Version-based**: Store database version, invalidate on mismatch
- **Timestamp-based**: Cache includes DB modification time
- **Manual**: User can clear cache explicitly

---

## 5. Benchmarking Query Performance

### 5.1 Benchmark Suite Design

**Goal**: Measure query performance across representative workloads

#### Test Categories

**Category 1: Simple Queries (<10ms target)**
```sql
-- Date range
SHOWS FROM 1977 LIMIT 10;

-- Song lookup
SONGS WHERE name = "Dark Star";

-- Venue lookup
SHOWS AT "Fillmore West" LIMIT 5;
```

**Category 2: Segue Queries (10-100ms target)**
```sql
-- 2-song segue
SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";

-- 3-song chain
SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";
```

**Category 3: Complex Queries (100-500ms target)**
```sql
-- 4-song chain
SHOWS WHERE "A" > "B" > "C" > "D";

-- Rarity calculation
SHOWS WHERE SET1 OPENED RARITY > 0.8;

-- Bust-out detection
PERFORMANCES OF "Attics" AFTER GAP > 50;
```

**Category 4: Expensive Queries (500ms+ acceptable)**
```sql
-- SANDWICH query
SHOWS WHERE "Playing in the Band" SANDWICH;

-- Long chain (5+ songs)
SHOWS WHERE "A" > "B" > "C" > "D" > "E";
```

#### Benchmark Implementation

```go
package benchmark

import (
    "testing"
    "time"
)

type BenchmarkQuery struct {
    Name        string
    Query       string
    Category    string
    TargetTime  time.Duration
    MaxTime     time.Duration
}

var benchmarkQueries = []BenchmarkQuery{
    {
        Name:       "date_range",
        Query:      "SHOWS FROM 1977 LIMIT 10",
        Category:   "simple",
        TargetTime: 10 * time.Millisecond,
        MaxTime:    50 * time.Millisecond,
    },
    {
        Name:       "2_song_segue",
        Query:      `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain"`,
        Category:   "segue",
        TargetTime: 50 * time.Millisecond,
        MaxTime:    200 * time.Millisecond,
    },
    // ... more queries
}

func BenchmarkQueries(b *testing.B) {
    db := setupTestDB()
    defer db.Close()
    
    for _, bq := range benchmarkQueries {
        b.Run(bq.Name, func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                start := time.Now()
                executeQuery(db, bq.Query)
                duration := time.Since(start)
                
                if duration > bq.MaxTime {
                    b.Errorf("Query %s exceeded max time: %v > %v", 
                        bq.Name, duration, bq.MaxTime)
                }
            }
        })
    }
}
```

### 5.2 Performance Monitoring

**Metrics to Track:**

1. **Query Latency**
   - P50 (median)
   - P95 (95th percentile)
   - P99 (99th percentile)
   - Max

2. **Throughput**
   - Queries per second
   - Concurrent query handling

3. **Resource Usage**
   - Memory (heap, cache)
   - CPU (query planning vs execution)
   - I/O (disk reads, if not in-memory)

4. **Cache Effectiveness**
   - Cache hit rate
   - Cache eviction rate

**Implementation**: Add metrics collection:
```go
type QueryMetrics struct {
    QueryHash    string
    QueryType    string
    Duration     time.Duration
    RowsReturned int
    CacheHit     bool
    Timestamp    time.Time
}

func (e *Evaluator) ExecuteWithMetrics(query *AST) (*Result, *QueryMetrics) {
    start := time.Now()
    
    // Check cache
    cached, hit := e.cache.Get(query)
    if hit {
        return cached.Result, &QueryMetrics{
            QueryHash: hashQuery(query),
            Duration:  time.Since(start),
            CacheHit:  true,
        }
    }
    
    // Execute query
    result := e.execute(query)
    duration := time.Since(start)
    
    // Record metrics
    metrics := &QueryMetrics{
        QueryHash:    hashQuery(query),
        QueryType:    query.Type,
        Duration:     duration,
        RowsReturned: len(result.Rows),
        CacheHit:     false,
        Timestamp:    time.Now(),
    }
    
    e.metrics.Record(metrics)
    return result, metrics
}
```

### 5.3 Continuous Benchmarking

**Recommendation**: 
- **Pre-commit**: Run fast benchmarks (<1s total)
- **CI/CD**: Run full suite on every PR
- **Nightly**: Run extended benchmarks with profiling

**CI Integration**:
```yaml
# .github/workflows/benchmark.yml
name: Benchmark
on: [push, pull_request]
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run benchmarks
        run: go test -bench=. -benchmem ./internal/benchmark
      - name: Compare results
        run: |
          # Compare against baseline
          # Fail if regression >10%
```

### 5.4 Profiling Tools

**Recommendation**: Use Go profiling tools

```go
// CPU profiling
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Run benchmark with profiling
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof
```

**Key Areas to Profile:**
1. Query parsing (lexer/parser)
2. Query planning (AST → SQL translation)
3. SQL execution (SQLite queries)
4. Result formatting

### 5.5 Baseline Performance Targets

| Query Type | Target P95 | Max Acceptable |
|------------|------------|----------------|
| Simple (date range, song lookup) | <10ms | <50ms |
| 2-song segue | <50ms | <200ms |
| 3-song chain | <100ms | <500ms |
| 4-song chain | <500ms | <2s |
| SANDWICH | <2s | <5s |
| Aggregations (AVG_LENGTH) | <200ms | <1s |
| Lyrics search | <50ms | <200ms |

**Note**: These assume in-memory database and proper indexing.

---

## 6. Specific Recommendations Summary

### Immediate Actions (P0)

1. ✅ **Load database into memory** (memory-mapped I/O)
2. ✅ **Add optimized segue indexes** (`idx_perf_segue_opt`, `idx_perf_song_lookup`)
3. ✅ **Pre-compute song statistics** (materialized `song_stats` table)
4. ✅ **Implement query result caching** (24h TTL, 100MB limit)

### Short-term (P1)

5. ⚠️ **Add denormalized `show_setlists` table** for pattern matching
6. ⚠️ **Pre-compute common segue patterns** (Help>Slip>Frank, etc.)
7. ⚠️ **Add covering indexes** for common query patterns
8. ⚠️ **Implement song name resolution cache**

### Medium-term (P2)

9. 🔄 **Add query performance benchmarking suite**
10. 🔄 **Implement metrics collection** (latency, cache hits)
11. 🔄 **Add bust-out gap pre-computation** (`days_since_last_played`)
12. 🔄 **Optimize SANDWICH queries** (pre-compute or denormalized)

### Long-term (P3)

13. 📊 **Add continuous benchmarking** (CI/CD integration)
14. 📊 **Implement query plan analysis** (EXPLAIN QUERY PLAN)
15. 📊 **Add adaptive caching** (learn from query patterns)
16. 📊 **Consider query result pagination** for large result sets

---

## 7. Performance Testing Checklist

Before release, verify:

- [ ] All simple queries (<10ms P95)
- [ ] 2-song segue queries (<50ms P95)
- [ ] 3-song chains (<200ms P95)
- [ ] Database loads in <500ms
- [ ] Memory usage <200MB
- [ ] Cache hit rate >50% for repeated queries
- [ ] No memory leaks (run 1000 queries, check memory)
- [ ] Concurrent query handling (10+ simultaneous queries)
- [ ] Query cancellation works (Ctrl+C during long query)

---

## Conclusion

**Performance Feasibility: ✅ Excellent**

With the recommended optimizations:
- **95% of queries** will execute in <100ms
- **Complex queries** (4+ song chains, SANDWICH) will be <2s
- **Memory footprint** will be <200MB (acceptable)
- **User experience** will feel instant for typical queries

**Critical Success Factors:**
1. In-memory database (memory-mapped I/O)
2. Proper indexing (especially segue queries)
3. Pre-computed aggregations (song stats, patterns)
4. Smart caching (query results, name resolution)

The proposed architecture is sound. The main performance risks are:
- Long segue chains (mitigated by denormalized setlists)
- SANDWICH queries (mitigated by pre-computation)
- Missing indexes (addressed in recommendations)

With these optimizations, GDQL will deliver excellent query performance for a CLI tool.
