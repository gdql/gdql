package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode"

	"github.com/gdql/gdql/internal/data"

	_ "modernc.org/sqlite"
)

// normalizeName strips punctuation, extra whitespace, and lowercases for fuzzy matching.
// "Franklin's Tower" → "franklins tower", "Truckin'" → "truckin"
// Also canonicalizes a small set of abbreviations so spelled-out forms match
// the band's setlist convention (e.g. "Saint Stephen" → "St. Stephen").
func normalizeName(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
		} else if r == ' ' || r == '\t' {
			if !lastSpace && b.Len() > 0 {
				b.WriteRune(' ')
				lastSpace = true
			}
		}
		// Apostrophes, periods, commas, etc. are silently dropped
	}
	out := strings.TrimSpace(b.String())

	// Whole-word abbreviation canonicalization. Both the query and the stored
	// song name flow through here, so the substitution is symmetric — querying
	// "Saint Stephen" or "St. Stephen" both reach "st stephen".
	if out != "" {
		words := strings.Fields(out)
		for i, w := range words {
			if canon, ok := nameAbbreviations[w]; ok {
				words[i] = canon
			}
		}
		out = strings.Join(words, " ")
	}
	return out
}

// nameAbbreviations maps spelled-out forms to the abbreviated form the
// band's setlists use. Keep this list minimal — only add entries when there
// is a real mismatch causing failed lookups.
var nameAbbreviations = map[string]string{
	"saint": "st",
}

// DB implements data.DataSource using SQLite.
type DB struct {
	conn *sql.DB
}

// Open opens a SQLite database at the given path (file path or ":memory:").
// Ensures song_aliases exists on existing DBs (migration).
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, _ = conn.Exec("CREATE TABLE IF NOT EXISTS song_aliases (alias TEXT PRIMARY KEY, song_id INTEGER NOT NULL REFERENCES songs(id))")
	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// DB returns the underlying *sql.DB for use with packages that need it (e.g. canonical import).
func (db *DB) DB() *sql.DB {
	return db.conn
}

// nullAcceptingScanner implements sql.Scanner to accept any value including NULL.
// Used by ExecuteQuery so NULL columns don't cause "converting NULL to string is unsupported".
type nullAcceptingScanner struct {
	v *interface{}
}

func (n *nullAcceptingScanner) Scan(src interface{}) error {
	*n.v = src
	return nil
}

