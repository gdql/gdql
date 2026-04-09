package planner

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/ir"
	"github.com/gdql/gdql/internal/planner/expander"
	"github.com/gdql/gdql/internal/planner/resolver"
	"github.com/stretchr/testify/require"
)

func TestPlan_ShowQuery_Simple(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{"Scarlet Begonias": 1, "Fire on the Mountain": 2})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.ShowQuery{
		From: &ast.DateRange{Start: &ast.Date{Year: 1977}, End: &ast.Date{Year: 1980}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeShows, got.Type)
	require.NotNil(t, got.DateRange)
	require.Equal(t, 1977, got.DateRange.Start.Year())
	require.Equal(t, 1980, got.DateRange.End.Year())
}

func TestPlan_ShowQuery_WithSegue(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{
		"Scarlet Begonias":      1,
		"Fire on the Mountain": 2,
	})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.ShowQuery{
		Where: &ast.WhereClause{
			Conditions: []ast.Condition{
				&ast.SegueCondition{
					Songs:     []*ast.SongRef{{Name: "Scarlet Begonias"}, {Name: "Fire on the Mountain"}},
					Operators: []ast.SegueOp{ast.SegueOpSegue},
				},
			},
		},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeShows, got.Type)
	require.NotNil(t, got.SegueChain)
	require.Equal(t, []int{1, 2}, got.SegueChain.SongIDs)
	require.Len(t, got.SegueChain.Operators, 1)
	require.Equal(t, ir.SegueOpSegue, got.SegueChain.Operators[0])
}

func TestPlan_ShowQuery_WherePosition(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{"Samson and Delilah": 5})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.ShowQuery{
		Where: &ast.WhereClause{
			Conditions: []ast.Condition{
				&ast.PositionCondition{
					Set:      ast.Set2,
					Operator: ast.PosOpened,
					Song:     &ast.SongRef{Name: "Samson and Delilah"},
				},
			},
		},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeShows, got.Type)
	require.Len(t, got.Conditions, 1)
	pos, ok := got.Conditions[0].(*ir.PositionConditionIR)
	require.True(t, ok)
	require.Equal(t, ir.Set2, pos.Set)
	require.Equal(t, ir.PosOpened, pos.Operator)
	require.Equal(t, 5, pos.SongID)
}

func TestPlan_PerformanceQuery(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{"Dark Star": 10})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.PerformanceQuery{
		Song: &ast.SongRef{Name: "Dark Star"},
		From: &ast.DateRange{Start: &ast.Date{Year: 1972}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypePerformances, got.Type)
	require.NotNil(t, got.SongID)
	require.Equal(t, 10, *got.SongID)
	require.NotNil(t, got.DateRange)
	require.Equal(t, 1972, got.DateRange.Start.Year())
}

func TestPlan_SetlistQuery(t *testing.T) {
	de := expander.New()
	pl := New(resolver.NewStaticResolver(nil), de)

	q := &ast.SetlistQuery{
		Date: &ast.Date{Year: 1977, Month: 5, Day: 8},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeSetlist, got.Type)
	require.NotNil(t, got.SingleDate)
	require.Equal(t, 1977, got.SingleDate.Year())
	require.Equal(t, 5, int(got.SingleDate.Month()))
	require.Equal(t, 8, got.SingleDate.Day())
}

func TestPlan_SongQuery_WithLyrics(t *testing.T) {
	pl := New(resolver.NewStaticResolver(nil), expander.New())
	q := &ast.SongQuery{
		With: &ast.WithClause{
			Conditions: []ast.WithCondition{
				&ast.LyricsCondition{Words: []string{"train", "road"}, Operator: ast.OpAnd},
			},
		},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeSongs, got.Type)
	require.Len(t, got.Conditions, 1)
	lyr, ok := got.Conditions[0].(*ir.LyricsConditionIR)
	require.True(t, ok)
	require.Equal(t, []string{"train", "road"}, lyr.Words)
}

func TestPlan_ShowQuery_UnknownSong_ReturnsError(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.ShowQuery{
		Where: &ast.WhereClause{
			Conditions: []ast.Condition{
				&ast.PlayedCondition{Song: &ast.SongRef{Name: "Nonexistent Song"}},
			},
		},
	}
	_, err := pl.Plan(context.Background(), q)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestPlan_UnknownSong_IncludesDidYouMean(t *testing.T) {
	sr := resolver.NewStaticResolver(map[string]int{"Scarlet Begonias": 1, "Fire on the Mountain": 2})
	de := expander.New()
	pl := New(sr, de)

	q := &ast.ShowQuery{
		Where: &ast.WhereClause{
			Conditions: []ast.Condition{
				&ast.PlayedCondition{Song: &ast.SongRef{Name: "Scarlet Begonia"}},
			},
		},
	}
	_, err := pl.Plan(context.Background(), q)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Did you mean:")
	require.Contains(t, err.Error(), "Scarlet Begonias")
}

// === New query types ===

func newPlanner(songs map[string]int) Planner {
	return New(resolver.NewStaticResolver(songs), expander.New())
}

func TestPlan_ShowQuery_AtVenue(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.ShowQuery{At: "Fillmore West"}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, "Fillmore West", got.VenueName)
}

