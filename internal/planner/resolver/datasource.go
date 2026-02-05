package resolver

import (
	"context"

	"github.com/gdql/gdql/internal/data"
)

// DataSourceResolver resolves song names via a DataSource (GetSong).
type DataSourceResolver struct {
	DataSource data.DataSource
}

// NewDataSourceResolver returns a SongResolver that uses the given DataSource.
func NewDataSourceResolver(ds data.DataSource) *DataSourceResolver {
	return &DataSourceResolver{DataSource: ds}
}

// Resolve returns the song ID for name via DataSource.GetSong.
func (r *DataSourceResolver) Resolve(ctx context.Context, name string) (int, error) {
	song, err := r.DataSource.GetSong(ctx, name)
	if err != nil {
		return 0, err
	}
	if song == nil {
		return 0, &ErrSongNotFound{Name: name}
	}
	return song.ID, nil
}

// ResolveFuzzy uses SearchSongs and returns matches with scores.
func (r *DataSourceResolver) ResolveFuzzy(ctx context.Context, name string) ([]SongMatch, error) {
	songs, err := r.DataSource.SearchSongs(ctx, name)
	if err != nil {
		return nil, err
	}
	out := make([]SongMatch, 0, len(songs))
	for _, s := range songs {
		score := 0.5
		if s.Name == name {
			score = 1.0
		}
		out = append(out, SongMatch{ID: s.ID, Name: s.Name, Score: score})
	}
	return out, nil
}

// Suggest returns song names from SearchSongs for "did you mean?".
func (r *DataSourceResolver) Suggest(ctx context.Context, name string) []string {
	songs, _ := r.DataSource.SearchSongs(ctx, name)
	out := make([]string, 0, len(songs))
	for _, s := range songs {
		out = append(out, s.Name)
	}
	return out
}
