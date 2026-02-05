package canonical

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestWriteShows_ResolvesVariantAndAddsAlias(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	// Fixture has song 1 "Scarlet Begonias". Import a show with source name "Scarlet Begonias-" (trailing dash).
	shows := []Show{
		{
			Date: "1980-05-15",
			Venue: Venue{Name: "Sportatorium", City: "Pembroke Pines", State: "FL", Country: "USA"},
			Sets: []Set{
				{Songs: []SongInSet{{Name: "Scarlet Begonias-", SegueBefore: false}}},
			},
		},
	}

	showsAdded, songsAdded, err := WriteShows(ctx, conn, shows)
	require.NoError(t, err)
	require.Equal(t, 1, showsAdded)
	require.Equal(t, 0, songsAdded, "variant resolved to existing song, no new song row")

	var aliasCount int
	err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM song_aliases WHERE alias = ? AND song_id = 1", "Scarlet Begonias-").Scan(&aliasCount)
	require.NoError(t, err)
	require.Equal(t, 1, aliasCount, "import should have inserted alias for raw variant")

	// Performance should reference song_id 1 (Scarlet Begonias), not a new song
	var songID int
	err = conn.QueryRowContext(ctx, "SELECT song_id FROM performances WHERE show_id = (SELECT id FROM shows WHERE date = '1980-05-15') LIMIT 1").Scan(&songID)
	require.NoError(t, err)
	require.Equal(t, 1, songID)
}

func TestWriteShows_NewSongStoredWithRawName(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	shows := []Show{
		{
			Date:  "1981-08-10",
			Venue: Venue{Name: "Rainbow Theatre", City: "London", State: "", Country: "UK"},
			Sets: []Set{
				{Songs: []SongInSet{{Name: "Unknown Song XYZ", SegueBefore: false}}},
			},
		},
	}

	showsAdded, songsAdded, err := WriteShows(ctx, conn, shows)
	require.NoError(t, err)
	require.Equal(t, 1, showsAdded)
	require.Equal(t, 1, songsAdded)

	var name string
	err = conn.QueryRowContext(ctx, "SELECT name FROM songs WHERE name = ?", "Unknown Song XYZ").Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "Unknown Song XYZ", name)
}
