package sqlgen

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/ir"
)

// BuildSegueShowsSQL builds SELECT DISTINCT shows for a segue chain (2+ songs).
func BuildSegueShowsSQL(q *ir.QueryIR) (*SQLQuery, error) {
	chain := q.SegueChain
	if chain == nil || len(chain.SongIDs) < 2 {
		return nil, fmt.Errorf("segue chain requires at least 2 songs")
	}
	n := len(chain.SongIDs)
	ops := chain.Operators
	if len(ops) < n-1 {
		// Pad with segue
		for len(ops) < n-1 {
			ops = append(ops, ir.SegueOpSegue)
		}
	}

	var b strings.Builder
	var args []interface{}

	// SELECT DISTINCT s.id, s.date, ...
	b.WriteString("SELECT DISTINCT s.id, s.date, s.venue_id, v.name AS venue, v.city, v.state, s.notes, s.rating FROM ")
	// p1 JOIN p2 ON ... JOIN p3 ON ... JOIN songs s1 ON p1.song_id = s1.id AND s1.id = ? ...
	for i := 0; i < n; i++ {
		alias := fmt.Sprintf("p%d", i+1)
		if i == 0 {
			b.WriteString("performances " + alias)
		} else {
			prev := fmt.Sprintf("p%d", i)
			b.WriteString(" JOIN performances " + alias + " ON " + prev + ".show_id = " + alias + ".show_id AND " + prev + ".set_number = " + alias + ".set_number AND " + prev + ".position = " + alias + ".position - 1")
		}
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, " JOIN songs s%d ON p%d.song_id = s%d.id AND s%d.id = ?", i+1, i+1, i+1, i+1)
		args = append(args, chain.SongIDs[i])
	}
	// segue_type for each transition
	for i := 0; i < n-1; i++ {
		fmt.Fprintf(&b, " AND p%d.segue_type = ?", i+1)
		args = append(args, segueOpToSQL(ops[i]))
	}
	b.WriteString(" JOIN shows s ON p1.show_id = s.id LEFT JOIN venues v ON s.venue_id = v.id")

	var whereParts []string
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
			whereParts = append(whereParts, "EXISTS (SELECT 1 FROM performances px WHERE px.show_id = s.id AND px.song_id = ?)")
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

func segueOpToSQL(op ir.SegueOp) string {
	switch op {
	case ir.SegueOpSegue:
		return ">"
	case ir.SegueOpBreak:
		return ">>"
	case ir.SegueOpTease:
		return "~>"
	}
	return ">"
}
