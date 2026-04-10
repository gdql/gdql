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

// === New query types ===

func TestExecutor_CountQuery(t *testing.T) {
	ds := &mock.DataSource{}
	ds.ExecuteQueryFunc = func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
		return &data.ResultSet{
			Columns: []string{"count", "name"},
			Rows:    []data.Row{{int64(236), "Dark Star"}},
		}, nil
	}
	ds.GetSongFunc = func(ctx context.Context, name string) (*data.Song, error) {
		return &data.Song{ID: 10, Name: "Dark Star"}, nil
	}
	ex := New(ds)
	q := &ast.CountQuery{Song: &ast.SongRef{Name: "Dark Star"}}
	result, err := ex.ExecuteAST(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ResultCount, result.Type)
	require.NotNil(t, result.Count)
	require.Equal(t, 236, result.Count.Count)
	require.Equal(t, "Dark Star", result.Count.SongName)
}

func TestExecutor_FirstLastQuery(t *testing.T) {
	ds := &mock.DataSource{}
	ds.ExecuteQueryFunc = func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
		return &data.ResultSet{
			Columns: []string{"id", "date", "venue_id", "venue", "city", "state", "notes", "rating"},
			Rows:    []data.Row{{1, "1967-11-14", 1, "American Studios", "North Hollywood", "CA", "", 0.0}},
		}, nil
	}
	ds.GetSongFunc = func(ctx context.Context, name string) (*data.Song, error) {
		return &data.Song{ID: 10, Name: "Dark Star"}, nil
	}
	ex := New(ds)
	q := &ast.FirstLastQuery{Song: &ast.SongRef{Name: "Dark Star"}, IsLast: false}
	result, err := ex.ExecuteAST(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ResultShows, result.Type, "FIRST/LAST returns a single show")
	require.Len(t, result.Shows, 1)
}

func TestExecutor_SongsAsCount(t *testing.T) {
	ds := &mock.DataSource{}
	ds.ExecuteQueryFunc = func(ctx context.Context, sql string, args ...interface{}) (*data.ResultSet, error) {
		return &data.ResultSet{
			Columns: []string{"count", "name"},
			Rows:    []data.Row{{int64(23), "songs"}},
		}, nil
	}
	ex := New(ds)
	q := &ast.SongQuery{
		With: &ast.WithClause{
			Conditions: []ast.WithCondition{
				&ast.LyricsCondition{Words: []string{"sun"}},
			},
		},
		OutputFmt: ast.OutputCount,
	}
	result, err := ex.ExecuteAST(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ResultCount, result.Type)
	require.Equal(t, 23, result.Count.Count)
}

// === Helper functions ===

func TestIntVal(t *testing.T) {
	require.Equal(t, 5, intVal(5))
	require.Equal(t, 5, intVal(int64(5)))
	require.Equal(t, 5, intVal(5.0))
	require.Equal(t, 42, intVal("42"))
	require.Equal(t, 0, intVal("not a number"))
	require.Equal(t, 0, intVal(nil))
	require.Equal(t, 0, intVal(struct{}{}))
}

func TestFloatVal(t *testing.T) {
	require.Equal(t, 4.9, floatVal(4.9))
	require.Equal(t, 5.0, floatVal(5))
	require.Equal(t, 5.0, floatVal(int64(5)))
	require.Equal(t, 0.0, floatVal(nil))
}

func TestStrVal(t *testing.T) {
	require.Equal(t, "hello", strVal("hello"))
	require.Equal(t, "", strVal(nil))
	require.Equal(t, "", strVal(42))
}

func TestTimeVal(t *testing.T) {
	tm := timeVal("1977-05-08")
	require.Equal(t, 1977, tm.Year())
	require.Equal(t, 5, int(tm.Month()))
	require.Equal(t, 8, tm.Day())

	zero := timeVal("not a date")
	require.True(t, zero.IsZero())

	zero = timeVal(nil)
	require.True(t, zero.IsZero())
}

func TestMapRowsToCount(t *testing.T) {
	rs := &data.ResultSet{
		Columns: []string{"count", "name"},
		Rows:    []data.Row{{int64(42), "Dark Star"}},
	}
	cr := mapRowsToCount(rs)
	require.NotNil(t, cr)
	require.Equal(t, 42, cr.Count)
	require.Equal(t, "Dark Star", cr.SongName)
}

func TestMapRowsToCount_Empty(t *testing.T) {
	rs := &data.ResultSet{Rows: nil}
	cr := mapRowsToCount(rs)
	require.NotNil(t, cr)
	require.Equal(t, 0, cr.Count)
}

func TestNormalizeSongName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"I KNOW YOU RIDER", "I Know You Rider"},
		{"FIRE ON THE MOUNTAIN", "Fire on the Mountain"},
		{"Dark Star", "Dark Star"},           // mixed case: unchanged
		{"", ""},                              // empty
		{"HELP ON THE WAY", "Help on the Way"},
		{"THE OTHER ONE", "The Other One"},    // "the" at start is capitalized
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeSongName(tt.input))
		})
	}
}
