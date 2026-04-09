package formatter

import (
	"testing"

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
