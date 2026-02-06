# GDQL Language Reference

**Living doc** — we update this as the language and implementation change. For the full design and future ideas, see [DESIGN.md](../DESIGN.md) and [SPEC.md](../SPEC.md).

---

## Overview

GDQL is a SQL-like language for querying Grateful Dead shows and songs. Keywords are case-insensitive. Song names and strings are in double quotes. Statements can end with `;` (optional in some contexts). Comments start with `--`.

---

## Query types

### SHOWS

List shows, optionally filtered by date and conditions.

```sql
SHOWS;
SHOWS FROM 1977;
SHOWS FROM 1977-1980;
SHOWS FROM PRIMAL;                    -- era alias (1965–1969)
SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain";
SHOWS FROM 77-80 LIMIT 10;
SHOWS FROM 1977 ORDER BY DATE DESC;
SHOWS FROM 1977 AS JSON;
```

**Try in Sandbox:** [SHOWS](https://sandbox.gdql.dev?example=shows&run=1) · [FROM 1977](https://sandbox.gdql.dev?example=shows-77&run=1) · [1977–1980](https://sandbox.gdql.dev?example=shows-range&run=1) · [PRIMAL](https://sandbox.gdql.dev?example=shows-primal&run=1) · [Scarlet→Fire](https://sandbox.gdql.dev?example=scarlet-fire&run=1) · [LIMIT 10](https://sandbox.gdql.dev?example=shows-77-80-limit&run=1) · [ORDER BY](https://sandbox.gdql.dev?example=shows-77-order&run=1) · [AS JSON](https://sandbox.gdql.dev?example=shows-77-json&run=1)

**Clauses:** `FROM` (date range or era), `WHERE` (conditions), `ORDER BY` field `ASC`|`DESC`, `LIMIT` n, `AS` (JSON | CSV | SETLIST | TABLE).

### SONGS

List songs, optionally filtered by lyrics and written date.

```sql
SONGS;
SONGS WITH LYRICS("train", "road");
SONGS WRITTEN 1968-1970;
SONGS WITH LYRICS("rose") WRITTEN 1970 LIMIT 20;
```

**Try in Sandbox:** [SONGS](https://sandbox.gdql.dev?example=songs&run=1) · [LYRICS](https://sandbox.gdql.dev?example=songs-lyrics&run=1) · [WRITTEN](https://sandbox.gdql.dev?example=songs-written&run=1) · [LYRICS+WRITTEN](https://sandbox.gdql.dev?example=songs-lyrics-rose&run=1)

**Clauses:** `WITH` (LYRICS(...), LENGTH, GUEST), `WRITTEN` (date range), `ORDER BY`, `LIMIT`.

### PERFORMANCES

List performances of a specific song.

```sql
PERFORMANCES OF "Dark Star";
PERFORMANCES OF "Dark Star" FROM 1972-1974;
PERFORMANCES OF "Dark Star" WITH LENGTH > 20min;
PERFORMANCES OF "Eyes of the World" ORDER BY LENGTH DESC LIMIT 5;
```

**Clauses:** `FROM`, `WITH`, `ORDER BY`, `LIMIT`.

### SETLIST

Get the setlist for a single date.

```sql
SETLIST FOR 5/8/77;
SETLIST FOR "Cornell 1977";
SETLIST FOR 1977;   -- single year (interpretation TBD)
```

**Try in Sandbox:** [5/8/77](https://sandbox.gdql.dev?example=setlist-cornell&run=1) · [Cornell 1977](https://sandbox.gdql.dev?example=setlist-cornell-name&run=1)

---

## Dates and eras

- **Year:** `1977`, `77` (two-digit expanded to 19xx).
- **Range:** `1977-1980`.
- **Specific date:** `5/8/77` (M/D/YY); used with `SETLIST FOR`.
- **Era aliases:** `PRIMAL` (1965–1969), `EUROPE72`, `WALLOFOUND`/`WALLOFSOUND`, `HIATUS`, `BRENT_ERA`/`BRENT`, `VINCE_ERA`/`VINCE`.

---

## WHERE conditions (SHOWS)

- **Segue:** `"Song A" > "Song B"` or `"A" >> "B"` or `"A" INTO "B"` or `"A" THEN "B"` or `"A" ~> "B"` or `"A" TEASE "B"`.
- **Chain:** `"Help on the Way" > "Slipknot!" > "Franklin's Tower"`.
- **Set position:** `SET1 OPENED "Jack Straw"`, `SET2 CLOSED "Sugar Magnolia"`, `ENCORE = "U.S. Blues"`.
- **Played:** `PLAYED "Scarlet Begonias"`.
- **Guest:** `GUEST "Branford Marsalis"`.
- **Length:** `LENGTH("Dark Star") > 20min` (when we support it in WHERE).
- **Combine:** `condition1 AND condition2`, `condition1 OR condition2`.

**Try in Sandbox:** [Scarlet→Fire](https://sandbox.gdql.dev?example=scarlet-fire&run=1) · [Help→Slip→Franklin’s](https://sandbox.gdql.dev?example=help-slip-frank&run=1) · [PLAYED St. Stephen](https://sandbox.gdql.dev?example=played-st-stephen&run=1) · [St. Stephen > The Eleven](https://sandbox.gdql.dev?example=st-stephen-eleven&run=1)

---

## WITH conditions (SONGS / PERFORMANCES)

- **LYRICS:** `LYRICS("word1", "word2", ...)`.
- **LENGTH:** `LENGTH > 20min`, `LENGTH < 10min`, etc. (for PERFORMANCES).
- **GUEST:** `GUEST "Name"`.

---

## Operators

| Token | Meaning |
|-------|--------|
| `>` / `INTO` | Segued into (no break) |
| `>>` / `THEN` | Followed by (with break) |
| `~>` / `TEASE` | Teased into |
| `AND` / `OR` | Logical (between conditions) |
| `NOT` | Negate (e.g. NOT "Song") |

Comparisons: `>`, `<`, `=`, `>=`, `<=`, `!=` (e.g. for LENGTH).

---

## Output formats

After `AS`: `JSON`, `CSV`, `SETLIST`, `TABLE`, `CALENDAR`. Default is table-like.

---

## Implementation status

- **Implemented (parser):** SHOWS, SONGS, PERFORMANCES, SETLIST; FROM (year, range, era); WHERE (segue, position, PLAYED, GUEST, LENGTH); WITH (LYRICS, LENGTH, GUEST); WRITTEN; ORDER BY; LIMIT; AS format; comments.
- **Not yet:** Execution against a database, AT/IN venue, SANDWICH, FIRST/LAST, COUNT, and some DESIGN.md ideas.

---

## Docs as we go

- **This file:** current, human-readable language reference; update when we add or change syntax or behavior.
- **SPEC.md:** implementation spec and grammar for the implementation (and AI).
- **DESIGN.md:** full design and future ideas.
- **Go API:** run `go doc ./...` from the repo root for package and symbol docs; add `// Comment` above types and functions to drive that.
