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
	// Fixed parts (venue, tour, date) — always ANDed
	var fixedParts []string
	if q.VenueName != "" {
		fixedParts = append(fixedParts, "(v.name LIKE ? ESCAPE '\\' OR v.city LIKE ? ESCAPE '\\')")
		args = append(args, "%"+escapeLike(q.VenueName)+"%", "%"+escapeLike(q.VenueName)+"%")
	}
	if q.TourName != "" {
		fixedParts = append(fixedParts, "s.tour LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLike(q.TourName)+"%")
	}
	if q.DateRange != nil {
		fixedParts = append(fixedParts, "s.date >= ? AND s.date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	// Condition parts — respect AND/OR operators
	var condParts []string
	for _, c := range q.Conditions {
		switch x := c.(type) {
		case *ir.PositionConditionIR:
			part, a := g.positionCondition(x)
			condParts = append(condParts, part)
			args = append(args, a...)
		case *ir.PlayedConditionIR:
			placeholders := make([]string, len(x.SongIDs))
			for i, id := range x.SongIDs {
				placeholders[i] = "?"
				args = append(args, id)
			}
			inClause := "p.song_id IN (" + strings.Join(placeholders, ",") + ")"
			if x.Negated {
				condParts = append(condParts, "NOT EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND "+inClause+")")
			} else {
				condParts = append(condParts, "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND "+inClause+")")
			}
		case *ir.GuestConditionIR:
			condParts = append(condParts, "EXISTS (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.guest IS NOT NULL AND p.guest != '' AND (p.guest = ? OR p.guest LIKE ? ESCAPE '\\'))")
			args = append(args, x.Name, "%"+escapeLike(x.Name)+"%")
		case *ir.SegueIntoConditionIR:
			part, a := segueIntoCondition(x)
			condParts = append(condParts, part)
			args = append(args, a...)
		case *ir.NegatedSegueConditionIR:
			part, a := negatedSegueCondition(x)
			condParts = append(condParts, part)
			args = append(args, a...)
		}
	}
	// Build combined WHERE
	var whereParts []string
	whereParts = append(whereParts, fixedParts...)
	if len(condParts) > 0 {
		var condSQL strings.Builder
		condSQL.WriteString(condParts[0])
		for i := 1; i < len(condParts); i++ {
			op := " AND "
			if i-1 < len(q.ConditionOps) && q.ConditionOps[i-1] == ir.OpOr {
				op = " OR "
			}
			condSQL.WriteString(op)
			condSQL.WriteString(condParts[i])
		}
		condStr := condSQL.String()
		if len(fixedParts) > 0 && strings.Contains(condStr, " OR ") {
			condStr = "(" + condStr + ")"
		}
		whereParts = append(whereParts, condStr)
	}
	return strings.Join(whereParts, " AND "), args
}

// segueIntoCondition generates SQL for standalone segue-into conditions: >"Song", >>"Song", ~>"Song".
// The segue_type is stored on the *preceding* performance row.
func segueIntoCondition(c *ir.SegueIntoConditionIR) (string, []interface{}) {
	placeholders := make([]string, len(c.SongIDs))
	var args []interface{}
	for i, id := range c.SongIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	inClause := strings.Join(placeholders, ",")
	var segueMatch string
	switch c.Operator {
	case ir.SegueOpTease:
		segueMatch = "prev.segue_type = '~>'"
	case ir.SegueOpBreak:
		// >> means played in same show but not directly segued
		segueMatch = "(prev.segue_type IS NULL OR prev.segue_type = '' OR prev.segue_type = '>>')"
	default:
		// > means directly segued into
		segueMatch = "prev.segue_type = '>'"
	}
	sql := "EXISTS (SELECT 1 FROM performances p JOIN performances prev ON prev.show_id = p.show_id AND prev.set_number = p.set_number AND prev.position = p.position - 1 WHERE p.show_id = s.id AND p.song_id IN (" + inClause + ") AND " + segueMatch + ")"
	return sql, args
}

func (g *generator) positionCondition(c *ir.PositionConditionIR) (string, []interface{}) {
	if c.SegueChain != nil {
		return positionConditionWithSegue(c)
	}
	// Build set filter: Encore matches set >= 3 (covers both set 3 and set 4 encores)
	var setFilter string
	var setArgs []interface{}
	isEncore := c.Set == ir.Encore
	setNum := setPositionToNumber(c.Set)
	if isEncore {
		setFilter = " AND p.set_number >= 3"
	} else if setNum > 0 {
		setFilter = " AND p.set_number = ?"
		setArgs = append(setArgs, setNum)
	}

	exists := "EXISTS"
	if c.Negated {
		exists = "NOT EXISTS"
	}

	switch c.Operator {
	case ir.PosOpened:
		args := append(setArgs, c.SongID)
		if setFilter == "" {
			return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ? AND p.is_opener = 1)", []interface{}{c.SongID}
		}
		return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ? AND p.is_opener = 1)", args
	case ir.PosClosed:
		args := append(setArgs, c.SongID)
		if setFilter == "" {
			return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ? AND p.is_closer = 1)", []interface{}{c.SongID}
		}
		return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ? AND p.is_closer = 1)", args
	case ir.PosEquals:
		args := append(setArgs, c.SongID)
		if setFilter == "" {
			return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id AND p.song_id = ?)", []interface{}{c.SongID}
		}
		return exists + " (SELECT 1 FROM performances p WHERE p.show_id = s.id" + setFilter + " AND p.song_id = ?)", args
	}
	return "", nil
}

