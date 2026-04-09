package sqlite

import (
	"context"
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestOpen_Close(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
}

func TestExecuteQuery_Simple(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	rs, err := db.ExecuteQuery(ctx, "SELECT id, date FROM shows LIMIT 1")
	require.NoError(t, err)
	require.NotEmpty(t, rs.Columns)
	require.Equal(t, []string{"id", "date"}, rs.Columns)
	require.GreaterOrEqual(t, len(rs.Rows), 1)
}

func TestExecuteQuery_Parameterized(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	rs, err := db.ExecuteQuery(ctx, "SELECT id, date FROM shows WHERE date >= ? AND date <= ?", "1977-01-01", "1977-12-31")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rs.Rows), 1)
}

func TestGetSong_ByName(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	song, err := db.GetSong(ctx, "Scarlet Begonias")
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, 1, song.ID)
	require.Equal(t, "Scarlet Begonias", song.Name)
}

func TestGetSong_ViaAlias(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	// minimal_data has alias "Scarlet Begonias-" -> song 1 (Scarlet Begonias)
	song, err := db.GetSong(ctx, "Scarlet Begonias-")
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, 1, song.ID)
	require.Equal(t, "Scarlet Begonias", song.Name)
}

func TestGetSong_FuzzyPunctuation(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// "Help on the Way" in fixture — test without "the"... actually test punctuation
	// Fixture has "Fire on the Mountain" — try "Fire On The Mountain" (case)
	song, err := db.GetSong(ctx, "Fire On The Mountain")
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, 2, song.ID)

	// Fixture has "Help on the Way" — try without punctuation variations
	// The fixture doesn't have apostrophe songs, but we can test normalizeName directly
}

func TestNormalizeName(t *testing.T) {
	require.Equal(t, "franklins tower", normalizeName("Franklin's Tower"))
	require.Equal(t, "franklins tower", normalizeName("Franklins Tower"))
	require.Equal(t, "truckin", normalizeName("Truckin'"))
	require.Equal(t, "truckin", normalizeName("Truckin"))
	require.Equal(t, "st stephen", normalizeName("St. Stephen"))
	require.Equal(t, "st stephen", normalizeName("St Stephen"))
	require.Equal(t, "us blues", normalizeName("U.S. Blues"))
	require.Equal(t, "fire on the mountain", normalizeName("Fire on the Mountain"))
	require.Equal(t, "fire on the mountain", normalizeName("Fire On The Mountain"))
	require.Equal(t, "good lovin", normalizeName("Good Lovin'"))
	require.Equal(t, "good lovin", normalizeName("Good Lovin"))
	require.Equal(t, "", normalizeName(""))
}

func TestGetSong_NotFound(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	song, err := db.GetSong(ctx, "Nonexistent Song XYZ")
	require.NoError(t, err)
	require.Nil(t, song)
}

func TestGetSongByID(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	song, err := db.GetSongByID(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, 2, song.ID)
	require.Equal(t, "Fire on the Mountain", song.Name)
}

func TestSearchSongs(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()
	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	songs, err := db.SearchSongs(ctx, "Scarlet")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(songs), 1)
	require.Contains(t, songs[0].Name, "Scarlet")
}
