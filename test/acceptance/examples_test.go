package acceptance

// Tests for every example query in the documentation (docs repo: content/docs/examples.md).
// Each query must parse and execute without error against the fixture database.
// Queries that require specific data (lyrics, length, guests) may return empty results
// but must not error.

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/executor"
	"github.com/stretchr/testify/require"
)

// mustParse verifies a query parses and executes without error.
func mustParse(t *testing.T, ex executor.Executor, query string) *executor.Result {
	t.Helper()
	result, err := ex.Execute(context.Background(), query)
	require.NoError(t, err, "query: %s", query)
	return result
}

func TestDocExamples_ShowsByYear(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	mustParse(t, ex, `SHOWS FROM 1977;`)
	mustParse(t, ex, `SHOWS FROM 77;`)
	mustParse(t, ex, `SHOWS FROM 1977-1980 ORDER BY DATE;`)
	mustParse(t, ex, `SHOWS FROM PRIMAL;`)
	mustParse(t, ex, `SHOWS FROM EUROPE72;`)
	mustParse(t, ex, `SHOWS FROM BRENT_ERA;`)
	mustParse(t, ex, `SHOWS FROM 1972 ORDER BY DATE DESC LIMIT 5;`)
}

func TestDocExamples_ShowsAtVenue(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `SHOWS AT "Barton Hall";`)
	require.Len(t, r.Shows, 1, "fixture has 1 show at Barton Hall")

	r = mustParse(t, ex, `SHOWS AT "Winterland" FROM 1977;`)
	require.Len(t, r.Shows, 1, "fixture has 1 Winterland show in 1977")

	mustParse(t, ex, `SHOWS AT "New York" LIMIT 20;`)
}

func TestDocExamples_Segues(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	// > operator (adjacency)
	r := mustParse(t, ex, `SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";`)
	require.GreaterOrEqual(t, len(r.Shows), 2, "fixture has Scarlet > Fire")

	r = mustParse(t, ex, `SHOWS WHERE "Help on the Way" > "Samson and Delilah";`)
	require.GreaterOrEqual(t, len(r.Shows), 1)

	// INTO alias
	mustParse(t, ex, `SHOWS WHERE "Scarlet Begonias" INTO "Fire on the Mountain";`)

	// >> (THEN)
	mustParse(t, ex, `SHOWS WHERE "Scarlet Begonias" >> "Fire on the Mountain";`)
	mustParse(t, ex, `SHOWS WHERE "Scarlet Begonias" THEN "Fire on the Mountain";`)

	// ~> (TEASE)
	mustParse(t, ex, `SHOWS WHERE "Scarlet Begonias" ~> "Fire on the Mountain";`)
	mustParse(t, ex, `SHOWS WHERE "Scarlet Begonias" TEASE "Fire on the Mountain";`)
}

func TestDocExamples_SetPosition(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	mustParse(t, ex, `SHOWS WHERE SET1 OPENED "Scarlet Begonias";`)
	mustParse(t, ex, `SHOWS WHERE SET2 CLOSED "Morning Dew";`)
	mustParse(t, ex, `SHOWS WHERE ENCORE = "Morning Dew";`)
}

func TestDocExamples_Played(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `SHOWS WHERE PLAYED "Dark Star";`)
	require.GreaterOrEqual(t, len(r.Shows), 2, "fixture has 2 shows with Dark Star")

	r = mustParse(t, ex, `SHOWS FROM 1977 WHERE PLAYED "Scarlet Begonias";`)
	require.Equal(t, 2, len(r.Shows))

	mustParse(t, ex, `SHOWS WHERE GUEST "Branford Marsalis";`)
}

func TestDocExamples_CombiningConditions(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	mustParse(t, ex, `SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain" AND PLAYED "Help on the Way";`)
	mustParse(t, ex, `SHOWS WHERE SET1 OPENED "Scarlet Begonias" OR SET1 OPENED "Samson and Delilah";`)
}

func TestDocExamples_VenueDateSegue(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `SHOWS AT "Winterland" WHERE "Scarlet Begonias" > "Fire on the Mountain";`)
	require.GreaterOrEqual(t, len(r.Shows), 1)

	mustParse(t, ex, `SHOWS AT "Barton" FROM 1977 WHERE PLAYED "Dark Star";`)
}

func TestDocExamples_Setlist(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `SETLIST FOR 5/8/77;`)
	require.NotNil(t, r.Setlist)
	require.GreaterOrEqual(t, len(r.Setlist.Performances), 5)
}

func TestDocExamples_Songs(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `SONGS LIMIT 50;`)
	require.GreaterOrEqual(t, len(r.Songs), 1)

	mustParse(t, ex, `SONGS WITH LYRICS("walkin");`)
	mustParse(t, ex, `SONGS WITH LYRICS("train", "road");`)
	mustParse(t, ex, `SONGS WRITTEN 1968-1970;`)
	mustParse(t, ex, `SONGS WRITTEN 1970;`)
}

func TestDocExamples_Performances(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	r := mustParse(t, ex, `PERFORMANCES OF "Dark Star";`)
	require.GreaterOrEqual(t, len(r.Performances), 2)

	mustParse(t, ex, `PERFORMANCES OF "Dark Star" FROM 1972-1974;`)
	mustParse(t, ex, `PERFORMANCES OF "Dark Star" WITH LENGTH > 20min ORDER BY LENGTH DESC LIMIT 5;`)
	mustParse(t, ex, `PERFORMANCES OF "Scarlet Begonias" FROM 77-79 ORDER BY DATE DESC LIMIT 20;`)
}

func TestDocExamples_OutputFormats(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	mustParse(t, ex, `SHOWS FROM 1977 LIMIT 3 AS TABLE;`)

	r := mustParse(t, ex, `SHOWS FROM 1977 LIMIT 3 AS JSON;`)
	require.NotNil(t, r)

	r = mustParse(t, ex, `SHOWS FROM 1977 LIMIT 3 AS CSV;`)
	require.NotNil(t, r)

	r = mustParse(t, ex, `SHOWS FROM 1977 LIMIT 3 AS SETLIST;`)
	require.NotNil(t, r)
}

func TestDocExamples_Count(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	// Count all Dark Star performances
	r := mustParse(t, ex, `COUNT "Dark Star";`)
	require.Equal(t, executor.ResultCount, r.Type)
	require.NotNil(t, r.Count)
	require.Equal(t, "Dark Star", r.Count.SongName)
	require.GreaterOrEqual(t, r.Count.Count, 2, "fixture has 2 Dark Star performances")

	// Count with date range
	r = mustParse(t, ex, `COUNT "Scarlet Begonias" FROM 1977;`)
	require.NotNil(t, r.Count)
	require.GreaterOrEqual(t, r.Count.Count, 2)

	// Count with fuzzy match
	r = mustParse(t, ex, `COUNT "Fire on the mountain";`)
	require.NotNil(t, r.Count)
	require.GreaterOrEqual(t, r.Count.Count, 1)
}

func TestDocExamples_TwoDigitYears(t *testing.T) {
	db := openTestDB(t)
	ex := executor.New(db)

	// 77 should equal 1977
	r1 := mustParse(t, ex, `SHOWS FROM 77;`)
	r2 := mustParse(t, ex, `SHOWS FROM 1977;`)
	require.Equal(t, len(r1.Shows), len(r2.Shows), "77 and 1977 should return same results")

	// 77-80 should equal 1977-1980
	r1 = mustParse(t, ex, `SHOWS FROM 77-80;`)
	r2 = mustParse(t, ex, `SHOWS FROM 1977-1980;`)
	require.Equal(t, len(r1.Shows), len(r2.Shows))
}
