package parser

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gdql/gdql/internal/ast"
	"github.com/gdql/gdql/internal/errors"
	"github.com/gdql/gdql/internal/lexer"
	"github.com/gdql/gdql/internal/token"
)

// Parser parses GDQL and produces an AST.
type Parser interface {
	Parse() (ast.Query, error)
}

type parser struct {
	lex   lexer.Lexer
	cur   token.Token
	peek  token.Token
	query string
}

// New creates a parser that reads from the given lexer.
func New(l lexer.Lexer) Parser {
	p := &parser{lex: l}
	p.cur = p.lex.NextToken()
	p.peek = p.lex.NextToken()
	return p
}

// NewFromString creates a parser for the given query string.
func NewFromString(input string) Parser {
	p := New(lexer.New(input)).(*parser)
	p.query = input
	return p
}

// NewFromReader reads all of r and creates a parser for it.
func NewFromReader(r io.Reader) (Parser, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return New(lexer.New(string(b))), nil
}

func (p *parser) advance() {
	p.cur = p.peek
	p.peek = p.lex.NextToken()
}

func (p *parser) expect(tt token.TokenType) error {
	if p.cur.Type != tt {
		return &errors.ParseError{
			Pos:     p.cur.Pos,
			Message: fmt.Sprintf("expected %s, got %s", tt, p.cur.Type),
			Query:   p.query,
			Hint:    fmt.Sprintf("expected %s", tt),
		}
	}
	return nil
}

func (p *parser) curIs(tt token.TokenType) bool {
	return p.cur.Type == tt
}

func (p *parser) peekIs(tt token.TokenType) bool {
	return p.peek.Type == tt
}

// Parse parses the input and returns an AST Query.
func (p *parser) Parse() (ast.Query, error) {
	if p.curIs(token.EOF) {
		return nil, &errors.ParseError{Message: "empty query"}
	}

	switch p.cur.Type {
	case token.SHOWS:
		return p.parseShowQuery()
	case token.SONGS:
		return p.parseSongQuery()
	case token.PERFORMANCES:
		return p.parsePerformanceQuery()
	case token.SETLIST:
		return p.parseSetlistQuery()
	default:
		return nil, &errors.ParseError{
			Pos:     p.cur.Pos,
			Message: fmt.Sprintf("unexpected %s, expected SHOWS, SONGS, PERFORMANCES, or SETLIST", p.cur.Type),
			Query:   p.query,
		}
	}
}

func (p *parser) parseShowQuery() (*ast.ShowQuery, error) {
	q := &ast.ShowQuery{}
	// consume SHOWS
	p.advance()

	if p.curIs(token.FROM) {
		p.advance()
		dr, err := p.parseDateRange()
		if err != nil {
			return nil, err
		}
		q.From = dr
	}

	if p.curIs(token.WHERE) {
		p.advance()
		wc, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		q.Where = wc
	}

	if err := p.parseModifiers(q, nil, nil); err != nil {
		return nil, err
	}

	return q, p.optionalSemicolon()
}

func (p *parser) parseDateRange() (*ast.DateRange, error) {
	dr := &ast.DateRange{}
	start, era, err := p.parseDate()
	if err != nil {
		return nil, err
	}
	if era != nil {
		dr.Era = era
	} else {
		dr.Start = start
	}

	if p.curIs(token.MINUS) && p.peekIs(token.NUMBER) {
		p.advance() // consume -
		end, _, err := p.parseDate()
		if err != nil {
			return nil, err
		}
		dr.End = end
	}

	return dr, nil
}

func (p *parser) parseDate() (*ast.Date, *ast.EraAlias, error) {
	era := p.parseEraAlias()
	if era != nil {
		p.advance()
		return nil, era, nil
	}
	switch p.cur.Type {
	case token.NUMBER:
		y, _ := strconv.Atoi(p.cur.Literal)
		d := &ast.Date{Year: y}
		p.advance()
		return d, nil, nil
	default:
		break
	}
	return nil, nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected date (year or era alias)", Query: p.query}
}

func (p *parser) parseEraAlias() *ast.EraAlias {
	lit := strings.ToUpper(p.cur.Literal)
	switch lit {
	case "PRIMAL":
		e := ast.EraPrimal
		return &e
	case "EUROPE72", "EUROPE":
		e := ast.EraEurope72
		return &e
	case "WALLOFOUND", "WALLOFSOUND":
		e := ast.EraWallOfSound
		return &e
	case "HIATUS":
		e := ast.EraHiatus
		return &e
	case "BRENT_ERA", "BRENT":
		e := ast.EraBrent
		return &e
	case "VINCE_ERA", "VINCE":
		e := ast.EraVince
		return &e
	}
	return nil
}

