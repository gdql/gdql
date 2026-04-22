package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// RelationEntry is one directed relation between two canonical songs.
// Kind must be one of: merge_into, variant_of, pairs_with.
type RelationEntry struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// LoadRelationsFromFile reads a JSON file of song relations and inserts them
// into song_relations. Song names are resolved to IDs via songs.name
// (exact or case-insensitive). Entries referencing unknown songs are skipped.
// Format: [{"from": "Minglewood Blues", "to": "New Minglewood Blues", "kind": "variant_of"}, ...]
func LoadRelationsFromFile(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var entries []RelationEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.From == "" || e.To == "" || e.Kind == "" {
			skipped++
			continue
		}
		if e.Kind != "merge_into" && e.Kind != "variant_of" && e.Kind != "pairs_with" {
			skipped++
			continue
		}
		fromID, ok, err := resolveSongID(ctx, db, e.From)
		if err != nil {
			return loaded, skipped, err
		}
		if !ok {
			skipped++
			continue
		}
		toID, ok, err := resolveSongID(ctx, db, e.To)
		if err != nil {
			return loaded, skipped, err
		}
		if !ok {
			skipped++
			continue
		}
		if fromID == toID {
			skipped++
			continue
		}
		_, err = db.ExecContext(ctx,
			"INSERT OR REPLACE INTO song_relations (from_song_id, to_song_id, kind) VALUES (?, ?, ?)",
			fromID, toID, e.Kind)
		if err != nil {
			return loaded, skipped, fmt.Errorf("insert %s -[%s]-> %s: %w", e.From, e.Kind, e.To, err)
		}
		loaded++
	}
	return loaded, skipped, nil
}

func resolveSongID(ctx context.Context, db *sql.DB, name string) (int64, bool, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		"SELECT id FROM songs WHERE name = ? OR LOWER(name) = LOWER(?) LIMIT 1",
		name, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}
