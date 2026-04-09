// Package shared provides common utilities for GDQL data importers.
package shared

import (
	"database/sql"
	"fmt"
)

// MaxID returns the maximum id in the given table. Only allows known table names.
func MaxID(db *sql.DB, table string) (int64, error) {
	switch table {
	case "venues", "shows", "songs", "performances":
		// allowed
	default:
		return 0, fmt.Errorf("maxID: unknown table %q", table)
	}
	var id sql.NullInt64
	// Table name is validated above, safe to interpolate.
	if err := db.QueryRow("SELECT MAX(id) FROM " + table).Scan(&id); err != nil {
		return 0, fmt.Errorf("maxID(%s): %w", table, err)
	}
	if id.Valid {
		return id.Int64, nil
	}
	return 0, nil
}

// LoadSongByName returns a map from song name (and alias) to song_id.
func LoadSongByName(db *sql.DB) (map[string]int64, error) {
	out := make(map[string]int64)
	rows, err := db.Query("SELECT id, name FROM songs")
	if err != nil {
		return nil, fmt.Errorf("loading songs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scanning song: %w", err)
		}
		out[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating songs: %w", err)
	}
	rows2, err := db.Query("SELECT alias, song_id FROM song_aliases")
	if err != nil {
		return nil, fmt.Errorf("loading aliases: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var alias string
		var songID int64
		if err := rows2.Scan(&alias, &songID); err != nil {
			return nil, fmt.Errorf("scanning alias: %w", err)
		}
		if _, exists := out[alias]; !exists {
			out[alias] = songID
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("iterating aliases: %w", err)
	}
	return out, nil
}

// ShowExists checks if a show already exists by date and venue details.
func ShowExists(db *sql.DB, dateStr, venueName, city, state, country string) bool {
	var n int
	err := db.QueryRow(
		"SELECT 1 FROM shows s JOIN venues v ON s.venue_id = v.id WHERE s.date = ? AND v.name = ? AND COALESCE(v.city,'') = ? AND COALESCE(v.state,'') = ? AND COALESCE(v.country,'') = ? LIMIT 1",
		dateStr, venueName, city, state, country,
	).Scan(&n)
	return err == nil
}

// NullStr returns nil for empty strings (for SQL NULL insertion).
func NullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
