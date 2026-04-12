package executor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/ir"
	"github.com/gdql/gdql/internal/parser"
	"github.com/gdql/gdql/internal/planner"
	"github.com/gdql/gdql/internal/planner/expander"
	"github.com/gdql/gdql/internal/planner/resolver"
	"github.com/gdql/gdql/internal/planner/sqlgen"
)

// ResultType identifies the kind of query result.
type ResultType int

const (
	ResultShows ResultType = iota
	ResultSongs
	ResultPerformances
	ResultSetlist
	ResultCount
)

// CountResult is the result of a COUNT query.
type CountResult struct {
	SongName string
	Count    int
}

// Result is the output of executing a query.
type Result struct {
	Type         ResultType
	Shows        []*data.Show
	Songs        []*data.Song
	Performances []*data.Performance
	Setlist      *SetlistResult
	Setlists     []*SetlistResult // AS SETLIST on SHOWS queries
	Count        *CountResult
	OutputFmt    ir.OutputFormat
	SQL          string
	Duration     time.Duration
}

// SetlistResult is the result of a SETLIST query.
type SetlistResult struct {
	Date         time.Time
	ShowID       int
	Venue        string `json:",omitempty"`
	City         string `json:",omitempty"`
	State        string `json:",omitempty"`
	Performances []*data.Performance
}

// Executor runs a GDQL query end-to-end.
type Executor interface {
	Execute(ctx context.Context, query string) (*Result, error)
	ExecuteAST(ctx context.Context, q ast.Query) (*Result, error)
}

type executor struct {
	planner    planner.Planner
	sqlGen     sqlgen.SQLGenerator
	dataSource data.DataSource
}

// New builds an Executor that uses the given DataSource for resolution and execution.
func New(ds data.DataSource) Executor {
	songResolver := resolver.NewDataSourceResolver(ds)
	dateExpander := expander.New()
	pl := planner.New(songResolver, dateExpander)
	return &executor{
		planner:    pl,
		sqlGen:     sqlgen.New(),
		dataSource: ds,
	}
}

// Execute parses the query string and runs it.
func (e *executor) Execute(ctx context.Context, query string) (*Result, error) {
	p := parser.NewFromString(query)
	ast, err := p.Parse()
	if err != nil {
		return nil, err
	}
	return e.ExecuteAST(ctx, ast)
}