func (p *parser) parseWhereClause() (*ast.WhereClause, error) {
	wc := &ast.WhereClause{}
	cond, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	wc.Conditions = append(wc.Conditions, cond)

	for p.curIs(token.AND) || p.curIs(token.OR) {
		if p.curIs(token.AND) {
			wc.Operators = append(wc.Operators, ast.OpAnd)
		} else {
			wc.Operators = append(wc.Operators, ast.OpOr)
		}
		p.advance()
		next, err := p.parseCondition()
		if err != nil {
			return nil, err
		}
		wc.Conditions = append(wc.Conditions, next)
	}

	return wc, nil
}

func (p *parser) parseCondition() (ast.Condition, error) {
	// NOT song_ref
	if p.curIs(token.NOT) {
		p.advance()
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		ref.Negated = true
		// Single NOT "X" isn't a full condition in our grammar; treat as segue with one negated song (unusual). Or require segue after.
		// For simplicity: NOT "X" means played condition with negated (we don't support that in WHERE). Omit for now.
		return &ast.PlayedCondition{Song: ref}, nil
	}

	// SET1 OPENED "Song" / ENCORE = "Song"
	if p.cur.Type == token.SET1 || p.cur.Type == token.SET2 || p.cur.Type == token.SET3 || p.cur.Type == token.ENCORE {
		set := p.parseSetPosition()
		p.advance()
		op := ast.PosOpened
		if p.curIs(token.OPENED) {
			p.advance()
		} else if p.curIs(token.CLOSED) {
			op = ast.PosClosed
			p.advance()
		} else if p.curIs(token.EQ) {
			op = ast.PosEquals
			p.advance()
		} else {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected OPENED, CLOSED, or =", Query: p.query}
		}
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		return &ast.PositionCondition{Set: set, Operator: op, Song: ref}, nil
	}

	// PLAYED "Song"
	if p.curIs(token.PLAYED) {
		p.advance()
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		return &ast.PlayedCondition{Song: ref}, nil
	}

	// GUEST "Name"
	if p.curIs(token.GUEST) {
		p.advance()
		if !p.curIs(token.STRING) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected string after GUEST", Query: p.query}
		}
		name := p.cur.Literal
		p.advance()
		return &ast.GuestCondition{Name: name}, nil
	}

	// LENGTH ( "Song" ) > 20min or LENGTH > 20min
	if p.curIs(token.LENGTH) {
		p.advance()
		var songRef *ast.SongRef
		if p.curIs(token.LPAREN) {
			p.advance()
			var err error
			songRef, err = p.parseSongRef()
			if err != nil {
				return nil, err
			}
			if !p.curIs(token.RPAREN) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected )", Query: p.query}
			}
			p.advance()
		}
		op := p.parseCompOp()
		if op == nil {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected comparison operator", Query: p.query}
		}
		p.advance()
		if !p.curIs(token.DURATION) && !p.curIs(token.NUMBER) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected duration (e.g. 20min)", Query: p.query}
		}
		dur := p.cur.Literal
		if p.curIs(token.NUMBER) && p.peekIs(token.DURATION) {
			// shouldn't happen - NUMBER then space then "min" would be one DURATION token
		}
		p.advance()
		return &ast.LengthCondition{Song: songRef, Operator: *op, Duration: dur}, nil
	}

	// Segue: "Song" > "Song" [> "Song" ...]
	if p.curIs(token.STRING) {
		return p.parseSegueCondition()
	}

	hint := "use quoted song names, e.g. WHERE \"Scarlet Begonias\" > \"Fire on the Mountain\""
	if p.cur.Type == token.ILLEGAL || strings.Contains(p.cur.Literal, "unterminated") {
		hint += "; in PowerShell use single quotes around the whole query: gdql 'SHOWS WHERE \"Scarlet Begonias\" > \"Fire on the Mountain\"', or use -f query.gdql"
	}
	return nil, &errors.ParseError{
		Pos:     p.cur.Pos,
		Message: fmt.Sprintf("expected condition (got %s %q)", p.cur.Type, p.cur.Literal),
		Query:   p.query,
		Hint:    hint,
	}
}

func (p *parser) parseSegueCondition() (*ast.SegueCondition, error) {
	sc := &ast.SegueCondition{}
	ref, err := p.parseSongRef()
	if err != nil {
		return nil, err
	}
	sc.Songs = append(sc.Songs, ref)

	for {
		op := p.parseSegueOp()
		if op == nil {
			break
		}
		p.advance()
		// INTO and THEN consume one token (the keyword)
		if p.curIs(token.STRING) {
			nextRef, err := p.parseSongRef()
			if err != nil {
				return nil, err
			}
			sc.Songs = append(sc.Songs, nextRef)
			sc.Operators = append(sc.Operators, *op)
		} else {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected song name after segue operator", Query: p.query}
		}
	}

	if len(sc.Songs) < 2 {
		return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "segue requires at least two songs", Query: p.query}
	}
	return sc, nil
}

