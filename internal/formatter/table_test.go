package formatter

import (
	"testing"
	"time"

	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/executor"
	"github.com/stretchr/testify/require"
)

func TestTablePerformances_OmitsLengthColumnWhenAllZero(t *testing.T) {
	perfs := []*data.Performance{
		{ShowID: 1, SetNumber: 1, Position: 1, SegueType: ">", LengthSeconds: 0},
		{ShowID: 1, SetNumber: 1, Position: 2, SegueType: "", LengthSeconds: 0},
	}
	result := &executor.Result{Type: executor.ResultPerformances, Performances: perfs}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.NotContains(t, out, "LENGTH")
	require.Contains(t, out, "SHOW_ID")
	require.Contains(t, out, "SEGUE")
}

func TestTablePerformances_ShowsLengthWhenPresent(t *testing.T) {
	perfs := []*data.Performance{
		{ShowID: 1, SetNumber: 1, Position: 1, SegueType: ">", LengthSeconds: 580},
		{ShowID: 1, SetNumber: 1, Position: 2, SegueType: "", LengthSeconds: 0},
	}
	result := &executor.Result{Type: executor.ResultPerformances, Performances: perfs}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "LENGTH")
	require.Contains(t, out, "9:40")
	require.Contains(t, out, "-") // zero length shown as "-"
}

func TestFormatLength(t *testing.T) {
	require.Equal(t, "-", formatLength(0))
	require.Equal(t, "9:40", formatLength(580))
	require.Equal(t, "22:00", formatLength(1320))
	require.Equal(t, "0:30", formatLength(30))
}

func TestTruncate(t *testing.T) {
	// ASCII
	require.Equal(t, "hello", truncate("hello world", 5))
	require.Equal(t, "hi", truncate("hi", 5))

	// Multi-byte: don't split UTF-8 characters
	require.Equal(t, "café", truncate("café au lait", 4))
	require.Equal(t, "日本", truncate("日本語テスト", 2))

	// Empty
	require.Equal(t, "", truncate("", 5))
}

func TestTableCount_WithSongName(t *testing.T) {
	result := &executor.Result{
		Type:  executor.ResultCount,
		Count: &executor.CountResult{SongName: "Dark Star", Count: 236},
	}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "Dark Star")
	require.Contains(t, out, "236")
}

func TestTableCount_NoSongName(t *testing.T) {
	result := &executor.Result{
		Type:  executor.ResultCount,
		Count: &executor.CountResult{Count: 2061},
	}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "2061")
}

func TestTableShows_Empty(t *testing.T) {
	result := &executor.Result{Type: executor.ResultShows, Shows: nil}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "No shows found")
	require.NotContains(t, out, "gdql init") // misleading tip removed
}

func TestFormatJSON_Count(t *testing.T) {
	result := &executor.Result{
		Type:  executor.ResultCount,
		Count: &executor.CountResult{SongName: "Dark Star", Count: 236},
	}
	out, err := formatJSON(result)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "count"`)
	require.Contains(t, out, `"Dark Star"`)
	require.Contains(t, out, `236`)
}

func TestFormatJSON_Shows(t *testing.T) {
	result := &executor.Result{
		Type:  executor.ResultShows,
		Shows: []*data.Show{{ID: 1, Venue: "Barton Hall"}},
	}
	out, err := formatJSON(result)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "shows"`)
	require.Contains(t, out, `Barton Hall`)
}

func TestFormatCSV_Count(t *testing.T) {
	result := &executor.Result{
		Type:  executor.ResultCount,
		Count: &executor.CountResult{SongName: "Dark Star", Count: 236},
	}
	out, err := formatCSV(result)
	require.NoError(t, err)
	require.Contains(t, out, "song,count")
	require.Contains(t, out, "Dark Star,236")
}

func TestFormatCSV_Shows(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultShows,
		Shows: []*data.Show{{ID: 1, Venue: "Barton Hall", City: "Ithaca"}},
	}
	out, err := formatCSV(result)
	require.NoError(t, err)
	require.Contains(t, out, "id,date")
	require.Contains(t, out, "Barton Hall")
	require.Contains(t, out, "Ithaca")
}

func TestFormat_CalendarReturnsError(t *testing.T) {
	f := New()
	result := &executor.Result{Type: executor.ResultShows}
	_, err := f.Format(result, FormatCalendar)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not yet implemented")
}

// === JSON ===

func TestFormatJSON_Songs(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSongs,
		Songs: []*data.Song{
			{ID: 1, Name: "Scarlet Begonias", Writers: "Hunter/Garcia"},
		},
	}
	out, err := formatJSON(result)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "songs"`)
	require.Contains(t, out, "Scarlet Begonias")
}

func TestFormatJSON_Performances(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultPerformances,
		Performances: []*data.Performance{
			{ID: 1, ShowID: 1, SongID: 6, SetNumber: 1, Position: 3},
		},
	}
	out, err := formatJSON(result)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "performances"`)
}