// ExecuteAST plans, generates SQL, executes, and maps rows to Result.
func (e *executor) ExecuteAST(ctx context.Context, q ast.Query) (*Result, error) {
	start := time.Now()

	irQ, err := e.planner.Plan(ctx, q)
	if err != nil {
		return nil, err
	}
	sq, err := e.sqlGen.Generate(irQ)
	if err != nil {
		return nil, err
	}

	rs, err := e.dataSource.ExecuteQuery(ctx, sq.SQL, sq.Args...)
	if err != nil {
		return nil, err
	}

	out := &Result{SQL: sq.SQL, Duration: time.Since(start), OutputFmt: irQ.OutputFmt}
	switch irQ.Type {
	case ir.QueryTypeShows:
		out.Type = ResultShows
		out.Shows, err = mapRowsToShows(rs)
		// AS SETLIST: expand each show into its full setlist
		if err == nil && irQ.OutputFmt == ir.OutputSetlist && len(out.Shows) > 0 {
			var setlists []*SetlistResult
			maxExpand := 20
			if len(out.Shows) < maxExpand {
				maxExpand = len(out.Shows)
			}
			for _, show := range out.Shows[:maxExpand] {
				perfRS, perr := e.dataSource.ExecuteQuery(ctx,
					"SELECT p.id, p.show_id, p.song_id, p.set_number, p.position, p.segue_type, p.length_seconds, songs.name FROM performances p JOIN songs ON p.song_id = songs.id WHERE p.show_id = ? ORDER BY p.set_number, p.position",
					show.ID)
				if perr != nil {
					continue
				}
				perfs, _ := mapRowsToPerformances(perfRS)
				setlists = append(setlists, &SetlistResult{
					Date:         show.Date,
					ShowID:       show.ID,
					Venue:        show.Venue,
					City:         show.City,
					State:        show.State,
					Performances: perfs,
				})
			}
			out.Setlists = setlists
		}
	case ir.QueryTypeSongs:
		if irQ.OutputFmt == ir.OutputCount {
			out.Type = ResultCount
			out.Count = mapRowsToCount(rs)
		} else {
			out.Type = ResultSongs
			out.Songs, err = mapRowsToSongs(rs)
		}
	case ir.QueryTypePerformances:
		out.Type = ResultPerformances
		out.Performances, err = mapRowsToPerformances(rs)
	case ir.QueryTypeSetlist:
		out.Type = ResultSetlist
		out.Setlist, err = mapRowsToSetlist(rs, irQ.SingleDate)
		// Populate venue context so the sandbox setlist header can show
		// "5/8/77 · Barton Hall, Ithaca NY" instead of bare date.
		if err == nil && out.Setlist != nil && out.Setlist.ShowID > 0 && e.dataSource != nil {
			if vrs, verr := e.dataSource.ExecuteQuery(ctx,
				"SELECT v.name, v.city, v.state FROM shows s LEFT JOIN venues v ON s.venue_id = v.id WHERE s.id = ?",
				out.Setlist.ShowID); verr == nil && len(vrs.Rows) > 0 {
				row := vrs.Rows[0]
				out.Setlist.Venue = strVal(row[0])
				out.Setlist.City = strVal(row[1])
				out.Setlist.State = strVal(row[2])
			}
		}
	case ir.QueryTypeCount:
		out.Type = ResultCount
		out.Count = mapRowsToCount(rs)
	case ir.QueryTypeFirstLast:
		out.Type = ResultShows
		out.Shows, err = mapRowsToShows(rs)
	case ir.QueryTypeRandomShow:
		out.Type = ResultSetlist
		out.Setlist, err = mapRowsToSetlist(rs, nil)
		// Get the actual show date from DB for display
		if out.Setlist != nil && out.Setlist.ShowID > 0 {
			var dateStr string
			if e.dataSource != nil {
				if drs, derr := e.dataSource.ExecuteQuery(ctx, "SELECT date FROM shows WHERE id = ?", out.Setlist.ShowID); derr == nil && len(drs.Rows) > 0 {
					dateStr = strVal(drs.Rows[0][0])
					t, _ := time.Parse("2006-01-02", dateStr)
					out.Setlist.Date = t
				}
			}
		}
	default:
		return nil, fmt.Errorf("unknown query type %d", irQ.Type)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func mapRowsToShows(rs *data.ResultSet) ([]*data.Show, error) {
	out := make([]*data.Show, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		if len(row) < 7 {
			continue
		}
		sh := &data.Show{
			ID:      intVal(row[0]),
			VenueID: intVal(row[2]),
			Venue:   strVal(row[3]),
			City:    strVal(row[4]),
			State:   strVal(row[5]),
			Tour:    strVal(row[6]),
		}
		sh.Date = timeVal(row[1])
		// If state is empty but city contains "City, ST" or "City, ST, Country", extract state
		if sh.State == "" && sh.City != "" {
			sh.City, sh.State = splitCityState(sh.City)
		}
		out = append(out, sh)
	}
	return out, nil
}

// splitCityState parses "City, ST" or "City, ST, Country" → (city, state).
// Returns the input city unchanged if it doesn't match the pattern.
func splitCityState(s string) (city, state string) {
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return s, ""
	}
	city = strings.TrimSpace(parts[0])
	state = strings.TrimSpace(parts[1])
	// Drop trailing country if state is too long to be a US state code
	if len(state) > 6 {
		// Probably full country name, not a state
		return city, ""
	}
	return city, state
}

func mapRowsToSongs(rs *data.ResultSet) ([]*data.Song, error) {
	out := make([]*data.Song, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		if len(row) < 7 {
			continue
		}
		s := &data.Song{
			ID:          intVal(row[0]),
			Name:        normalizeSongName(strVal(row[1])),
			ShortName:   strVal(row[2]),
			Writers:     strVal(row[3]),
			TimesPlayed: intVal(row[6]),
		}
		s.FirstPlayed = timeVal(row[4])
		s.LastPlayed = timeVal(row[5])
		out = append(out, s)
	}
	return out, nil
}

func mapRowsToPerformances(rs *data.ResultSet) ([]*data.Performance, error) {
	out := make([]*data.Performance, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		if len(row) < 7 {
			continue
		}
		perf := &data.Performance{
			ID:            intVal(row[0]),
			ShowID:        intVal(row[1]),
			SongID:        intVal(row[2]),
			SetNumber:     intVal(row[3]),
			Position:      intVal(row[4]),
			SegueType:     strVal(row[5]),
			LengthSeconds: intVal(row[6]),
		}
		if len(row) >= 8 {
			perf.SongName = strVal(row[7])
		}
		if len(row) >= 9 {
			d := strVal(row[8])
			if len(d) >= 10 {
				d = d[:10]
			}
			perf.Date = d
		}
		if len(row) >= 10 {
			perf.Venue = strVal(row[9])
		}
		out = append(out, perf)
	}
	return out, nil
}

func mapRowsToSetlist(rs *data.ResultSet, singleDate *time.Time) (*SetlistResult, error) {
	perfs, err := mapRowsToPerformances(rs)
	if err != nil || len(perfs) == 0 {
		return &SetlistResult{Performances: perfs}, err
	}
	date := time.Time{}
	if singleDate != nil {
		date = *singleDate
	}
	return &SetlistResult{
		Date:         date,
		ShowID:       perfs[0].ShowID,
		Performances: perfs,
	}, nil
}

func mapRowsToCount(rs *data.ResultSet) *CountResult {
	if len(rs.Rows) == 0 {
		return &CountResult{}
	}
	row := rs.Rows[0]
	cr := &CountResult{Count: intVal(row[0])}
	if len(row) >= 2 {
		cr.SongName = strVal(row[1])
	}
	return cr
}

func intVal(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		if n, err := strconv.Atoi(x); err == nil {
			return n
		}
	}
	return 0
}

func floatVal(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func strVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func timeVal(v interface{}) time.Time {
	if s, ok := v.(string); ok {
		t, _ := time.Parse("2006-01-02", s)
		return t
	}
	return time.Time{}
}

// normalizeSongName converts ALL CAPS song names to title case for display.
// Names that are already mixed-case are returned unchanged.
func normalizeSongName(name string) string {
	if name == "" {
		return name
	}
	// Only normalize if the name is all uppercase (with allowed non-letter chars)
	hasLower := false
	for _, r := range name {
		if unicode.IsLower(r) {
			hasLower = true
			break
		}
	}
	if hasLower {
		return name
	}
	// Title-case: capitalize first letter of each word, lowercase rest.
	// Preserve short words (articles/prepositions) in lowercase except at start.
	lower := map[string]bool{
		"a": true, "an": true, "the": true, "of": true, "on": true,
		"in": true, "at": true, "to": true, "for": true, "and": true,
		"or": true, "but": true, "is": true, "it": true, "by": true,
	}
	words := strings.Fields(strings.ToLower(name))
	for i, w := range words {
		if i == 0 || !lower[w] {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
