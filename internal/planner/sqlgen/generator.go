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
	case ir.QueryTypeCount:
		return g.genCount(q)
	case ir.QueryTypeFirstLast:
		return g.genFirstLast(q)
	case ir.QueryTypeRandomShow:
		return g.genRandomShow(q)
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
	b.WriteString("SELECT s.id, s.date, s.venue_id, v.name AS venue, v.city, v.state, s.tour FROM shows s LEFT JOIN venues v ON s.venue_id = v.id")
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
	if q.VenueName != "" {
		parts = append(parts, "(v.name LIKE ? OR v.city LIKE ?)")
		args = append(args, "%"+q.VenueName+"%", "%"+q.VenueName+"%")
	}
	if q.TourName != "" {
		parts = append(parts, "s.tour LIKE ?")
		args = append(args, "%"+q.TourName+"%")
	}
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
	setFilter := " AND p.set_number = ?"
	if setNum == 0 {
		setFilter = "" // SetAny: don't filter by set
	}
	switch c.Operator {
	case ir.PosOpened:
		if setNum == 0 {
			return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ? AND p.is_opener = 1)", []interface{}{c.SongID}
		}
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ? AND p.is_opener = 1)", []interface{}{setNum, c.SongID}
	case ir.PosClosed:
		if setNum == 0 {
			return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ? AND p.is_closer = 1)", []interface{}{c.SongID}
		}
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ? AND p.is_closer = 1)", []interface{}{setNum, c.SongID}
	case ir.PosEquals:
		if setNum == 0 {
			return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ?)", []interface{}{c.SongID}
		}
		return "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ?)", []interface{}{setNum, c.SongID}
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
		return 4
	}
	return 0
}

func (g *generator) genShowsWithSegue(q *ir.QueryIR) (*SQLQuery, error) {
	return BuildSegueShowsSQL(q)
}

func (g *generator) genSongs(q *ir.QueryIR) (*SQLQuery, error) {
	var b strings.Builder
	var args []interface{}
	isCount := q.OutputFmt == ir.OutputCount
	if isCount {
		b.WriteString("SELECT count(*) AS count, 'songs' AS name FROM songs")
	} else {
		b.WriteString("SELECT id, name, short_name, writers, first_played, last_played, times_played FROM songs")
	}
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

func (g *generator) genCount(q *ir.QueryIR) (*SQLQuery, error) {
	var b strings.Builder
	var args []interface{}
	if q.SongID == nil {
		// COUNT SHOWS
		b.WriteString("SELECT count(*) AS count, 'shows' AS name FROM shows s")
		if q.DateRange != nil {
			b.WriteString(" WHERE s.date >= ? AND s.date <= ?")
			args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
		}
	} else {
		b.WriteString("SELECT count(*) AS count, songs.name FROM performances p JOIN shows s ON p.show_id = s.id JOIN songs ON p.song_id = songs.id WHERE p.song_id = ?")
		args = append(args, *q.SongID)
		if q.DateRange != nil {
			b.WriteString(" AND s.date >= ? AND s.date <= ?")
			args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
		}
	}
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

func (g *generator) genFirstLast(q *ir.QueryIR) (*SQLQuery, error) {
	dir := "ASC"
	if q.IsLast {
		dir = "DESC"
	}
	sql := fmt.Sprintf("SELECT s.id, s.date, s.venue_id, v.name AS venue, v.city, v.state, s.tour FROM performances p JOIN shows s ON p.show_id = s.id LEFT JOIN venues v ON s.venue_id = v.id WHERE p.song_id = ? ORDER BY s.date %s LIMIT 1", dir)
	return &SQLQuery{SQL: sql, Args: []interface{}{*q.SongID}}, nil
}

func (g *generator) genRandomShow(q *ir.QueryIR) (*SQLQuery, error) {
	// Pick a random show, then return its setlist
	var b strings.Builder
	var args []interface{}
	b.WriteString("SELECT p.id, p.show_id, p.song_id, p.set_number, p.position, p.segue_type, p.length_seconds, songs.name FROM performances p JOIN shows s ON p.show_id = s.id JOIN songs ON p.song_id = songs.id WHERE s.id = (SELECT id FROM shows")
	if q.DateRange != nil {
		b.WriteString(" WHERE date >= ? AND date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	b.WriteString(" ORDER BY RANDOM() LIMIT 1) ORDER BY p.set_number, p.position")
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

func (g *generator) orderBy(q *ir.QueryIR, prefix string) string {
	if q.OrderBy == nil {
		return ""
	}
	field := strings.ToUpper(q.OrderBy.Field)
	if field == "" {
		field = "DATE"
	}
	dir := "ASC"
	if q.OrderBy.Desc {
		dir = "DESC"
	}
	// SECURITY: only allow whitelisted columns. Never interpolate user input.
	var col string
	switch field {
	case "DATE":
		if prefix == "p" {
			col = "s.date" // performances join shows as s
		} else {
			col = prefix + ".date"
		}
	case "LENGTH":
		if prefix == "p" {
			col = "p.length_seconds"
		} else {
			return "" // LENGTH only valid for performances
		}
	case "NAME":
		col = prefix + ".name"
	case "TIMES_PLAYED":
		col = prefix + ".times_played"
	case "POSITION":
		if prefix == "p" {
			col = "p.position"
		} else {
			return ""
		}
	default:
		// Unknown field — silently drop the ORDER BY rather than risk injection.
		return ""
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
