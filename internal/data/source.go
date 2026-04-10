package data

import (
	"context"
	"encoding/json"
	"time"
)

// jsonMarshal is a thin wrapper to keep MarshalJSON implementations terse.
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// DataSource executes SQL and returns domain results.
type DataSource interface {
	ExecuteQuery(ctx context.Context, sql string, args ...interface{}) (*ResultSet, error)
	GetSong(ctx context.Context, name string) (*Song, error)
	GetSongByID(ctx context.Context, id int) (*Song, error)
	GetSongVariantIDs(ctx context.Context, name string) ([]int, error)
	SearchSongs(ctx context.Context, pattern string) ([]*Song, error)
	Close() error
}

// ResultSet is the result of a query.
type ResultSet struct {
	Columns []string
	Rows    []Row
}

// Row is a single row (slice of column values).
type Row []interface{}

// Show is a single show.
type Show struct {
	ID      int       `json:"id"`
	Date    time.Time `json:"date"`
	VenueID int       `json:"venue_id,omitempty"`
	Venue   string    `json:"venue"`
	City    string    `json:"city,omitempty"`
	State   string    `json:"state,omitempty"`
	Tour    string    `json:"tour,omitempty"`
}

// MarshalJSON renders Date as YYYY-MM-DD instead of full RFC3339.
func (s Show) MarshalJSON() ([]byte, error) {
	type showOut struct {
		ID      int    `json:"id"`
		Date    string `json:"date"`
		VenueID int    `json:"venue_id,omitempty"`
		Venue   string `json:"venue"`
		City    string `json:"city,omitempty"`
		State   string `json:"state,omitempty"`
		Tour    string `json:"tour,omitempty"`
	}
	out := showOut{
		ID: s.ID, VenueID: s.VenueID, Venue: s.Venue,
		City: s.City, State: s.State, Tour: s.Tour,
	}
	if !s.Date.IsZero() {
		out.Date = s.Date.Format("2006-01-02")
	}
	return jsonMarshal(out)
}

// Song is a song in the catalog.
type Song struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ShortName   string    `json:"short_name,omitempty"`
	Writers     string    `json:"writers,omitempty"`
	FirstPlayed time.Time `json:"first_played,omitempty"`
	LastPlayed  time.Time `json:"last_played,omitempty"`
	TimesPlayed int       `json:"times_played,omitempty"`
}

// MarshalJSON omits zero-time fields entirely (Go's encoding/json renders zero
// time as "0001-01-01T00:00:00Z" with omitempty, which isn't what we want).
func (s Song) MarshalJSON() ([]byte, error) {
	type songOut struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		ShortName   string `json:"short_name,omitempty"`
		Writers     string `json:"writers,omitempty"`
		FirstPlayed string `json:"first_played,omitempty"`
		LastPlayed  string `json:"last_played,omitempty"`
		TimesPlayed int    `json:"times_played,omitempty"`
	}
	out := songOut{
		ID: s.ID, Name: s.Name, ShortName: s.ShortName, Writers: s.Writers,
		TimesPlayed: s.TimesPlayed,
	}
	if !s.FirstPlayed.IsZero() {
		out.FirstPlayed = s.FirstPlayed.Format("2006-01-02")
	}
	if !s.LastPlayed.IsZero() {
		out.LastPlayed = s.LastPlayed.Format("2006-01-02")
	}
	return jsonMarshal(out)
}

// Performance is a song performed at a show.
// SongName is set when the query joins with songs (e.g. setlist) for display.
type Performance struct {
	ID            int    `json:"id"`
	ShowID        int    `json:"show_id"`
	SongID        int    `json:"song_id"`
	SetNumber     int    `json:"set_number,omitempty"`
	Position      int    `json:"position,omitempty"`
	SegueType     string `json:"segue,omitempty"`
	LengthSeconds int    `json:"length_seconds,omitempty"`
	SongName      string `json:"song,omitempty"`
}
