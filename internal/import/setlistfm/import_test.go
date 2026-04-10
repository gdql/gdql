package setlistfm

import (
	"database/sql"
	"fmt"
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

func TestParseEventDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"08-05-1977", "1977-05-08", true},
		{"01-01-1965", "1965-01-01", true},
		{"31-12-1995", "1995-12-31", true},
		{"", "", false},
		{"08-05", "", false},
		{"a-b-c", "c-b-a", true}, // 3 parts splits, function doesn't validate content
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := parseEventDate(tc.input)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestVenueFields(t *testing.T) {
	v := &Venue{
		Name: "Barton Hall",
		City: &City{
			Name:      "Ithaca",
			StateCode: "NY",
			Country:   &Country{Code: "US"},
		},
	}
	name, city, state, country := venueFields(v)
	require.Equal(t, "Barton Hall", name)
	require.Equal(t, "Ithaca", city)
	require.Equal(t, "NY", state)
	require.Equal(t, "US", country)
}

func TestVenueFields_NoCity(t *testing.T) {
	v := &Venue{Name: "Madison Square Garden"}
	name, city, state, country := venueFields(v)
	require.Equal(t, "Madison Square Garden", name)
	require.Empty(t, city)
	require.Empty(t, state)
	require.Empty(t, country)
}

func TestVenueFields_NoCountry(t *testing.T) {
	v := &Venue{
		Name: "Wembley",
		City: &City{Name: "London", StateCode: ""},
	}
	name, city, _, country := venueFields(v)
	require.Equal(t, "Wembley", name)
	require.Equal(t, "London", city)
	require.Empty(t, country)
}

func TestVenueKey(t *testing.T) {
	v := &Venue{
		Name: "Barton Hall",
		City: &City{Name: "Ithaca", StateCode: "NY", Country: &Country{Code: "US"}},
	}
	k := venueKey(v)
	require.Contains(t, k, "Barton Hall")
	require.Contains(t, k, "Ithaca")

	// Same venue produces same key
	require.Equal(t, k, venueKey(v))

	// Different city produces different key
	v2 := &Venue{
		Name: "Barton Hall",
		City: &City{Name: "Different", StateCode: "NY", Country: &Country{Code: "US"}},
	}
	require.NotEqual(t, k, venueKey(v2))
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

func TestInferSetBreaks_WithDrums(t *testing.T) {
	songs := []Song{
		{Name: "Jack Straw"}, {Name: "Sugaree"}, {Name: "Cassidy"},
		{Name: "Deal"}, {Name: "Brown Eyed Women"}, {Name: "Loser"},
		// Set 2 starts at Drums
		{Name: "Scarlet Begonias"}, {Name: "Fire on the Mountain"},
		{Name: "Drums"}, {Name: "Space"}, {Name: "Wharf Rat"},
		{Name: "Sugar Magnolia"},
		// Encore
		{Name: "One More Saturday Night"},
	}
	sets := InferSetBreaks(songs)
	require.Len(t, sets, 3, "should produce set1, set2, encore")
	require.Equal(t, "Set 1", sets[0].Name)
	require.Len(t, sets[0].Songs, 8, "set 1: everything before Drums")
	require.Equal(t, "Set 2", sets[1].Name)
	require.Len(t, sets[1].Songs, 4, "set 2: Drums through Sugar Magnolia")
	require.Equal(t, 1, sets[2].Encore)
	require.Len(t, sets[2].Songs, 1, "encore: One More Saturday Night")
}

func TestInferSetBreaks_NoDrums_LongShow(t *testing.T) {
	songs := make([]Song, 20)
	for i := range songs {
		songs[i] = Song{Name: fmt.Sprintf("Song %d", i+1)}
	}
	// Put an encore marker at the end
	songs[19] = Song{Name: "U.S. Blues"}

	sets := InferSetBreaks(songs)
	require.Len(t, sets, 3)
	require.Equal(t, "Set 1", sets[0].Name)
	require.Equal(t, "Set 2", sets[1].Name)
	require.Equal(t, 1, sets[2].Encore)
	require.Equal(t, "U.S. Blues", sets[2].Songs[0].Name)
}

func TestInferSetBreaks_ShortShow(t *testing.T) {
	songs := []Song{
		{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"},
	}
	sets := InferSetBreaks(songs)
	require.Len(t, sets, 1, "short show stays as one set")
	require.Len(t, sets[0].Songs, 5)
}

func TestUpsertShow_InfersSetBreaks(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	require.NoError(t, sqlite.InitSchema(dbPath))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	venueByKey := make(map[string]int64)
	songByName := make(map[string]int64)
	var nextVenueID, nextShowID, nextSongID, nextPerfID int64 = 1, 1, 1, 1

	// Single set with Drums — should be split
	sl := &Setlist{
		EventDate: "31-12-1976",
		Venue: Venue{
			Name: "Cow Palace",
			City: &City{Name: "Daly City", StateCode: "CA", Country: &Country{Code: "US"}},
		},
		Set: []Set{{
			Songs: []Song{
				{Name: "Jack Straw"}, {Name: "Sugaree"}, {Name: "Cassidy"},
				{Name: "Deal"}, {Name: "Brown Eyed Women"}, {Name: "Loser"},
				{Name: "Scarlet Begonias"}, {Name: "Fire on the Mountain"},
				{Name: "Drums"}, {Name: "Space"}, {Name: "Wharf Rat"},
				{Name: "Sugar Magnolia"},
				{Name: "One More Saturday Night"},
			},
		}},
	}

	added, err := upsertShow(db, sl, venueByKey, songByName, &nextVenueID, &nextShowID, &nextSongID, &nextPerfID)
	require.NoError(t, err)
	require.True(t, added)

	// Check set numbers
	rows, err := db.Query("SELECT set_number, count(*) FROM performances GROUP BY set_number ORDER BY set_number")
	require.NoError(t, err)
	defer rows.Close()

	setMap := make(map[int]int)
	for rows.Next() {
		var sn, cnt int
		require.NoError(t, rows.Scan(&sn, &cnt))
		setMap[sn] = cnt
	}
	require.NoError(t, rows.Err())
	require.Greater(t, len(setMap), 1, "should have multiple sets, got: %v", setMap)
	require.Contains(t, setMap, 1, "should have set 1")
	require.Contains(t, setMap, 2, "should have set 2")
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
