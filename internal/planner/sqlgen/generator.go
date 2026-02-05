package sqlgen

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdql/gdql/internal/ir"
)

// SQLQuery is a parameterized SQL statement.
type SQLQuery struct {
	SQL  string
	Args []interface{}
}

// SQLGenerator generates SQL from IR.
type SQLGenerator interface {
	Generate(*ir.QueryIR) (*SQLQuery, error)
}

type generator struct{}

// New returns a SQLGenerator.
func New() SQLGenerator {
	return &generator{}
}

func (g *generator) Generate(q *ir.QueryIR) (*SQLQuery, error) {
	switch q.Type {
	case ir.QueryTypeShows:
		return g.genShows(q)
	case ir.QueryTypeSongs:
		return g.genSongs(q)
	case ir.QueryTypePerformances:
		return g.genPerformances(q)
	case ir.QueryTypeSetlist:
		return g.genSetlist(q)
	default:
		return nil, fmt.Errorf("unknown query type: %d", q.Type)
	}
}

func (g *generator) genShows(q *ir.QueryIR) (*SQLQuery, error) {
	if q.SegueChain != nil && len(q.SegueChain.SongIDs) >= 2 {
		return g.genShowsWithSegue(q)
	}
	var b strings.Builder
	var args []interface{}
	b.WriteString("SELECT s.id, s.date, s.venue_id, v.name AS venue, v.city, v.state, s.notes, s.rating FROM shows s LEFT JOIN venues v ON s.venue_id = v.id")
	where, wa := g.whereShows(q)
	if where != "" {
		b.WriteString(" WHERE ")
		b.WriteString(where)
		args = append(args, wa...)
	}
	order := g.orderBy(q, "s")
	if order != "" {
		b.WriteString(" ")
		b.WriteString(order)
	}
	limit := g.limit(q)
	if limit != "" && q.Limit != nil {
		b.WriteString(" ")
		b.WriteString(limit)
		args = append(args, *q.Limit)
	}
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

func (g *generator) whereShows(q *ir.QueryIR) (clause string, args []interface{}) {
	var parts []string
	if q.DateRange != nil {
		parts = append(parts, "s.date >= ? AND s.date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	for _, c := range q.Conditions {
		switch x := c.(type) {
		case *ir.PositionConditionIR:
			part, a := g.positionCondition(x)
			parts = append(parts, part)
			args = append(args, a...)
		case *ir.PlayedConditionIR:
			parts = append(parts, "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ?)")
			args = append(args, x.SongID)
		case *ir.GuestConditionIR:
			parts = append(parts, "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.guest IS NOT NULL AND p.guest != '' AND (p.guest = ? OR p.guest LIKE ?))")
			args = append(args, x.Name, "%"+x.Name+"%")
		}
	}
	return strings.Join(parts, " AND "), args
}

func (g *generator) positionCondition(c *ir.PositionConditionIR) (string, []interface{}) {
	setNum := setPositionToNumber(c.Set)
	switch c.Operator {
	case ir.PosOpened:
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.set_number = ? AND p.song_id = ? AND p.is_opener = 1)", []interface{}{setNum, c.SongID}
	case ir.PosClosed:
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.set_number = ? AND p.song_id = ? AND p.is_closer = 1)", []interface{}{setNum, c.SongID}
	case ir.PosEquals:
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.set_number = ? AND p.song_id = ?)", []interface{}{setNum, c.SongID}
	}
	return "", nil
}

func setPositionToNumber(s ir.SetPosition) int {
	switch s {
	case ir.Set1:
		return 1
	case ir.Set2:
		return 2
	case ir.Set3:
		return 3
	case ir.Encore:
		return 3
	}
	return 0
}

func (g *generator) genShowsWithSegue(q *ir.QueryIR) (*SQLQuery, error) {
	return BuildSegueShowsSQL(q)
}

