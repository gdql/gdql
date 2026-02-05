package planner

import (
	"context"
	"strconv"
	"strings"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/errors"
	"github.com/gdql/gdql/internal/ir"
	"github.com/gdql/gdql/internal/planner/expander"
	"github.com/gdql/gdql/internal/planner/resolver"
)

// Planner converts an AST query into IR (resolved song IDs, expanded dates).
type Planner interface {
	Plan(ctx context.Context, q ast.Query) (*ir.QueryIR, error)
}

type planner struct {
	songResolver resolver.SongResolver
	dateExpander expander.DateExpander
}

// New returns a Planner that uses the given resolver and date expander.
func New(sr resolver.SongResolver, de expander.DateExpander) Planner {
	return &planner{songResolver: sr, dateExpander: de}
}

func (p *planner) Plan(ctx context.Context, q ast.Query) (*ir.QueryIR, error) {
	switch x := q.(type) {
	case *ast.ShowQuery:
		return p.planShow(ctx, x)
	case *ast.SongQuery:
		return p.planSong(ctx, x)
	case *ast.PerformanceQuery:
		return p.planPerformance(ctx, x)
	case *ast.SetlistQuery:
		return p.planSetlist(x)
	default:
		return nil, nil
	}
}

func (p *planner) planShow(ctx context.Context, s *ast.ShowQuery) (*ir.QueryIR, error) {
	out := &ir.QueryIR{Type: ir.QueryTypeShows}
	var err error
	if s.From != nil {
		out.DateRange, err = p.dateExpander.Expand(s.From)
		if err != nil {
			return nil, err
		}
	}
	if s.Where != nil {
		for i, c := range s.Where.Conditions {
			if seg, ok := c.(*ast.SegueCondition); ok && i == 0 && out.SegueChain == nil {
		chain, err := p.segueToIR(ctx, seg)
		if err != nil {
			return nil, p.wrapSongNotFound(ctx, err)
		}
				out.SegueChain = chain
				continue
			}
			cond, err := p.conditionToIR(ctx, c)
			if err != nil {
				return nil, p.wrapSongNotFound(ctx, err)
			}
			if cond != nil {
				out.Conditions = append(out.Conditions, cond)
			}
		}
	}
	if s.OrderBy != nil {
		out.OrderBy = &ir.OrderByIR{Field: s.OrderBy.Field, Desc: s.OrderBy.Desc}
	}
	out.Limit = s.Limit
	out.OutputFmt = astOutputToIR(s.OutputFmt)
	return out, nil
}

func (p *planner) planSong(ctx context.Context, s *ast.SongQuery) (*ir.QueryIR, error) {
	out := &ir.QueryIR{Type: ir.QueryTypeSongs}
	if s.Written != nil {
		out.DateRange, _ = p.dateExpander.Expand(s.Written)
	}
	if s.With != nil {
		for _, c := range s.With.Conditions {
			cond, err := p.withConditionToIR(ctx, c)
			if err != nil {
				return nil, err
			}
			out.Conditions = append(out.Conditions, cond)
		}
	}
	if s.OrderBy != nil {
		out.OrderBy = &ir.OrderByIR{Field: s.OrderBy.Field, Desc: s.OrderBy.Desc}
	}
	out.Limit = s.Limit
	return out, nil
}

func (p *planner) planPerformance(ctx context.Context, perf *ast.PerformanceQuery) (*ir.QueryIR, error) {
	out := &ir.QueryIR{Type: ir.QueryTypePerformances}
	id, err := p.songResolver.Resolve(ctx, perf.Song.Name)
	if err != nil {
		return nil, p.wrapSongNotFound(ctx, err)
	}
	out.SongID = &id
	if perf.From != nil {
		out.DateRange, _ = p.dateExpander.Expand(perf.From)
	}
	if perf.With != nil {
		for _, c := range perf.With.Conditions {
			cond, err := p.withConditionToIR(ctx, c)
			if err != nil {
				return nil, err
			}
			out.Conditions = append(out.Conditions, cond)
		}
	}
	if perf.OrderBy != nil {
		out.OrderBy = &ir.OrderByIR{Field: perf.OrderBy.Field, Desc: perf.OrderBy.Desc}
	}
	out.Limit = perf.Limit
	return out, nil
}

func (p *planner) planSetlist(sl *ast.SetlistQuery) (*ir.QueryIR, error) {
	out := &ir.QueryIR{Type: ir.QueryTypeSetlist}
	if sl.Date != nil {
		t, err := p.dateExpander.ExpandDate(sl.Date)
		if err != nil {
			return nil, err
		}
		out.SingleDate = &t
	}
	return out, nil
}

func (p *planner) segueToIR(ctx context.Context, seg *ast.SegueCondition) (*ir.SegueChainIR, error) {
	ids := make([]int, 0, len(seg.Songs))
	for _, ref := range seg.Songs {
		id, err := p.songResolver.Resolve(ctx, ref.Name)
		if err != nil {
			return nil, p.wrapSongNotFound(ctx, err)
		}
		ids = append(ids, id)
	}
	ops := make([]ir.SegueOp, len(seg.Operators))
	for i, o := range seg.Operators {
		ops[i] = astSegueOpToIR(o)
	}
	return &ir.SegueChainIR{SongIDs: ids, Operators: ops}, nil
}

