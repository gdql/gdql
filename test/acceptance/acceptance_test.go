package acceptance

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

// Example query strings (same as query.gdql and examples/performances-dark-star.gdql).
const (
	queryScarletFire  = `SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain"`
	queryDarkStarPerf = `PERFORMANCES OF "Dark Star" FROM 1977 WITH LENGTH > 20min LIMIT 5`
)

func TestE2E_ShowsFromYear(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), "SHOWS FROM 1977 LIMIT 5")
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 2, "seed has 2 shows in 1977 (Winterland, Cornell)")
	dates := make([]string, len(result.Shows))
	for i, s := range result.Shows {
		dates[i] = s.Date.Format("2006-01-02")
	}
	require.Contains(t, dates, "1977-02-26")
	require.Contains(t, dates, "1977-05-08")
}

func TestE2E_SegueScarletFire(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), `SHOWS FROM 1977-1978 WHERE "Scarlet Begonias" > "Fire on the Mountain"`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 3, "seed has Scarlet > Fire at Cornell, Winterland, Landover")
}

func TestE2E_PerformancesDarkStar(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), queryDarkStarPerf)
	require.NoError(t, err)
	require.Equal(t, executor.ResultPerformances, result.Type)
	require.GreaterOrEqual(t, len(result.Performances), 1, "seed has Dark Star with length > 20min at Cornell and Winterland 77")
	for _, p := range result.Performances {
		require.GreaterOrEqual(t, p.LengthSeconds, 1200, "WITH LENGTH > 20min implies >= 1200 seconds")
	}
}

func TestE2E_SetlistForDate(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), "SETLIST FOR 5/8/77")
	require.NoError(t, err)
	require.Equal(t, executor.ResultSetlist, result.Type)
	require.NotNil(t, result.Setlist)
	require.Equal(t, "1977-05-08", result.Setlist.Date.Format("2006-01-02"))
	require.GreaterOrEqual(t, len(result.Setlist.Performances), 5, "Cornell 77 set 2 has Scarlet, Fire, Help, Samson, Dew")
}

func TestE2E_SongsWithLyrics(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), `SONGS WITH LYRICS("walkin")`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultSongs, result.Type)
	require.GreaterOrEqual(t, len(result.Songs), 1, "Scarlet Begonias lyrics contain 'walkin'")
	require.Contains(t, result.Songs[0].Name, "Scarlet")
}

// TestE2E_ExampleQueryFile runs the same query as query.gdql (Scarlet > Fire, 1977 only).
func TestE2E_ExampleQueryFile(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), queryScarletFire)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 2, "query.gdql is FROM 1977; fixture has 2 shows in 1977 (Cornell, Winterland)")
}

// TestE2E_ExamplePerformancesDarkStarFile runs the same query as examples/performances-dark-star.gdql.
func TestE2E_ExamplePerformancesDarkStarFile(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), queryDarkStarPerf)
	require.NoError(t, err)
	require.Equal(t, executor.ResultPerformances, result.Type)
	require.GreaterOrEqual(t, len(result.Performances), 1)
}