func (g *generator) genSongs(q *ir.QueryIR) (*SQLQuery, error) {
	var b strings.Builder
	var args []interface{}
	b.WriteString("SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs")
	var parts []string
	for _, c := range q.Conditions {
		switch x := c.(type) {
		case *ir.LyricsConditionIR:
			if len(x.Words) == 0 {
				continue
			}
			likes := make([]string, len(x.Words))
			for i, w := range x.Words {
				likes[i] = "l.lyrics LIKE ?"
				args = append(args, "%"+w+"%")
			}
			op := " AND "
			if x.Operator == ir.OpOr {
				op = " OR "
			}
			parts = append(parts, "EXISTS (SELECT 1 FROM lyrics l WHERE l.song_id = songs.id AND ("+strings.Join(likes, op)+"))")
		}
	}
	if q.DateRange != nil {
		parts = append(parts, "first_played >= ? AND last_played <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	if len(parts) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(parts, " AND "))
	}
	order := g.orderBy(q, "songs")
	if order != "" {
		b.WriteString(" ")
		b.WriteString(order)
	}
	if q.Limit != nil {
		b.WriteString(" LIMIT ?")
		args = append(args, *q.Limit)
	}
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

func (g *generator) genPerformances(q *ir.QueryIR) (*SQLQuery, error) {
	var b strings.Builder
	var args []interface{}
	b.WriteString("SELECT p.id, p.show_id, p.song_id, p.set_number, p.position, p.segue_type, p.length_seconds FROM performances p JOIN shows s ON p.show_id = s.id WHERE p.song_id = ?")
	args = append(args, *q.SongID)
	if q.DateRange != nil {
		b.WriteString(" AND s.date >= ? AND s.date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	for _, c := range q.Conditions {
		if l, ok := c.(*ir.LengthConditionIR); ok {
			b.WriteString(" AND p.length_seconds " + compOpSQL(l.Operator) + " ?")
			args = append(args, l.Seconds)
		}
	}
	order := g.orderBy(q, "p")
	if order != "" {
		b.WriteString(" ")
		b.WriteString(order)
	}
	if q.Limit != nil {
		b.WriteString(" LIMIT ?")
		args = append(args, *q.Limit)
	}
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

func (g *generator) genSetlist(q *ir.QueryIR) (*SQLQuery, error) {
	if q.SingleDate == nil {
		return nil, fmt.Errorf("setlist query requires a date")
	}
	sql := "SELECT p.id, p.show_id, p.song_id, p.set_number, p.position, p.segue_type, p.length_seconds, songs.name FROM performances p JOIN shows s ON p.show_id = s.id JOIN songs ON p.song_id = songs.id WHERE s.date = ? ORDER BY p.set_number, p.position"
	return &SQLQuery{SQL: sql, Args: []interface{}{formatDate(*q.SingleDate)}}, nil
}

func (g *generator) orderBy(q *ir.QueryIR, prefix string) string {
	if q.OrderBy == nil {
		return ""
	}
	field := q.OrderBy.Field
	if field == "" {
		field = "date"
	}
	dir := "ASC"
	if q.OrderBy.Desc {
		dir = "DESC"
	}
	col := prefix + "." + strings.ToLower(field)
	switch strings.ToUpper(field) {
	case "DATE":
		col = prefix + ".date"
	case "LENGTH":
		if prefix == "p" {
			col = "p.length_seconds"
		}
	case "RATING":
		col = prefix + ".rating"
	case "NAME":
		col = prefix + ".name"
	case "TIMES_PLAYED":
		col = prefix + ".times_played"
	}
	return "ORDER BY " + col + " " + dir
}

func (g *generator) limit(q *ir.QueryIR) string {
	if q.Limit == nil {
		return ""
	}
	return "LIMIT ?"
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func compOpSQL(op ir.CompOp) string {
	switch op {
	case ir.CompGT:
		return ">"
	case ir.CompLT:
		return "<"
	case ir.CompEQ:
		return "="
	case ir.CompGTE:
		return ">="
	case ir.CompLTE:
		return "<="
	case ir.CompNEQ:
		return "!="
	}
	return ">"
}
