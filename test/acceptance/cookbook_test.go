package acceptance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gdql/gdql/run"
	"github.com/stretchr/testify/require"
)

// TestCookbook_* tests mirror every section of the Cookbook docs page.
// Every query from the docs must return non-empty, correct-type results.

func runQuery(t *testing.T, query string) map[string]interface{} {
	t.Helper()
	result, err := run.RunWithEmbeddedDB(context.Background(), query)
	require.NoError(t, err, "query: %s", query)
	require.NotEmpty(t, result, "query returned empty: %s", query)
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &data), "bad JSON for: %s", query)
	return data
}

func runQueryExpectShows(t *testing.T, query string) {
	t.Helper()
	data := runQuery(t, query)
	shows, ok := data["shows"].([]interface{})
	require.True(t, ok, "expected shows array for: %s", query)
	require.NotEmpty(t, shows, "no shows returned for: %s", query)
}

func runQueryExpectSongs(t *testing.T, query string) {
	t.Helper()
	data := runQuery(t, query)
	songs, ok := data["songs"].([]interface{})
	require.True(t, ok, "expected songs array for: %s", query)
	require.NotEmpty(t, songs, "no songs returned for: %s", query)
}

func runQueryExpectSetlist(t *testing.T, query string) {
	t.Helper()
	data := runQuery(t, query)
	require.Contains(t, data, "setlist", "expected setlist for: %s", query)
}

func runQueryExpectCount(t *testing.T, query string) {
	t.Helper()
	data := runQuery(t, query)
	require.Contains(t, data, "count", "expected count for: %s", query)
	count := data["count"].(map[string]interface{})
	require.Greater(t, count["Count"].(float64), float64(0), "count was 0 for: %s", query)
}

// === What shows were played? ===

func TestCookbook_ShowsFromYear(t *testing.T) {
	runQueryExpectShows(t, `SHOWS FROM 1977 LIMIT 5;`)
}

func TestCookbook_ShowsFromRange(t *testing.T) {
	runQueryExpectShows(t, `SHOWS FROM 1977-1980 ORDER BY DATE LIMIT 5;`)
}

func TestCookbook_ShowsFromEra(t *testing.T) {
	for _, era := range []string{"PRIMAL", "EUROPE72", "BRENT_ERA", "EUROPE", "BRENT", "VINCE"} {
		t.Run(era, func(t *testing.T) {
			runQueryExpectShows(t, `SHOWS FROM `+era+` LIMIT 5;`)
		})
	}
}

func TestCookbook_ShowsIn(t *testing.T) {
	runQueryExpectShows(t, `SHOWS IN 1977 LIMIT 5;`)
}

func TestCookbook_ShowsAt(t *testing.T) {
	runQueryExpectShows(t, `SHOWS AT "Fillmore West" FROM 1969;`)
	runQueryExpectShows(t, `SHOWS AT "Winterland" FROM 1977;`)
	runQueryExpectShows(t, `SHOWS AT "New York" LIMIT 5;`)
}

func TestCookbook_ShowsTour(t *testing.T) {
	// Tour data not available in Deadlists import — just verify it parses without error
	_, err := run.RunWithEmbeddedDB(context.Background(), `SHOWS TOUR "Europe" FROM 1972 LIMIT 5;`)
	require.NoError(t, err)
}

func TestCookbook_ShowsAfterBefore(t *testing.T) {
	runQueryExpectShows(t, `SHOWS AFTER 1985 LIMIT 5;`)
	runQueryExpectShows(t, `SHOWS BEFORE 1970 LIMIT 5;`)
}

// === Setlists ===

func TestCookbook_SetlistMDY(t *testing.T) {
	runQueryExpectSetlist(t, `SETLIST FOR 5/8/77;`)
}

func TestCookbook_SetlistISO(t *testing.T) {
	runQueryExpectSetlist(t, `SETLIST 1977-05-08;`)
}

func TestCookbook_SetlistWithoutFor(t *testing.T) {
	runQueryExpectSetlist(t, `SETLIST 5/8/77;`)
}

// === Segue queries ===

func TestCookbook_SegueChain(t *testing.T) {
	runQueryExpectShows(t, `SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain" LIMIT 5;`)
	runQueryExpectShows(t, `SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower" LIMIT 5;`)
}

func TestCookbook_SegueINTO(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "China Cat Sunflower" INTO "I Know You Rider" LIMIT 5;`)
}

func TestCookbook_SegueArrow(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "Dark Star" -> "Saint Stephen" LIMIT 5;`)
}

