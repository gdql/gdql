package mock

import (
	"context"

	"github.com/gdql/gdql/internal/data"
)

// DataSource is a mock that returns configurable results (for executor/planner tests without a real DB).
type DataSource struct {
	ExecuteQueryFunc func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error)
	GetSongFunc      func(ctx context.Context, name string) (*data.Song, error)
	GetSongByIDFunc  func(ctx context.Context, id int) (*data.Song, error)
	SearchSongsFunc  func(ctx context.Context, pattern string) ([]*data.Song, error)
	CloseFunc        func() error
}

// ExecuteQuery calls ExecuteQueryFunc if set, else returns empty result.
func (m *DataSource) ExecuteQuery(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
	if m.ExecuteQueryFunc != nil {
		return m.ExecuteQueryFunc(ctx, sql, args...)
	}
	return &data.ResultSet{Columns: nil, Rows: nil}, nil
}

// GetSong calls GetSongFunc if set, else returns nil.
func (m *DataSource) GetSong(ctx context.Context, name string) (*data.Song, error) {
	if m.GetSongFunc != nil {
		return m.GetSongFunc(ctx, name)
	}
	return nil, nil
}

// GetSongByID calls GetSongByIDFunc if set, else returns nil.
func (m *DataSource) GetSongByID(ctx context.Context, id int) (*data.Song, error) {
	if m.GetSongByIDFunc != nil {
		return m.GetSongByIDFunc(ctx, id)
	}
	return nil, nil
}

// SearchSongs calls SearchSongsFunc if set, else returns nil slice.
func (m *DataSource) SearchSongs(ctx context.Context, pattern string) ([]*data.Song, error) {
	if m.SearchSongsFunc != nil {
		return m.SearchSongsFunc(ctx, pattern)
	}
	return nil, nil
}

// Close calls CloseFunc if set, else returns nil.
func (m *DataSource) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
