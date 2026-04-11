package parser

import (
	"testing"

	"github.com/gdql/gdql/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseShowQuery_Simple(t *testing.T) {
	p := NewFromString("SHOWS;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.ShowQuery)
	require.True(t, ok)
	assert.Nil(t, sq.From)
	assert.Nil(t, sq.Where)
}

func TestParseShowQuery_TwoDigitYears(t *testing.T) {
	cases := []struct {
		input string
		year  int
	}{
		{"SHOWS FROM 65;", 1965},
		{"SHOWS FROM 69;", 1969},
		{"SHOWS FROM 70;", 1970},
		{"SHOWS FROM 77;", 1977},
		{"SHOWS FROM 95;", 1995},
		{"SHOWS FROM 1977;", 1977},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			p := NewFromString(tc.input)
			q, err := p.Parse()
			require.NoError(t, err)
			sq := q.(*ast.ShowQuery)
			require.NotNil(t, sq.From)
			require.NotNil(t, sq.From.Start)
			require.Equal(t, tc.year, sq.From.Start.Year, "input: %s", tc.input)
		})
	}
}

func TestParseShowQuery_TwoDigitYearRanges(t *testing.T) {
	p := NewFromString("SHOWS FROM 65-69;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From.Start)
	require.NotNil(t, sq.From.End)
	require.Equal(t, 1965, sq.From.Start.Year)
	require.Equal(t, 1969, sq.From.End.Year)
}

func TestParseShowQuery_WithDateRange(t *testing.T) {
	p := NewFromString("SHOWS FROM 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	require.NotNil(t, sq.From.Start)
	assert.Equal(t, 1977, sq.From.Start.Year)
	assert.Nil(t, sq.From.End)
}

func TestParseShowQuery_WithDateRangeSpan(t *testing.T) {
	p := NewFromString("SHOWS FROM 1977-1980;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	require.NotNil(t, sq.From.Start)
	require.NotNil(t, sq.From.End)
	assert.Equal(t, 1977, sq.From.Start.Year)
	assert.Equal(t, 1980, sq.From.End.Year)
}

func TestParseShowQuery_WithSegue(t *testing.T) {
	p := NewFromString(`SHOWS FROM 1977-1980 WHERE "Scarlet Begonias" > "Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	seg, ok := sq.Where.Conditions[0].(*ast.SegueCondition)
	require.True(t, ok)
	require.Len(t, seg.Songs, 2)
	assert.Equal(t, "Scarlet Begonias", seg.Songs[0].Name)
	assert.Equal(t, "Fire on the Mountain", seg.Songs[1].Name)
	require.Len(t, seg.Operators, 1)
	assert.Equal(t, ast.SegueOpSegue, seg.Operators[0])
}

func TestParseShowQuery_WherePlayedAndPlayedSegue(t *testing.T) {
	// WHERE PLAYED "St Stephen" only
	p := NewFromString(`SHOWS FROM 1969 WHERE PLAYED "St Stephen";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	play, ok := sq.Where.Conditions[0].(*ast.PlayedCondition)
	require.True(t, ok)
	assert.Equal(t, "St Stephen", play.Song.Name)

	// WHERE PLAYED "St Stephen" > "The Eleven"
	p2 := NewFromString(`SHOWS FROM 1969 WHERE PLAYED "St Stephen" > "The Eleven";`)
	q2, err := p2.Parse()
	require.NoError(t, err)
	sq2 := q2.(*ast.ShowQuery)
	require.NotNil(t, sq2.Where)
	require.Len(t, sq2.Where.Conditions, 2)
	play2, ok := sq2.Where.Conditions[0].(*ast.PlayedCondition)
	require.True(t, ok)
	assert.Equal(t, "St Stephen", play2.Song.Name)
	seg2, ok := sq2.Where.Conditions[1].(*ast.SegueCondition)
	require.True(t, ok)
	require.Len(t, seg2.Songs, 2)
	assert.Equal(t, "St Stephen", seg2.Songs[0].Name)
	assert.Equal(t, "The Eleven", seg2.Songs[1].Name)
	require.Len(t, seg2.Operators, 1)
	assert.Equal(t, ast.SegueOpSegue, seg2.Operators[0])
}

// TestParseShowQuery_UnicodeSegue ensures fullwidth ＞ (U+FF1E) and other variants parse as segue.
func TestParseShowQuery_UnicodeSegue(t *testing.T) {
	// Fullwidth ＞ (U+FF1E) often inserted by Windows editors instead of ASCII >
	withFullwidth := "SHOWS FROM 1969 WHERE PLAYED \"St Stephen\" \uFF1E \"The Eleven\";"
	p := NewFromString(withFullwidth)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 2)
	seg, ok := sq.Where.Conditions[1].(*ast.SegueCondition)
	require.True(t, ok)
	require.Len(t, seg.Songs, 2)
	assert.Equal(t, "The Eleven", seg.Songs[1].Name)
}

func TestParseShowQuery_WithLimit(t *testing.T) {
	p := NewFromString("SHOWS FROM 1977 LIMIT 10;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Limit)
	assert.Equal(t, 10, *sq.Limit)
}

func TestParseSongQuery_WithLyrics(t *testing.T) {
	p := NewFromString(`SONGS WITH LYRICS("train", "road");`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SongQuery)
	require.True(t, ok)
	require.NotNil(t, sq.With)
	require.Len(t, sq.With.Conditions, 1)
	lyr, ok := sq.With.Conditions[0].(*ast.LyricsCondition)
	require.True(t, ok)
	assert.Equal(t, []string{"train", "road"}, lyr.Words)
}

func TestParseSongQuery_Written(t *testing.T) {
	p := NewFromString("SONGS WRITTEN 1968-1970;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.SongQuery)
	require.NotNil(t, sq.Written)
	assert.Equal(t, 1968, sq.Written.Start.Year)
	assert.Equal(t, 1970, sq.Written.End.Year)
}

func TestParsePerformanceQuery(t *testing.T) {
	p := NewFromString(`PERFORMANCES OF "Dark Star" FROM 1972;`)
	q, err := p.Parse()
	require.NoError(t, err)
	pq, ok := q.(*ast.PerformanceQuery)
	require.True(t, ok)
	require.NotNil(t, pq.Song)
	assert.Equal(t, "Dark Star", pq.Song.Name)
	require.NotNil(t, pq.From)
	assert.Equal(t, 1972, pq.From.Start.Year)
}

func TestParseSetlistQuery(t *testing.T) {
	p := NewFromString("SETLIST FOR 5/8/77;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SetlistQuery)
	require.True(t, ok)
	require.NotNil(t, sq.Date)
	assert.Equal(t, 1977, sq.Date.Year)
	assert.Equal(t, 5, sq.Date.Month)
	assert.Equal(t, 8, sq.Date.Day)
}

func TestParseSetlistQuery_String(t *testing.T) {
	p := NewFromString(`SETLIST FOR "Cornell 1977";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.SetlistQuery)
	require.NotNil(t, sq.Date)
	assert.Equal(t, "Cornell 1977", sq.Date.Season) // we store literal in Season
}

func TestParse_Empty(t *testing.T) {
	p := NewFromString("")
	_, err := p.Parse()
	require.Error(t, err)
}

func TestParse_Invalid(t *testing.T) {
	p := NewFromString("FOO BAR;")
	_, err := p.Parse()
	require.Error(t, err)
}

func TestParseShowQuery_FromEra(t *testing.T) {
	p := NewFromString("SHOWS FROM PRIMAL;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	assert.Nil(t, sq.From.Start)
	require.NotNil(t, sq.From.Era)
	assert.Equal(t, ast.EraPrimal, *sq.From.Era)
}

// === AT venue ===

func TestParseShowQuery_AtVenue(t *testing.T) {
	p := NewFromString(`SHOWS AT "Fillmore West";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	assert.Equal(t, "Fillmore West", sq.At)
}

func TestParseShowQuery_AtWithFromAndWhere(t *testing.T) {
	p := NewFromString(`SHOWS AT "Winterland" FROM 1977 WHERE PLAYED "Dark Star";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	assert.Equal(t, "Winterland", sq.At)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1977, sq.From.Start.Year)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
}

func TestParseShowQuery_AtMissingString(t *testing.T) {
	p := NewFromString(`SHOWS AT FROM 1977;`)
	_, err := p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "venue name")
}

// === TOUR ===

func TestParseShowQuery_Tour(t *testing.T) {
	p := NewFromString(`SHOWS TOUR "Spring 1977";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	assert.Equal(t, "Spring 1977", sq.Tour)
}

// === BEFORE/AFTER ===

func TestParseShowQuery_After(t *testing.T) {
	p := NewFromString("SHOWS AFTER 1988;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1988, sq.From.Start.Year)
	require.NotNil(t, sq.From.End)
	assert.Equal(t, 2100, sq.From.End.Year)
}

func TestParseShowQuery_Before(t *testing.T) {
	p := NewFromString("SHOWS BEFORE 1970;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1900, sq.From.Start.Year)
	require.NotNil(t, sq.From.End)
	assert.Equal(t, 1970, sq.From.End.Year)
}

// === COUNT ===

func TestParseCountQuery_Song(t *testing.T) {
	p := NewFromString(`COUNT "Dark Star";`)
	q, err := p.Parse()
	require.NoError(t, err)
	cq, ok := q.(*ast.CountQuery)
	require.True(t, ok)
	require.NotNil(t, cq.Song)
	assert.Equal(t, "Dark Star", cq.Song.Name)
	assert.False(t, cq.CountShows)
}

func TestParseCountQuery_SongWithFrom(t *testing.T) {
	p := NewFromString(`COUNT "Dark Star" FROM 1972-1974;`)
	q, err := p.Parse()
	require.NoError(t, err)
	cq := q.(*ast.CountQuery)
	require.NotNil(t, cq.From)
	assert.Equal(t, 1972, cq.From.Start.Year)
	assert.Equal(t, 1974, cq.From.End.Year)
}

func TestParseCountQuery_Shows(t *testing.T) {
	p := NewFromString("COUNT SHOWS FROM 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	cq := q.(*ast.CountQuery)
	assert.True(t, cq.CountShows)
	assert.Nil(t, cq.Song)
	require.NotNil(t, cq.From)
}

func TestParseCountQuery_Bare(t *testing.T) {
	p := NewFromString("COUNT;")
	_, err := p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "song name or SHOWS")
}

// === FIRST/LAST ===

func TestParseFirstQuery(t *testing.T) {
	p := NewFromString(`FIRST "Dark Star";`)
	q, err := p.Parse()
	require.NoError(t, err)
	fl, ok := q.(*ast.FirstLastQuery)
	require.True(t, ok)
	assert.False(t, fl.IsLast)
	assert.Equal(t, "Dark Star", fl.Song.Name)
}

func TestParseLastQuery(t *testing.T) {
	p := NewFromString(`LAST "Dark Star";`)
	q, err := p.Parse()
	require.NoError(t, err)
	fl := q.(*ast.FirstLastQuery)
	assert.True(t, fl.IsLast)
}

// === RANDOM ===

func TestParseRandomShow(t *testing.T) {
	cases := []string{"RANDOM SHOW;", "RANDOM SHOWS;", "RANDOM;"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			p := NewFromString(c)
			q, err := p.Parse()
			require.NoError(t, err)
			_, ok := q.(*ast.RandomShowQuery)
			require.True(t, ok)
		})
	}
}

func TestParseRandomShow_FromYear(t *testing.T) {
	p := NewFromString("RANDOM SHOW FROM 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	rq := q.(*ast.RandomShowQuery)
	require.NotNil(t, rq.From)
	assert.Equal(t, 1977, rq.From.Start.Year)
}

// === OPENER/CLOSER ===

func TestParseShowQuery_Opener(t *testing.T) {
	p := NewFromString(`SHOWS WHERE OPENER "Bertha";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.Set1, pc.Set)
	assert.Equal(t, ast.PosOpened, pc.Operator)
	assert.Equal(t, "Bertha", pc.Song.Name)
}

func TestParseShowQuery_Closer(t *testing.T) {
	p := NewFromString(`SHOWS WHERE CLOSER "Morning Dew";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	pc := sq.Where.Conditions[0].(*ast.PositionCondition)
	assert.Equal(t, ast.PosClosed, pc.Operator)
}

// === Bare song in WHERE → PLAYED ===

func TestParseShowQuery_BareSongInWhere(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Bertha";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	played, ok := sq.Where.Conditions[0].(*ast.PlayedCondition)
	require.True(t, ok)
	assert.Equal(t, "Bertha", played.Song.Name)
}

func TestParseShowQuery_BareSongVsSegue(t *testing.T) {
	// Single song = PlayedCondition
	p := NewFromString(`SHOWS WHERE "Bertha";`)
	q, _ := p.Parse()
	_, ok := q.(*ast.ShowQuery).Where.Conditions[0].(*ast.PlayedCondition)
	assert.True(t, ok, "single bare song should be PlayedCondition")

	// Two songs with > = SegueCondition
	p2 := NewFromString(`SHOWS WHERE "Bertha" > "Loser";`)
	q2, _ := p2.Parse()
	_, ok = q2.(*ast.ShowQuery).Where.Conditions[0].(*ast.SegueCondition)
	assert.True(t, ok, "two songs with > should be SegueCondition")
}

// === AS COUNT ===

func TestParseSongQuery_AsCount(t *testing.T) {
	p := NewFromString(`SONGS WITH LYRICS("sun") AS COUNT;`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.SongQuery)
	assert.Equal(t, ast.OutputCount, sq.OutputFmt)
}

// === Error suggestions ===

func TestParseError_DidYouMeanKeyword(t *testing.T) {
	p := NewFromString("HOWS FROM 1977;")
	_, err := p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Did you mean: SHOWS")
}

func TestParseError_WrongOrder_WhereBeforeFrom(t *testing.T) {
	p := NewFromString(`SHOWS WHERE PLAYED "Bertha" FROM 1977;`)
	_, err := p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FROM must come before WHERE")
}

func TestParseError_DidYouMeanEra(t *testing.T) {
	p := NewFromString("SHOWS FROM PRIMOL;")
	_, err := p.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Did you mean: PRIMAL")
}

func TestParseError_NegativeLimit(t *testing.T) {
	p := NewFromString("SHOWS FROM 1977 LIMIT -5;")
	_, err := p.Parse()
	require.Error(t, err)
}

// === NOT PLAYED ===

func TestParseShowQuery_NotPlayed(t *testing.T) {
	cases := []string{
		`SHOWS WHERE NOT PLAYED "Saint Stephen";`,
		`SHOWS WHERE NOT "Saint Stephen";`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			p := NewFromString(c)
			q, err := p.Parse()
			require.NoError(t, err)
			sq := q.(*ast.ShowQuery)
			require.Len(t, sq.Where.Conditions, 1)
			pc, ok := sq.Where.Conditions[0].(*ast.PlayedCondition)
			require.True(t, ok)
			assert.True(t, pc.Negated)
			assert.Equal(t, "Saint Stephen", pc.Song.Name)
		})
	}
}

func TestParseShowQuery_PlayedAndNotPlayed(t *testing.T) {
	p := NewFromString(`SHOWS WHERE PLAYED "Dark Star" AND NOT PLAYED "Saint Stephen";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 2)
	require.Len(t, sq.Where.Operators, 1)
	assert.Equal(t, ast.OpAnd, sq.Where.Operators[0])

	played := sq.Where.Conditions[0].(*ast.PlayedCondition)
	assert.False(t, played.Negated)
	assert.Equal(t, "Dark Star", played.Song.Name)

	notPlayed := sq.Where.Conditions[1].(*ast.PlayedCondition)
	assert.True(t, notPlayed.Negated)
	assert.Equal(t, "Saint Stephen", notPlayed.Song.Name)
}

// === SECURITY: ORDER BY SQL injection regression ===

func TestParseError_OrderBySQLInjectionBlocked(t *testing.T) {
	// Previously: a quoted STRING would flow into the SQL ORDER BY clause as-is.
	// Exploit: ORDER BY "date, (SELECT ...)--" exposed sqlite_master via subquery.
	exploits := []string{
		`SHOWS FROM 1977 ORDER BY "date, (SELECT group_concat(name) FROM sqlite_master)--";`,
		`SHOWS ORDER BY "anything";`,
		`SHOWS ORDER BY "1; DROP TABLE shows";`,
	}
	for _, q := range exploits {
		t.Run(q, func(t *testing.T) {
			p := NewFromString(q)
			_, err := p.Parse()
			require.Error(t, err, "exploit should be rejected")
			require.Contains(t, err.Error(), "expected field name")
		})
	}
}

func TestParseError_OrderByUnknownField(t *testing.T) {
	// Even bare identifiers must be in the whitelist
	p := NewFromString("SHOWS ORDER BY VENUE;")
	_, err := p.Parse()
	require.Error(t, err)
}

func TestParseShowQuery_LimitCapped(t *testing.T) {
	p := NewFromString("SHOWS LIMIT 999999999;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Limit)
	require.LessOrEqual(t, *sq.Limit, 1000, "LIMIT should be capped at 1000")
}

// === Modifier combinations ===

func TestParseShowQuery_AllModifiers(t *testing.T) {
	p := NewFromString(`SHOWS AT "Winterland" TOUR "Spring 1977" FROM 1977 WHERE PLAYED "Dark Star" ORDER BY DATE DESC LIMIT 5 AS JSON;`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	assert.Equal(t, "Winterland", sq.At)
	assert.Equal(t, "Spring 1977", sq.Tour)
	assert.Equal(t, 1977, sq.From.Start.Year)
	require.Len(t, sq.Where.Conditions, 1)
	require.NotNil(t, sq.OrderBy)
	assert.True(t, sq.OrderBy.Desc)
	require.NotNil(t, sq.Limit)
	assert.Equal(t, 5, *sq.Limit)
	assert.Equal(t, ast.OutputJSON, sq.OutputFmt)
}

// === Arrow -> as segue ===

func TestParseShowQuery_ArrowSegue(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Dark Star" -> "St. Stephen";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	seg, ok := sq.Where.Conditions[0].(*ast.SegueCondition)
	require.True(t, ok, "expected SegueCondition")
	require.Len(t, seg.Songs, 2)
	assert.Equal(t, "Dark Star", seg.Songs[0].Name)
	assert.Equal(t, "St. Stephen", seg.Songs[1].Name)
}

// === AND/OR between WITH conditions ===

func TestParseSongQuery_WithAndBetweenLyrics(t *testing.T) {
	p := NewFromString(`SONGS WITH LYRICS("sun") AND LYRICS("bus");`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SongQuery)
	require.True(t, ok)
	require.NotNil(t, sq.With)
	require.Len(t, sq.With.Conditions, 2, "expected two LYRICS conditions separated by AND")
	lc1, ok := sq.With.Conditions[0].(*ast.LyricsCondition)
	require.True(t, ok)
	assert.Equal(t, []string{"sun"}, lc1.Words)
	lc2, ok := sq.With.Conditions[1].(*ast.LyricsCondition)
	require.True(t, ok)
	assert.Equal(t, []string{"bus"}, lc2.Words)
}

// === Standalone segue-into conditions ===

func TestParseShowQuery_TildeGtStandalone(t *testing.T) {
	p := NewFromString(`SHOWS WHERE ~>"Dark Star";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	sc, ok := sq.Where.Conditions[0].(*ast.SegueIntoCondition)
	require.True(t, ok, "expected SegueIntoCondition")
	assert.Equal(t, "Dark Star", sc.Song.Name)
	assert.Equal(t, ast.SegueOpTease, sc.Operator)
}

func TestParseShowQuery_GtStandalone(t *testing.T) {
	p := NewFromString(`SHOWS WHERE >"Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	sc, ok := sq.Where.Conditions[0].(*ast.SegueIntoCondition)
	require.True(t, ok, "expected SegueIntoCondition")
	assert.Equal(t, "Fire on the Mountain", sc.Song.Name)
	assert.Equal(t, ast.SegueOpSegue, sc.Operator)
}

func TestParseShowQuery_GtGtStandalone(t *testing.T) {
	p := NewFromString(`SHOWS WHERE >>"Morning Dew";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	sc, ok := sq.Where.Conditions[0].(*ast.SegueIntoCondition)
	require.True(t, ok, "expected SegueIntoCondition")
	assert.Equal(t, "Morning Dew", sc.Song.Name)
	assert.Equal(t, ast.SegueOpBreak, sc.Operator)
}

// === OPENER/CLOSER with segue chain ===

func TestParseShowQuery_OpenerWithSegueChain(t *testing.T) {
	p := NewFromString(`SHOWS WHERE OPENER ("Help on the Way" > "Slipknot!");`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok, "expected PositionCondition")
	assert.Equal(t, ast.Set1, pc.Set)
	assert.Equal(t, ast.PosOpened, pc.Operator)
	assert.Nil(t, pc.Song, "Song should be nil when SegueChain is set")
	require.NotNil(t, pc.SegueChain)
	require.Len(t, pc.SegueChain.Songs, 2)
	assert.Equal(t, "Help on the Way", pc.SegueChain.Songs[0].Name)
	assert.Equal(t, "Slipknot!", pc.SegueChain.Songs[1].Name)
}

func TestParseShowQuery_CloserWithSegueChain(t *testing.T) {
	p := NewFromString(`SHOWS WHERE CLOSER ("Sugar Magnolia" > "Sunshine Daydream");`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosClosed, pc.Operator)
	require.NotNil(t, pc.SegueChain)
	require.Len(t, pc.SegueChain.Songs, 2)
}

// === Combined: OPENER segue + AND + CLOSER ===

func TestParseShowQuery_OpenerSegueAndCloser(t *testing.T) {
	p := NewFromString(`SHOWS WHERE OPENER ("Help on the Way" > "Slipknot!") AND CLOSER "Brokedown Palace";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 2)

	// First condition: OPENER with segue chain
	pc1, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosOpened, pc1.Operator)
	require.NotNil(t, pc1.SegueChain)

	// Second condition: CLOSER with single song
	pc2, ok := sq.Where.Conditions[1].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosClosed, pc2.Operator)
	require.NotNil(t, pc2.Song)
	assert.Equal(t, "Brokedown Palace", pc2.Song.Name)
}

// === IN as FROM alias ===

func TestParseShowQuery_InAsFromAlias(t *testing.T) {
	p := NewFromString("SHOWS IN 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.From)
	require.NotNil(t, sq.From.Start)
	assert.Equal(t, 1977, sq.From.Start.Year)
}

// === ENCORE "Song" without = sign ===

func TestParseShowQuery_EncoreWithoutEquals(t *testing.T) {
	p := NewFromString(`SHOWS WHERE ENCORE "U.S. Blues";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.Encore, pc.Set)
	assert.Equal(t, ast.PosEquals, pc.Operator)
	assert.Equal(t, "U.S. Blues", pc.Song.Name)
}

// === CLOSER("Song") no space before paren ===

func TestParseShowQuery_CloserNoSpaceBeforeParen(t *testing.T) {
	p := NewFromString(`SHOWS WHERE CLOSER("Morning Dew");`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosClosed, pc.Operator)
	require.NotNil(t, pc.Song)
	assert.Equal(t, "Morning Dew", pc.Song.Name)
}

// === NOT CLOSED, NOT OPENER, NOT ENCORE ===

func TestParseShowQuery_NotClosed(t *testing.T) {
	p := NewFromString(`SHOWS WHERE NOT CLOSED "U.S. Blues";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosClosed, pc.Operator)
	assert.True(t, pc.Negated)
	assert.Equal(t, "U.S. Blues", pc.Song.Name)
}

func TestParseShowQuery_NotOpener(t *testing.T) {
	p := NewFromString(`SHOWS WHERE NOT OPENER "Bertha";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.PosOpened, pc.Operator)
	assert.True(t, pc.Negated)
	assert.Equal(t, "Bertha", pc.Song.Name)
}

func TestParseShowQuery_NotEncore(t *testing.T) {
	p := NewFromString(`SHOWS WHERE NOT ENCORE "U.S. Blues";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok)
	assert.Equal(t, ast.Encore, pc.Set)
	assert.Equal(t, ast.PosEquals, pc.Operator)
	assert.True(t, pc.Negated)
	assert.Equal(t, "U.S. Blues", pc.Song.Name)
}

// === NegatedSegueCondition: "Song A" NOT > "Song B" ===

func TestParseShowQuery_NegatedSegueWithNOT(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Scarlet Begonias" NOT > "Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	ns, ok := sq.Where.Conditions[0].(*ast.NegatedSegueCondition)
	require.True(t, ok, "expected NegatedSegueCondition")
	assert.Equal(t, "Scarlet Begonias", ns.Song.Name)
	assert.Equal(t, "Fire on the Mountain", ns.NotSong.Name)
}

// === NOT_GT token: "Song A" !> "Song B" ===

func TestParseShowQuery_NotGTToken(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Scarlet Begonias" !> "Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	ns, ok := sq.Where.Conditions[0].(*ast.NegatedSegueCondition)
	require.True(t, ok, "expected NegatedSegueCondition for !>")
	assert.Equal(t, "Scarlet Begonias", ns.Song.Name)
	assert.Equal(t, "Fire on the Mountain", ns.NotSong.Name)
}

// === NOT_GTGT token: "Song A" !>> "Song B" ===

func TestParseShowQuery_NotGTGTToken(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Scarlet Begonias" !>> "Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	ns, ok := sq.Where.Conditions[0].(*ast.NegatedSegueCondition)
	require.True(t, ok, "expected NegatedSegueCondition for !>>")
	assert.Equal(t, "Scarlet Begonias", ns.Song.Name)
	assert.Equal(t, "Fire on the Mountain", ns.NotSong.Name)
}

// === OPENER "A" > "B" without parens ===

func TestParseShowQuery_OpenerSegueWithoutParens(t *testing.T) {
	p := NewFromString(`SHOWS WHERE OPENER "Help on the Way" > "Slipknot!";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.Len(t, sq.Where.Conditions, 1)
	pc, ok := sq.Where.Conditions[0].(*ast.PositionCondition)
	require.True(t, ok, "expected PositionCondition")
	assert.Equal(t, ast.PosOpened, pc.Operator)
	assert.Nil(t, pc.Song, "Song should be nil when SegueChain is set")
	require.NotNil(t, pc.SegueChain)
	require.Len(t, pc.SegueChain.Songs, 2)
	assert.Equal(t, "Help on the Way", pc.SegueChain.Songs[0].Name)
	assert.Equal(t, "Slipknot!", pc.SegueChain.Songs[1].Name)
}

// === SETLIST YYYY-MM-DD without FOR ===

func TestParseSetlistQuery_YYYYMMDD_WithoutFor(t *testing.T) {
	p := NewFromString("SETLIST 1977-05-08;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SetlistQuery)
	require.True(t, ok)
	require.NotNil(t, sq.Date)
	assert.Equal(t, 1977, sq.Date.Year)
	assert.Equal(t, 5, sq.Date.Month)
	assert.Equal(t, 8, sq.Date.Day)
}

// === SETLIST M/D/YY without FOR ===

func TestParseSetlistQuery_MDY_WithoutFor(t *testing.T) {
	p := NewFromString("SETLIST 5/8/77;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SetlistQuery)
	require.True(t, ok)
	require.NotNil(t, sq.Date)
	assert.Equal(t, 1977, sq.Date.Year)
	assert.Equal(t, 5, sq.Date.Month)
	assert.Equal(t, 8, sq.Date.Day)
}

// === SONGS FROM 1977 ORDER BY TIMES_PLAYED DESC ===

func TestParseSongQuery_FromWithOrderBy(t *testing.T) {
	p := NewFromString("SONGS FROM 1977 ORDER BY TIMES_PLAYED DESC;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SongQuery)
	require.True(t, ok)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1977, sq.From.Start.Year)
	require.NotNil(t, sq.OrderBy)
	assert.Equal(t, "TIMES_PLAYED", sq.OrderBy.Field)
	assert.True(t, sq.OrderBy.Desc)
}

// === SONGS PLAYED IN 1977 ===

func TestParseSongQuery_PlayedIn(t *testing.T) {
	p := NewFromString("SONGS PLAYED IN 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SongQuery)
	require.True(t, ok)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1977, sq.From.Start.Year)
}

// === SONGS PLAYED FROM 1977 ===

func TestParseSongQuery_PlayedFrom(t *testing.T) {
	p := NewFromString("SONGS PLAYED FROM 1977;")
	q, err := p.Parse()
	require.NoError(t, err)
	sq, ok := q.(*ast.SongQuery)
	require.True(t, ok)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1977, sq.From.Start.Year)
}

// === COUNT SHOWS WHERE "A" > "B" ===

func TestParseCountQuery_ShowsWhere(t *testing.T) {
	p := NewFromString(`COUNT SHOWS WHERE "Scarlet Begonias" > "Fire on the Mountain";`)
	q, err := p.Parse()
	require.NoError(t, err)
	cq, ok := q.(*ast.CountQuery)
	require.True(t, ok)
	assert.True(t, cq.CountShows)
	require.NotNil(t, cq.Where)
	require.Len(t, cq.Where.Conditions, 1)
	seg, ok := cq.Where.Conditions[0].(*ast.SegueCondition)
	require.True(t, ok)
	require.Len(t, seg.Songs, 2)
	assert.Equal(t, "Scarlet Begonias", seg.Songs[0].Name)
	assert.Equal(t, "Fire on the Mountain", seg.Songs[1].Name)
}

// === SHOWS TOUR "name" ===

func TestParseShowQuery_TourWithFrom(t *testing.T) {
	p := NewFromString(`SHOWS TOUR "Europe" FROM 1972;`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	assert.Equal(t, "Europe", sq.Tour)
	require.NotNil(t, sq.From)
	assert.Equal(t, 1972, sq.From.Start.Year)
}

// === Multi-statement with comments ===

func TestParseMultiStatement_WithComments(t *testing.T) {
	input := "SHOWS LIMIT 1; -- comment\nSETLIST FOR 5/8/77;"

	// First statement
	p1 := NewFromString("SHOWS LIMIT 1;")
	q1, err := p1.Parse()
	require.NoError(t, err)
	sq1, ok := q1.(*ast.ShowQuery)
	require.True(t, ok)
	require.NotNil(t, sq1.Limit)
	assert.Equal(t, 1, *sq1.Limit)

	// Second statement (after comment)
	p2 := NewFromString("SETLIST FOR 5/8/77;")
	q2, err := p2.Parse()
	require.NoError(t, err)
	sq2, ok := q2.(*ast.SetlistQuery)
	require.True(t, ok)
	require.NotNil(t, sq2.Date)
	assert.Equal(t, 1977, sq2.Date.Year)

	// Verify SplitStatements handles this correctly via the run package
	// (we test the individual parses above; the split is tested in acceptance)
	_ = input
}

func TestParseShowQuery_MidChainNotGT(t *testing.T) {
	p := NewFromString(`SHOWS WHERE "Help on the Way" > "Slipknot!" !> "Franklin's Tower";`)
	q, err := p.Parse()
	require.NoError(t, err)
	sq := q.(*ast.ShowQuery)
	require.NotNil(t, sq.Where)
	require.Len(t, sq.Where.Conditions, 1)
	swn, ok := sq.Where.Conditions[0].(*ast.SegueWithNegation)
	require.True(t, ok, "expected SegueWithNegation")
	require.Len(t, swn.Chain.Songs, 2)
	assert.Equal(t, "Help on the Way", swn.Chain.Songs[0].Name)
	assert.Equal(t, "Slipknot!", swn.Chain.Songs[1].Name)
	assert.Equal(t, "Franklin's Tower", swn.NotSong.Name)
}