// positionConditionWithSegue handles OPENER/CLOSER with a segue chain.
// E.g., OPENER ("Help on the Way" > "Slipknot!") means: first song in some set
// is "Help on the Way" and next song is "Slipknot!" with adjacency.
func positionConditionWithSegue(c *ir.PositionConditionIR) (string, []interface{}) {
	chain := c.SegueChain
	n := len(chain.SongIDs)
	if n < 2 {
		return "", nil
	}
	ops := chain.Operators
	for len(ops) < n-1 {
		ops = append(ops, ir.SegueOpSegue)
	}

	var b strings.Builder
	var args []interface{}
	b.WriteString("EXISTS (SELECT 1 FROM ")
	for i := 0; i < n; i++ {
		alias := fmt.Sprintf("pc%d", i+1)
		if i == 0 {
			b.WriteString("performances " + alias)
		} else {
			prev := fmt.Sprintf("pc%d", i)
			b.WriteString(" JOIN performances " + alias + " ON " + joinForOp(prev, alias, ops[i-1]))
		}
	}
	b.WriteString(" WHERE pc1.show_id = s.id")

	// First song must be opener or closer
	if c.Operator == ir.PosOpened {
		b.WriteString(" AND pc1.is_opener = 1")
	} else if c.Operator == ir.PosClosed {
		// Last song in chain must be closer
		b.WriteString(fmt.Sprintf(" AND pc%d.is_closer = 1", n))
	}

	// Match each song ID
	for i, id := range chain.SongIDs {
		fmt.Fprintf(&b, " AND pc%d.song_id = ?", i+1)
		args = append(args, id)
	}
	b.WriteString(")")
	return b.String(), args
}

// setPositionToNumber maps set position to set_number.
// Returns 0 for Encore — callers handle encore specially with >= 3.
func setPositionToNumber(s ir.SetPosition) int {
	switch s {
	case ir.Set1:
		return 1
	case ir.Set2:
		return 2
	case ir.Set3:
		return 3
	case ir.Encore:
		return 0 // handled specially
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
				// Whole-word match: normalize punctuation to spaces, then match with space boundaries
				likes[i] = "(' ' || REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(LOWER(l.lyrics), ',', ' '), '.', ' '), '!', ' '), '?', ' '), '''', ' ') || ' ') LIKE ?"
				args = append(args, "% "+strings.ToLower(w)+" %")
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
	// COUNT SHOWS with WHERE/segue — reuse the shows query and wrap in COUNT
	if q.SongID == nil && (q.SegueChain != nil || len(q.Conditions) > 0) {
		showsQ := &ir.QueryIR{
			Type:       ir.QueryTypeShows,
			DateRange:  q.DateRange,
			VenueName:  q.VenueName,
			SegueChain: q.SegueChain,
			Conditions: q.Conditions,
		}
		inner, err := g.genShows(showsQ)
		if err != nil {
			return nil, err
		}
		sql := "SELECT count(*) AS count, 'shows' AS name FROM (" + inner.SQL + ")"
		return &SQLQuery{SQL: sql, Args: inner.Args}, nil
	}

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

// escapeLike escapes LIKE pattern metacharacters (% and _) in user input.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// negatedSegueCondition: "Song A" NOT > "Song B"
// Shows where Song A was played and the next song in the same set was NOT Song B.
func negatedSegueCondition(c *ir.NegatedSegueConditionIR) (string, []interface{}) {
	sql := `EXISTS (SELECT 1 FROM performances pa
		WHERE pa.show_id = s.id AND pa.song_id = ?
		AND NOT EXISTS (SELECT 1 FROM performances pb
			WHERE pb.show_id = pa.show_id AND pb.set_number = pa.set_number
			AND pb.position = pa.position + 1 AND pb.song_id = ?))`
	return sql, []interface{}{c.SongID, c.NotSongID}
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
