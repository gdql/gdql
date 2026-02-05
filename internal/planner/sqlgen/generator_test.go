package sqlgen

import (
	"testing"
	"time"

	"github.com/gdql/gdql/internal/ir"
	"github.com/stretchr/testify/require"
)

func TestGenerate_Shows_Simple(t *testing.T) {
	g := New()
	q := &ir.QueryIR{Type: ir.QueryTypeShows}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "SELECT")
	require.Contains(t, sql.SQL, "shows")
	require.Contains(t, sql.SQL, "venues")
	require.Empty(t, sql.Args)
}

func TestGenerate_Shows_WithDateRange(t *testing.T) {
	g := New()
	start := time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(1980, 12, 31, 23, 59, 59, 0, time.UTC)
	q := &ir.QueryIR{
		Type:      ir.QueryTypeShows,
		DateRange: &ir.ResolvedDateRange{Start: start, End: end},
	}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "s.date >= ? AND s.date <= ?")
	require.Len(t, sql.Args, 2)
	require.Equal(t, "1977-01-01", sql.Args[0])
	require.Equal(t, "1980-12-31", sql.Args[1])
}

func TestGenerate_Shows_WithLimit(t *testing.T) {
	g := New()
	lim := 5
	q := &ir.QueryIR{Type: ir.QueryTypeShows, Limit: &lim}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "LIMIT ?")
	require.Len(t, sql.Args, 1)
	require.Equal(t, 5, sql.Args[0])
}

func TestGenerate_Shows_WithSegue(t *testing.T) {
	g := New()
	q := &ir.QueryIR{
		Type: ir.QueryTypeShows,
		SegueChain: &ir.SegueChainIR{
			SongIDs:   []int{1, 2},
			Operators: []ir.SegueOp{ir.SegueOpSegue},
		},
	}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "SELECT DISTINCT")
	require.Contains(t, sql.SQL, "p1")
	require.Contains(t, sql.SQL, "p2")
	require.Contains(t, sql.SQL, "segue_type")
	require.Len(t, sql.Args, 3) // s1.id, s2.id, p1.segue_type
}

func TestGenerate_Performances(t *testing.T) {
	g := New()
	songID := 10
	q := &ir.QueryIR{
		Type:   ir.QueryTypePerformances,
		SongID: &songID,
	}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "performances")
	require.Contains(t, sql.SQL, "p.song_id = ?")
	require.Len(t, sql.Args, 1)
	require.Equal(t, 10, sql.Args[0])
}

func TestGenerate_Setlist(t *testing.T) {
	g := New()
	d := time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC)
	q := &ir.QueryIR{
		Type:       ir.QueryTypeSetlist,
		SingleDate: &d,
	}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "s.date = ?")
	require.Len(t, sql.Args, 1)
	require.Equal(t, "1977-05-08", sql.Args[0])
}

func TestGenerate_Songs_WithLyrics(t *testing.T) {
	g := New()
	q := &ir.QueryIR{
		Type: ir.QueryTypeSongs,
		Conditions: []ir.ConditionIR{
			&ir.LyricsConditionIR{Words: []string{"train", "road"}, Operator: ir.OpAnd},
		},
	}
	sql, err := g.Generate(q)
	require.NoError(t, err)
	require.Contains(t, sql.SQL, "lyrics")
	require.Contains(t, sql.SQL, "LIKE")
	require.Len(t, sql.Args, 2)
}
