package acceptance

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

// openTestDB creates a fixture DB and returns a *sqlite.DB ready for executor use.
func openTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	path, cleanup := fixtures.CreateTestDB(t)
	t.Cleanup(cleanup)
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Example query strings (same as query.gdql and examples/performances-dark-star.gdql).
const (
	queryScarletFire  = `SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain"`
	queryDarkStarPerf = `PERFORMANCES OF "Dark Star" FROM 1977 WITH LENGTH > 20min LIMIT 5`
)

func TestE2E_ShowsFromYear(t *testing.T) {
	db := openTestDB(t)
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
	db := openTestDB(t)
	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), `SHOWS FROM 1977-1978 WHERE "Scarlet Begonias" > "Fire on the Mountain"`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 3, "seed has Scarlet > Fire at Cornell, Winterland, Landover")
}

func TestE2E_PerformancesDarkStar(t *testing.T) {
	db := openTestDB(t)
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
	db := openTestDB(t)
	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), "SETLIST FOR 5/8/77")
	require.NoError(t, err)
	require.Equal(t, executor.ResultSetlist, result.Type)
	require.NotNil(t, result.Setlist)
	require.Equal(t, "1977-05-08", result.Setlist.Date.Format("2006-01-02"))
	require.GreaterOrEqual(t, len(result.Setlist.Performances), 5, "Cornell 77 set 2 has Scarlet, Fire, Help, Samson, Dew")
}

func TestE2E_SongsWithLyrics(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), `SONGS WITH LYRICS("walkin")`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultSongs, result.Type)
	require.GreaterOrEqual(t, len(result.Songs), 1, "Scarlet Begonias lyrics contain 'walkin'")
	require.Contains(t, result.Songs[0].Name, "Scarlet")
}

// TestE2E_SegueWorksWithoutSegueMetadata verifies that "A" > "B" matches by
// positional adjacency even when segue_type is empty (as with setlist.fm imports).
func TestE2E_SegueWorksWithoutSegueMetadata(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)

	// Create schema and insert data with empty segue_type (like setlist.fm)
	_, err = db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS venues (id INTEGER PRIMARY KEY, name TEXT, city TEXT, state TEXT, country TEXT);
		CREATE TABLE IF NOT EXISTS shows (id INTEGER PRIMARY KEY, date TEXT, venue_id INTEGER, tour TEXT, notes TEXT, rating REAL);
		CREATE TABLE IF NOT EXISTS songs (id INTEGER PRIMARY KEY, name TEXT, short_name TEXT, writers TEXT, first_played TEXT, last_played TEXT, times_played INTEGER DEFAULT 0);
		CREATE TABLE IF NOT EXISTS performances (id INTEGER PRIMARY KEY, show_id INTEGER, song_id INTEGER, set_number INTEGER, position INTEGER, segue_type TEXT, length_seconds INTEGER DEFAULT 0, is_opener INTEGER DEFAULT 0, is_closer INTEGER DEFAULT 0, guest TEXT);
		CREATE TABLE IF NOT EXISTS lyrics (song_id INTEGER PRIMARY KEY, lyrics TEXT, lyrics_fts TEXT);

		INSERT INTO venues (id, name, city, state) VALUES (1, 'Winterland', 'San Francisco', 'CA');
		INSERT INTO shows (id, date, venue_id) VALUES (1, '1977-12-31', 1);
		INSERT INTO songs (id, name) VALUES (1, 'Scarlet Begonias'), (2, 'Fire on the Mountain');
		INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type) VALUES
			(1, 1, 1, 1, 1, ''),
			(2, 1, 2, 1, 2, '');
	`)
	require.NoError(t, err)
	db.Close()

	db2, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	ex := executor.New(db2)
	result, err := ex.Execute(context.Background(), `SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain"`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 1, "should find show by adjacency even without segue_type metadata")
	require.Equal(t, "1977-12-31", result.Shows[0].Date.Format("2006-01-02"))
}

func TestE2E_ShowsAtVenue(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	// "Barton Hall" is the venue for Cornell 77
	result, err := ex.Execute(context.Background(), `SHOWS AT "Barton Hall"`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 1)
	require.Equal(t, "1977-05-08", result.Shows[0].Date.Format("2006-01-02"))

	// AT with FROM
	result, err = ex.Execute(context.Background(), `SHOWS AT "Winterland" FROM 1977`)
	require.NoError(t, err)
	require.Len(t, result.Shows, 1)
	require.Equal(t, "1977-02-26", result.Shows[0].Date.Format("2006-01-02"))

	// No match
	result, err = ex.Execute(context.Background(), `SHOWS AT "Madison Square Garden"`)
	require.NoError(t, err)
	require.Len(t, result.Shows, 0)
}

// TestE2E_ExampleQueryFile runs the same query as query.gdql (Scarlet > Fire, 1977 only).
func TestE2E_ExampleQueryFile(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), queryScarletFire)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 2, "query.gdql is FROM 1977; fixture has 2 shows in 1977 (Cornell, Winterland)")
}

// TestE2E_ExamplePerformancesDarkStarFile runs the same query as examples/performances-dark-star.gdql.
func TestE2E_ExamplePerformancesDarkStarFile(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), queryDarkStarPerf)
	require.NoError(t, err)
	require.Equal(t, executor.ResultPerformances, result.Type)
	require.GreaterOrEqual(t, len(result.Performances), 1)
}
