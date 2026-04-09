package sqlgen

import (
	"context"
	"testing"
	"time"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/ir"
	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func openDB(t *testing.T) *sqlite.DB {
	t.Helper()
	path, cleanup := fixtures.CreateTestDB(t)
	t.Cleanup(cleanup)
	db, err := sqlite.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func execQuery(t *testing.T, db *sqlite.DB, q *ir.QueryIR) int {
	t.Helper()
	g := New()
	sq, err := g.Generate(q)
	require.NoError(t, err)
	rs, err := db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	return len(rs.Rows)
}

func TestGenerate_Shows_Simple(t *testing.T) {
	db := openDB(t)
	// Fixture has 3 shows total
	rows := execQuery(t, db, &ir.QueryIR{Type: ir.QueryTypeShows})
	require.Equal(t, 3, rows)
}

func TestGenerate_Shows_WithDateRange(t *testing.T) {
	db := openDB(t)
	start := time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC)
	rows := execQuery(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeShows,
		DateRange: &ir.ResolvedDateRange{Start: start, End: end},
	})
	require.Equal(t, 2, rows, "fixture has 2 shows in 1977")
}

func TestGenerate_Shows_WithLimit(t *testing.T) {
	db := openDB(t)
	lim := 1
	rows := execQuery(t, db, &ir.QueryIR{Type: ir.QueryTypeShows, Limit: &lim})
	require.Equal(t, 1, rows)
}

func TestGenerate_Shows_WithSegue(t *testing.T) {
	db := openDB(t)
	// Scarlet (1) > Fire (2) — fixture has 3 shows with this adjacency
	rows := execQuery(t, db, &ir.QueryIR{
		Type: ir.QueryTypeShows,
		SegueChain: &ir.SegueChainIR{
			SongIDs:   []int{1, 2},
			Operators: []ir.SegueOp{ir.SegueOpSegue},
		},
	})
	require.Equal(t, 3, rows, "fixture has Scarlet > Fire at Cornell, Winterland, Landover")
}

func TestGenerate_Shows_WithVenue(t *testing.T) {
	db := openDB(t)
	rows := execQuery(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeShows,
		VenueName: "Barton",
	})
	require.Equal(t, 1, rows, "only Cornell is at Barton Hall")
}

func TestGenerate_Performances(t *testing.T) {
	db := openDB(t)
	songID := 6 // Dark Star
	rows := execQuery(t, db, &ir.QueryIR{
		Type:   ir.QueryTypePerformances,
		SongID: &songID,
	})
	require.Equal(t, 2, rows, "fixture has 2 Dark Star performances")
}

func TestGenerate_Setlist(t *testing.T) {
	db := openDB(t)
	d := time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC)
	rows := execQuery(t, db, &ir.QueryIR{
		Type:       ir.QueryTypeSetlist,
		SingleDate: &d,
	})
	require.GreaterOrEqual(t, rows, 5, "Cornell 77 setlist has at least 5 songs in fixture")
}

func TestGenerate_Songs_WithLyrics(t *testing.T) {
	db := openDB(t)
	rows := execQuery(t, db, &ir.QueryIR{
		Type: ir.QueryTypeSongs,
		Conditions: []ir.ConditionIR{
			&ir.LyricsConditionIR{Words: []string{"walkin"}, Operator: ir.OpAnd},
		},
	})
	require.Equal(t, 1, rows, "only Scarlet Begonias has 'walkin' in lyrics")
}
