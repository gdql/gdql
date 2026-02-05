package data

import (
	"context"
	"time"
)

// DataSource executes SQL and returns domain results.
type DataSource interface {
	ExecuteQuery(ctx context.Context, sql string, args ...interface{}) (*ResultSet, error)
	GetSong(ctx context.Context, name string) (*Song, error)
	GetSongByID(ctx context.Context, id int) (*Song, error)
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
	ID       int
	Date     time.Time
	VenueID  int
	Venue    string
	City     string
	State    string
	Notes    string
	Rating   float64
}

// Song is a song in the catalog.
type Song struct {
	ID          int
	Name        string
	ShortName   string
	Writers     string
	FirstPlayed time.Time
	LastPlayed  time.Time
	TimesPlayed int
}

// Performance is a song performed at a show.
// SongName is set when the query joins with songs (e.g. setlist) for display.
type Performance struct {
	ID            int
	ShowID        int
	SongID        int
	SetNumber     int
	Position      int
	SegueType     string
	LengthSeconds int
	SongName      string // optional, for setlist display
}