func (p *parser) parseSegueOp() *ast.SegueOp {
	switch p.cur.Type {
	case token.GT:
		o := ast.SegueOpSegue
		return &o
	case token.GTGT:
		o := ast.SegueOpBreak
		return &o
	case token.TILDE_GT:
		o := ast.SegueOpTease
		return &o
	case token.INTO:
		o := ast.SegueOpSegue
		return &o
	case token.THEN:
		o := ast.SegueOpBreak
		return &o
	case token.TEASE:
		o := ast.SegueOpTease
		return &o
	}
	return nil
}

func (p *parser) parseCompOp() *ast.CompOp {
	switch p.cur.Type {
	case token.GT:
		o := ast.CompGT
		return &o
	case token.LT:
		o := ast.CompLT
		return &o
	case token.GTEQ:
		o := ast.CompGTE
		return &o
	case token.LTEQ:
		o := ast.CompLTE
		return &o
	case token.EQ:
		o := ast.CompEQ
		return &o
	case token.NEQ:
		o := ast.CompNEQ
		return &o
	default:
		return nil
	}
}

func (p *parser) parseSetPosition() ast.SetPosition {
	switch p.cur.Type {
	case token.SET1:
		return ast.Set1
	case token.SET2:
		return ast.Set2
	case token.SET3:
		return ast.Set3
	case token.ENCORE:
		return ast.Encore
	}
	return ast.SetAny
}

func (p *parser) parseSongRef() (*ast.SongRef, error) {
	if !p.curIs(token.STRING) {
		return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected quoted song name", Query: p.query}
	}
	ref := &ast.SongRef{Name: p.cur.Literal}
	p.advance()
	return ref, nil
}

func (p *parser) parseModifiers(show *ast.ShowQuery, song *ast.SongQuery, perf *ast.PerformanceQuery) error {
	for {
		if p.curIs(token.ORDER) {
			p.advance()
			if !p.curIs(token.BY) {
				return &errors.ParseError{Pos: p.cur.Pos, Message: "expected BY after ORDER", Query: p.query}
			}
			p.advance()
			if p.cur.Type != token.STRING && !isOrderField(p.cur) {
				return &errors.ParseError{Pos: p.cur.Pos, Message: "expected field name (DATE, LENGTH, RATING, etc.)", Query: p.query}
			}
			field := p.cur.Literal
			p.advance()
			desc := false
			if p.curIs(token.DESC) {
				desc = true
				p.advance()
			} else if p.curIs(token.ASC) {
				p.advance()
			}
			oc := &ast.OrderClause{Field: field, Desc: desc}
			if show != nil {
				show.OrderBy = oc
			}
			if song != nil {
				song.OrderBy = oc
			}
			if perf != nil {
				perf.OrderBy = oc
			}
			continue
		}
		if p.curIs(token.LIMIT) {
			p.advance()
			if !p.curIs(token.NUMBER) {
				return &errors.ParseError{Pos: p.cur.Pos, Message: "expected number after LIMIT", Query: p.query}
			}
			n, _ := strconv.Atoi(p.cur.Literal)
			p.advance()
			if show != nil {
				show.Limit = &n
			}
			if song != nil {
				song.Limit = &n
			}
			if perf != nil {
				perf.Limit = &n
			}
			continue
		}
		if p.curIs(token.AS) {
			p.advance()
			fmt := p.parseOutputFormat()
			p.advance()
			if show != nil {
				show.OutputFmt = fmt
			}
			continue
		}
		break
	}
	return nil
}

func isOrderField(t token.Token) bool {
	switch t.Type {
	case token.STRING:
		return true
	}
	s := t.Literal
	return s == "DATE" || s == "LENGTH" || s == "RATING" || s == "NAME" || s == "TIMES_PLAYED"
}

func (p *parser) parseOutputFormat() ast.OutputFormat {
	switch strings.ToUpper(p.cur.Literal) {
	case "JSON":
		return ast.OutputJSON
	case "CSV":
		return ast.OutputCSV
	case "SETLIST":
		return ast.OutputSetlist
	case "CALENDAR":
		return ast.OutputCalendar
	case "TABLE":
		return ast.OutputTable
	}
	return ast.OutputDefault
}

func (p *parser) optionalSemicolon() error {
	if p.curIs(token.SEMICOLON) {
		p.advance()
	}
	if !p.curIs(token.EOF) {
		return &errors.ParseError{Pos: p.cur.Pos, Message: "unexpected token after query", Query: p.query}
	}
	return nil
}

