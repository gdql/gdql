package sqlgen

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/ir"
)

// BuildSegueShowsSQL builds SELECT DISTINCT shows for a segue chain (2+ songs).
//
// Operator semantics:
//   >  (segue):    songs are positionally adjacent in the same set (B at position A+1)
//   >> (then):     both songs played in the same show, A's position < B's position (not necessarily adjacent)
//   ~> (tease):    requires explicit segue_type='~>' metadata in performances row
func BuildSegueShowsSQL(q *ir.QueryIR) (*SQLQuery, error) {
	chain := q.SegueChain
	if chain == nil || len(chain.SongIDs) < 2 {
		return nil, fmt.Errorf("segue chain requires at least 2 songs")
	}
	n := len(chain.SongIDs)
	ops := chain.Operators
	if len(ops) < n-1 {
		for len(ops) < n-1 {
			ops = append(ops, ir.SegueOpSegue)
		}
	}

	var b strings.Builder
	var args []interface{}

	b.WriteString("SELECT DISTINCT s.id, s.date, s.venue_id, v.name AS venue, v.city, v.state, s.tour FROM ")
	for i := 0; i < n; i++ {
		alias := fmt.Sprintf("p%d", i+1)
		if i == 0 {
			b.WriteString("performances " + alias)
		} else {
			prev := fmt.Sprintf("p%d", i)
			op := ops[i-1]
			joinCond := joinForOp(prev, alias, op)
			b.WriteString(" JOIN performances " + alias + " ON " + joinCond)
		}
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, " JOIN songs s%d ON p%d.song_id = s%d.id AND s%d.id = ?", i+1, i+1, i+1, i+1)
		args = append(args, chain.SongIDs[i])
	}
	b.WriteString(" JOIN shows s ON p1.show_id = s.id LEFT JOIN venues v ON s.venue_id = v.id")

	var whereParts []string
	if q.VenueName != "" {
		whereParts = append(whereParts, "(v.name LIKE ? OR v.city LIKE ?)")
		args = append(args, "%"+q.VenueName+"%", "%"+q.VenueName+"%")
	}
	if q.DateRange != nil {
		whereParts = append(whereParts, "s.date >= ? AND s.date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	for _, c := range q.Conditions {
		switch x := c.(type) {
		case *ir.PositionConditionIR:
			setNum := setPositionToNumber(x.Set)
			switch x.Operator {
			case ir.PosOpened:
				whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ? AND px.is_opener = 1)")
			case ir.PosClosed:
				whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ? AND px.is_closer = 1)")
			case ir.PosEquals:
				whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ?)")
			}
			args = append(args, setNum, x.SongID)
		case *ir.PlayedConditionIR:
			if x.Negated {
				whereParts = append(whereParts, "NOT EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.song_id = ?)")
			} else {
				whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.song_id = ?)")
			}
			args = append(args, x.SongID)
		case *ir.GuestConditionIR:
			whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.guest IS NOT NULL AND (px.guest = ? OR px.guest LIKE ?))")
			args = append(args, x.Name, "%"+x.Name+"%")
		}
	}
	if len(whereParts) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(whereParts, " AND "))
	}
	if q.OrderBy != nil {
		dir := "ASC"
		if q.OrderBy.Desc {
			dir = "DESC"
		}
		b.WriteString(" ORDER BY s.date " + dir)
	}
	if q.Limit != nil {
		b.WriteString(" LIMIT ?")
		args = append(args, *q.Limit)
	}
	return &SQLQuery{SQL: b.String(), Args: args}, nil
}

// joinForOp returns the JOIN ON clause linking two adjacent performance aliases
// based on the segue operator semantics.
func joinForOp(prev, curr string, op ir.SegueOp) string {
	switch op {
	case ir.SegueOpBreak:
		// >> (then): same show, A's position before B's, but NOT adjacent.
		// Either different sets, or same set with position gap > 1.
		return prev + ".show_id = " + curr + ".show_id AND (" +
			prev + ".set_number != " + curr + ".set_number OR " +
			prev + ".position + 1 < " + curr + ".position)"
	case ir.SegueOpTease:
		// ~> (tease): adjacent AND segue_type='~>' explicitly recorded
		return prev + ".show_id = " + curr + ".show_id AND " +
			prev + ".set_number = " + curr + ".set_number AND " +
			prev + ".position = " + curr + ".position - 1 AND " +
			prev + ".segue_type = '~>'"
	default:
		// > (segue): positional adjacency in same set
		return prev + ".show_id = " + curr + ".show_id AND " +
			prev + ".set_number = " + curr + ".set_number AND " +
			prev + ".position = " + curr + ".position - 1"
	}
}
