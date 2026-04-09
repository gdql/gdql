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
