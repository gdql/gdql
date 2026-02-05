# Data Authority Problems

> Issues that arise when setlists and songs come from many uploaders with no single authority. How we detect, represent, and handle them.

---

## The Root Cause

Setlist and song data is **crowd-sourced**: setlist.fm, Archive.org, Jerrybase, and fans all contribute. There is no official schema, no required vocabulary, and no single source of truth. The same show or song can appear under different names, orderings, and granularities. This document catalogs those problems and proposes handling strategies.

Related: **[SONG_NORMALIZATION.md](SONG_NORMALIZATION.md)** (titles with ">", jams, spelling).

---

## 1. Duplicate and Near-Duplicate Songs

**Problem**: The same song exists under multiple names, so we get several "songs" in the catalog.

| Variant A | Variant B | Notes |
|-----------|-----------|--------|
| China Cat Sunflower | China Cat | Abbreviation |
| Friend of the Devil | FOTD | Acronym |
| St. Stephen | Saint Stephen | Spelling |
| New Minglewood Blues | Minglewood Blues | "New" prefix |
| Playing in the Band | Playin' in the Band | Contraction |
| Bertha | Bertha (song) | Disambiguator in parens |
| Morning Dew | Morning Dew (Grateful Dead) | Version qualifier |

**Consequences**: Queries for one name miss performances stored under the other; stats (times played, first/last) are split across rows.

**Strategy**:
- **Canonical song**: Pick one official name per “real” song (e.g. "China Cat Sunflower", "Friend of the Devil").
- **Merge at ETL**: When ingesting, map all variants to the canonical song_id via `song_aliases` and a **song merge map** (maintained manually or from a curated list).
- **Deduplication job**: Periodically find candidates (e.g. same writers + similar name, or same first_played date). Flag for human review; merge only after approval.
- **Schema**: One row in `songs` per canonical song; all performances reference that row. No duplicate song rows for the same tune.

---

## 2. Set and Song Order Disagreement

**Problem**: Different sources list the same show with songs in a different order.

- Source A: Deal → Jack Straw → Tennessee Jed  
- Source B: Jack Straw → Deal → Tennessee Jed  
- Or: same songs, but one source has an extra "Unknown" or "Jam" in the middle.

**Causes**: Memory, different recordings (setlist from tape vs from someone’s notes), or edits over time.

**Strategy**:
- **Primary source**: Define a priority (e.g. setlist.fm > archive.org > first-seen). Store **one** canonical order per show.
- **Conflict tracking**: If we merge multiple sources, record when order differs (e.g. in a `setlist_conflicts` table: show_id, source_a_order, source_b_order). Do not auto-merge order; pick primary or manual review.
- **Display**: Show one setlist. Optional: "Order may vary by source" in UI/docs.
- **GDQL**: Queries use our canonical order (e.g. for "SET1 OPENED" or "Song A > Song B"). We do not support "according to source X" unless we explicitly model multiple setlists per show later.

---

## 3. Set Boundaries and Set Numbering

**Problem**: No standard for how many “sets” there are or what to call them.

- **Encore**: Some sources use Set 3 for encore; others use a separate "Encore" flag or "Set 3 (Encore)".
- **Double encore**: One or two songs in “Encore 1” and “Encore 2” — stored as Set 3 and Set 4, or both as Set 3?
- **Soundcheck**: Set 0, or a separate "Soundcheck" section, or omitted.
- **Acoustic set**: 1970 style — is it Set 1 and then “Set 2” electric, or “Acoustic” / “Electric” as set names?

**Strategy**:
- **Normalize set numbers**: We use integers: 0 = soundcheck (if present), 1 = first set, 2 = second set, 3 = encore (or third set). No half-sets.
- **Convention**: “Encore” = Set 3 (or highest set number for that show). If there are two encores, Set 3 and Set 4.
- **ETL**: Map source-specific labels ("Encore", "Encore 1", "Double encore") to these set numbers; document the mapping.
- **Schema**: `performances.set_number` is an integer. Optional: `sets(show_id, set_number, label)` with label = "Set 1", "Set 2", "Encore", "Soundcheck" for display only.
- **Soundcheck**: Include only if the source has it; set_number = 0. Do not invent soundchecks.