func TestCookbook_SegueTHEN(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "Estimated Prophet" THEN "Eyes of the World" LIMIT 5;`)
}

func TestCookbook_NegatedSegueNOTINTO(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "Scarlet Begonias" NOT INTO "Fire on the Mountain" LIMIT 5;`)
}

func TestCookbook_NegatedSegueBangGT(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "Scarlet Begonias" !> "Fire on the Mountain" LIMIT 5;`)
}

func TestCookbook_NegatedSegueBangGTGT(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE "China Cat Sunflower" !>> "I Know You Rider" LIMIT 5;`)
}

// === COUNT ===

func TestCookbook_CountSong(t *testing.T) {
	runQueryExpectCount(t, `COUNT "Dark Star";`)
}

func TestCookbook_CountSongFromRange(t *testing.T) {
	runQueryExpectCount(t, `COUNT "Dark Star" FROM 1972-1974;`)
}

func TestCookbook_CountSongAfter(t *testing.T) {
	runQueryExpectCount(t, `COUNT "Scarlet Begonias" AFTER 1977;`)
}

func TestCookbook_CountShowsWhere(t *testing.T) {
	runQueryExpectCount(t, `COUNT SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower";`)
}

func TestCookbook_CountShowsWhereOpener(t *testing.T) {
	runQueryExpectCount(t, `COUNT SHOWS WHERE OPENER "Bertha";`)
}

func TestCookbook_CountShowsBefore(t *testing.T) {
	runQueryExpectCount(t, `COUNT SHOWS BEFORE 1976;`)
}

// === Most played songs ===

func TestCookbook_SongsOrderByTimesPlayed(t *testing.T) {
	runQueryExpectSongs(t, `SONGS ORDER BY TIMES_PLAYED DESC LIMIT 20;`)
}

func TestCookbook_SongsFromYearOrderByTimesPlayed(t *testing.T) {
	runQueryExpectSongs(t, `SONGS FROM 1977 ORDER BY TIMES_PLAYED DESC LIMIT 20;`)
}

func TestCookbook_SongsPlayedIn(t *testing.T) {
	runQueryExpectSongs(t, `SONGS PLAYED IN 1977 ORDER BY TIMES_PLAYED DESC LIMIT 5;`)
}

func TestCookbook_SongsPlayedFrom(t *testing.T) {
	runQueryExpectSongs(t, `SONGS PLAYED FROM 1972 ORDER BY TIMES_PLAYED DESC LIMIT 5;`)
}

func TestCookbook_SongsFromEra(t *testing.T) {
	runQueryExpectSongs(t, `SONGS FROM EUROPE72 ORDER BY TIMES_PLAYED DESC LIMIT 5;`)
}

// === Set position ===

func TestCookbook_Opener(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE OPENER "Bertha" LIMIT 5;`)
	runQueryExpectShows(t, `SHOWS WHERE SET1 OPENED "Jack Straw" LIMIT 5;`)
}

func TestCookbook_Closer(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE CLOSER "Morning Dew" LIMIT 5;`)
	runQueryExpectShows(t, `SHOWS WHERE SET2 CLOSED "Sugar Magnolia" LIMIT 5;`)
}

func TestCookbook_EncoreWithEquals(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE ENCORE = "U.S. Blues" LIMIT 5;`)
}

func TestCookbook_EncoreWithoutEquals(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE ENCORE "U.S. Blues" LIMIT 5;`)
}

func TestCookbook_CloserNoSpaceParen(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE CLOSER("Morning Dew") LIMIT 5;`)
}

func TestCookbook_OpenerSegueChain(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE OPENER "Help on the Way" > "Slipknot!" LIMIT 5;`)
}

func TestCookbook_OpenerSegueChainWithParens(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE OPENER ("Help on the Way" > "Slipknot!") LIMIT 5;`)
}

func TestCookbook_CloserSegueChain(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE CLOSER "Throwin' Stones" > "Not Fade Away" LIMIT 5;`)
}

// === Negated position ===

func TestCookbook_NotClosed(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE PLAYED "U.S. Blues" AND NOT CLOSED "U.S. Blues" LIMIT 5;`)
}

func TestCookbook_NotOpener(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE PLAYED "Bertha" AND NOT OPENER "Bertha" LIMIT 5;`)
}

func TestCookbook_NotEncore(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE PLAYED "U.S. Blues" AND NOT ENCORE "U.S. Blues" LIMIT 5;`)
}

// === Exclusion ===

func TestCookbook_NotPlayed(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE PLAYED "Dark Star" AND NOT PLAYED "Saint Stephen" LIMIT 5;`)
}

