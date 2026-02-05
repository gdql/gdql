package lexer

import (
	"strings"
	"unicode"

	"github.com/gdql/gdql/internal/token"
)

// Lexer tokenizes GDQL input.
type Lexer interface {
	NextToken() token.Token
	PeekToken() token.Token
	Position() token.Position
}

type lexer struct {
	input   string
	runes   []rune
	pos     int
	readPos int
	ch      rune
	line    int
	col     int
	offset  int
	peeked  *token.Token
}

// isQuote returns true for ASCII and common Unicode double-quote characters
// (e.g. Windows/PowerShell may send different code points).
func isQuote(r rune) bool {
	switch r {
	case '"', '\u201C', '\u201D', '\u201E', '\u201F', '\uFF02':
		return true
	}
	return false
}

// New creates a lexer for the given input.
func New(input string) Lexer {
	l := &lexer{
		input:   input,
		runes:   []rune(input),
		line:    1,
		col:     1,
		offset:  0,
	}
	l.readChar()
	return l
}

func (l *lexer) readChar() {
	if l.readPos >= len(l.runes) {
		l.ch = 0
	} else {
		l.ch = l.runes[l.readPos]
	}
	l.pos = l.readPos
	l.offset = l.readPos
	l.readPos++
	if l.ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

func (l *lexer) peekChar() rune {
	if l.readPos >= len(l.runes) {
		return 0
	}
	return l.runes[l.readPos]
}

func (l *lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *lexer) skipComment() bool {
	if l.ch == '-' && l.peekChar() == '-' {
		for l.ch != 0 && l.ch != '\n' {
			l.readChar()
		}
		return true
	}
	return false
}

func (l *lexer) Position() token.Position {
	return token.Position{Line: l.line, Column: l.col, Offset: l.offset}
}

func (l *lexer) NextToken() token.Token {
	if l.peeked != nil {
		t := *l.peeked
		l.peeked = nil
		return t
	}
	return l.nextToken()
}

func (l *lexer) nextToken() token.Token {
	for {
		l.skipWhitespace()
		for l.ch == '-' && l.peekChar() == '-' {
			l.skipComment()
			l.skipWhitespace()
		}
		if l.ch == 0 {
			return token.Token{Type: token.EOF, Pos: l.Position()}
		}

		pos := l.Position()

		switch l.ch {
		case ';':
			l.readChar()
			return token.Token{Type: token.SEMICOLON, Literal: ";", Pos: pos}
		case ',':
			l.readChar()
			return token.Token{Type: token.COMMA, Literal: ",", Pos: pos}
		case '/':
			l.readChar()
			return token.Token{Type: token.SLASH, Literal: "/", Pos: pos}
		case '(':
			l.readChar()
			return token.Token{Type: token.LPAREN, Literal: "(", Pos: pos}
		case ')':
			l.readChar()
			return token.Token{Type: token.RPAREN, Literal: ")", Pos: pos}
		case '=':
			l.readChar()
			return token.Token{Type: token.EQ, Literal: "=", Pos: pos}
		case '!':
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				return token.Token{Type: token.NEQ, Literal: "!=", Pos: pos}
			}
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Pos: pos}
		case '<':
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				return token.Token{Type: token.LTEQ, Literal: "<=", Pos: pos}
			}
			l.readChar()
			return token.Token{Type: token.LT, Literal: "<", Pos: pos}
		case '>':
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				return token.Token{Type: token.GTEQ, Literal: ">=", Pos: pos}
			}
			if l.peekChar() == '>' {
				l.readChar()
				l.readChar()
				return token.Token{Type: token.GTGT, Literal: ">>", Pos: pos}
			}
			l.readChar()
			return token.Token{Type: token.GT, Literal: ">", Pos: pos}
		case '-':
			l.readChar()
			return token.Token{Type: token.MINUS, Literal: "-", Pos: pos}
		case '~':
			if l.peekChar() == '>' {
				l.readChar()
				l.readChar()
				return token.Token{Type: token.TILDE_GT, Literal: "~>", Pos: pos}
			}
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Pos: pos}
		case '\\':
			// Skip \ before " so that \" from PowerShell/bash is treated as start of string
			if isQuote(l.peekChar()) {
				l.readChar()
				continue
			}
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: "\\", Pos: pos}
		default:
			if isQuote(l.ch) {
				return l.readString(pos)
			}
			if unicode.IsLetter(l.ch) || l.ch == '_' {
				return l.readIdent(pos)
			}
			if unicode.IsDigit(l.ch) {
				return l.readNumberOrDuration(pos)
			}
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Pos: pos}
		}
	}
}