// wrapSongNotFound turns resolver.ErrSongNotFound into a QueryError with "Did you mean?" suggestions or a hint.
func (p *planner) wrapSongNotFound(ctx context.Context, err error) error {
	nf, ok := err.(*resolver.ErrSongNotFound)
	if !ok {
		return err
	}
	suggestions := p.songResolver.Suggest(ctx, nf.Name)
	qe := &errors.QueryError{
		Type:        errors.ErrSongNotFound,
		Message:     nf.Name,
		Suggestions: suggestions,
	}
	if len(suggestions) == 0 {
		qe.Hint = "The database may be empty or this song wasn't imported. If you use setlist.fm import, the daily API limit may have been reached—run again tomorrow to resume. You can also use a pre-built shows.db from GitHub Releases."
	}
	return qe
}

func (p *planner) conditionToIR(ctx context.Context, c ast.Condition) (ir.ConditionIR, error) {
	switch x := c.(type) {
	case *ast.SegueCondition:
		// Handled in planShow by lifting to SegueChain; should not appear here for first condition.
		return nil, nil
	case *ast.PositionCondition:
		id, err := p.songResolver.Resolve(ctx, x.Song.Name)
		if err != nil {
			return nil, p.wrapSongNotFound(ctx, err)
		}
		return &ir.PositionConditionIR{
			Set:      astSetPosToIR(x.Set),
			Operator: astPosOpToIR(x.Operator),
			SongID:   id,
		}, nil
	case *ast.PlayedCondition:
		id, err := p.songResolver.Resolve(ctx, x.Song.Name)
		if err != nil {
			return nil, p.wrapSongNotFound(ctx, err)
		}
		return &ir.PlayedConditionIR{SongID: id}, nil
	case *ast.LengthCondition:
		var songID *int
		if x.Song != nil {
			id, err := p.songResolver.Resolve(ctx, x.Song.Name)
			if err != nil {
				return nil, p.wrapSongNotFound(ctx, err)
			}
			songID = &id
		}
		sec, _ := parseDuration(x.Duration)
		return &ir.LengthConditionIR{SongID: songID, Operator: astCompOpToIR(x.Operator), Seconds: sec}, nil
	case *ast.GuestCondition:
		return &ir.GuestConditionIR{Name: x.Name}, nil
	default:
		return nil, nil
	}
}

func (p *planner) withConditionToIR(ctx context.Context, c ast.WithCondition) (ir.ConditionIR, error) {
	switch x := c.(type) {
	case *ast.LyricsCondition:
		return &ir.LyricsConditionIR{Words: x.Words, Operator: astLogicOpToIR(x.Operator)}, nil
	case *ast.LengthWithCondition:
		sec, _ := parseDuration(x.Duration)
		return &ir.LengthConditionIR{Operator: astCompOpToIR(x.Operator), Seconds: sec}, nil
	case *ast.GuestWithCondition:
		return &ir.GuestConditionIR{Name: x.Name}, nil
	default:
		return nil, nil
	}
}

func astSegueOpToIR(o ast.SegueOp) ir.SegueOp {
	switch o {
	case ast.SegueOpSegue:
		return ir.SegueOpSegue
	case ast.SegueOpBreak:
		return ir.SegueOpBreak
	case ast.SegueOpTease:
		return ir.SegueOpTease
	}
	return ir.SegueOpSegue
}

func astSetPosToIR(s ast.SetPosition) ir.SetPosition {
	switch s {
	case ast.Set1:
		return ir.Set1
	case ast.Set2:
		return ir.Set2
	case ast.Set3:
		return ir.Set3
	case ast.Encore:
		return ir.Encore
	}
	return ir.SetAny
}

func astPosOpToIR(o ast.PositionOp) ir.PositionOp {
	switch o {
	case ast.PosOpened:
		return ir.PosOpened
	case ast.PosClosed:
		return ir.PosClosed
	case ast.PosEquals:
		return ir.PosEquals
	}
	return ir.PosOpened
}

func astCompOpToIR(o ast.CompOp) ir.CompOp {
	switch o {
	case ast.CompGT:
		return ir.CompGT
	case ast.CompLT:
		return ir.CompLT
	case ast.CompEQ:
		return ir.CompEQ
	case ast.CompGTE:
		return ir.CompGTE
	case ast.CompLTE:
		return ir.CompLTE
	case ast.CompNEQ:
		return ir.CompNEQ
	}
	return ir.CompGT
}

func astLogicOpToIR(o ast.LogicOp) ir.LogicOp {
	if o == ast.OpOr {
		return ir.OpOr
	}
	return ir.OpAnd
}

func astOutputToIR(o ast.OutputFormat) ir.OutputFormat {
	switch o {
	case ast.OutputJSON:
		return ir.OutputJSON
	case ast.OutputCSV:
		return ir.OutputCSV
	case ast.OutputSetlist:
		return ir.OutputSetlist
	case ast.OutputCalendar:
		return ir.OutputCalendar
	case ast.OutputTable:
		return ir.OutputTable
	}
	return ir.OutputDefault
}

// parseDuration parses "20min", "15 min", "30sec" into seconds.
func parseDuration(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	var mult int
	if strings.HasSuffix(s, "min") {
		mult = 60
		s = strings.TrimSuffix(s, "min")
	} else if strings.HasSuffix(s, "sec") {
		mult = 1
		s = strings.TrimSuffix(s, "sec")
	} else {
		return 0, nil
	}
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}