func TestCookbook_NotPlayedShortForm(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE PLAYED "Truckin'" AND NOT "Drums" LIMIT 5;`)
}

// === AND / OR ===

func TestCookbook_AndConditions(t *testing.T) {
	runQueryExpectShows(t, `SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain" AND PLAYED "Estimated Prophet" LIMIT 5;`)
}

func TestCookbook_OrConditions(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE OPENER "Jack Straw" OR OPENER "Bertha" LIMIT 5;`)
}

// === Lyrics ===

func TestCookbook_LyricsSingleWord(t *testing.T) {
	runQueryExpectSongs(t, `SONGS WITH LYRICS("sun");`)
}

func TestCookbook_LyricsMultipleWords(t *testing.T) {
	runQueryExpectSongs(t, `SONGS WITH LYRICS("train", "road");`)
}

func TestCookbook_LyricsAndLyrics(t *testing.T) {
	runQueryExpectSongs(t, `SONGS WITH LYRICS("sun") AND LYRICS("shine");`)
}

func TestCookbook_LyricsAsCount(t *testing.T) {
	runQueryExpectCount(t, `SONGS WITH LYRICS("rose") AS COUNT;`)
}

// === First / Last ===

func TestCookbook_First(t *testing.T) {
	data := runQuery(t, `FIRST "Help on the Way";`)
	require.Contains(t, data, "shows")
}

func TestCookbook_Last(t *testing.T) {
	data := runQuery(t, `LAST "Dark Star";`)
	require.Contains(t, data, "shows")
}

// === Random ===

func TestCookbook_RandomShow(t *testing.T) {
	data := runQuery(t, `RANDOM SHOW FROM EUROPE72;`)
	require.Contains(t, data, "setlist")
}

// === Output formats ===

func TestCookbook_AsJSON(t *testing.T) {
	result, err := run.RunWithEmbeddedDB(context.Background(), `SHOWS FROM 1977 LIMIT 3 AS JSON;`)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.TrimSpace(result), "{"), "expected JSON object")
}

func TestCookbook_AsCSV(t *testing.T) {
	result, err := run.RunWithEmbeddedDB(context.Background(), `SHOWS FROM 1977 LIMIT 3 AS CSV;`)
	require.NoError(t, err)
	require.Contains(t, result, ",") // CSV has commas
}

func TestCookbook_AsSetlistOnShows(t *testing.T) {
	result, err := run.RunWithEmbeddedDB(context.Background(), `SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain" AS SETLIST;`)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &data))
	// Should have setlists array (multi-show)
	require.Contains(t, data, "setlists")
}

// === Comments ===

func TestCookbook_CommentsStripped(t *testing.T) {
	result, err := run.RunWithEmbeddedDB(context.Background(), "SHOWS FROM 1977 LIMIT 1; -- this is a comment")
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

// === Multi-statement ===

func TestCookbook_MultiStatement(t *testing.T) {
	result, err := run.RunWithEmbeddedDB(context.Background(), "COUNT \"Dark Star\";\nCOUNT SHOWS FROM 1977;")
	require.NoError(t, err)
	// Multi-statement returns JSON array
	var data []interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &data))
	require.Len(t, data, 2)
}

// === Performances ===

func TestCookbook_Performances(t *testing.T) {
	data := runQuery(t, `PERFORMANCES OF "Dark Star" LIMIT 5;`)
	require.Contains(t, data, "performances")
}

func TestCookbook_PerformancesFromRange(t *testing.T) {
	data := runQuery(t, `PERFORMANCES OF "Dark Star" FROM 1972-1974 LIMIT 5;`)
	require.Contains(t, data, "performances")
}

// === OPENED/CLOSED as standalone aliases ===

func TestCookbook_OpenedAsOpener(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE OPENED "Bertha" LIMIT 5;`)
}

func TestCookbook_ClosedAsCloser(t *testing.T) {
	runQueryExpectShows(t, `SHOWS WHERE CLOSED "Morning Dew" LIMIT 5;`)
}

// === Compare eras ===

func TestCookbook_CompareEras(t *testing.T) {
	for _, era := range []string{"PRIMAL", "EUROPE72", "BRENT_ERA", "VINCE_ERA"} {
		t.Run(era, func(t *testing.T) {
			runQueryExpectCount(t, `COUNT SHOWS FROM `+era+`;`)
		})
	}
}
