package setlistfm

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gdql/gdql/internal/data/sqlite"

	_ "modernc.org/sqlite"
)

// Import fetches Grateful Dead setlists from the API and writes them to the SQLite DB at path.
// Schema is applied if the DB is new. API key must be set on the client.
func Import(ctx context.Context, dbPath string, client *Client) (showsAdded, songsAdded int, err error) {
	if err := sqlite.InitSchema(dbPath); err != nil {
		return 0, 0, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()

	venueByKey := make(map[string]int64)
	songByName := loadSongByName(db)
	nextVenueID := maxID(db, "venues") + 1
	nextShowID := maxID(db, "shows") + 1
	nextSongID := maxID(db, "songs") + 1
	nextPerfID := maxID(db, "performances") + 1
	songsBefore := len(songByName)

	page := 1
	for {
		select {
		case <-ctx.Done():
			return showsAdded, songsAdded, ctx.Err()
		default:
		}
		resp, err := client.GetArtistSetlists(GratefulDeadMBID, page)
		if err != nil {
			return showsAdded, songsAdded, err
		}
		if len(resp.Setlist) == 0 {
			break
		}
		for i := range resp.Setlist {
			sl := &resp.Setlist[i]
			dateStr, ok := parseEventDate(sl.EventDate)
			if !ok {
				continue
			}
			venueName, city, state, country := venueFields(&sl.Venue)
			if showExists(db, dateStr, venueName, city, state, country) {
				continue // already have this show; skip so we can resume later
			}
			// List endpoint often returns empty set[]; fetch by version ID for full set/song data.
			if len(sl.Set) == 0 && sl.VersionID != "" {
				select {
				case <-ctx.Done():
					return showsAdded, songsAdded, ctx.Err()
				default:
				}
				full, err := client.GetSetlist(sl.VersionID)
				if err != nil {
					return showsAdded, songsAdded, err
				}
				sl = full
			}
			added, err := upsertShow(db, sl, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
			if err != nil {
				return showsAdded, songsAdded, err
			}
			if added {
				showsAdded++
			}
		}
		if page*resp.ItemsPerPage >= resp.Total {
			break
		}
		page++
	}
	songsAdded = len(songByName) - songsBefore
	if songsAdded < 0 {
		songsAdded = 0
	}
	return showsAdded, songsAdded, nil
}

// parseEventDate converts dd-MM-yyyy to yyyy-MM-dd. Returns ("", false) on invalid.
func parseEventDate(eventDate string) (string, bool) {
	parts := strings.Split(eventDate, "-")
	if len(parts) != 3 {
		return "", false
	}
	return parts[2] + "-" + parts[1] + "-" + parts[0], true
}

func venueFields(v *Venue) (name, city, state, country string) {
	name = v.Name
	if v.City != nil {
		city = v.City.Name
		state = v.City.StateCode
		if v.City.Country != nil {
			country = v.City.Country.Code
		}
	}
	return name, city, state, country
}

func showExists(db *sql.DB, dateStr, venueName, city, state, country string) bool {
	var n int
	err := db.QueryRow(
		"SELECT 1 FROM shows s JOIN venues v ON s.venue_id = v.id WHERE s.date = ? AND v.name = ? AND COALESCE(v.city,'') = ? AND COALESCE(v.state,'') = ? AND COALESCE(v.country,'') = ? LIMIT 1",
		dateStr, venueName, city, state, country,
	).Scan(&n)
	return err == nil
}

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

func venueKey(v *Venue) string {
	city := ""
	if v.City != nil {
		city = v.City.Name + "|" + v.City.StateCode
		if v.City.Country != nil {
			city += "|" + v.City.Country.Code
		}
	}
	return v.Name + "\t" + city
}

func upsertShow(db *sql.DB, sl *Setlist, venueByKey map[string]int64, songByName map[string]int64, nextVenueID, nextShowID, nextSongID, nextPerfID *int64) (bool, error) {
	// Parse date dd-MM-yyyy -> yyyy-MM-dd
	parts := strings.Split(sl.EventDate, "-")
	if len(parts) != 3 {
		return false, nil
	}
	dateStr := parts[2] + "-" + parts[1] + "-" + parts[0]

	// Venue
	v := &sl.Venue
	key := venueKey(v)
	venueID, ok := venueByKey[key]
	if !ok {
		city, state, country := "", "", ""
		if v.City != nil {
			city = v.City.Name
			state = v.City.StateCode
			if v.City.Country != nil {
				country = v.City.Country.Code
			}
		}
		_, err := db.Exec("INSERT INTO venues (id, name, city, state, country) VALUES (?, ?, ?, ?, ?)", *nextVenueID, v.Name, city, state, country)
		if err != nil {
			return false, err
		}
		venueID = *nextVenueID
		venueByKey[key] = venueID
		*nextVenueID++
	}

	// Avoid duplicate show (e.g. when resuming after 429)
	var exist int
	if db.QueryRow("SELECT 1 FROM shows WHERE date = ? AND venue_id = ? LIMIT 1", dateStr, venueID).Scan(&exist) == nil {
		return false, nil
	}

	tour := ""
	if sl.Tour != nil {
		tour = sl.Tour.Name
	}
	res, err := db.Exec("INSERT OR IGNORE INTO shows (id, date, venue_id, tour, notes) VALUES (?, ?, ?, ?, ?)", *nextShowID, dateStr, venueID, tour, sl.Info)
	if err != nil {
		return false, err
	}
	showID := *nextShowID
	inserted, _ := res.RowsAffected()
	if inserted == 0 {
		return false, nil
	}
	*nextShowID++

	setNumber := 0
	for _, set := range sl.Set {
		if set.Encore > 0 {
			setNumber = 2 + set.Encore
		} else {
			setNumber++
		}
		if setNumber > 3 {
			setNumber = 3
		}
		position := 0
		for i, song := range set.Songs {
			names, segueAfter := splitSongName(song.Name)
			for j, name := range names {
				if name == "" {
					continue
				}
				position++
				songID, ok := songByName[name]
				if !ok {
					_, err := db.Exec("INSERT INTO songs (id, name, times_played) VALUES (?, ?, 0)", *nextSongID, name)
					if err != nil {
						return false, err
					}
					songID = *nextSongID
					songByName[name] = songID
					*nextSongID++
				}
				segueType := ""
				if j < len(segueAfter) && segueAfter[j] {
					segueType = ">"
				}
				if i+1 < len(set.Songs) && strings.TrimSpace(set.Songs[i+1].Info) == ">" {
					segueType = ">"
				}
				isOpener := 0
				if position == 1 && setNumber == 1 {
					isOpener = 1
				}
				isCloser := 0
				lastSongInSet := (i == len(set.Songs)-1 && j == len(names)-1)
				if lastSongInSet {
					isCloser = 1
				}
				_, err = db.Exec("INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, is_opener, is_closer) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
					*nextPerfID, showID, songID, setNumber, position, nullStr(segueType), isOpener, isCloser)
				if err != nil {
					return false, err
				}
				*nextPerfID++
			}
		}
	}
	return true, nil
}

func splitSongName(s string) (names []string, segueAfter []bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.Contains(s, " > ") {
		return []string{s}, []bool{false}
	}
	parts := strings.Split(s, " > ")
	names = make([]string, 0, len(parts))
	segueAfter = make([]bool, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			names = append(names, p)
			if i < len(parts)-1 {
				segueAfter[len(names)-1] = true
			}
		}
	}
	return names, segueAfter
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
