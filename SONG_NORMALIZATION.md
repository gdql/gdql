# Song Normalization Strategy

> How we handle messy real-world song titles: ">" in names, jams, segments, and spelling variants.

---

## The Problems

### 1. Titles that contain ">"

**In the wild**: Setlist sources sometimes store a segue as a single item:

- `"Scarlet Begonias > Fire on the Mountain"` (one "song" in the API)
- `"Dark Star > Jam"`
- `"Playing in the Band > Uncle John's Band"` (re-entry)

**In GDQL**: We use `>` as the segue operator. If a song's **canonical name** contained `>`, parsing would break:

```sql
-- Ambiguous: is this one song or a segue?
SHOWS WHERE "Scarlet Begonias > Fire on the Mountain";
```

**Rule**: **No canonical song name in the database may contain `>`.**  
We normalize at ETL time (see below).

---

### 2. Jams and segments as "songs"

**In the wild**: Jams and segments appear under many names:

| Source / era      | Examples |
|------------------|----------|
| Generic          | Jam, Improv, Improvisation, Unknown |
| Drums            | Drums, Drumz, Drums Segment, Mickey & Bill |
| Space            | Space, Spacy, Space Jam |
| Combined         | Drums & Space, Drums/Space, Drumz > Space |
| Named jams       | Phil's Jam, The Other One Jam, Dark Star Jam |
| Setlist.fm style | (often just "Jam" or folded into previous song) |

Sometimes the jam is **inside** a song (e.g. "Playing in the Band" includes a long jam) and not a separate setlist item. Sometimes it's a **separate** item: "Drums", "Space". We need a consistent way to store and query both.

---

### 3. General title inconsistency

- Punctuation: `St. Stephen` vs `St Stephen`, `U.S. Blues` vs `US Blues`
- Apostrophes: `Franklin's Tower` vs `Franklins Tower`
- Ampersands: `Drums & Space` vs `Drums and Space`
- Spacing/case: `scarlet begonias` vs `Scarlet Begonias`
- Abbreviations: `FOTD`, `Playin'`, `Scarlet` (for Scarlet Begonias)

We already have **aliases** and **short_name** in the schema; this doc ties them to a concrete normalization pipeline.

---

## Design Decisions

### A. Canonical song names (what we store)

1. **No `>` in canonical names**  
   Stored `songs.name` and any `song_aliases.alias` used for matching must not contain `>`.

2. **One canonical form per “real” song**  
   We pick a single preferred spelling (e.g. `St. Stephen`, `Franklin's Tower`) and map variants to it via `song_aliases` and/or `short_name`.

3. **Jams/segments are special “songs”**  
   We introduce a small set of **canonical segment types** and map all jam-like titles to them (see Jam Taxonomy below). They live in `songs` (and performances) like other songs, but are tagged so we can query them differently (e.g. “Drums”, “Space”, “Jam”).

---

### B. Handling "A > B" in source data

When the **source** gives us one item that looks like a segue:

- **Pattern**: One setlist item whose name contains `" > "` (space–angle–space).

**ETL behavior**:

1. **Split** the string on `" > "` into a list of parts.
2. **Normalize** each part (see below): resolve to canonical song/segment name.
3. **Emit one performance per part**, in order, with:
   - Consecutive positions (e.g. 5 and 6).
   - `segue_type = '>'` on the performance that **precedes** the next (e.g. on position 5).

**Examples**:

| Source item                               | Result in DB |
|------------------------------------------|--------------|
| `Scarlet Begonias > Fire on the Mountain` | 2 performances: Scarlet Begonias (segue_type='>'), Fire on the Mountain |
| `Dark Star > Jam`                         | 2 performances: Dark Star (segue_type='>'), Jam (canonical segment) |
| `Help on the Way > Slipknot! > Franklin's Tower` | 3 performances, two with segue_type='>' |

So we **never** store `"Scarlet Begonias > Fire on the Mountain"` as a single song name; we only store the two (or more) canonical song names and the segue relationship in `performances`.

**Edge case**: If the source has a song that is **literally** titled something like "Song A > Song B" (e.g. a bootleg title), we still split it. If we discover a rare case where ">" is part of the true title (e.g. "Song > Chorus"), we could add a single exception in the alias table (e.g. alias `"Song > Chorus"` → song_id for that song) and still **never** use that string as the canonical `songs.name`.

---

### C. Jam / segment taxonomy

We define a **small set of canonical “segment” songs** and map all jam-like names to them.

**Canonical segment names** (stored in `songs` with a segment type):

| Canonical name | Segment type | Purpose |
|----------------|-------------|--------|
| **Jam**        | `segment`   | Generic / unidentified jam |
| **Drums**      | `segment`   | Drum solo segment |
| **Space**      | `segment`   | Space / ambient segment |
| **Drums & Space** | `segment` | Combined drums then space (when listed as one item) |

