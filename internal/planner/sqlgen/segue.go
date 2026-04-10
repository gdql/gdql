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

	// Fixed parts (venue, date) always ANDed
	var fixedParts []string
	if q.VenueName != "" {
		fixedParts = append(fixedParts, "(v.name LIKE ? OR v.city LIKE ?)")
		args = append(args, "%"+q.VenueName+"%", "%"+q.VenueName+"%")
	}
	if q.DateRange != nil {
		fixedParts = append(fixedParts, "s.date >= ? AND s.date <= ?")
		args = append(args, formatDate(q.DateRange.Start), formatDate(q.DateRange.End))
	}
	// Condition parts respect AND/OR operators
	var condParts []string
	for _, c := range q.Conditions {
		switch x := c.(type) {
		case *ir.PositionConditionIR:
			if x.SegueChain != nil {
				part, a := positionConditionWithSegue(x)
				condParts = append(condParts, part)
				args = append(args, a...)
			} else {
				setNum := setPositionToNumber(x.Set)
				switch x.Operator {
				case ir.PosOpened:
					condParts = append(condParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ? AND px.is_opener = 1)")
				case ir.PosClosed:
					condParts = append(condParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ? AND px.is_closer = 1)")
				case ir.PosEquals:
					condParts = append(condParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.set_number = ? AND px.song_id = ?)")
				}
				args = append(args, setNum, x.SongID)
			}
		case *ir.PlayedConditionIR:
			placeholders := make([]string, len(x.SongIDs))
			for i, id := range x.SongIDs {
				placeholders[i] = "?"
				args = append(args, id)
			}
			inClause := "px.song_id IN (" + strings.Join(placeholders, ",") + ")"
			if x.Negated {
				condParts = append(condParts, "NOT EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND "+inClause+")")
			} else {
				condParts = append(condParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND "+inClause+")")
			}
		case *ir.GuestConditionIR:
			condParts = append(condParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.guest IS NOT NULL AND (px.guest = ? OR px.guest LIKE ?))")
			args = append(args, x.Name, "%"+x.Name+"%")
		case *ir.SegueIntoConditionIR:
			part, a := segueIntoCondition(x)
			condParts = append(condParts, part)
			args = append(args, a...)
		}
	}
	// Build combined WHERE
	var whereParts []string
	whereParts = append(whereParts, fixedParts...)
	if len(condParts) > 0 {
		// Join conditions with their operators
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
		// Wrap in parens if mixed with fixed parts and has OR
		condStr := condSQL.String()
		if len(fixedParts) > 0 && strings.Contains(condStr, " OR ") {
			condStr = "(" + condStr + ")"
		}
		whereParts = append(whereParts, condStr)
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
