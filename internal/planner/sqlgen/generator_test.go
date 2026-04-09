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

// === COUNT ===

func execScalar(t *testing.T, db *sqlite.DB, q *ir.QueryIR) (int, string) {
	t.Helper()
	g := New()
	sq, err := g.Generate(q)
	require.NoError(t, err)
	rs, err := db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	row := rs.Rows[0]
	var count int
	switch v := row[0].(type) {
	case int64:
		count = int(v)
	case int:
		count = v
	}
	var name string
	if len(row) >= 2 {
		if s, ok := row[1].(string); ok {
			name = s
		}
	}
	return count, name
}

func TestGenerate_Count_Song(t *testing.T) {
	db := openDB(t)
	songID := 1 // Scarlet Begonias
	count, name := execScalar(t, db, &ir.QueryIR{
		Type:   ir.QueryTypeCount,
		SongID: &songID,
	})
	require.Equal(t, 3, count, "fixture has 3 Scarlet Begonias performances")
	require.Equal(t, "Scarlet Begonias", name)
}

func TestGenerate_Count_SongWithRange(t *testing.T) {
	db := openDB(t)
	songID := 1
	start := time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC)
	count, _ := execScalar(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeCount,
		SongID:    &songID,
		DateRange: &ir.ResolvedDateRange{Start: start, End: end},
	})
	require.Equal(t, 2, count, "Scarlet appears in 2 1977 shows in fixture")
}

func TestGenerate_Count_Shows(t *testing.T) {
	db := openDB(t)
	count, name := execScalar(t, db, &ir.QueryIR{
		Type: ir.QueryTypeCount,
		// SongID nil → COUNT SHOWS
	})
	require.Equal(t, 3, count, "fixture has 3 shows")
	require.Equal(t, "shows", name)
}

func TestGenerate_Count_ShowsWithRange(t *testing.T) {
	db := openDB(t)
	start := time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC)
	count, _ := execScalar(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeCount,
		DateRange: &ir.ResolvedDateRange{Start: start, End: end},
	})
	require.Equal(t, 2, count, "fixture has 2 shows in 1977")
}

// === FIRST/LAST ===

func TestGenerate_FirstLast(t *testing.T) {
	db := openDB(t)
	songID := 1 // Scarlet Begonias

	// FIRST: earliest date
	g := New()
	sq, err := g.Generate(&ir.QueryIR{
		Type:   ir.QueryTypeFirstLast,
		SongID: &songID,
		IsLast: false,
	})
	require.NoError(t, err)
	rs, err := db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	require.Equal(t, "1977-02-26", rs.Rows[0][1], "first Scarlet was Winterland 2/26/77")

	// LAST: latest date
	sq, err = g.Generate(&ir.QueryIR{
		Type:   ir.QueryTypeFirstLast,
		SongID: &songID,
		IsLast: true,
	})
	require.NoError(t, err)
	rs, err = db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	require.Equal(t, "1978-04-24", rs.Rows[0][1], "last Scarlet was Landover 4/24/78")
}

// === RANDOM SHOW ===

func TestGenerate_RandomShow(t *testing.T) {
	db := openDB(t)
	g := New()
	sq, err := g.Generate(&ir.QueryIR{Type: ir.QueryTypeRandomShow})
	require.NoError(t, err)
	rs, err := db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	require.Greater(t, len(rs.Rows), 0, "random show should return at least 1 performance")
}

func TestGenerate_RandomShow_WithRange(t *testing.T) {
	db := openDB(t)
	g := New()
	start := time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC)
	sq, err := g.Generate(&ir.QueryIR{
		Type:      ir.QueryTypeRandomShow,
		DateRange: &ir.ResolvedDateRange{Start: start, End: end},
	})
	require.NoError(t, err)
	rs, err := db.ExecuteQuery(context.Background(), sq.SQL, sq.Args...)
	require.NoError(t, err)
	require.Greater(t, len(rs.Rows), 0)
}

// === AT venue ===

func TestGenerate_Shows_AtVenuePartialMatch(t *testing.T) {
	db := openDB(t)
	rows := execQuery(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeShows,
		VenueName: "Bart", // Should match "Barton Hall"
	})
	require.Equal(t, 1, rows)
}

func TestGenerate_Shows_AtCity(t *testing.T) {
	db := openDB(t)
	rows := execQuery(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeShows,
		VenueName: "San Francisco", // Should match Winterland's city
	})
	require.GreaterOrEqual(t, rows, 1)
}

// === OPENER/CLOSER (any set) ===

func TestGenerate_Shows_OpenerAnySet(t *testing.T) {
	db := openDB(t)
	songID := 4 // Samson — fixture has Cornell set 2 pos 1 as opener AND Landover set 2 pos 1 as opener
	rows := execQuery(t, db, &ir.QueryIR{
		Type: ir.QueryTypeShows,
		Conditions: []ir.ConditionIR{
			&ir.PositionConditionIR{
				Set:      ir.SetAny,
				Operator: ir.PosOpened,
				SongID:   songID,
			},
		},
	})
	require.GreaterOrEqual(t, rows, 1, "Samson opens at least one set in fixture")
}

// === AS COUNT for SONGS ===

func TestGenerate_Songs_AsCount(t *testing.T) {
	db := openDB(t)
	count, _ := execScalar(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeSongs,
		OutputFmt: ir.OutputCount,
	})
	require.Equal(t, 6, count, "fixture has 6 songs")
}

func TestGenerate_Songs_AsCountWithLyricsFilter(t *testing.T) {
	db := openDB(t)
	count, _ := execScalar(t, db, &ir.QueryIR{
		Type:      ir.QueryTypeSongs,
		OutputFmt: ir.OutputCount,
		Conditions: []ir.ConditionIR{
			&ir.LyricsConditionIR{Words: []string{"walkin"}, Operator: ir.OpAnd},
		},
	})
	require.Equal(t, 1, count, "only Scarlet has 'walkin'")
}
