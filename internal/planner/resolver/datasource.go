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

// ResolveVariants returns ALL song IDs whose normalized name matches.
// Used for set-membership tests (PLAYED, NOT PLAYED) so duplicates count as one song.
// Falls back to GetSong (which supports fuzzy/prefix matching) if no exact variants found.
func (r *DataSourceResolver) ResolveVariants(ctx context.Context, name string) ([]int, error) {
	ids, err := r.DataSource.GetSongVariantIDs(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}
	// Fallback: try GetSong which does fuzzy/prefix matching, then get variants by resolved name.
	song, err := r.DataSource.GetSong(ctx, name)
	if err != nil {
		return nil, err
	}
	if song == nil {
		return nil, &ErrSongNotFound{Name: name}
	}
	ids, err = r.DataSource.GetSongVariantIDs(ctx, song.Name)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []int{song.ID}, nil
	}
	return ids, nil
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
