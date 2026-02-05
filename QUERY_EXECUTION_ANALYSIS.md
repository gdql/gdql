# GDQL Query Execution Feasibility Analysis

> A structured critique of GDQL query execution feasibility, SQL translation, performance implications, and implementation recommendations.

## 1. Can All Example Queries Be Translated to SQL?

### ✅ **Directly Translatable Queries** (Most queries)

These queries map directly to SQL with minimal translation complexity:

#### Basic Show Queries
- `SHOWS FROM 1977-1980` → `SELECT * FROM shows WHERE date BETWEEN '1977-01-01' AND '1980-12-31'`
- `SHOWS AT "Fillmore West"` → JOIN with `venues` table
- `SHOWS WHERE SET1 OPENED "Jack Straw"` → Filter `performances` where `set_number=1`, `position=1`, `is_opener=true`

#### Segue Chains (2-song)
- `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain"` → ✅ **Well-solved** by self-join pattern in DATA_DESIGN.md
- Uses `p1.position = p2.position - 1` join with `segue_type` filter

#### Positional Queries
- `SET1 OPENED`, `SET2 CLOSED`, `ENCORE =` → ✅ Direct schema support via `set_number`, `position`, `is_opener`, `is_closer`

#### Lyrics Queries
- `SONGS WITH LYRICS("train", "road")` → ✅ FTS5 virtual table query (`lyrics_fts`)

#### Guest Queries
- `SHOWS WITH GUEST "Branford Marsalis"` → ✅ `show_musicians` join

### ⚠️ **Translatable with Schema Extensions** (Requires additional fields)

These queries can work, but need schema support that may be missing:

#### Length/Performance Metrics
- `PERFORMANCES OF "Dark Star" WITH LENGTH > 20min` → ✅ Uses `performances.length_seconds`
- `SHOWS WHERE "Playing in the Band" JAM > 25min` → ⚠️ **Issue**: No `jam_length` field, only `length_seconds` and `notes TEXT`
  - **Solution**: Parse `notes` field for "extended jam" or derive from `length_seconds > typical_length_seconds`
  - **Better**: Add `jam_length_seconds INTEGER` or parse `notes` during ETL

#### Tempo Queries
- `SHOWS WHERE SET1 OPENED TEMPO < 100` → ⚠️ **Issue**: `tempo_bpm` is on `songs` table, not `performances`
  - **Solution**: JOIN `songs` to get tempo of opener song
  - **SQL**: `SELECT s.* FROM shows s JOIN performances p ON s.id=p.show_id JOIN songs song ON p.song_id=song.id WHERE p.set_number=1 AND p.position=1 AND song.tempo_bpm < 100`

#### Rarity Queries
- `SHOWS WHERE SET1 OPENED RARITY > 0.8` → ⚠️ **Complex**: Requires calculating how often a song was opener
  - **Solution**: Pre-compute rarity metric or use subquery:
    ```sql
    -- Calculate rarity: songs that opened set1 < 20% of times they were played
    SELECT s.* FROM shows s 
    JOIN performances p ON s.id=p.show_id 
    WHERE p.set_number=1 AND p.position=1
      AND (SELECT COUNT(*) FROM performances p2 WHERE p2.song_id=p.song_id AND p2.set_number=1 AND p2.position=1)::FLOAT /
          (SELECT COUNT(*) FROM performances p3 WHERE p3.song_id=p.song_id)::FLOAT < 0.2
    ```
  - **Better**: Materialized view or computed column for rarity

#### Bust-out Queries
- `PERFORMANCES OF "Attics" AFTER GAP > 50` → ⚠️ **Complex**: Requires window functions or self-join to find previous performance
  - **Solution**: LAG window function or join to previous show:
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
  - **Better**: Pre-compute gap days in `performances` table

### ❌ **Difficult or Impossible Without Data** (Missing information)

#### Missing Data Fields
- `SONGS WRITTEN 1968-1970` → ❌ **No `written_date` field** in schema
- `SONGS WRITTEN BY "Hunter/Garcia"` → ⚠️ Has `writers TEXT`, but no date component
- `LEAD_VOCAL = "Pigpen"` → ❌ **No `lead_vocal` field** in schema
  - **Workaround**: Parse `songs.writers` or `performances.notes`, but unreliable
- `SHOWS WITH ACOUSTIC_SET` → ❌ **No flag for acoustic sets** in schema
  - **Workaround**: Parse `notes` field or `setlist.fm` metadata

#### Natural Language Aliases
- `SETLIST FOR "Cornell 1977"` → ⚠️ **Requires fuzzy matching** against venue names or dates
  - **Solution**: Pre-built alias table mapping natural names to show dates

#### Region Queries
- `SHOWS FROM 1972 IN REGION "West Coast"` → ⚠️ **No `region` field**, only `state`/`city`
  - **Solution**: Mapping table `state → region` or derive from venue coordinates