func TestPlan_ShowQuery_Tour(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.ShowQuery{Tour: "Spring 1977"}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, "Spring 1977", got.TourName)
}

func TestPlan_ShowQuery_AtAndTourAndFrom(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.ShowQuery{
		At:   "Winterland",
		Tour: "Spring 1977",
		From: &ast.DateRange{Start: &ast.Date{Year: 1977}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, "Winterland", got.VenueName)
	require.Equal(t, "Spring 1977", got.TourName)
	require.NotNil(t, got.DateRange)
}

func TestPlan_CountQuery_Song(t *testing.T) {
	pl := newPlanner(map[string]int{"Dark Star": 10})
	q := &ast.CountQuery{Song: &ast.SongRef{Name: "Dark Star"}}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeCount, got.Type)
	require.NotNil(t, got.SongID)
	require.Equal(t, 10, *got.SongID)
}

func TestPlan_CountQuery_SongWithRange(t *testing.T) {
	pl := newPlanner(map[string]int{"Dark Star": 10})
	q := &ast.CountQuery{
		Song: &ast.SongRef{Name: "Dark Star"},
		From: &ast.DateRange{Start: &ast.Date{Year: 1972}, End: &ast.Date{Year: 1974}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.NotNil(t, got.DateRange)
	require.Equal(t, 1972, got.DateRange.Start.Year())
	require.Equal(t, 1974, got.DateRange.End.Year())
}

func TestPlan_CountQuery_Shows(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.CountQuery{
		CountShows: true,
		From:       &ast.DateRange{Start: &ast.Date{Year: 1977}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeCount, got.Type)
	require.Nil(t, got.SongID, "COUNT SHOWS should not set SongID")
	require.NotNil(t, got.DateRange)
}

func TestPlan_CountQuery_UnknownSong(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.CountQuery{Song: &ast.SongRef{Name: "Nonexistent"}}
	_, err := pl.Plan(context.Background(), q)
	require.Error(t, err)
}

func TestPlan_FirstLastQuery(t *testing.T) {
	pl := newPlanner(map[string]int{"Dark Star": 10})

	first := &ast.FirstLastQuery{Song: &ast.SongRef{Name: "Dark Star"}, IsLast: false}
	got, err := pl.Plan(context.Background(), first)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeFirstLast, got.Type)
	require.False(t, got.IsLast)
	require.Equal(t, 10, *got.SongID)

	last := &ast.FirstLastQuery{Song: &ast.SongRef{Name: "Dark Star"}, IsLast: true}
	got, err = pl.Plan(context.Background(), last)
	require.NoError(t, err)
	require.True(t, got.IsLast)
}

func TestPlan_RandomShowQuery(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.RandomShowQuery{}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.QueryTypeRandomShow, got.Type)
}

func TestPlan_RandomShowQuery_WithRange(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.RandomShowQuery{
		From: &ast.DateRange{Start: &ast.Date{Year: 1977}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.NotNil(t, got.DateRange)
	require.Equal(t, 1977, got.DateRange.Start.Year())
}

func TestPlan_SongQuery_AsCount(t *testing.T) {
	pl := newPlanner(nil)
	q := &ast.SongQuery{
		With: &ast.WithClause{
			Conditions: []ast.WithCondition{
				&ast.LyricsCondition{Words: []string{"sun"}},
			},
		},
		OutputFmt: ast.OutputCount,
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, ir.OutputCount, got.OutputFmt)
}

func TestPlan_PerformanceQuery_BeforeAfter(t *testing.T) {
	pl := newPlanner(map[string]int{"Dark Star": 10})

	// AFTER 1988 → start=1988, end=2100
	q := &ast.PerformanceQuery{
		Song: &ast.SongRef{Name: "Dark Star"},
		From: &ast.DateRange{Start: &ast.Date{Year: 1988}, End: &ast.Date{Year: 2100}},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.NotNil(t, got.DateRange)
	require.Equal(t, 1988, got.DateRange.Start.Year())
	require.Equal(t, 2100, got.DateRange.End.Year())
}

func TestPlan_OpenerCondition(t *testing.T) {
	pl := newPlanner(map[string]int{"Bertha": 7})
	q := &ast.ShowQuery{
		Where: &ast.WhereClause{
			Conditions: []ast.Condition{
				&ast.PositionCondition{
					Set:      ast.SetAny,
					Operator: ast.PosOpened,
					Song:     &ast.SongRef{Name: "Bertha"},
				},
			},
		},
	}
	got, err := pl.Plan(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, got.Conditions, 1)
	pc := got.Conditions[0].(*ir.PositionConditionIR)
	require.Equal(t, ir.SetAny, pc.Set)
	require.Equal(t, ir.PosOpened, pc.Operator)
	require.Equal(t, 7, pc.SongID)
}
