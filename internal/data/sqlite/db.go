package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/gdql/gdql/internal/data"

	_ "modernc.org/sqlite"
)

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

// GetSong returns a song by exact or case-insensitive name match, then by song_aliases, then by a best-effort trim of trailing " -".
// For 100% accuracy on variants (parentheses, segues, spelling), add explicit rows to song_aliases (see SONG_NORMALIZATION.md).
func (db *DB) GetSong(ctx context.Context, name string) (*data.Song, error) {
	var id int
	var sname string
	var short, writers sql.NullString
	var first, last sql.NullString
	var times int
	err := db.conn.QueryRowContext(ctx, "SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs WHERE name = ? OR LOWER(name) = LOWER(?) LIMIT 1", name, name).
		Scan(&id, &sname, &short, &writers, &first, &last, &times)
	if err == sql.ErrNoRows {
		// Explicit alias (alias -> song_id) is the only 100% accurate way to handle variants.
		err = db.conn.QueryRowContext(ctx, "SELECT s.id, s.name, s.short_name, s.writers, s.first_played, s.last_played, s.times_played FROM songs s JOIN song_aliases a ON s.id = a.song_id WHERE a.alias = ? OR LOWER(a.alias) = LOWER(?) LIMIT 1", name, name).
			Scan(&id, &sname, &short, &writers, &first, &last, &times)
	}
	if err == sql.ErrNoRows {
		// Best-effort: Relisten often uses trailing " -" for segues. Prefer adding an alias.
		err = db.conn.QueryRowContext(ctx, "SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs WHERE LOWER(TRIM(name, '- ')) = LOWER(TRIM(?, '- ')) LIMIT 1", name, name).
			Scan(&id, &sname, &short, &writers, &first, &last, &times)
	}
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