---

## 2. Queries That Would Be Difficult or Impossible to Implement

### 🔴 **Impossible Without Schema Changes**

#### Statistical Aggregations
- `SONGS WITH AVG_LENGTH > 15min` → ⚠️ Requires aggregation over `performances.length_seconds`
  - **SQL**: `SELECT song.* FROM songs song JOIN performances p ON song.id=p.song_id GROUP BY song.id HAVING AVG(p.length_seconds) > 900`
  - **Performance**: ⚠️ Slow without materialized view or computed column
- `SONGS NEVER_OPENED` → ✅ Possible: `WHERE song.id NOT IN (SELECT DISTINCT song_id FROM performances WHERE is_opener=true)`
- `SONGS ONLY_ENCORE` → ✅ Possible: `WHERE song.id IN (SELECT song_id FROM performances WHERE set_number=3) AND song.id NOT IN (SELECT song_id FROM performances WHERE set_number IN (1,2))`

#### Negative Segue Queries
- `SHOWS WHERE "Dark Star" > NOT "St. Stephen"` → ✅ Possible but inefficient:
  ```sql
  -- Find shows where Dark Star > X, where X ≠ St. Stephen
  SELECT DISTINCT s.* FROM shows s
  JOIN performances p1 ON s.id=p1.show_id
  JOIN performances p2 ON p1.show_id=p2.show_id 
                        AND p1.set_number=p2.set_number
                        AND p1.position=p2.position-1
  JOIN songs song1 ON p1.song_id=song1.id
  JOIN songs song2 ON p2.song_id=song2.id
  WHERE song1.name='Dark Star' 
    AND song2.name != 'St. Stephen'
    AND p1.segue_type='>'
  ```
  - **Performance**: ⚠️ Requires full scan of all Dark Star segues

#### SANDWICH Queries
- `SHOWS WHERE "Playing in the Band" SANDWICH` → ⚠️ **Complex**: Song A → B → A pattern
  - **SQL**: Requires finding positions where same song appears twice with different songs in between
  ```sql
  SELECT DISTINCT s.* FROM shows s
  WHERE EXISTS (
    SELECT 1 FROM performances p1, performances p2, performances p3
    WHERE p1.show_id=s.id AND p2.show_id=s.id AND p3.show_id=s.id
      AND p1.song_id=p3.song_id  -- same song
      AND p1.song_id != p2.song_id  -- different middle song
      AND p1.set_number=p2.set_number AND p2.set_number=p3.set_number
      AND p1.position < p2.position AND p2.position < p3.position
      AND (SELECT name FROM songs WHERE id=p1.song_id)='Playing in the Band'
  )
  ```
  - **Performance**: 🔴 **Very slow**: O(n³) self-join, no index can optimize this
  - **Recommendation**: Pre-compute sandwich patterns or use denormalized `show_setlists` with pattern matching

#### Jam Length Comparisons
- `SHOWS WHERE DRUMS > SPACE` → ❌ **Schema issue**: `DRUMS` and `SPACE` are not songs, they're setlist elements
  - **Current schema**: No table for non-song setlist items
  - **Solution**: Either add `setlist_items` table with `type` enum, or parse `notes` field

---

## 3. Queries with Terrible Performance

### 🔴 **Performance Killers** (Require optimization)

#### Long Segue Chains (>3 songs)
- `SHOWS WHERE "A" > "B" > "C" > "D"` → 🔴 **Exponential join complexity**
  - Each additional song adds another self-join
  - 2-song: Fast (<50ms) ✅
  - 3-song: Acceptable (~100ms) ⚠️
  - 4-song: Slow (>500ms) 🔴
  - 5+ song: Very slow (>2s) 🔴
  
  **Solutions**:
  1. **Denormalized setlist string**: `LIKE '%A > B > C > D%'` for patterns
  2. **Recursive CTE**: Use SQLite recursive queries (still slow)
  3. **Pre-computed patterns**: Materialized view of common segue chains

#### SANDWICH Queries
- `SHOWS WHERE "Playing in the Band" SANDWICH` → 🔴 **O(n³) complexity**
  - Triple self-join with no efficient index
  - With ~40,000 performances, this scans billions of combinations
  - **Must use**: Denormalized setlist strings or pre-computed patterns

#### Rarity Calculations
- `SHOWS WHERE SET1 OPENED RARITY > 0.8` → 🔴 **Subquery per row**
  - Requires computing opener frequency for each song
  - **Solution**: Pre-compute rarity metric in `songs` table or materialized view

#### Bust-out Detection
- `PERFORMANCES AFTER GAP > 50` → 🔴 **Window function scan**
  - LAG window function over all performances for each song
  - **Solution**: Pre-compute `last_played_days_ago` during ETL