**Schema addition**:

```sql
-- Add to songs table
ALTER TABLE songs ADD COLUMN segment_type TEXT 
  CHECK (segment_type IN (NULL, 'segment'));

-- Optional: more specific for querying
-- segment_type: NULL (normal song), 'jam', 'drums', 'space', 'drums_space'
```

So we have a few rows in `songs` like:

- name = `Jam`, segment_type = `segment` (or `jam`)
- name = `Drums`, segment_type = `segment` (or `drums`)
- name = `Space`, segment_type = `segment` (or `space`)
- name = `Drums & Space`, segment_type = `segment` (or `drums_space`)

**Alias mapping** (in `song_aliases` or in ETL code):

- "Improv", "Improvisation", "Unknown", "Unidentified" → **Jam**
- "Drumz", "Drums Segment", "Mickey & Bill" → **Drums**
- "Spacy", "Space Jam" → **Space**
- "Drums/Space", "Drumz > Space" (split first, then map) → **Drums** + **Space** (two performances)
- "Phil's Jam", "The Other One Jam", "Dark Star Jam" → **Jam** (or we could add more canonical jams later)

So: **strange jam names** are normalized to one of these canonical segment “songs” via a fixed mapping table + optional alias table. We can still support queries like:

- `SHOWS WITH DRUMS > 15min` → performances where song is Drums and length_seconds > 900.
- `SHOWS WHERE "Dark Star" > "Jam"` → segue from Dark Star into the generic Jam segment.

---

### D. Normalization pipeline (ETL)

**Order of operations** when we see a raw song name from setlist.fm (or any source):

1. **Trim** whitespace.
2. **Split on `" > "`**:  
   - If multiple parts: treat as segue chain, normalize each part (step 3), then create multiple performances (see B above).  
   - If single part: continue with that string.
3. **Normalize the single token**:
   - **Punctuation**: Apply one canonical form (e.g. keep "St." and "U.S." as in official releases). We can have a small table or rules: "St Stephen" → "St. Stephen", "US Blues" → "U.S. Blues".
   - **Ampersand**: Normalize to "&" or "and" consistently (e.g. "Drums and Space" → "Drums & Space" if that’s our canonical form).
   - **Case**: Store canonical name in Title Case; match case-insensitively in the resolver.
4. **Resolve to canonical song**:
   - Look up in `songs.name` (case-insensitive).
   - Else look up in `song_aliases.alias`.
   - Else look up in **segment/jam mapping** (strange jam name → Jam/Drums/Space/Drums & Space).
   - Else **create new song** only if it’s not a known jam variant (and optionally flag for review).
5. **Store**: One row in `performances` per song in the (possibly split) list, with correct `segue_type` between them.

**Reserved characters**: We treat `>` as structural (segue). If we ever need to store a title that really contains `>` (e.g. for display from a bootleg), we store a **canonical name without `>`** and use an alias for the display form, or store the raw form only in `performances.notes`.

---

## Summary Table

| Issue | Approach |
|-------|----------|
| Title contains ">" | Split on " > " at ETL; create multiple performances; never store ">" in canonical name. |
| Jam named strangely | Map to canonical segments: Jam, Drums, Space, Drums & Space via alias/mapping table. |
| Jam inside a song | No separate performance; use `length_seconds` / `notes` / future jam_length_seconds for "Playin' JAM > 25min". |
| Punctuation / spelling | One canonical form per song; aliases for variants; case-insensitive match. |
| Query "Jam" | Resolve to canonical "Jam" song (segment_type = segment); query performances where song_id = Jam. |

---

## Implementation Notes

1. **Parser**: Quoted strings in GDQL can contain any character **except** unescaped `"`. So a user could type `"Scarlet Begonias > Fire on the Mountain"` as a **single** song name; the resolver would then try to find a song with that exact name (or alias). We will **not** have such a song; we’ll only have "Scarlet Begonias" and "Fire on the Mountain". So the resolver can return "Song not found" or suggest: "Did you mean a segue? Try: \"Scarlet Begonias\" > \"Fire on the Mountain\"."
2. **Resolver**: When resolving a string that contains `">"`, we can special-case: suggest splitting into a segue query rather than treating it as one song name.
3. **Display**: When we **output** setlists (e.g. SETLIST FOR 5/8/77), we render from our normalized data: "Scarlet Begonias" then "Fire on the Mountain" with a ">" between them from `segue_type`, not from a single combined title.

This keeps the language and the stored data consistent: **songs never contain ">"**, **jams are normalized to a small set of segment names**, and **all variants are handled in ETL and aliases**.
