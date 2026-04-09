package canonical

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SongLyrics is a single song's lyrics for import.
type SongLyrics struct {
	Song   string `json:"song"`
	Lyrics string `json:"lyrics"`
}

// ImportLyrics reads a JSON file of [{song, lyrics}] and inserts into the lyrics table.
// Songs are matched by name (case-insensitive). Returns (loaded, skipped).
func ImportLyrics(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("reading %s: %w", path, err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return 0, 0, nil
	}
	var entries []SongLyrics
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, fmt.Errorf("parsing JSON: %w\nExpected format: [{\"song\": \"Song Name\", \"lyrics\": \"...\"}]", err)
	}
	for _, e := range entries {
		if e.Song == "" || e.Lyrics == "" {
			skipped++
			continue
		}
		var songID int
		err := db.QueryRowContext(ctx, "SELECT id FROM songs WHERE name = ? OR LOWER(name) = LOWER(?) LIMIT 1", e.Song, e.Song).Scan(&songID)
		if err != nil {
			skipped++
			continue
		}
		// Normalize lyrics for FTS: lowercase, strip punctuation
		fts := strings.ToLower(e.Lyrics)
		_, err = db.ExecContext(ctx, "INSERT OR REPLACE INTO lyrics (song_id, lyrics, lyrics_fts) VALUES (?, ?, ?)", songID, e.Lyrics, fts)
		if err != nil {
			return loaded, skipped, err
		}
		loaded++
	}
	return loaded, skipped, nil
}