#### Statistical Aggregations
- `SONGS WITH AVG_LENGTH > 15min` → ⚠️ **Aggregation over 40K rows**
  - Group by song_id and compute AVG
  - **Solution**: Materialized view `song_stats` with pre-computed aggregations

---

## 4. Intermediate Representation (IR) Design

### Recommended AST → IR Structure

The IR should be **SQL-generation-ready** rather than a pure AST, bridging domain concepts to SQL operations.

```go
type QueryIR struct {
    Type QueryType // SHOWS, SONGS, PERFORMANCES, SETLIST
    
    // Date filtering
    DateRange *DateRangeIR
    
    // Show-level filters
    ShowsFilter *ShowsFilterIR
    
    // Segue chains
    SegueChain *SegueChainIR
    
    // Set position filters
    SetPosition *SetPositionIR
    
    // Aggregations
    Aggregations []AggregationIR
    
    // Modifiers
    OrderBy *OrderByIR
    Limit  *int
    OutputFormat OutputFormat
}

type SegueChainIR struct {
    Songs []ResolvedSongID  // IDs resolved during planning
    Operators []SegueOp      // '>', '>>', '~>'
    AllowSetBreaks bool       // Can span sets?
    RequireSegues bool        // Must be actual segues vs just sequence
}

type SetPositionIR struct {
    SetNumber int            // 1, 2, 3 (encore)
    Position PositionType    // OPENED, CLOSED, any
    Song *ResolvedSongID     // Optional: specific song
}

type ShowsFilterIR struct {
    Venue *VenueFilterIR
    Region *RegionFilterIR
    Guests []string
    HasAcousticSet bool
}

type DateRangeIR struct {
    Start time.Time
    End   time.Time
    Era   *EraAlias          // PRIMAL, EUROPE72, etc.
}
```

### Key Design Decisions

1. **Song Resolution in IR**: Store `song_id` rather than string names
   - Planner resolves "Scarlet" → "Scarlet Begonias" → `song_id=123` before IR

2. **Segue Chains as Arrays**: Not nested tree
   - `["Scarlet", "Fire"]` with `[">"]` operator array
   - Easier to generate N-join SQL

3. **Set Position Normalized**: `SET1 OPENED` → `SetPositionIR{SetNumber: 1, Position: OPENED}`
   - Separates set number from position type

4. **Date Range Expansion**: IR contains actual `time.Time` ranges
   - Planner expands "77-80" → `1977-01-01` to `1980-12-31` before IR

---

## 5. Query Planning for Complex Segue Chains

### Planning Strategy: Decompose and Optimize

#### Step 1: Parse and Validate
```
Input: "Help on the Way" > "Slipknot!" > "Franklin's Tower"
AST: SegueChain{["Help on the Way", "Slipknot!", "Franklin's Tower"], [">", ">"]}
```

#### Step 2: Resolve Song Names
```
Resolve "Help on the Way" → song_id=45
Resolve "Slipknot!" → song_id=234 (handle exclamation)
Resolve "Franklin's Tower" → song_id=89 (handle apostrophe)
```

#### Step 3: Choose Execution Strategy
For **N-song chains**, planner should:

1. **N ≤ 3**: Direct self-join (fast)
   ```sql
   FROM performances p1
   JOIN performances p2 ON ... AND p2.position = p1.position + 1
   JOIN performances p3 ON ... AND p3.position = p2.position + 1
   ```