func (l *lexer) readString(start token.Position) token.Token {
	l.readChar() // consume opening quote
	var b strings.Builder
	closedByBackslashQuote := false
	for l.ch != 0 && !isQuote(l.ch) {
		if l.ch == '\\' && isQuote(l.peekChar()) {
			// \" from PowerShell/shell: treat as closing quote, not literal quote in content
			l.readChar()
			l.readChar()
			closedByBackslashQuote = true
			break
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			case '"', '\\':
				b.WriteRune(l.ch)
			default:
				b.WriteRune('\\')
				b.WriteRune(l.ch)
			}
			l.readChar()
			continue
		}
		b.WriteRune(l.ch)
		l.readChar()
	}
	if closedByBackslashQuote {
		return token.Token{Type: token.STRING, Literal: b.String(), Pos: start}
	}
	if !isQuote(l.ch) {
		return token.Token{Type: token.ILLEGAL, Literal: "unterminated string", Pos: start}
	}
	l.readChar() // consume closing quote
	return token.Token{Type: token.STRING, Literal: b.String(), Pos: start}
}

func (l *lexer) readIdent(start token.Position) token.Token {
	var b strings.Builder
	for unicode.IsLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' || l.ch == '.' {
		b.WriteRune(l.ch)
		l.readChar()
	}
	lit := b.String()
	tt := lookupIdent(strings.ToUpper(lit))
	if tt != token.ILLEGAL {
		return token.Token{Type: tt, Literal: lit, Pos: start}
	}
	return token.Token{Type: token.ILLEGAL, Literal: lit, Pos: start}
}

func (l *lexer) readNumberOrDuration(start token.Position) token.Token {
	var b strings.Builder
	for unicode.IsDigit(l.ch) {
		b.WriteRune(l.ch)
		l.readChar()
	}
	numLit := b.String()
	// Check for duration: 20min, 20 min, 15sec, etc.
	if l.ch == ' ' {
		next := l.readPos
		for next < len(l.runes) && l.runes[next] == ' ' {
			next++
		}
		if next < len(l.runes) {
			rest := string(l.runes[next:])
			if strings.HasPrefix(strings.ToLower(rest), "min") ||
				strings.HasPrefix(strings.ToLower(rest), "minute") ||
				strings.HasPrefix(strings.ToLower(rest), "sec") ||
				strings.HasPrefix(strings.ToLower(rest), "second") {
				for l.ch == ' ' {
					l.readChar()
				}
				for unicode.IsLetter(l.ch) {
					b.WriteRune(l.ch)
					l.readChar()
				}
				return token.Token{Type: token.DURATION, Literal: b.String(), Pos: start}
			}
		}
	}
	if unicode.IsLetter(l.ch) {
		for unicode.IsLetter(l.ch) {
			b.WriteRune(l.ch)
			l.readChar()
		}
		full := b.String()
		suffix := strings.ToLower(strings.TrimPrefix(full, numLit))
		if isDurationSuffix(suffix) {
			return token.Token{Type: token.DURATION, Literal: full, Pos: start}
		}
		return token.Token{Type: token.NUMBER, Literal: numLit, Pos: start}
	}
	return token.Token{Type: token.NUMBER, Literal: numLit, Pos: start}
}

func isDurationSuffix(s string) bool {
	s = strings.ToLower(s)
	return s == "min" || s == "mins" || s == "minute" || s == "minutes" ||
		s == "sec" || s == "secs" || s == "second" || s == "seconds"
}

func lookupIdent(ident string) token.TokenType {
	switch ident {
	case "SHOWS":
		return token.SHOWS
	case "SONGS":
		return token.SONGS
	case "PERFORMANCES":
		return token.PERFORMANCES
	case "SETLIST":
		return token.SETLIST
	case "FROM":
		return token.FROM
	case "WHERE":
		return token.WHERE
	case "WITH":
		return token.WITH
	case "WRITTEN":
		return token.WRITTEN
	case "ORDER":
		return token.ORDER
	case "BY":
		return token.BY
	case "LIMIT":
		return token.LIMIT
	case "AS":
		return token.AS
	case "AND":
		return token.AND
	case "OR":
		return token.OR
	case "NOT":
		return token.NOT
	case "OF":
		return token.OF
	case "INTO":
		return token.INTO
	case "THEN":
		return token.THEN
	case "TEASE":
		return token.TEASE
	case "SET1":
		return token.SET1
	case "SET2":
		return token.SET2
	case "SET3":
		return token.SET3
	case "ENCORE":
		return token.ENCORE
	case "OPENED":
		return token.OPENED
	case "CLOSED":
		return token.CLOSED
	case "LYRICS":
		return token.LYRICS
	case "LENGTH":
		return token.LENGTH
	case "FIRST":
		return token.FIRST
	case "LAST":
		return token.LAST
	case "COUNT":
		return token.COUNT
	case "DISTINCT":
		return token.DISTINCT
	case "PLAYED":
		return token.PLAYED
	case "GUEST":
		return token.GUEST
	case "FOR":
		return token.FOR
	case "ASC":
		return token.ASC
	case "DESC":
		return token.DESC
	default:
		return token.ILLEGAL
	}
}

// PeekToken returns the next token without consuming it.
func (l *lexer) PeekToken() token.Token {
	if l.peeked != nil {
		return *l.peeked
	}
	t := l.nextToken()
	l.peeked = &t
	return t
}