func TestFormatJSON_Setlist(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSetlist,
		Setlist: &executor.SetlistResult{
			ShowID:       1,
			Performances: []*data.Performance{{Position: 1, SongName: "Bertha"}},
		},
	}
	out, err := formatJSON(result)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "setlist"`)
	require.Contains(t, out, "Bertha")
}

func TestResultTypeStr(t *testing.T) {
	require.Equal(t, "shows", resultTypeStr(executor.ResultShows))
	require.Equal(t, "songs", resultTypeStr(executor.ResultSongs))
	require.Equal(t, "performances", resultTypeStr(executor.ResultPerformances))
	require.Equal(t, "setlist", resultTypeStr(executor.ResultSetlist))
	require.Equal(t, "count", resultTypeStr(executor.ResultCount))
}

// === CSV ===

func TestFormatCSV_Songs(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSongs,
		Songs: []*data.Song{
			{ID: 1, Name: "Scarlet Begonias", ShortName: "Scarlet", Writers: "Hunter/Garcia"},
		},
	}
	out, err := formatCSV(result)
	require.NoError(t, err)
	require.Contains(t, out, "id,name,short_name,writers,times_played")
	require.Contains(t, out, "Scarlet Begonias")
	require.Contains(t, out, "Hunter/Garcia")
}

func TestFormatCSV_Performances(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultPerformances,
		Performances: []*data.Performance{
			{ID: 1, ShowID: 1, SongID: 6, SetNumber: 1, Position: 3, SegueType: ">", LengthSeconds: 580},
		},
	}
	out, err := formatCSV(result)
	require.NoError(t, err)
	require.Contains(t, out, "id,show_id,song_id")
	require.Contains(t, out, "580")
}

func TestFormatCSV_Setlist(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSetlist,
		Setlist: &executor.SetlistResult{
			Performances: []*data.Performance{
				{SetNumber: 1, Position: 1, SegueType: ">", LengthSeconds: 580},
				{SetNumber: 1, Position: 2, SegueType: "", LengthSeconds: 620},
			},
		},
	}
	out, err := formatCSV(result)
	require.NoError(t, err)
	require.Contains(t, out, "set_number,position")
	require.Contains(t, out, "580")
	require.Contains(t, out, "620")
}

// === Setlist format ===

func TestFormatSetlist_Basic(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSetlist,
		Setlist: &executor.SetlistResult{
			Date: time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC),
			Performances: []*data.Performance{
				{SetNumber: 1, Position: 1, SongName: "Minglewood Blues"},
				{SetNumber: 1, Position: 2, SongName: "Loser"},
				{SetNumber: 2, Position: 1, SongName: "Scarlet Begonias", SegueType: ">"},
				{SetNumber: 2, Position: 2, SongName: "Fire on the Mountain"},
			},
		},
	}
	out, err := formatSetlist(result)
	require.NoError(t, err)
	require.Contains(t, out, "Set 1")
	require.Contains(t, out, "Set 2")
	require.Contains(t, out, "Minglewood Blues")
	require.Contains(t, out, "Scarlet Begonias")
	require.Contains(t, out, "Fire on the Mountain")
	require.Contains(t, out, "1977")
}

func TestFormatSetlist_FallsBackForNonSetlist(t *testing.T) {
	result := &executor.Result{Type: executor.ResultShows, Shows: nil}
	out, err := formatSetlist(result)
	require.NoError(t, err)
	require.Contains(t, out, "No shows")
}

func TestFmtSetName(t *testing.T) {
	require.Equal(t, "Set 1", fmtSetName(1))
	require.Equal(t, "Set 2", fmtSetName(2))
	require.Equal(t, "Set 3 / Encore", fmtSetName(3))
	require.Equal(t, "Soundcheck", fmtSetName(0))
	require.Equal(t, "Set 7", fmtSetName(7))
}

// === Table setlist ===

func TestTableSetlist(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSetlist,
		Setlist: &executor.SetlistResult{
			Date:   time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC),
			ShowID: 1,
			Performances: []*data.Performance{
				{SetNumber: 1, Position: 1, SongName: "Minglewood Blues"},
				{SetNumber: 1, Position: 2, SongName: "Loser"},
			},
		},
	}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "1977-05-08")
	require.Contains(t, out, "Minglewood Blues")
}

func TestTableSongs(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultSongs,
		Songs: []*data.Song{
			{ID: 1, Name: "Scarlet Begonias", TimesPlayed: 314},
		},
	}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "Scarlet Begonias")
	require.Contains(t, out, "314")
}

func TestTableShows_WithRows(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultShows,
		Shows: []*data.Show{
			{ID: 1, Date: time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC), Venue: "Barton Hall", City: "Ithaca", State: "NY"},
		},
	}
	out, err := formatTable(result)
	require.NoError(t, err)
	require.Contains(t, out, "Barton Hall")
	require.Contains(t, out, "Ithaca")
	require.Contains(t, out, "1977-05-08")
}

func TestFormatSetlist_MultiShow(t *testing.T) {
	result := &executor.Result{
		Type: executor.ResultShows,
		Shows: []*data.Show{
			{ID: 1, Date: time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC), Venue: "Barton Hall"},
		},
		Setlists: []*executor.SetlistResult{
			{
				Date:   time.Date(1977, 5, 8, 0, 0, 0, 0, time.UTC),
				ShowID: 1,
				Performances: []*data.Performance{
					{ShowID: 1, SetNumber: 1, Position: 1, SongName: "Minglewood Blues"},
					{ShowID: 1, SetNumber: 1, Position: 2, SongName: "Loser"},
					{ShowID: 1, SetNumber: 2, Position: 1, SongName: "Scarlet Begonias"},
				},
			},
			{
				Date:   time.Date(1977, 5, 9, 0, 0, 0, 0, time.UTC),
				ShowID: 2,
				Performances: []*data.Performance{
					{ShowID: 2, SetNumber: 1, Position: 1, SongName: "Jack Straw"},
				},
			},
		},
	}
	f := New()
	out, err := f.Format(result, FormatSetlist)
	require.NoError(t, err)
	require.Contains(t, out, "Minglewood Blues")
	require.Contains(t, out, "Jack Straw")
	require.Contains(t, out, "Set 1")
	require.Contains(t, out, "Set 2")
	require.Contains(t, out, "---") // separator between shows
}
