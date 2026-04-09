package setlistfm

import (
	"database/sql"
	"testing"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestUpsertShow_PreservesSetBoundaries(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	require.NoError(t, sqlite.InitSchema(dbPath))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	venueByKey := make(map[string]int64)
	songByName := make(map[string]int64)
	var nextVenueID, nextShowID, nextSongID, nextPerfID int64 = 1, 1, 1, 1

	sl := &Setlist{
		EventDate: "08-05-1977",
		Venue: Venue{
			Name: "Barton Hall",
			City: &City{Name: "Ithaca", StateCode: "NY", Country: &Country{Code: "US"}},
		},
		Set: []Set{
			{
				Name: "Set 1",
				Songs: []Song{
					{Name: "Minglewood Blues"},
					{Name: "Loser"},
				},
			},
			{
				Name: "Set 2",
				Songs: []Song{
					{Name: "Scarlet Begonias"},
					{Name: "Fire on the Mountain", Info: ">"},
				},
			},
			{
				Encore: 1,
				Songs: []Song{
					{Name: "One More Saturday Night"},
				},
			},
		},
	}

	added, err := upsertShow(db, sl, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
	require.NoError(t, err)
	require.True(t, added)

	rows, err := db.Query(`
		SELECT s.name, p.set_number, p.position, COALESCE(p.segue_type, '')
		FROM performances p
		JOIN songs s ON p.song_id = s.id
		ORDER BY p.set_number, p.position
	`)
	require.NoError(t, err)
	defer rows.Close()

	type perf struct {
		name      string
		setNumber int
		position  int
		segue     string
	}
	var perfs []perf
	for rows.Next() {
		var p perf
		require.NoError(t, rows.Scan(&p.name, &p.setNumber, &p.position, &p.segue))
		perfs = append(perfs, p)
	}
	require.NoError(t, rows.Err())
	require.Len(t, perfs, 5)

	// Set 1
	require.Equal(t, perf{"Minglewood Blues", 1, 1, ""}, perfs[0])
	require.Equal(t, perf{"Loser", 1, 2, ""}, perfs[1])
	// Set 2 — Scarlet has segue ">" because Fire's Info is ">"
	require.Equal(t, perf{"Scarlet Begonias", 2, 1, ">"}, perfs[2])
	require.Equal(t, perf{"Fire on the Mountain", 2, 2, ""}, perfs[3])
	// Encore (set_number = 4)
	require.Equal(t, perf{"One More Saturday Night", 4, 1, ""}, perfs[4])
}

func TestUpsertShow_DeduplicatesCaseVariants(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	require.NoError(t, sqlite.InitSchema(dbPath))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	venueByKey := make(map[string]int64)
	songByName := make(map[string]int64)
	var nextVenueID, nextShowID, nextSongID, nextPerfID int64 = 1, 1, 1, 1

	// First show: "Fire on the Mountain"
	sl1 := &Setlist{
		EventDate: "18-03-1977",
		Venue:     Venue{Name: "Winterland", City: &City{Name: "San Francisco", StateCode: "CA", Country: &Country{Code: "US"}}},
		Set:       []Set{{Songs: []Song{{Name: "Fire on the Mountain"}}}},
	}
	_, err = upsertShow(db, sl1, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
	require.NoError(t, err)

	// Second show: "Fire On The Mountain" (different case)
	sl2 := &Setlist{
		EventDate: "22-04-1977",
		Venue:     Venue{Name: "The Spectrum", City: &City{Name: "Philadelphia", StateCode: "PA", Country: &Country{Code: "US"}}},
		Set:       []Set{{Songs: []Song{{Name: "Fire On The Mountain"}}}},
	}
	_, err = upsertShow(db, sl2, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
	require.NoError(t, err)

	// Should only have 1 song, not 2
	var songCount int
	err = db.QueryRow("SELECT COUNT(*) FROM songs").Scan(&songCount)
	require.NoError(t, err)
	require.Equal(t, 1, songCount, "case variants should not create duplicate songs")
}

func TestSplitSongName(t *testing.T) {
	// Simple name
	names, segue := splitSongName("Scarlet Begonias")
	require.Equal(t, []string{"Scarlet Begonias"}, names)
	require.Equal(t, []bool{false}, segue)

	// Segue chain
	names, segue = splitSongName("Scarlet Begonias > Fire on the Mountain")
	require.Equal(t, []string{"Scarlet Begonias", "Fire on the Mountain"}, names)
	require.Equal(t, []bool{true, false}, segue)

	// Empty parts don't cause index mismatch
	names, segue = splitSongName("A >  > B")
	require.Equal(t, []string{"A", "B"}, names)
	require.Len(t, segue, len(names), "segueAfter must match names length")
	require.True(t, segue[0])
	require.False(t, segue[1])

	// Empty string
	names, segue = splitSongName("")
	require.Nil(t, names)
	require.Nil(t, segue)
}

func TestUpsertShow_SingleSetAllInSet1(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	require.NoError(t, sqlite.InitSchema(dbPath))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	venueByKey := make(map[string]int64)
	songByName := make(map[string]int64)
	var nextVenueID, nextShowID, nextSongID, nextPerfID int64 = 1, 1, 1, 1

	sl := &Setlist{
		EventDate: "31-12-1978",
		Venue: Venue{
			Name: "Winterland",
			City: &City{Name: "San Francisco", StateCode: "CA", Country: &Country{Code: "US"}},
		},
		Set: []Set{
			{
				Songs: []Song{
					{Name: "Sugar Magnolia"},
					{Name: "Scarlet Begonias"},
				},
			},
		},
	}

	added, err := upsertShow(db, sl, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
	require.NoError(t, err)
	require.True(t, added)

	var setNum int
	err = db.QueryRow("SELECT DISTINCT set_number FROM performances").Scan(&setNum)
	require.NoError(t, err)
	require.Equal(t, 1, setNum, "single set should be set_number 1")
}
