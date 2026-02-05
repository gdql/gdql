package executor

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/data/mock"
	"github.com/gdql/gdql/internal/parser"
	"github.com/stretchr/testify/require"
)

func TestExecutor_Execute_ParseError(t *testing.T) {
	ds := &mock.DataSource{}
	pl := New(ds)
	_, err := pl.Execute(context.Background(), "NOT A VALID QUERY")
	require.Error(t, err)
}

func TestExecutor_ExecuteAST_ShowQuery_NoDBRows(t *testing.T) {
	ds := &mock.DataSource{}
	ds.ExecuteQueryFunc = func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
		return &data.ResultSet{Columns: []string{"id", "date", "venue_id", "venue", "city", "state", "notes", "rating"}, Rows: nil}, nil
	}
	ds.GetSongFunc = func(ctx context.Context, name string) (*data.Song, error) {
		return nil, nil
	}
	ex := New(ds)
	p := parser.NewFromString("SHOWS FROM 1977 LIMIT 5")
	ast, err := p.Parse()
	require.NoError(t, err)

	result, err := ex.ExecuteAST(context.Background(), ast)
	require.NoError(t, err)
	require.Equal(t, ResultShows, result.Type)
	require.Empty(t, result.Shows)
}

func TestExecutor_ExecuteAST_ShowQuery_WithRows(t *testing.T) {
	ds := &mock.DataSource{}
	ds.ExecuteQueryFunc = func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
		return &data.ResultSet{
			Columns: []string{"id", "date", "venue_id", "venue", "city", "state", "notes", "rating"},
			Rows: []data.Row{
				{1, "1977-05-08", 1, "Barton Hall", "Ithaca", "NY", "", 4.9},
			},
		}, nil
	}
	ds.GetSongFunc = func(ctx context.Context, name string) (*data.Song, error) {
		return nil, nil
	}
	ex := New(ds)
	q := &ast.ShowQuery{From: &ast.DateRange{Start: &ast.Date{Year: 1977}}}
	result, err := ex.ExecuteAST(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ResultShows, result.Type)
	require.Len(t, result.Shows, 1)
	require.Equal(t, 1, result.Shows[0].ID)
	require.Equal(t, "Barton Hall", result.Shows[0].Venue)
}
