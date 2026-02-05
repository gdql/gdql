package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
)

// AliasEntry is one row for gdql import aliases.
type AliasEntry struct {
	Alias     string `json:"alias"`
	Canonical string `json:"canonical"`
}

// LoadAliasesFromFile reads a JSON file of alias -> canonical name pairs and inserts
// them into song_aliases. Canonical is resolved to song_id via songs.name (exact or case-insensitive).
// Format: [{"alias": "Scarlet Begonias-", "canonical": "Scarlet Begonias"}, ...]
// Entries whose canonical name is not found in songs are skipped.
func LoadAliasesFromFile(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var entries []AliasEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.Alias == "" || e.Canonical == "" {
			skipped++
			continue
		}
		var songID int64
		err := db.QueryRowContext(ctx, "SELECT id FROM songs WHERE name = ? OR LOWER(name) = LOWER(?) LIMIT 1", e.Canonical, e.Canonical).Scan(&songID)
		if err == sql.ErrNoRows {
			err = db.QueryRowContext(ctx, "SELECT id FROM songs WHERE LOWER(TRIM(name, '- ')) = LOWER(TRIM(?, '- ')) LIMIT 1", e.Canonical, e.Canonical).Scan(&songID)
		}
		if err == sql.ErrNoRows {
			skipped++
			continue
		}
		if err != nil {
			return loaded, skipped, err
		}
		_, err = db.ExecContext(ctx, "INSERT OR REPLACE INTO song_aliases (alias, song_id) VALUES (?, ?)", e.Alias, songID)
		if err != nil {
			return loaded, skipped, err
		}
		loaded++
	}
	return loaded, skipped, nil
}
ped, err
		}
		loaded++
	}
	return loaded, skipped, nil
}
