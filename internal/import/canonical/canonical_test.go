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

func TestWriteShows_PreservesSetBoundaries(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	shows := []Show{
		{
			Date:  "1977-05-08",
			Venue: Venue{Name: "Test Hall", City: "Ithaca", State: "NY", Country: "USA"},
			Sets: []Set{
				{Songs: []SongInSet{
					{Name: "Minglewood Blues"},
					{Name: "Loser"},
				}},
				{Songs: []SongInSet{
					{Name: "Estimated Prophet"},
					{Name: "Eyes of the World"},
				}},
				{Songs: []SongInSet{
					{Name: "One More Saturday Night"},
				}},
			},
		},
	}

	showsAdded, _, err := WriteShows(ctx, conn, shows)
	require.NoError(t, err)
	require.Equal(t, 1, showsAdded)

	// Verify set numbers
	rows, err := conn.QueryContext(ctx, `
		SELECT s.name, p.set_number, p.position
		FROM performances p
		JOIN songs s ON p.song_id = s.id
		JOIN shows sh ON p.show_id = sh.id
		WHERE sh.date = '1977-05-08' AND sh.venue_id = (SELECT id FROM venues WHERE name = 'Test Hall')
		ORDER BY p.set_number, p.position
	`)
	require.NoError(t, err)
	defer rows.Close()

	type perf struct {
		name      string
		setNumber int
		position  int
	}
	var perfs []perf
	for rows.Next() {
		var p perf
		require.NoError(t, rows.Scan(&p.name, &p.setNumber, &p.position))
		perfs = append(perfs, p)
	}
	require.NoError(t, rows.Err())
	require.Len(t, perfs, 5)

	require.Equal(t, perf{"Minglewood Blues", 1, 1}, perfs[0])
	require.Equal(t, perf{"Loser", 1, 2}, perfs[1])
	require.Equal(t, perf{"Estimated Prophet", 2, 1}, perfs[2])
	require.Equal(t, perf{"Eyes of the World", 2, 2}, perfs[3])
	require.Equal(t, perf{"One More Saturday Night", 3, 1}, perfs[4])
}

func TestWriteShows_DeduplicatesCaseVariants(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	// Import two shows with different capitalizations of the same song.
	// Fixture already has "Fire on the Mountain" (id=2).
	shows := []Show{
		{
			Date:  "1977-04-22",
			Venue: Venue{Name: "The Spectrum", City: "Philadelphia", State: "PA", Country: "USA"},
			Sets: []Set{
				{Songs: []SongInSet{{Name: "Fire On The Mountain"}}},
			},
		},
		{
			Date:  "1977-04-25",
			Venue: Venue{Name: "Capitol Theater", City: "Passaic", State: "NJ", Country: "USA"},
			Sets: []Set{
				{Songs: []SongInSet{{Name: "Fire On THe Mountain"}}},
			},
		},
	}

	showsAdded, songsAdded, err := WriteShows(ctx, conn, shows)
	require.NoError(t, err)
	require.Equal(t, 2, showsAdded)
	require.Equal(t, 0, songsAdded, "case variants should resolve to existing song, not create new ones")

	// Both performances should reference the same song_id (2 = "Fire on the Mountain")
	var ids []int
	rows, err := conn.QueryContext(ctx, "SELECT DISTINCT song_id FROM performances WHERE show_id IN (SELECT id FROM shows WHERE date IN ('1977-04-22', '1977-04-25'))")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.Len(t, ids, 1, "all case variants should map to the same song")
	require.Equal(t, 2, ids[0])
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
