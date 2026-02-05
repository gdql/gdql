package expander

import (
	"time"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/ir"
)

// DateExpander expands AST date ranges and era aliases into concrete time ranges.
type DateExpander interface {
	Expand(*ast.DateRange) (*ir.ResolvedDateRange, error)
	ExpandEra(ast.EraAlias) (*ir.ResolvedDateRange, error)
	ExpandDate(*ast.Date) (time.Time, error)
}

type dateExpander struct{}

// New returns a DateExpander.
func New() DateExpander {
	return &dateExpander{}
}

func (d *dateExpander) Expand(dr *ast.DateRange) (*ir.ResolvedDateRange, error) {
	if dr == nil {
		return nil, nil
	}
	if dr.Era != nil {
		return d.ExpandEra(*dr.Era)
	}
	if dr.Start == nil {
		return nil, nil
	}
	start := time.Date(dr.Start.Year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(dr.Start.Year, 12, 31, 23, 59, 59, 0, time.UTC)
	if dr.End != nil {
		end = time.Date(dr.End.Year, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	return &ir.ResolvedDateRange{Start: start, End: end}, nil
}

func (d *dateExpander) ExpandEra(era ast.EraAlias) (*ir.ResolvedDateRange, error) {
	var start, end time.Time
	switch era {
	case ast.EraPrimal:
		start = time.Date(1965, 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC)
	case ast.EraEurope72:
		start = time.Date(1972, 3, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1972, 5, 31, 23, 59, 59, 0, time.UTC)
	case ast.EraWallOfSound:
		start = time.Date(1974, 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1974, 12, 31, 23, 59, 59, 0, time.UTC)
	case ast.EraHiatus:
		start = time.Date(1975, 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1975, 12, 31, 23, 59, 59, 0, time.UTC)
	case ast.EraBrent:
		start = time.Date(1979, 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1990, 12, 31, 23, 59, 59, 0, time.UTC)
	case ast.EraVince:
		start = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(1995, 12, 31, 23, 59, 59, 0, time.UTC)
	default:
		return nil, nil
	}
	return &ir.ResolvedDateRange{Start: start, End: end}, nil
}

func (d *dateExpander) ExpandDate(date *ast.Date) (time.Time, error) {
	if date == nil {
		return time.Time{}, nil
	}
	year := date.Year
	if year == 0 {
		year = 1970
	}
	month := date.Month
	if month == 0 {
		month = 1
	}
	day := date.Day
	if day == 0 {
		day = 1
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}
