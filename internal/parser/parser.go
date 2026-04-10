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
	case token.COUNT:
		return p.parseCountQuery()
	case token.FIRST, token.LAST:
		return p.parseFirstLastQuery()
	case token.RANDOM:
		return p.parseRandomShowQuery()
	default:
		// Suggest closest matching top-level keyword
		topLevel := []string{"SHOWS", "SONGS", "PERFORMANCES", "SETLIST", "COUNT", "FIRST", "LAST", "RANDOM"}
		suggestion := errors.SuggestKeyword(p.cur.Literal, topLevel)
		hint := "Queries start with SHOWS, SONGS, PERFORMANCES, SETLIST, COUNT, FIRST, LAST, or RANDOM."
		return nil, &errors.ParseError{
			Pos:        p.cur.Pos,
			Message:    fmt.Sprintf("unexpected %q, expected a query keyword", p.cur.Literal),
			Query:      p.query,
			Hint:       hint,
			DidYouMean: suggestion,
		}
	}
}

func (p *parser) parseShowQuery() (*ast.ShowQuery, error) {
	q := &ast.ShowQuery{}
	// consume SHOWS
	p.advance()

	if p.curIs(token.AT) {
		p.advance()
		if !p.curIs(token.STRING) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected venue name after AT", Query: p.query}
		}
		q.At = p.cur.Literal
		p.advance()
	}

	if p.curIs(token.TOUR) {
		p.advance()
		if !p.curIs(token.STRING) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected tour name after TOUR", Query: p.query}
		}
		q.Tour = p.cur.Literal
		p.advance()
	}

	if p.curIs(token.FROM) || p.curIs(token.AFTER) || p.curIs(token.BEFORE) {
		dr, err := p.parseDateRangeWithDirection()
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

// parseDateRangeWithDirection handles FROM, AFTER, and BEFORE.
// FROM 1977 = start 1977, end 1977. AFTER 1988 = start 1988, end 2100. BEFORE 1970 = start 1900, end 1970.
func (p *parser) parseDateRangeWithDirection() (*ast.DateRange, error) {
	if p.curIs(token.AFTER) {
		p.advance()
		start, _, err := p.parseDate()
		if err != nil {
			return nil, err
		}
		return &ast.DateRange{Start: start, End: &ast.Date{Year: 2100}}, nil
	}
	if p.curIs(token.BEFORE) {
		p.advance()
		end, _, err := p.parseDate()
		if err != nil {
			return nil, err
		}
		return &ast.DateRange{Start: &ast.Date{Year: 1900}, End: end}, nil
	}
	// FROM
	p.advance()
	return p.parseDateRange()
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
		// Two-digit years map to 19xx (the Grateful Dead were active 1965-1995).
		// 65-99 → 1965-1999. 00-64 → assume nothing (Dead history doesn't extend).
		if y < 100 {
			y += 1900
		}
		d := &ast.Date{Year: y}
		p.advance()
		return d, nil, nil
	default:
		break
	}
	// Suggest closest era alias if input looks like an attempted era
	eras := []string{"PRIMAL", "EUROPE72", "WALLOFSOUND", "HIATUS", "BRENT_ERA", "VINCE_ERA"}
	suggestion := errors.SuggestKeyword(p.cur.Literal, eras)
	hint := "Use a year (1977), range (1977-1980), date (5/8/77), or era alias (PRIMAL, EUROPE72, BRENT_ERA, etc.)."
	return nil, nil, &errors.ParseError{
		Pos:        p.cur.Pos,
		Message:    fmt.Sprintf("expected date or era, got %q", p.cur.Literal),
		Query:      p.query,
		Hint:       hint,
		DidYouMean: suggestion,
	}
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
	case "WALLOFSOUND":
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
	// PLAYED "A" > "B" — after PLAYED we may have a segue; parse it and add as second condition
	if playCond, ok := cond.(*ast.PlayedCondition); ok && p.parseSegueOp() != nil {
		segCond, segErr := p.parseSegueRest(playCond.Song)
		if segErr != nil {
			return nil, segErr
		}
		wc.Conditions = append(wc.Conditions, segCond)
	}

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
	// NOT PLAYED "Song" / NOT "Song" — negated played condition
	if p.curIs(token.NOT) {
		p.advance()
		// Optional PLAYED keyword: NOT PLAYED "X" === NOT "X"
		if p.curIs(token.PLAYED) {
			p.advance()
		}
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		return &ast.PlayedCondition{Song: ref, Negated: true}, nil
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

	// OPENER "Song" / CLOSER "Song" — any set opened/closed with this song
	// OPENER ("Song" > "Song") / CLOSER ("Song" > "Song") — segue chain variant
	if p.curIs(token.OPENER) || p.curIs(token.CLOSER) {
		op := ast.PosOpened
		if p.curIs(token.CLOSER) {
			op = ast.PosClosed
		}
		p.advance()
		if p.curIs(token.LPAREN) {
			p.advance()
			seg, err := p.parseSegueCondition()
			if err != nil {
				return nil, err
			}
			if !p.curIs(token.RPAREN) {
				return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected ) after segue chain", Query: p.query}
			}
			p.advance()
			return &ast.PositionCondition{Set: ast.SetAny, Operator: op, SegueChain: seg}, nil
		}
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		return &ast.PositionCondition{Set: ast.SetAny, Operator: op, Song: ref}, nil
	}

	// PLAYED "Song" [> "Song" ...] — optional segue after PLAYED
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

	// Standalone segue operator: >"Song", >>"Song", ~>"Song"
	if p.curIs(token.GT) || p.curIs(token.GTGT) || p.curIs(token.TILDE_GT) {
		op := p.parseSegueOp()
		p.advance()
		ref, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		return &ast.SegueIntoCondition{Song: ref, Operator: *op}, nil
	}

	// Segue: "Song" > "Song" [> "Song" ...] — or bare "Song" implies PLAYED
	if p.curIs(token.STRING) {
		return p.parseSegueOrPlayed()
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

// parseSegueOrPlayed parses a quoted song name. If followed by >, it's a segue chain.
// Otherwise, it's an implicit PLAYED condition (WHERE "Bertha" = WHERE PLAYED "Bertha").
func (p *parser) parseSegueOrPlayed() (ast.Condition, error) {
	ref, err := p.parseSongRef()
	if err != nil {
		return nil, err
	}
	// If no segue operator follows, treat as PLAYED
	if p.parseSegueOp() == nil {
		return &ast.PlayedCondition{Song: ref}, nil
	}
	// Otherwise, build a segue chain starting with this song
	sc := &ast.SegueCondition{Songs: []*ast.SongRef{ref}}
	for {
		op := p.parseSegueOp()
		if op == nil {
			break
		}
		p.advance()
		if !p.curIs(token.STRING) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected song name after segue operator", Query: p.query}
		}
		nextRef, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		sc.Songs = append(sc.Songs, nextRef)
		sc.Operators = append(sc.Operators, *op)
	}
	return sc, nil
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

// parseSegueRest parses " > \"Song\" [> \"Song\" ...]" when the first song is already known (e.g. after PLAYED "A").
func (p *parser) parseSegueRest(firstRef *ast.SongRef) (*ast.SegueCondition, error) {
	sc := &ast.SegueCondition{}
	sc.Songs = append(sc.Songs, firstRef)
	for {
		op := p.parseSegueOp()
		if op == nil {
			break
		}
		p.advance()
		if !p.curIs(token.STRING) {
			return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected song name after segue operator", Query: p.query}
		}
		nextRef, err := p.parseSongRef()
		if err != nil {
			return nil, err
		}
		sc.Songs = append(sc.Songs, nextRef)
		sc.Operators = append(sc.Operators, *op)
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
		msg := "expected quoted song name"
		hint := "Use double or single quotes around song names (e.g. \"St Stephen\"). In PowerShell the shell may strip quotes from the query; use a file: gdql -f query.gdql, or wrap the whole query in single quotes: gdql 'SHOWS FROM 1969 WHERE PLAYED \"St Stephen\" > \"The Eleven\";'"
		return nil, &errors.ParseError{Pos: p.cur.Pos, Message: msg, Query: p.query, Hint: hint}
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
			if !isOrderField(p.cur) {
				return &errors.ParseError{
					Pos:     p.cur.Pos,
					Message: "expected field name after ORDER BY",
					Query:   p.query,
					Hint:    "Allowed fields: DATE, LENGTH, NAME, TIMES_PLAYED, POSITION",
				}
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
			n, err := strconv.Atoi(p.cur.Literal)
			if err != nil || n < 0 {
				return &errors.ParseError{Pos: p.cur.Pos, Message: "LIMIT must be a non-negative integer", Query: p.query}
			}
			// SECURITY: cap LIMIT to prevent OOM/DoS via huge result sets
			const maxLimit = 1000
			if n > maxLimit {
				n = maxLimit
			}
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
			if song != nil {
				song.OutputFmt = fmt
			}
			continue
		}
		break
	}
	return nil
}

// isOrderField returns true if the token is a known ORDER BY field name.
// SECURITY: do NOT accept arbitrary STRING tokens — that allowed SQL injection
// because the field name was concatenated into the generated SQL.
func isOrderField(t token.Token) bool {
	s := strings.ToUpper(t.Literal)
	return s == "DATE" || s == "LENGTH" || s == "NAME" || s == "TIMES_PLAYED" || s == "POSITION"
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
	case "COUNT":
		return ast.OutputCount
	}
	return ast.OutputDefault
}

func (p *parser) optionalSemicolon() error {
	if p.curIs(token.SEMICOLON) {
		p.advance()
	}
	for p.curIs(token.SEMICOLON) {
		p.advance()
	}
	if p.curIs(token.EOF) {
		return nil
	}

	// Detect common "wrong order" mistakes — clause keywords appearing too late.
	var msg, hint string
	switch p.cur.Type {
	case token.FROM:
		msg = "FROM must come before WHERE"
		hint = "Try: SHOWS FROM 1977 WHERE PLAYED \"Bertha\";"
	case token.AT:
		msg = "AT must come before FROM and WHERE"
		hint = "Try: SHOWS AT \"Fillmore West\" FROM 1969;"
	case token.TOUR:
		msg = "TOUR must come before FROM and WHERE"
		hint = "Try: SHOWS TOUR \"Spring 1977\";"
	case token.WHERE:
		msg = "WHERE must come after FROM (or directly after SHOWS)"
		hint = "Try: SHOWS FROM 1977 WHERE \"Bertha\";"
	case token.ORDER:
		msg = "ORDER BY must come after WHERE"
	case token.LIMIT:
		msg = "LIMIT must come after ORDER BY (or after WHERE)"
	case token.AS:
		msg = "AS must come at the end of the query"
		hint = "Try: SHOWS FROM 1977 LIMIT 5 AS JSON;"
	}
	if msg == "" {
		// Generic — try to suggest a closest keyword
		clauseKeywords := []string{"FROM", "WHERE", "AT", "TOUR", "ORDER", "LIMIT", "AS", "WITH", "WRITTEN"}
		suggestion := errors.SuggestKeyword(p.cur.Literal, clauseKeywords)
		msg = fmt.Sprintf("unexpected %q after query", p.cur.Literal)
		if suggestion == "" {
			hint = "End the query with a semicolon, or remove unexpected text."
		}
		return &errors.ParseError{Pos: p.cur.Pos, Message: msg, Query: p.query, Hint: hint, DidYouMean: suggestion}
	}
	return &errors.ParseError{Pos: p.cur.Pos, Message: msg, Query: p.query, Hint: hint}
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
			if p.curIs(token.COMMA) || p.curIs(token.AND) || p.curIs(token.OR) {
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
			if p.curIs(token.COMMA) || p.curIs(token.AND) || p.curIs(token.OR) {
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
			if p.curIs(token.COMMA) || p.curIs(token.AND) || p.curIs(token.OR) {
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

	if p.curIs(token.FROM) || p.curIs(token.AFTER) || p.curIs(token.BEFORE) {
		dr, err := p.parseDateRangeWithDirection()
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

func (p *parser) parseCountQuery() (*ast.CountQuery, error) {
	q := &ast.CountQuery{}
	p.advance() // consume COUNT
	// COUNT SHOWS [FROM ...] — count shows
	if p.curIs(token.SHOWS) {
		q.CountShows = true
		p.advance()
	} else if p.curIs(token.STRING) {
		ref, _ := p.parseSongRef()
		q.Song = ref
	} else {
		return nil, &errors.ParseError{
			Pos:     p.cur.Pos,
			Message: "expected song name or SHOWS after COUNT",
			Query:   p.query,
			Hint:    "Try: COUNT \"Dark Star\" or COUNT SHOWS FROM 1977;",
		}
	}
	if p.curIs(token.FROM) || p.curIs(token.AFTER) || p.curIs(token.BEFORE) {
		dr, err := p.parseDateRangeWithDirection()
		if err != nil {
			return nil, err
		}
		q.From = dr
	}
	return q, p.optionalSemicolon()
}

func (p *parser) parseRandomShowQuery() (*ast.RandomShowQuery, error) {
	q := &ast.RandomShowQuery{}
	p.advance() // consume RANDOM
	if p.curIs(token.SHOWS) {
		p.advance() // consume SHOW/SHOWS
	}
	if p.curIs(token.FROM) || p.curIs(token.AFTER) || p.curIs(token.BEFORE) {
		dr, err := p.parseDateRangeWithDirection()
		if err != nil {
			return nil, err
		}
		q.From = dr
	}
	return q, p.optionalSemicolon()
}

func (p *parser) parseFirstLastQuery() (*ast.FirstLastQuery, error) {
	q := &ast.FirstLastQuery{IsLast: p.curIs(token.LAST)}
	p.advance() // consume FIRST/LAST
	ref, err := p.parseSongRef()
	if err != nil {
		return nil, err
	}
	q.Song = ref
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
			}
			p.advance()
			return &ast.Date{Year: y, Month: m, Day: day}, nil
		}
		// Just a year
		if m >= 1900 || m < 100 {
			yr := m
			if yr < 100 {
				yr += 1900
			}
			return &ast.Date{Year: yr}, nil
		}
		return &ast.Date{Year: m}, nil
	}
	return nil, &errors.ParseError{Pos: p.cur.Pos, Message: "expected date or string for SETLIST FOR", Query: p.query}
}