---

## 4. One Item vs Many (Splits and Merges)

**Problem**: The same musical segment can be one “song” or several.

- **Combined**: "Help on the Way / Slipknot! / Franklin's Tower" as one setlist item vs three.
- **Split**: We already handle "Scarlet > Fire" as one item → split into two (see SONG_NORMALIZATION.md).
- **Re-entry**: "Playing in the Band" → jam → "Playing in the Band" (reprise). One source: two items; another: one long item with a note.

**Strategy**:
- **Known segue chains**: Maintain a list of known multi-song sequences (Help/Slip/Frank, Scarlet/Fire, etc.). If the source gives one combined item, **split** into multiple performances with correct segue_type.
- **Re-entry**: Prefer two performances (Playin’ → … → Playin’) when the source shows two. If the source shows one, store one; optional `notes` like "includes reprise".
- **No merge**: Do not merge two distinct performances (e.g. two "Deal"s in one set) into one. Keep as two rows.

---

## 5. Soundcheck and Pre-Show

**Problem**: Inconsistent inclusion and naming.

- Sometimes "Soundcheck", sometimes "Pre-show", sometimes Set 0, sometimes omitted.
- Content may be partial: "Soundcheck: unknown" or "Jam".

**Strategy**:
- **set_number = 0** for soundcheck/pre-show when present.
- **Omit** if the source doesn’t list it (do not invent).
- **Songs in soundcheck**: Normalize song names same as main sets. If the source only says "Jam", store one performance: song_id = Jam, set_number = 0.

---

## 6. Guest Appearances and Attributes

**Problem**: Guests are sometimes a separate “song” or a tag on a song.

- "With Branford Marsalis" as its own line vs "Eyes of the World (with Branford Marsalis)".
- "Bobby and acoustic" vs "Acoustic set".

**Strategy**:
- **Do not** create a "song" for the guest. Store guest as metadata: `performances.guest` or a join table `performance_guests(performance_id, musician_id)`.
- **ETL**: If the source has "With X" as a separate item, either attach to the previous performance (guest = X) or drop the line; do not create a song named "With Branford Marsalis".
- **Acoustic**: Prefer a set-level or performance-level flag (`performance.acoustic` or `sets.acoustic`) over a fake song "Acoustic Set".

---

## 7. Covers vs Originals

**Problem**: Same title for a cover and an original; no authority on how to label.

- "Good Lovin'" — GD version vs original.
- "Johnny B. Goode", "Not Fade Away" — clearly covers but sometimes listed with/without "(cover)" or writer info.

**Strategy**:
- **One song row** per distinct tune we care about (e.g. "Good Lovin'" = the GD-performed song). `songs.is_cover` and `songs.original_artist` for display/query.
- **No duplicate rows** for "Good Lovin' (cover)" vs "Good Lovin'"; merge to one canonical song.
- **Attribution**: Use writers/original_artist when known; accept incomplete data. Do not infer "is_cover" from title alone.

---

## 8. Medleys and Teases (One vs Many Performances)

**Problem**: "St. Stephen > Not Fade Away > St. Stephen" can be represented as:
- Three performances: St. Stephen, NFA, St. Stephen (with segues), or
- One St. Stephen with a note "tease of NFA in middle", or
- Two performances: St. Stephen (with NFA tease), St. Stephen.

**Strategy**:
- **Prefer multiple performances** when the source lists multiple items. So: three rows (St. Stephen, NFA, St. Stephen) with segue_type between them.
- **Tease-only**: If the source explicitly says "Dark Star (tease of The Other One)" and does not list "The Other One" as a separate item, store one performance (Dark Star) and use `notes` or a future `tease_of` field. GDQL can support "TEASE" in queries later.
- **Rule of thumb**: If it’s a separate line in the setlist, it’s a separate performance; if it’s parenthetical/note, it’s metadata.

