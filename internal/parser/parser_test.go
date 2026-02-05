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