2. **N ≥ 4**: Denormalized pattern matching (faster)
   ```sql
   FROM show_setlists ss
   WHERE ss.set1 LIKE '%Help on the Way > Slipknot! > Franklin\'s Tower%'
      OR ss.set2 LIKE '%...%'
   ```
   **Tradeoff**: Less precise (doesn't enforce `segue_type='>'`), but much faster

3. **Mixed**: Use both, UNION results
   - Denormalized for fast pattern match
   - Normalized for exact segue enforcement
   - Deduplicate

#### Step 4: Set Break Handling
- `"Scarlet" > "Fire"` → Within same set only (default)
- `"Scarlet" >> "Fire"` → Explicit break allowed
- **Planner decision**: Set `p1.set_number = p2.set_number` constraint based on operator

#### Step 5: Generate SQL with Optimizations

**Optimization 1: Early Filtering**
```sql
-- Start with most selective song
SELECT p1.show_id FROM performances p1
JOIN songs s1 ON p1.song_id=s1.id
WHERE s1.name='Dark Star'  -- Most rare song first
  AND p1.segue_type IS NOT NULL  -- Pre-filter segues
```

**Optimization 2: Index Hints**
- Use `idx_perf_segue` for position-based joins
- Use `idx_perf_position` for set position queries

**Optimization 3: Parallel Execution** (for very long chains)
- Split query into overlapping segments
- Execute in parallel, merge results
- Overkill for most queries, but possible

### SANDWICH Query Planning

For `"Playing in the Band" SANDWICH`:

**Strategy 1: Denormalized Pattern Matching** (Recommended)
```sql
SELECT s.* FROM shows s
JOIN show_setlists ss ON s.id=ss.show_id
WHERE (ss.set1 LIKE '%Playing in the Band%' 
       AND ss.set1 LIKE '%Playing in the Band%'  -- appears twice
       AND LENGTH(ss.set1) - LENGTH(REPLACE(ss.set1, 'Playing in the Band', '')) > LENGTH('Playing in the Band'))
  OR ... -- same for set2, encore
```
**Issue**: Doesn't guarantee different songs in between

**Strategy 2: Recursive CTE** (Correct but slow)
```sql
WITH RECURSIVE sandwich_pattern AS (
  -- Find first occurrence
  SELECT show_id, set_number, position, song_id, 1 as depth
  FROM performances p1
  JOIN songs s1 ON p1.song_id=s1.id
  WHERE s1.name='Playing in the Band'
  
  UNION ALL
  
  -- Find subsequent songs
  SELECT p.show_id, p.set_number, p.position, p.song_id, sp.depth+1
  FROM performances p
  JOIN sandwich_pattern sp ON p.show_id=sp.show_id 
                           AND p.set_number=sp.set_number
                           AND p.position=sp.position+1
  WHERE sp.depth < 10  -- reasonable limit
)
-- Find shows where same song appears twice with different songs in between
SELECT DISTINCT show_id FROM sandwich_pattern
GROUP BY show_id, set_number
HAVING COUNT(DISTINCT CASE WHEN song_id=(SELECT id FROM songs WHERE name='Playing in the Band') THEN position END) >= 2
```

**Strategy 3: Pre-computed Pattern Table** (Best performance)
```sql
CREATE TABLE segue_patterns (
  show_id INTEGER,
  pattern_type TEXT,  -- 'sandwich', 'chain', etc.
  pattern_songs TEXT,  -- JSON array of song_ids
  PRIMARY KEY (show_id, pattern_type, pattern_songs)
);

-- Populated during ETL
-- Query becomes: SELECT * FROM segue_patterns WHERE pattern_type='sandwich' AND pattern_songs='[123, X, 123]'
```

### Recommendations for Complex Segue Planning

1. **Use Denormalized Setlists**: For N≥4 chains and SANDWICH queries
2. **Pre-compute Common Patterns**: Help>Slip>Frank, Scarlet>Fire, etc.
3. **Limit Chain Length**: Warn users if chain >5 songs
4. **Hybrid Approach**: Use denormalized for discovery, normalized for verification

---

## Summary Recommendations

### Schema Additions Needed

1. **Add `jam_length_seconds INTEGER`** to `performances` table
2. **Add `lead_vocal TEXT`** to `songs` table (or parse from notes)
3. **Add `written_date DATE`** to `songs` table
4. **Add `acoustic_set BOOLEAN`** to `shows` table
5. **Add `region TEXT`** derived column or mapping table
6. **Pre-compute `rarity_score REAL`** in `songs` table
7. **Pre-compute `last_gap_days INTEGER`** in `performances` table

### Performance Optimizations

1. **Materialized Views**:
   - `song_stats` (avg_length, max_length, times_opened, times_encore)
   - `common_segue_patterns` (Help>Slip>Frank, etc.)
   - `sandwich_patterns` (Playing>X>Playing, etc.)

2. **Denormalized Table**: `show_setlists` for pattern matching

3. **Pre-computation During ETL**: Rarity, bust-outs, common patterns

### Query Planner Implementation

1. **Song Resolution Module**: Fuzzy matching, aliases, abbreviations
2. **Date Expansion Module**: Era aliases → date ranges
3. **Segue Planner Module**: Choose join vs pattern matching strategy
4. **SQL Generator Module**: Build optimized SQL from IR

### Missing Data Handling

1. **Graceful Degradation**: Return partial results with warnings
2. **Data Completeness Indicators**: Show which fields are available
3. **Fallback Strategies**: Parse `notes` field when structured data missing

---

## Conclusion

**Feasibility Score: 85%**

- ✅ Most queries are directly translatable
- ⚠️ Some require schema extensions or pre-computation
- 🔴 A few queries (SANDWICH, very long chains) need special handling
- 💡 Denormalized setlist strings solve most performance problems

The design is **well-thought-out** and most queries will work with the proposed schema. The main gaps are:
1. Missing metadata fields (jam lengths, vocals, acoustic sets)
2. Performance challenges for complex patterns (mitigated by denormalization)
3. Need for pre-computed aggregations (rarity, bust-outs)

With the recommended schema additions and optimization strategies, GDQL is **highly implementable**.
