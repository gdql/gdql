package canonical

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gdql/gdql/internal/import/shared"
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
	songByName, err := shared.LoadSongByName(db)
	if err != nil {
		return 0, 0, err
	}
	venueMax, _ := shared.MaxID(db, "venues")
	showMax, _ := shared.MaxID(db, "shows")
	songMax, _ := shared.MaxID(db, "songs")
	perfMax, _ := shared.MaxID(db, "performances")
	nextVenueID := venueMax + 1
	nextShowID := showMax + 1
	nextSongID := songMax + 1
	startSongID := nextSongID
	nextPerfID := perfMax + 1

	for i := range shows {
		select {
		case <-ctx.Done():
			return showsAdded, int(nextSongID - startSongID), ctx.Err()
		default:
		}
		s := &shows[i]
		dateStr := normalizeDate(s.Date)
		if dateStr == "" {
			continue
		}
		vkey := venueKey(s.Venue)
		if shared.ShowExists(db, dateStr, s.Venue.Name, s.Venue.City, s.Venue.State, s.Venue.Country) {
			continue
		}
		venueID, ok := venueByKey[vkey]
		if !ok {
			_, execErr := db.ExecContext(ctx, "INSERT INTO venues (id, name, city, state, country) VALUES (?, ?, ?, ?, ?)",
				nextVenueID, s.Venue.Name, s.Venue.City, s.Venue.State, s.Venue.Country)
			if execErr != nil {
				return showsAdded, int(nextSongID - startSongID), execErr
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
			nextShowID, dateStr, venueID, shared.NullStr(s.Tour), shared.NullStr(s.Notes))
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
						return showsAdded, int(nextSongID - startSongID), execErr
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
					nextPerfID, showID, songID, setNumber, position, shared.NullStr(segueType), isOpener, isCloser)
				if execErr != nil {
					return showsAdded, int(nextSongID - startSongID), execErr
				}
				nextPerfID++
			}
		}
	}
	return showsAdded, int(nextSongID - startSongID), nil
}

// resolveSong resolves rawName to an existing song_id using the name+alias map, or heuristics
// (case-insensitive match, trim trailing " -"). When a heuristic matches, it inserts the variant
// into song_aliases so future lookups are exact. Returns (id, true) when resolved, (0, false)
// when the caller should create a new song with rawName.
func resolveSong(ctx context.Context, db *sql.DB, rawName string, songByName map[string]int64, _ int64) (int64, bool) {
	if id, ok := songByName[rawName]; ok {
		return id, true
	}
	// Case-insensitive match
	lowerRaw := strings.ToLower(rawName)
	for name, id := range songByName {
		if strings.ToLower(name) == lowerRaw {
			_, _ = db.ExecContext(ctx, "INSERT OR IGNORE INTO song_aliases (alias, song_id) VALUES (?, ?)", rawName, id)
			songByName[rawName] = id
			return id, true
		}
	}
	// Trim trailing segue marker and retry
	trimmed := trimTrailingSegue(rawName)
	if trimmed != rawName {
		if id, ok := songByName[trimmed]; ok {
			_, _ = db.ExecContext(ctx, "INSERT OR IGNORE INTO song_aliases (alias, song_id) VALUES (?, ?)", rawName, id)
			songByName[rawName] = id
			return id, true
		}
		lowerTrimmed := strings.ToLower(trimmed)
		for name, id := range songByName {
			if strings.ToLower(name) == lowerTrimmed {
				_, _ = db.ExecContext(ctx, "INSERT OR IGNORE INTO song_aliases (alias, song_id) VALUES (?, ?)", rawName, id)
				songByName[rawName] = id
				return id, true
			}
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

