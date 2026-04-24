package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// Enrichment loaders pull committed JSON artifacts (produced by
// scripts/geocode_venues.py and scripts/fetch_weather.py) into their
// respective DB tables. Idempotent via INSERT OR REPLACE.
//
// Called from cmd/gdql-import so the embedded DB ships with the data
// baked in — consumers query gdql and get lat/lon + weather without
// needing separate JSON files.

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyVenue(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type geoEntry struct {
	Lat  *float64 `json:"lat"`
	Lon  *float64 `json:"lon"`
	Name string   `json:"name"`
}

// LoadGeoFromFile reads data/venues_geo.json (slug -> {name, lat, lon, ...})
// and fills venue_coords for every venue whose slugified name matches.
func LoadGeoFromFile(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var entries map[string]geoEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, err
	}

	// Build slug -> venue_id map once.
	rows, err := db.QueryContext(ctx, "SELECT id, name FROM venues")
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	slugToID := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return loaded, skipped, err
		}
		slug := slugifyVenue(name)
		if slug != "" {
			// First match wins; duplicates are rare
			if _, exists := slugToID[slug]; !exists {
				slugToID[slug] = id
			}
		}
	}

	for slug, e := range entries {
		if e.Lat == nil || e.Lon == nil {
			continue
		}
		id, ok := slugToID[slug]
		if !ok {
			skipped++
			continue
		}
		_, err = db.ExecContext(ctx, `
			INSERT OR REPLACE INTO venue_coords (venue_id, lat, lon, source)
			VALUES (?, ?, ?, 'nominatim')
		`, id, *e.Lat, *e.Lon)
		if err != nil {
			return loaded, skipped, err
		}
		loaded++
	}
	return loaded, skipped, nil
}

type weatherEntry struct {
	HighC       *float64 `json:"high_c"`
	LowC        *float64 `json:"low_c"`
	PrecipMM    *float64 `json:"precip_mm"`
	WindKPH     *float64 `json:"wind_kph"`
	WeatherCode *int     `json:"code"`
}

// LoadWeatherFromFile reads data/weather.json (date -> {high_c, low_c, ...})
// and populates show_weather for every show whose date matches.
func LoadWeatherFromFile(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var entries map[string]weatherEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, err
	}

	rows, err := db.QueryContext(ctx, "SELECT id, date FROM shows")
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	dateToIDs := map[string][]int64{}
	for rows.Next() {
		var id int64
		var date string
		if err := rows.Scan(&id, &date); err != nil {
			return loaded, skipped, err
		}
		dateToIDs[date] = append(dateToIDs[date], id)
	}

	for date, w := range entries {
		ids, ok := dateToIDs[date]
		if !ok {
			skipped++
			continue
		}
		for _, id := range ids {
			_, err = db.ExecContext(ctx, `
				INSERT OR REPLACE INTO show_weather
				  (show_id, temp_high_c, temp_low_c, precip_mm, wind_kph, weather_code)
				VALUES (?, ?, ?, ?, ?, ?)
			`, id, w.HighC, w.LowC, w.PrecipMM, w.WindKPH, w.WeatherCode)
			if err != nil {
				return loaded, skipped, err
			}
			loaded++
		}
	}
	return loaded, skipped, nil
}

type recordingEntry struct {
	ID        string   `json:"id"`
	Source    string   `json:"src"`
	Downloads *int     `json:"dl"`
	Rating    *float64 `json:"r"`
	Title     string   `json:"t"`
}

// LoadRecordingsFromFile reads a date -> [recordings] map (the shape that
// deaddaily-listen's scripts/build-recordings.py already produces) and
// fills show_recordings.
func LoadRecordingsFromFile(ctx context.Context, db *sql.DB, path string) (loaded, skipped int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var entries map[string][]recordingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, err
	}

	rows, err := db.QueryContext(ctx, "SELECT id, date FROM shows")
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	dateToIDs := map[string][]int64{}
	for rows.Next() {
		var id int64
		var date string
		if err := rows.Scan(&id, &date); err != nil {
			return loaded, skipped, err
		}
		dateToIDs[date] = append(dateToIDs[date], id)
	}

	for date, recs := range entries {
		ids, ok := dateToIDs[date]
		if !ok {
			skipped += len(recs)
			continue
		}
		for _, showID := range ids {
			for _, r := range recs {
				if r.ID == "" {
					continue
				}
				_, err = db.ExecContext(ctx, `
					INSERT OR REPLACE INTO show_recordings
					  (show_id, identifier, source, downloads, rating, title)
					VALUES (?, ?, ?, ?, ?, ?)
				`, showID, r.ID, nullableStr(r.Source), r.Downloads, r.Rating, nullableStr(r.Title))
				if err != nil {
					return loaded, skipped, err
				}
				loaded++
			}
		}
	}
	return loaded, skipped, nil
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
