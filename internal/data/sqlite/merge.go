package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// MergeRecord is written to the caller-supplied records file for each applied
// merge so that the "to" song can surface a transparency note in downstream
// UIs ("also counted in setlist data as: …") without needing to query the DB
// after the "from" row is gone.
type MergeRecord struct {
	FromName         string `json:"from_name"`
	ToName           string `json:"to_name"`
	FromTimesPlayed  int    `json:"from_times_played"`
	FromFirstPlayed  string `json:"from_first_played,omitempty"`
	FromLastPlayed   string `json:"from_last_played,omitempty"`
	MergedAtUnix     int64  `json:"merged_at_unix"`
}

// MergeSongsFromFile applies kind=merge_into rows from a song_relations.json
// file destructively:
//   1. records pre-merge metadata for the "from" song
//   2. reattributes performances from -> to
//   3. recomputes times_played / first_played / last_played on "to"
//   4. inserts from_name as an alias of "to" so future imports normalize
//   5. clears song_relations and song_aliases rows that reference the "from"
//   6. deletes the "from" song row
//
// Idempotent: if the "from" song is already gone, the entry is silently
// skipped. Returns applied records (callers typically persist these to a
// records JSON file alongside data/song_relations.json).
func MergeSongsFromFile(ctx context.Context, db *sql.DB, relationsPath string) (records []MergeRecord, skipped int, err error) {
	data, err := os.ReadFile(relationsPath)
	if err != nil {
		return nil, 0, err
	}
	var entries []RelationEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	now := time.Now().Unix()
	for _, e := range entries {
		if e.Kind != "merge_into" {
			continue
		}
		rec, applied, err := applyOneMerge(ctx, db, e.From, e.To, now)
		if err != nil {
			return records, skipped, fmt.Errorf("merge %q -> %q: %w", e.From, e.To, err)
		}
		if !applied {
			skipped++
			continue
		}
		records = append(records, rec)
	}
	return records, skipped, nil
}

func applyOneMerge(ctx context.Context, db *sql.DB, fromName, toName string, mergedAt int64) (MergeRecord, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MergeRecord{}, false, err
	}
	defer tx.Rollback()

	fromID, ok, err := resolveSongIDTx(ctx, tx, fromName)
	if err != nil {
		return MergeRecord{}, false, err
	}
	if !ok {
		// already merged away (or never existed) — skip.
		return MergeRecord{}, false, nil
	}
	toID, ok, err := resolveSongIDTx(ctx, tx, toName)
	if err != nil {
		return MergeRecord{}, false, err
	}
	if !ok {
		return MergeRecord{}, false, fmt.Errorf("target song %q not found", toName)
	}

	// Capture pre-merge state of the "from" row.
	var rec MergeRecord
	rec.FromName = fromName
	rec.ToName = toName
	rec.MergedAtUnix = mergedAt
	var fp, lp sql.NullString
	err = tx.QueryRowContext(ctx,
		"SELECT times_played, first_played, last_played FROM songs WHERE id = ?",
		fromID).Scan(&rec.FromTimesPlayed, &fp, &lp)
	if err != nil {
		return MergeRecord{}, false, err
	}
	if fp.Valid {
		rec.FromFirstPlayed = fp.String
	}
	if lp.Valid {
		rec.FromLastPlayed = lp.String
	}

	// Reattribute performances.
	if _, err := tx.ExecContext(ctx,
		"UPDATE performances SET song_id = ? WHERE song_id = ?",
		toID, fromID); err != nil {
		return MergeRecord{}, false, err
	}

	// Recompute aggregates on the "to" song from its (now-combined) performances.
	if _, err := tx.ExecContext(ctx, `
		UPDATE songs
		SET times_played = (
			SELECT count(*) FROM performances WHERE song_id = ?
		),
		first_played = (
			SELECT min(s.date) FROM performances p JOIN shows s ON s.id = p.show_id
			WHERE p.song_id = ?
		),
		last_played = (
			SELECT max(s.date) FROM performances p JOIN shows s ON s.id = p.show_id
			WHERE p.song_id = ?
		)
		WHERE id = ?
	`, toID, toID, toID, toID); err != nil {
		return MergeRecord{}, false, err
	}

	// Insert the from-name as an alias of the to song so future imports
	// normalize automatically. INSERT OR REPLACE in case a stale alias
	// still points at the vanished from_id.
	if _, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO song_aliases (alias, song_id) VALUES (?, ?)",
		fromName, toID); err != nil {
		return MergeRecord{}, false, err
	}

	// Clean up rows that reference the doomed "from" song.
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM song_aliases WHERE song_id = ?", fromID); err != nil {
		return MergeRecord{}, false, err
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM song_relations WHERE from_song_id = ? OR to_song_id = ?",
		fromID, fromID); err != nil {
		return MergeRecord{}, false, err
	}
	// Preserve lyrics: if the "from" row has lyrics and the "to" row does
	// not, hand them off before deleting the "from" entry. INSERT OR IGNORE
	// so we never overwrite an existing canonical lyric with a duplicate.
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO lyrics (song_id, lyrics, lyrics_fts)
		SELECT ?, lyrics, lyrics_fts FROM lyrics WHERE song_id = ?
	`, toID, fromID); err != nil {
		return MergeRecord{}, false, err
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM lyrics WHERE song_id = ?", fromID); err != nil {
		return MergeRecord{}, false, err
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM songs WHERE id = ?", fromID); err != nil {
		return MergeRecord{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return MergeRecord{}, false, err
	}
	return rec, true, nil
}

func resolveSongIDTx(ctx context.Context, tx *sql.Tx, name string) (int64, bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
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