---

## 9. Incomplete and Placeholder Setlists

**Problem**: Partial data.

- "Set 2: ... (unknown)" or "Set 2: Scarlet > Fire > ??? > Sugar Magnolia".
- "Unknown", "Unidentified", "—" as placeholders.
- Only Set 1 known; Set 2 missing.

**Strategy**:
- **Store what we have**: Real songs as normal performances; do not invent missing songs.
- **Placeholders**: Map "Unknown", "Unidentified", "???", "—" to the canonical **Jam** song (or a dedicated "Unknown" segment) so we don’t create hundreds of fake songs. Option: `performance.notes = "setlist incomplete"` or a show-level flag `shows.setlist_complete = false`.
- **Queries**: Segue queries only consider known songs; placeholder "Jam" can match "SHOWS WHERE ... > Jam" but we don’t treat placeholders as specific tunes.
- **Display**: Indicate incomplete setlists (e.g. "Set 2 partial" or show "?" where data is missing).

---

## 10. Venue and Date Disambiguation

**Problem**: Same show, different venue names or dates.

- **Venue**: "Barton Hall", "Barton Hall, Cornell University", "Cornell University", "Ithaca, NY".
- **Date**: Late show (e.g. after midnight) → May 8 or May 9? Timezone differences.
- **Duplicate shows**: Same real show ingested twice with different date or venue string → two rows in `shows`.

**Strategy**:
- **Venue normalization**: One canonical venue per place (e.g. "Barton Hall, Cornell University" with city/state). Map variants to canonical venue_id via alias table or ETL rules. Dedupe venues before assigning to shows.
- **Date**: Adopt a convention (e.g. "show date = date the show started" in local time). If sources disagree, keep one (e.g. primary source) and log conflict; optional manual override.
- **Duplicate detection**: Before inserting a show, match on (date, normalized_venue_name) or (date, venue_id). If match found, merge or skip; do not create a second show row. Store source and any conflicting alternative date/venue in a conflicts table for review.

---

## 11. Character Encoding and Punctuation

**Problem**: Different encodings and glyphs from different uploaders.

- Apostrophe: `'` (U+0027) vs `’` (U+2019) vs `'` (backtick).
- Dash: `-` vs `–` (en-dash) vs `—` (em-dash).
- "Franklin's Tower" vs "Franklin's Tower" (curly apostrophe).
- Non-ASCII: rare but possible (e.g. in venue names).

**Strategy**:
- **Normalize at ETL**: Convert to a single canonical form (e.g. ASCII apostrophe and hyphen for song names). Store canonical in `songs.name`; use aliases for known alternate glyphs so search still finds them.
- **Unicode**: Accept UTF-8; normalize NFD/NFC if needed. For display, use the canonical form we stored.
- **Researcher**: Maintain a small mapping (e.g. "’" → "'") so "Franklin's Tower" and "Franklin's Tower" resolve to the same song.

---

## 12. Typos That Recur (Typos as “Canonical”)

**Problem**: If many setlists say "Scarlet Begonia" (missing s), does that become an alias for "Scarlet Begonias" or do we create a fake song?

**Strategy**:
- **Do not** create a song for a clear typo. Map common typos to canonical name via `song_aliases` (e.g. "Scarlet Begonia" → Scarlet Begonias).
- **Discovery**: If we see the same typo in many setlists, add it as an alias. Option: automated job that suggests aliases when a string is very similar to a canonical name (e.g. Levenshtein distance 1–2) and appears often.
- **Authority**: Canonical name is the one we choose (e.g. official release or most common correct spelling). Typos are always aliases.

---

## 13. Segue vs Break (Conflict Between Sources)

