package canonical

import (
	"context"
	"database/sql"
	"strings"
)

// Show is a single show in a source-agnostic format. Any importer (API, scrape, JSON, CSV)
// can produce []Show and call WriteShows to merge into the GDQL DB.
// JSON: date, venue, tour, notes, sets (see docs/CANONICAL_IMPORT.md).
type Show struct {
	Date  string `json:"date"`
	Venue Venue  `json:"venue"`
	Tour  string `json:"tour"`
	Notes string `json:"notes"`
	Sets  []Set  `json:"sets"`
}

type Venue struct {
	Name    string `json:"name"`
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

// Set is one set (first set, second set, encore). Songs in order.
type Set struct {
	Songs []SongInSet `json:"songs"`
}

// SongInSet is one song in a set. SegueBefore true means ">" from previous.
type SongInSet struct {
	Name        string `json:"name"`
	SegueBefore bool   `json:"segue_before"`
}

// WriteShows inserts shows into the DB. It creates venues and songs as needed,
// skips shows that already exist (same date + venue), and returns (showsAdded, songsAdded).
// Use this from setlist.fm, Archive.org, scrapers, or JSON/CSV import.
func WriteShows(ctx context.Context, db *sql.DB, shows []Show) (showsAdded, songsAdded int, err error) {
	venueByKey := make(map[string]int64)
	songByName := loadSongByName(db)
	nextVenueID := maxID(db, "venues") + 1
	nextShowID := maxID(db, "shows") + 1
	nextSongID := maxID(db, "songs") + 1
	nextPerfID := maxID(db, "performances") + 1
	songsBefore := len(songByName)

	for i := range shows {
		select {
		case <-ctx.Done():
			return showsAdded, len(songByName) - songsBefore, ctx.Err()
		default:
		}
		s := &shows[i]
		dateStr := normalizeDate(s.Date)
		if dateStr == "" {
			continue
		}
		vkey := venueKey(s.Venue)
		if showExists(db, dateStr, s.Venue.Name, s.Venue.City, s.Venue.State, s.Venue.Country) {
			continue
		}
		venueID, ok := venueByKey[vkey]
		if !ok {
			_, execErr := db.ExecContext(ctx, "INSERT INTO venues (id, name, city, state, country) VALUES (?, ?, ?, ?, ?)",
				nextVenueID, s.Venue.Name, s.Venue.City, s.Venue.State, s.Venue.Country)
			if execErr != nil {
				return showsAdded, len(songByName) - songsBefore, execErr
			}
			venueID = nextVenueID
			venueByKey[vkey] = venueID
			nextVenueID++
		}
		var exist int
		if db.QueryRowContext(ctx, "SELECT 1 FROM shows WHERE date = ? AND venue_id = ? LIMIT 1", dateStr, venueID).Scan(&exist) == nil {
			continue
		}
		_, err := db.ExecContext(ctx, "INSERT INTO shows (id, date, venue_id, tour, notes) VALUES (?, ?, ?, ?, ?)",
			nextShowID, dateStr, venueID, nullStr(s.Tour), nullStr(s.Notes))
		if err != nil {
			return showsAdded, songsAdded, err
		}
		showID := nextShowID
		nextShowID++
		showsAdded++

		setNumber := 0
		for _, set := range s.Sets {
			setNumber++
			if setNumber > 3 {
				setNumber = 3
			}
			position := 0
			for j, song := range set.Songs {
				position++
				rawName := strings.TrimSpace(song.Name)
				songID, ok := resolveSong(ctx, db, rawName, songByName, nextSongID)
				if !ok {
					_, execErr := db.ExecContext(ctx, "INSERT INTO songs (id, name, times_played) VALUES (?, ?, 0)", nextSongID, rawName)
					if execErr != nil {
						return showsAdded, len(songByName) - songsBefore, execErr
					}
					songID = nextSongID
					songByName[rawName] = songID
					nextSongID++
				}
				segueType := ""
				if song.SegueBefore {
					segueType = ">"
				}
				isOpener := 0
				if setNumber == 1 && position == 1 {
					isOpener = 1
				}
				isCloser := 0
				if j == len(set.Songs)-1 {
					isCloser = 1
				}
				_, execErr := db.ExecContext(ctx, "INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, is_opener, is_closer) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
					nextPerfID, showID, songID, setNumber, position, nullStr(segueType), isOpener, isCloser)
				if execErr != nil {
					return showsAdded, len(songByName) - songsBefore, execErr
				}
				nextPerfID++
			}
		}
	}
	songsAdded = len(songByName) - songsBefore
	if songsAdded < 0 {
		songsAdded = 0
	}
	return showsAdded, songsAdded, nil
}

// resolveSong resolves rawName to an existing song_id using the name+alias map, or a one-time heuristic (trim trailing " -").
// When the heuristic matches, it inserts the variant into song_aliases so future lookups are exact. Returns (id, true) when resolved, (0, false) when the caller should create a new song with rawName.
func resolveSong(ctx context.Context, db *sql.DB, rawName string, songByName map[string]int64, _ int64) (int64, bool) {
	if id, ok := songByName[rawName]; ok {
		return id, true
	}
	trimmed := trimTrailingSegue(rawName)
	if trimmed != rawName {
		if id, ok := songByName[trimmed]; ok {
			_, _ = db.ExecContext(ctx, "INSERT OR IGNORE INTO song_aliases (alias, song_id) VALUES (?, ?)", rawName, id)
			songByName[rawName] = id
			return id, true
		}
	}
	return 0, false
}

// trimTrailingSegue removes trailing " -" / "-" from source-style names (e.g. "Scarlet Begonias-").
// Used only to suggest a merge at import time; the mapping is then stored in song_aliases.
func trimTrailingSegue(s string) string {
	for strings.HasSuffix(s, "-") || strings.HasSuffix(s, " -") {
		s = strings.TrimSuffix(s, " -")
		s = strings.TrimSuffix(s, "-")
		s = strings.TrimSpace(s)
	}
	return s
}

func normalizeDate(d string) string {
	d = strings.TrimSpace(d)
	// Already YYYY-MM-DD
	if len(d) == 10 && d[4] == '-' && d[7] == '-' {
		return d
	}
	// DD-MM-YYYY -> YYYY-MM-DD
	parts := strings.Split(d, "-")
	if len(parts) != 3 {
		return ""
	}
	return parts[2] + "-" + parts[1] + "-" + parts[0]
}

func venueKey(v Venue) string {
	return v.Name + "\t" + v.City + "\t" + v.State + "\t" + v.Country
}

func showExists(db *sql.DB, dateStr, venueName, city, state, country string) bool {
	var n int
	err := db.QueryRow(
		"SELECT 1 FROM shows s JOIN venues v ON s.venue_id = v.id WHERE s.date = ? AND v.name = ? AND COALESCE(v.city,'') = ? AND COALESCE(v.state,'') = ? AND COALESCE(v.country,'') = ? LIMIT 1",
		dateStr, venueName, city, state, country,
	).Scan(&n)
	return err == nil
}

// loadSongByName returns a map from song name or alias to song_id (for import resolution).
func loadSongByName(db *sql.DB) map[string]int64 {
	out := make(map[string]int64)
	rows, err := db.Query("SELECT id, name FROM songs")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		out[name] = id
	}
	rows2, err := db.Query("SELECT alias, song_id FROM song_aliases")
	if err != nil {
		return out
	}
	defer rows2.Close()
	for rows2.Next() {
		var alias string
		var songID int64
		if err := rows2.Scan(&alias, &songID); err != nil {
			continue
		}
		if _, exists := out[alias]; !exists {
			out[alias] = songID
		}
	}
	return out
}

func maxID(db *sql.DB, table string) int64 {
	var id sql.NullInt64
	_ = db.QueryRow("SELECT MAX(id) FROM " + table).Scan(&id)
	if id.Valid {
		return id.Int64
	}
	return 0
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
