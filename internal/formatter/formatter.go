package formatter

import (
	"fmt"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/ir"
)

// OutputFormat selects output style.
type OutputFormat int

const (
	FormatTable OutputFormat = iota
	FormatJSON
	FormatCSV
	FormatTSV
	FormatSetlist
	FormatCalendar
)

// Formatter renders a Result as a string.
type Formatter interface {
	Format(result *executor.Result, format OutputFormat) (string, error)
}

type formatter struct{}

// New returns a Formatter.
func New() Formatter {
	return &formatter{}
}

// Format dispatches to the appropriate formatter by format.
func (f *formatter) Format(result *executor.Result, format OutputFormat) (string, error) {
	switch format {
	case FormatJSON:
		return formatJSON(result)
	case FormatCSV:
		return formatCSV(result)
	case FormatTSV:
		return formatTSV(result)
	case FormatSetlist:
		return formatSetlist(result)
	case FormatCalendar:
		return "", fmt.Errorf("CALENDAR output format is not yet implemented")
	default:
		return formatTable(result)
	}
}

// FromIR converts ir.OutputFormat to formatter.OutputFormat.
func FromIR(o ir.OutputFormat) OutputFormat {
	switch o {
	case ir.OutputJSON:
		return FormatJSON
	case ir.OutputCSV:
		return FormatCSV
	case ir.OutputTSV:
		return FormatTSV
	case ir.OutputSetlist:
		return FormatSetlist
	case ir.OutputTable:
		return FormatTable
	case ir.OutputCalendar:
		return FormatCalendar
	case ir.OutputCount:
		return FormatTable // count results use table formatter's count handler
	}
	return FormatTable
}
