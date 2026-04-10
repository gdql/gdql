package setlistfm

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/import/shared"

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
	songByName, loadErr := shared.LoadSongByName(db)
	if loadErr != nil {
		return 0, 0, loadErr
	}
	venueMax, _ := shared.MaxID(db, "venues")
	showMax, _ := shared.MaxID(db, "shows")
	songMax, _ := shared.MaxID(db, "songs")
	perfMax, _ := shared.MaxID(db, "performances")
	nextVenueID := venueMax + 1
	nextShowID := showMax + 1
	nextSongID := songMax + 1
	nextPerfID := perfMax + 1
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
			if shared.ShowExists(db, dateStr, venueName, city, state, country) {
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

	// If the API gave us a single set with many songs, infer set breaks
	sets := sl.Set
	if len(sets) == 1 && len(sets[0].Songs) > 8 {
		sets = InferSetBreaks(sets[0].Songs)
	}

	setNumber := 0
	for _, set := range sets {
		if set.Encore > 0 {
			setNumber = 3 + set.Encore // encore 1 → 4, encore 2 → 5
		} else {
			setNumber++
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
					// Case-insensitive match before creating a new song
					lowerName := strings.ToLower(name)
					for existing, id := range songByName {
						if strings.ToLower(existing) == lowerName {
							songID = id
							songByName[name] = id
							ok = true
							break
						}
					}
				}
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
					*nextPerfID, showID, songID, setNumber, position, shared.NullStr(segueType), isOpener, isCloser)
				if err != nil {
					return false, err
				}
				*nextPerfID++
			}
		}
	}
	return true, nil
}

// InferSetBreaks splits a single flat set into set 1, set 2, and encore
// using Grateful Dead-specific heuristics. Only applies when the API
// returns all songs in one set object (common for older shows).
func InferSetBreaks(songs []Song) []Set {
	if len(songs) <= 8 {
		return []Set{{Songs: songs}}
	}

	// Flatten song names for analysis
	names := make([]string, len(songs))
	for i, s := range songs {
		names[i] = strings.ToLower(strings.TrimSpace(s.Name))
	}

	// Find Drums/Space — reliable set 2 marker
	drumsIdx := -1
	for i, n := range names {
		if n == "drums" || n == "space" || n == "drums/space" || n == "rhythm devils" {
			drumsIdx = i
			break
		}
	}

	// Known encore openers (last few songs)
	encoreOpeners := map[string]bool{
		"one more saturday night": true, "u.s. blues": true,
		"johnny b. goode": true, "not fade away": true,
		"brokedown palace": true, "we bid you goodnight": true,
		"and we bid you good night": true, "box of rain": true,
		"satisfaction": true, "turn on your lovelight": true,
	}

	// Find encore start — scan from end, max 3 songs
	encoreStart := len(songs)
	for i := len(songs) - 1; i >= len(songs)-3 && i >= 0; i-- {
		if encoreOpeners[names[i]] {
			encoreStart = i
			break
		}
	}
	// If Drums was found very late and no encore marker, don't misidentify
	if encoreStart == len(songs) && drumsIdx >= 0 && drumsIdx < len(songs)-3 {
		// No explicit encore, that's fine
	}

	var sets []Set
	if drumsIdx >= 0 {
		// Set 1: everything before Drums
		// Set 2: Drums through end (or through encore start)
		sets = append(sets, Set{Name: "Set 1", Songs: songs[:drumsIdx]})
		if encoreStart < len(songs) && encoreStart > drumsIdx {
			sets = append(sets, Set{Name: "Set 2", Songs: songs[drumsIdx:encoreStart]})
			sets = append(sets, Set{Encore: 1, Songs: songs[encoreStart:]})
		} else {
			sets = append(sets, Set{Name: "Set 2", Songs: songs[drumsIdx:]})
		}
	} else if len(songs) > 12 {
		// No Drums found — split roughly in half
		mid := len(songs) / 2
		if encoreStart < len(songs) {
			sets = append(sets, Set{Name: "Set 1", Songs: songs[:mid]})
			sets = append(sets, Set{Name: "Set 2", Songs: songs[mid:encoreStart]})
			sets = append(sets, Set{Encore: 1, Songs: songs[encoreStart:]})
		} else {
			sets = append(sets, Set{Name: "Set 1", Songs: songs[:mid]})
			sets = append(sets, Set{Name: "Set 2", Songs: songs[mid:]})
		}
	} else {
		// Short-ish show with no Drums — leave as one set, maybe with encore
		if encoreStart < len(songs) {
			sets = append(sets, Set{Name: "Set 1", Songs: songs[:encoreStart]})
			sets = append(sets, Set{Encore: 1, Songs: songs[encoreStart:]})
		} else {
			sets = append(sets, Set{Songs: songs})
		}
	}
	return sets
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
	segueAfter = make([]bool, 0, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			names = append(names, p)
			segueAfter = append(segueAfter, i < len(parts)-1)
		}
	}
	return names, segueAfter
}

