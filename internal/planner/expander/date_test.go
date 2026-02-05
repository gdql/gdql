package expander

import (
	"testing"
	"time"

	"github.com/gdql/gdql/internal/ast"
	"github.com/stretchr/testify/require"
)

func TestExpand_YearOnly(t *testing.T) {
	de := New()
	dr := &ast.DateRange{Start: &ast.Date{Year: 1977}}
	r, err := de.Expand(dr)
	require.NoError(t, err)
	require.Equal(t, time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC), r.Start)
	require.Equal(t, time.Date(1977, 12, 31, 23, 59, 59, 0, time.UTC), r.End)
}

func TestExpand_YearRange(t *testing.T) {
	de := New()
	dr := &ast.DateRange{
		Start: &ast.Date{Year: 1977},
		End:   &ast.Date{Year: 1980},
	}
	r, err := de.Expand(dr)
	require.NoError(t, err)
	require.Equal(t, time.Date(1977, 1, 1, 0, 0, 0, 0, time.UTC), r.Start)
	require.Equal(t, time.Date(1980, 12, 31, 23, 59, 59, 0, time.UTC), r.End)
}

func TestExpand_NilRange(t *testing.T) {
	de := New()
	r, err := de.Expand(nil)
	require.NoError(t, err)
	require.Nil(t, r)
}

func TestExpand_EraPrimal(t *testing.T) {
	de := New()
	era := ast.EraPrimal
	r, err := de.ExpandEra(era)
	require.NoError(t, err)
	require.Equal(t, time.Date(1965, 1, 1, 0, 0, 0, 0, time.UTC), r.Start)
	require.True(t, r.End.Year() == 1969)
}

func TestExpand_EraEurope72(t *testing.T) {
	de := New()
	r, err := de.ExpandEra(ast.EraEurope72)
	require.NoError(t, err)
	require.Equal(t, 1972, r.Start.Year())
	require.Equal(t, 3, int(r.Start.Month()))
	require.Equal(t, 1972, r.End.Year())
	require.Equal(t, 5, int(r.End.Month()))
}

func TestExpandDate_SingleDay(t *testing.T) {
	de := New()
	d := &ast.Date{Year: 1977, Month: 5, Day: 8}
	tm, err := de.ExpandDate(d)
	require.NoError(t, err)
	require.Equal(t, 1977, tm.Year())
	require.Equal(t, 5, int(tm.Month()))
	require.Equal(t, 8, tm.Day())
}

func TestExpandDate_YearOnly(t *testing.T) {
	de := New()
	d := &ast.Date{Year: 1977}
	tm, err := de.ExpandDate(d)
	require.NoError(t, err)
	require.Equal(t, 1977, tm.Year())
	require.Equal(t, 1, int(tm.Month()))
	require.Equal(t, 1, tm.Day())
}

func TestExpandDate_Nil(t *testing.T) {
	de := New()
	tm, err := de.ExpandDate(nil)
	require.NoError(t, err)
	require.True(t, tm.IsZero())
}