func (p *parser) parseSongQuery() (*ast.SongQuery, error) {
	q := &ast.SongQuery{}
	p.advance()

	if p.curIs(token.WITH) {
		p.advance()
		wc, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		q.With = wc
	}

	if p.curIs(token.WRITTEN) {
		p.advance()
		dr, err := p.parseDateRange()
		if err != nil {
			return nil, err
		}
		q.Written = dr
	}

	if err := p.parseModifiers(nil, q, nil); err != nil {
		return nil, err
	}

	return q, p.optionalSemicolon()
}

func (p *parser) parseWithClause() (*ast.WithClause, error) {
	wc := &ast.WithClause{}
	for {
		// LYRICS ( "a", "b" )
		if p.curIs(token.LYRICS) {
			p.advance()
			if !p.curIs(token.LPAREN) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected ( after LYRICS", Query: p.query}
			}
			p.advance()
			var words []string
			for p.curIs(token.STRING) {
				words = append(words, p.cur.Literal)
				p.advance()
				if p.curIs(token.COMMA) {
					p.advance()
				}
			}
			if !p.curIs(token.RPAREN) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected )", Query: p.query}
			}
			p.advance()
			wc.Conditions = append(wc.Conditions, &ast.LyricsCondition{Words: words})
			if p.curIs(token.COMMA) {
				p.advance()
				continue
			}
			break
		}
		if p.curIs(token.LENGTH) {
			p.advance()
			op := p.parseCompOp()
			if op == nil {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected comparison after LENGTH", Query: p.query}
			}
			p.advance()
			if !p.curIs(token.DURATION) && !p.curIs(token.NUMBER) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected duration", Query: p.query}
			}
			dur := p.cur.Literal
			p.advance()
			wc.Conditions = append(wc.Conditions, &ast.LengthWithCondition{Operator: *op, Duration: dur})
			if p.curIs(token.COMMA) {
				p.advance()
				continue
			}
			break
		}
		if p.curIs(token.GUEST) {
			p.advance()
			if !p.curIs(token.STRING) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected string after GUEST", Query: p.query}
			}
			wc.Conditions = append(wc.Conditions, &ast.GuestWithCondition{Name: p.cur.Literal})
			p.advance()
			if p.curIs(token.COMMA) {
				p.advance()
				continue
			}
			break
		}
		break
	}
	return wc, nil
}

func (p *parser) parsePerformanceQuery() (*ast.PerformanceQuery, error) {
	q := &ast.PerformanceQuery{}
	p.advance()
	if !p.curIs(token.OF) {
		return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected OF after PERFORMANCES", Query: p.query}
	}
	p.advance()
	ref, err := p.parseSongRef()
	if err != nil {
		return nil, err
	}
	q.Song = ref

	if p.curIs(token.FROM) {
		p.advance()
		dr, err := p.parseDateRange()
		if err != nil {
			return nil, err
		}
		q.From = dr
	}

	if p.curIs(token.WITH) {
		p.advance()
		wc, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		q.With = wc
	}

	if err := p.parseModifiers(nil, nil, q); err != nil {
		return nil, err
	}
	return q, p.optionalSemicolon()
}

func (p *parser) parseSetlistQuery() (*ast.SetlistQuery, error) {
	q := &ast.SetlistQuery{}
	p.advance()
	if !p.curIs(token.FOR) {
		return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected FOR after SETLIST", Query: p.query}
	}
	p.advance()
	date, err := p.parseDateForSetlist()
	if err != nil {
		return nil, err
	}
	q.Date = date
	return q, p.optionalSemicolon()
}

func (p *parser) parseDateForSetlist() (*ast.Date, error) {
	if p.curIs(token.STRING) {
		lit := p.cur.Literal
		p.advance()
		return &ast.Date{Year: 0, Season: lit}, nil
	}
	if p.curIs(token.NUMBER) {
		m, _ := strconv.Atoi(p.cur.Literal)
		p.advance()
		if p.curIs(token.SLASH) {
			p.advance()
			if !p.curIs(token.NUMBER) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected day in M/D/YY", Query: p.query}
			}
			day, _ := strconv.Atoi(p.cur.Literal)
			p.advance()
			if !p.curIs(token.SLASH) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected / and year in M/D/YY", Query: p.query}
			}
			p.advance()
			if !p.curIs(token.NUMBER) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected year", Query: p.query}
			}
			y, _ := strconv.Atoi(p.cur.Literal)
			if y < 100 {
				y += 1900
				if y < 1970 {
					y += 100
				}
			}
			p.advance()
			return &ast.Date{Year: y, Month: m, Day: day}, nil
		}
		// Just a year
		if m >= 1900 || m < 100 {
			yr := m
			if yr < 100 {
				yr += 1900
				if yr < 1970 {
					yr += 100
				}
			}
			return &ast.Date{Year: yr}, nil
		}
		return &ast.Date{Year: m}, nil
	}
	return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected date or string for SETLIST FOR", Query: p.query}
}