**Problem**: One source says Scarlet → Fire was a direct segue (no break); another says there was a break. We store one `segue_type` per performance.

**Strategy**:
- **Primary source** wins when we have a single source. When merging: prefer the "tighter" interpretation (e.g. `>` over `>>`) if we want to avoid missing a segue in queries, or document "segue_type from setlist.fm".
- **Conflict table**: If we merge two sources and they disagree on segue_type, store the primary choice but record the conflict (e.g. `performance_source_segue` or a generic `data_conflicts` table) for possible manual review.
- **GDQL**: User queries `>` or `>>`; we match on what we stored. We do not expose "source A said segue, source B said break" in the language.

---

## 14. Length and Timing

**Problem**: Same performance, different reported lengths (e.g. 12:30 vs 12:45 vs no length).

**Strategy**:
- **Single value**: Store one `length_seconds` per performance. Prefer primary source; if merging, prefer the more precise or the one that matches recording length when available.
- **Conflicts**: Log when sources disagree by more than some threshold (e.g. 30 seconds). Optional: store `length_source` (e.g. "setlist.fm" vs "archive.org").
- **Missing**: NULL is fine. Queries like "LENGTH > 20min" only match rows that have length_seconds set; we don’t infer length.

---

## 15. Duplicate Shows (Same Show, Multiple Rows)

**Problem**: Same real show appears twice because of different date format, venue name, or source.

**Strategy**:
- **Normalize before insert**: Date as ISO (YYYY-MM-DD); venue as venue_id from normalized venue table.
- **Unique key**: (date, venue_id) or (date, show_id from primary source). Before insert, check for existing show; if found, update or skip and attach new source metadata to existing row.
- **Merge**: If we discover duplicates later (e.g. same date + same city), run a deduplication job: suggest merges for human approval, then merge performances into one show and retire the duplicate show row.

---

## 16. Attribution and “Written By”

**Problem**: No authority on writer credits. "Traditional", "Unknown", "Hunter/Garcia" vs "Garcia/Hunter", or missing.

**Strategy**:
- **Store as given**: `songs.writers` is a string; accept incomplete or inconsistent data. Optionally normalize common variants ("Hunter/Garcia" vs "Garcia/Hunter") to one form for display.
- **Do not** infer writer from title. Use for display and for optional filters (e.g. "SONGS WRITTEN Hunter") with the understanding that results may be incomplete.

---

## Summary: Principles

| Principle | Application |
|-----------|-------------|
| **One canonical entity** | One row per song (with aliases), one row per show (with normalized date/venue). No duplicate songs or shows. |
| **Primary source + conflicts** | When merging, one source wins; log conflicts for review. |
| **Split, don’t merge** | Combined items (e.g. "A > B") split into multiple performances; do not merge two performances into one. |
| **Placeholders → Jam/Unknown** | "Unknown", "???", etc. map to a generic segment, not new fake songs. |
| **Guests and attributes are metadata** | Not separate songs. |
| **Normalize early** | Encoding, punctuation, venue, date at ETL so the rest of the system sees one form. |
| **Document and flag** | Incomplete setlists, conflicting segue_type or length, duplicate candidates — flag and optionally expose for manual review. |

---

## Implementation Hooks

- **ETL**: Song merge map, venue normalization, date convention, split rules for "A > B", placeholder → Jam mapping, encoding/punctuation normalization.
- **Schema**: `song_aliases`, `venue_aliases` or normalized venue table, `data_conflicts` (or per-type conflict tables), `shows.setlist_complete`, `performances.guest`, optional `performance_source` / `length_source`.
- **Jobs**: Duplicate-show detection, duplicate-song candidates, recurring typo → alias suggestions.
- **GDQL**: No changes to the language; we query over the normalized, authoritative view we store. Optional future: "SHOWS WHERE setlist_complete = false" or "CONFLICTS" for operators who care.

This keeps the query language simple while making explicit how we handle the mess of uncoordinated, authority-free data.