// ExecuteQuery runs the SQL with args and returns columns and rows.
func (db *DB) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (*data.ResultSet, error) {
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []data.Row
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		scanners := make([]nullAcceptingScanner, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			scanners[i] = nullAcceptingScanner{v: &vals[i]}
			ptrs[i] = &scanners[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		// Convert []byte to string for TEXT columns
		for i := range vals {
			if b, ok := vals[i].([]byte); ok {
				vals[i] = string(b)
			}
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &data.ResultSet{Columns: cols, Rows: out}, nil
}

// GetSong returns a song by name, trying in order: exact match, case-insensitive, alias,
// trim trailing dash, then fuzzy (punctuation-stripped). Always prefers the variant with
// the most performances to handle duplicates like "Franklins Tower" vs "Franklin's Tower".
func (db *DB) GetSong(ctx context.Context, name string) (*data.Song, error) {
	// Try exact/case-insensitive, alias, and trim-dash lookups
	queries := []string{
		"SELECT s.id, s.name, s.short_name, s.writers, s.first_played, s.last_played, s.times_played FROM songs s WHERE s.name = ? OR LOWER(s.name) = LOWER(?) ORDER BY (SELECT count(*) FROM performances p WHERE p.song_id = s.id) DESC LIMIT 1",
		"SELECT s.id, s.name, s.short_name, s.writers, s.first_played, s.last_played, s.times_played FROM songs s JOIN song_aliases a ON s.id = a.song_id WHERE a.alias = ? OR LOWER(a.alias) = LOWER(?) LIMIT 1",
		"SELECT s.id, s.name, s.short_name, s.writers, s.first_played, s.last_played, s.times_played FROM songs s WHERE LOWER(TRIM(s.name, '- ')) = LOWER(TRIM(?, '- ')) ORDER BY (SELECT count(*) FROM performances p WHERE p.song_id = s.id) DESC LIMIT 1",
	}
	for _, q := range queries {
		song, err := db.scanSong(ctx, q, name, name)
		if err != nil {
			return nil, err
		}
		if song != nil {
			// Check if fuzzy finds a better match (more performances).
			// This handles "Franklins Tower" (5 plays) vs "Franklin's Tower" (213 plays).
			fuzzy, ferr := db.getSongFuzzy(ctx, name)
			if ferr != nil || fuzzy == nil {
				return song, nil
			}
			if fuzzy.ID == song.ID {
				return song, nil
			}
			// Prefer the one with more performances
			var songPlays, fuzzyPlays int
			db.conn.QueryRowContext(ctx, "SELECT count(*) FROM performances WHERE song_id = ?", song.ID).Scan(&songPlays)
			db.conn.QueryRowContext(ctx, "SELECT count(*) FROM performances WHERE song_id = ?", fuzzy.ID).Scan(&fuzzyPlays)
			if fuzzyPlays > songPlays {
				return fuzzy, nil
			}
			return song, nil
		}
	}
	return db.getSongFuzzy(ctx, name)
}

func (db *DB) scanSong(ctx context.Context, query string, args ...interface{}) (*data.Song, error) {
	var id, times int
	var sname string
	var short, writers, first, last sql.NullString
	err := db.conn.QueryRowContext(ctx, query, args...).
		Scan(&id, &sname, &short, &writers, &first, &last, &times)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &data.Song{ID: id, Name: sname, TimesPlayed: times}
	if short.Valid {
		s.ShortName = short.String
	}
	if writers.Valid {
		s.Writers = writers.String
	}
	if first.Valid {
		s.FirstPlayed, _ = time.Parse("2006-01-02", first.String)
	}
	if last.Valid {
		s.LastPlayed, _ = time.Parse("2006-01-02", last.String)
	}
	return s, nil
}

// GetSongVariantIDs returns ALL song IDs whose normalized name matches.
// Used by PLAYED/NOT PLAYED so multiple spellings of the same song are
// treated as one for set-membership tests.
func (db *DB) GetSongVariantIDs(ctx context.Context, name string) ([]int, error) {
	target := normalizeName(name)
	if target == "" {
		return nil, nil
	}
	rows, err := db.conn.QueryContext(ctx, "SELECT id, name FROM songs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			continue
		}
		if normalizeName(n) == target {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// getSongFuzzy finds a song by normalizing both the query and all song names
// (stripping punctuation, lowercasing). When multiple variants match, prefers
// the one with the most performances (via a subquery count).
// Falls back to prefix/substring matching if no exact normalized match found.
func (db *DB) getSongFuzzy(ctx context.Context, name string) (*data.Song, error) {
	target := normalizeName(name)
	if target == "" {
		return nil, nil
	}

	type candidate struct {
		id, times int
		sname     string
		short     sql.NullString
		writers   sql.NullString
		first     sql.NullString
		last      sql.NullString
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT s.id, s.name, s.short_name, s.writers, s.first_played, s.last_played, s.times_played,
		       (SELECT count(*) FROM performances p WHERE p.song_id = s.id) AS play_count
		FROM songs s ORDER BY play_count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Track the best prefix match as fallback.
	// Results are ordered by play_count DESC so the first match has the most performances.
	var prefixMatch *candidate

	// Track best token-overlap match as last-resort fallback.
	// Score = sum of lengths of query tokens that appear as words in the song name,
	// divided by sum of all query token lengths. Length-weighting discounts stopwords.
	queryTokens := strings.Fields(target)
	queryLen := 0
	for _, t := range queryTokens {
		queryLen += len(t)
	}
	var tokenMatch *candidate
	var tokenScore float64

	for rows.Next() {
		var c candidate
		var playCount int
		if err := rows.Scan(&c.id, &c.sname, &c.short, &c.writers, &c.first, &c.last, &c.times, &playCount); err != nil {
			continue
		}
		norm := normalizeName(c.sname)

		// Exact normalized match — return immediately.
		if norm == target {
			return db.buildSong(c.id, c.sname, c.short, c.writers, c.first, c.last, c.times), nil
		}

		// Track the best prefix match. Require target to be at least 5 chars
		// and cover at least 40% of the song name to avoid overly broad matching
		// (e.g. "Fire" matching "Fire on the Mountain").
		if prefixMatch == nil && len(target) >= 5 && strings.HasPrefix(norm, target) {
			ratio := float64(len(target)) / float64(len(norm))
			if ratio >= 0.4 {
				prefixMatch = &c
			}
		}

		// Token-overlap match. Requires the query to have at least 2 tokens so
		// single-word queries don't accidentally match anything containing them.
		if len(queryTokens) >= 2 && queryLen > 0 {
			songWords := make(map[string]bool)
			for _, w := range strings.Fields(norm) {
				songWords[w] = true
			}
			matched := 0
			for _, t := range queryTokens {
				if songWords[t] {
					matched += len(t)
				}
			}
			score := float64(matched) / float64(queryLen)
			// Threshold: 70% of query length must match. Iterate in play-count
			// order so on ties the more-played variant wins.
			if score >= 0.70 && score > tokenScore {
				tokenScore = score
				cc := c
				tokenMatch = &cc
			}
		}
	}

	// Auto-resolve prefix match (first found = most performances).
	if prefixMatch != nil {
		return db.buildSong(prefixMatch.id, prefixMatch.sname, prefixMatch.short, prefixMatch.writers, prefixMatch.first, prefixMatch.last, prefixMatch.times), nil
	}

	// Last resort: token-overlap match.
	if tokenMatch != nil {
		return db.buildSong(tokenMatch.id, tokenMatch.sname, tokenMatch.short, tokenMatch.writers, tokenMatch.first, tokenMatch.last, tokenMatch.times), nil
	}

	return nil, nil
}

func (db *DB) buildSong(id int, name string, short, writers, first, last sql.NullString, times int) *data.Song {
	s := &data.Song{ID: id, Name: name, TimesPlayed: times}
	if short.Valid {
		s.ShortName = short.String
	}
	if writers.Valid {
		s.Writers = writers.String
	}
	if first.Valid {
		s.FirstPlayed, _ = time.Parse("2006-01-02", first.String)
	}
	if last.Valid {
		s.LastPlayed, _ = time.Parse("2006-01-02", last.String)
	}
	return s
}

// GetSongByID returns a song by ID.
func (db *DB) GetSongByID(ctx context.Context, id int) (*data.Song, error) {
	var sname string
	var short, writers sql.NullString
	var first, last sql.NullString
	var times int
	err := db.conn.QueryRowContext(ctx, "SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs WHERE id = ?", id).
		Scan(&id, &sname, &short, &writers, &first, &last, &times)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	shortVal := ""
	if short.Valid {
		shortVal = short.String
	}
	writersVal := ""
	if writers.Valid {
		writersVal = writers.String
	}
	s := &data.Song{ID: id, Name: sname, ShortName: shortVal, Writers: writersVal, TimesPlayed: times}
	if first.Valid {
		t, _ := time.Parse("2006-01-02", first.String)
		s.FirstPlayed = t
	}
	if last.Valid {
		t, _ := time.Parse("2006-01-02", last.String)
		s.LastPlayed = t
	}
	return s, nil
}

// SearchSongs returns songs whose name contains the pattern (case-insensitive).
func (db *DB) SearchSongs(ctx context.Context, pattern string) ([]*data.Song, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs WHERE name LIKE ? OR short_name LIKE ? ORDER BY name", "%"+pattern+"%", "%"+pattern+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*data.Song
	for rows.Next() {
		var id, times int
		var sname string
		var short, writers sql.NullString
		var first, last sql.NullString
		if err := rows.Scan(&id, &sname, &short, &writers, &first, &last, &times); err != nil {
			return nil, err
		}
		shortVal := ""
		if short.Valid {
			shortVal = short.String
		}
		writersVal := ""
		if writers.Valid {
			writersVal = writers.String
		}
		s := &data.Song{ID: id, Name: sname, ShortName: shortVal, Writers: writersVal, TimesPlayed: times}
		if first.Valid {
			t, _ := time.Parse("2006-01-02", first.String)
			s.FirstPlayed = t
		}
		if last.Valid {
			t, _ := time.Parse("2006-01-02", last.String)
			s.LastPlayed = t
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
