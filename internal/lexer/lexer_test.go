package lexer

import (
	"testing"

	"github.com/gdql/gdql/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer_NextToken_Keywords(t *testing.T) {
	tests := []struct {
		input string
		want  []token.TokenType
	}{
		{"SHOWS", []token.TokenType{token.SHOWS, token.EOF}},
		{"SHOWS FROM 1977;", []token.TokenType{token.SHOWS, token.FROM, token.NUMBER, token.SEMICOLON, token.EOF}},
		{"SHOWS FROM 1977-1980;", []token.TokenType{token.SHOWS, token.FROM, token.NUMBER, token.MINUS, token.NUMBER, token.SEMICOLON, token.EOF}},
		{"shows from 1977;", []token.TokenType{token.SHOWS, token.FROM, token.NUMBER, token.SEMICOLON, token.EOF}},
		{"WHERE AND OR NOT", []token.TokenType{token.WHERE, token.AND, token.OR, token.NOT, token.EOF}},
		{"SET1 SET2 SET3 ENCORE OPENED CLOSED", []token.TokenType{token.SET1, token.SET2, token.SET3, token.ENCORE, token.OPENED, token.CLOSED, token.EOF}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			for i, wantType := range tt.want {
				tok := l.NextToken()
				assert.Equal(t, wantType, tok.Type, "token %d: want %s got %s (literal %q)", i, wantType, tok.Type, tok.Literal)
				if tok.Type == token.EOF {
					break
				}
			}
		})
	}
}

func TestLexer_NextToken_Strings(t *testing.T) {
	l := New(`"Scarlet Begonias" > "Fire on the Mountain"`)
	require.Equal(t, token.Token{Type: token.STRING, Literal: "Scarlet Begonias"}, tokenWithoutPos(l.NextToken()))
	require.Equal(t, token.GT, l.NextToken().Type)
	require.Equal(t, token.Token{Type: token.STRING, Literal: "Fire on the Mountain"}, tokenWithoutPos(l.NextToken()))
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_NextToken_Operators(t *testing.T) {
	l := New("> >> ~> = >= <=")
	require.Equal(t, token.GT, l.NextToken().Type)
	require.Equal(t, token.GTGT, l.NextToken().Type)
	require.Equal(t, token.TILDE_GT, l.NextToken().Type)
	require.Equal(t, token.EQ, l.NextToken().Type)
	require.Equal(t, token.GTEQ, l.NextToken().Type)
	require.Equal(t, token.LTEQ, l.NextToken().Type)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_NextToken_Duration(t *testing.T) {
	l := New("20min 15 min 30sec")
	require.Equal(t, token.Token{Type: token.DURATION, Literal: "20min"}, tokenWithoutPos(l.NextToken()))
	require.Equal(t, token.Token{Type: token.DURATION, Literal: "15min"}, tokenWithoutPos(l.NextToken()))
	require.Equal(t, token.Token{Type: token.DURATION, Literal: "30sec"}, tokenWithoutPos(l.NextToken()))
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_NextToken_Comment(t *testing.T) {
	l := New("SHOWS -- comment\nFROM 1977;")
	require.Equal(t, token.SHOWS, l.NextToken().Type)
	require.Equal(t, token.FROM, l.NextToken().Type)
	require.Equal(t, token.NUMBER, l.NextToken().Type)
	require.Equal(t, token.SEMICOLON, l.NextToken().Type)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_PeekToken(t *testing.T) {
	l := New("SHOWS FROM")
	require.Equal(t, token.SHOWS, l.PeekToken().Type)
	require.Equal(t, token.SHOWS, l.PeekToken().Type)
	require.Equal(t, token.SHOWS, l.NextToken().Type)
	require.Equal(t, token.FROM, l.NextToken().Type)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_FullQuery(t *testing.T) {
	input := `SHOWS FROM 1977-1980 WHERE "Scarlet Begonias" > "Fire on the Mountain";`
	l := New(input)
	types := []token.TokenType{
		token.SHOWS, token.FROM, token.NUMBER, token.MINUS, token.NUMBER, token.WHERE,
		token.STRING, token.GT, token.STRING, token.SEMICOLON, token.EOF,
	}
	for i, want := range types {
		tok := l.NextToken()
		assert.Equal(t, want, tok.Type, "token %d", i)
	}
}

func TestLexer_BackslashBeforeQuote(t *testing.T) {
	// PowerShell passes \" as literal backslash+quote; we skip the backslash so the quote starts the string
	input := `WHERE \"Scarlet Begonias\" > \"Fire on the Mountain"`
	l := New(input)
	require.Equal(t, token.WHERE, l.NextToken().Type)
	tok := l.NextToken()
	require.Equal(t, token.STRING, tok.Type)
	require.Equal(t, "Scarlet Begonias", tok.Literal)
	require.Equal(t, token.GT, l.NextToken().Type)
	tok2 := l.NextToken()
	require.Equal(t, token.STRING, tok2.Type)
	require.Equal(t, "Fire on the Mountain", tok2.Literal)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_SingleQuotedStrings(t *testing.T) {
	// Single-quoted strings work (e.g. PowerShell: outer double quotes are awkward, so users use single quotes)
	input := `SHOWS FROM 1969 WHERE PLAYED 'St Stephen' > 'The Eleven';`
	l := New(input)
	require.Equal(t, token.SHOWS, l.NextToken().Type)
	require.Equal(t, token.FROM, l.NextToken().Type)
	require.Equal(t, token.NUMBER, l.NextToken().Type)
	require.Equal(t, token.WHERE, l.NextToken().Type)
	require.Equal(t, token.PLAYED, l.NextToken().Type)
	tok := l.NextToken()
	require.Equal(t, token.STRING, tok.Type)
	require.Equal(t, "St Stephen", tok.Literal)
	require.Equal(t, token.GT, l.NextToken().Type)
	tok2 := l.NextToken()
	require.Equal(t, token.STRING, tok2.Type)
	require.Equal(t, "The Eleven", tok2.Literal)
	require.Equal(t, token.SEMICOLON, l.NextToken().Type)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_UnicodeSingleQuotes(t *testing.T) {
	// Windows/PowerShell sometimes sends Unicode single quotes U+2018/U+2019 instead of ASCII '
	input := "SHOWS WHERE PLAYED \u2018St Stephen\u2019 > \u2019The Eleven\u2018;"
	l := New(input)
	require.Equal(t, token.SHOWS, l.NextToken().Type)
	require.Equal(t, token.WHERE, l.NextToken().Type)
	require.Equal(t, token.PLAYED, l.NextToken().Type)
	tok := l.NextToken()
	require.Equal(t, token.STRING, tok.Type)
	require.Equal(t, "St Stephen", tok.Literal)
	require.Equal(t, token.GT, l.NextToken().Type)
	tok2 := l.NextToken()
	require.Equal(t, token.STRING, tok2.Type)
	require.Equal(t, "The Eleven", tok2.Literal)
	require.Equal(t, token.SEMICOLON, l.NextToken().Type)
	require.Equal(t, token.EOF, l.NextToken().Type)
}

func tokenWithoutPos(t token.Token) token.Token {
	t.Pos = token.Position{}
	return t
}

// === New tokens (AT, BEFORE, AFTER, TOUR, COUNT, FIRST, LAST, RANDOM, OPENER, CLOSER) ===

func TestLexer_NewKeywords(t *testing.T) {
	cases := []struct {
		input string
		want  token.TokenType
	}{
		{"AT", token.AT},
		{"BEFORE", token.BEFORE},
		{"AFTER", token.AFTER},
		{"TOUR", token.TOUR},
		{"COUNT", token.COUNT},
		{"FIRST", token.FIRST},
		{"LAST", token.LAST},
		{"RANDOM", token.RANDOM},
		{"OPENER", token.OPENER},
		{"CLOSER", token.CLOSER},
		// Case-insensitive
		{"at", token.AT},
		{"after", token.AFTER},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			l := New(tc.input)
			tok := l.NextToken()
			assert.Equal(t, tc.want, tok.Type)
		})
	}
}

func TestLexer_ShowAlias(t *testing.T) {
	// Both SHOWS and SHOW lex to token.SHOWS
	for _, in := range []string{"SHOW", "SHOWS", "show", "shows"} {
		l := New(in)
		assert.Equal(t, token.SHOWS, l.NextToken().Type, in)
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	l := New(`"Bertha`)
	tok := l.NextToken()
	assert.Equal(t, token.ILLEGAL, tok.Type)
	assert.Contains(t, tok.Literal, "unterminated")
}

func TestLexer_NumericLiteral(t *testing.T) {
	l := New("1977")
	tok := l.NextToken()
	assert.Equal(t, token.NUMBER, tok.Type)
	assert.Equal(t, "1977", tok.Literal)
}

func TestLexer_DurationVariants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"20min", "20min"},
		{"15 min", "15min"},
		{"30sec", "30sec"},
		{"5 minutes", "5minutes"},
		{"45 seconds", "45seconds"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			l := New(tc.in)
			tok := l.NextToken()
			assert.Equal(t, token.DURATION, tok.Type)
			assert.Equal(t, tc.want, tok.Literal)
		})
	}
}

func TestLexer_NotEquals(t *testing.T) {
	l := New("!=")
	assert.Equal(t, token.NEQ, l.NextToken().Type)
}

func TestLexer_LessThan(t *testing.T) {
	l := New("< <=")
	assert.Equal(t, token.LT, l.NextToken().Type)
	assert.Equal(t, token.LTEQ, l.NextToken().Type)
}

func TestLexer_Slash(t *testing.T) {
	// Slash is used in M/D/YY dates
	l := New("5/8/77")
	assert.Equal(t, token.NUMBER, l.NextToken().Type)
	assert.Equal(t, token.SLASH, l.NextToken().Type)
	assert.Equal(t, token.NUMBER, l.NextToken().Type)
	assert.Equal(t, token.SLASH, l.NextToken().Type)
	assert.Equal(t, token.NUMBER, l.NextToken().Type)
}

func TestLexer_Parens(t *testing.T) {
	l := New(`LYRICS("train", "road")`)
	assert.Equal(t, token.LYRICS, l.NextToken().Type)
	assert.Equal(t, token.LPAREN, l.NextToken().Type)
	assert.Equal(t, token.STRING, l.NextToken().Type)
	assert.Equal(t, token.COMMA, l.NextToken().Type)
	assert.Equal(t, token.STRING, l.NextToken().Type)
	assert.Equal(t, token.RPAREN, l.NextToken().Type)
}

func TestLexer_SingleQuotedString(t *testing.T) {
	l := New(`'Hello World'`)
	tok := l.NextToken()
	assert.Equal(t, token.STRING, tok.Type)
	assert.Equal(t, "Hello World", tok.Literal)
}

func TestLexer_SingleQuoteEscape(t *testing.T) {
	// '' inside a single-quoted string = literal '
	cases := []struct {
		input string
		want  string
	}{
		{`'Truckin'''`, "Truckin'"},
		{`'A''B'`, "A'B"},
		{`'don''t'`, "don't"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			l := New(tc.input)
			tok := l.NextToken()
			assert.Equal(t, token.STRING, tok.Type)
			assert.Equal(t, tc.want, tok.Literal)
			assert.Equal(t, token.EOF, l.NextToken().Type)
		})
	}
}

func TestLexer_DoubleQuoteEscape(t *testing.T) {
	// "" inside double-quoted string = literal "
	l := New(`"say ""hi"" friend"`)
	tok := l.NextToken()
	assert.Equal(t, token.STRING, tok.Type)
	assert.Equal(t, `say "hi" friend`, tok.Literal)
}

func TestLexer_DoubleSlashIsNotComment(t *testing.T) {
	// Only -- starts a comment, not //
	l := New("// not comment")
	assert.Equal(t, token.SLASH, l.NextToken().Type)
}

func TestLexer_BlankInput(t *testing.T) {
	l := New("")
	assert.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_OnlyWhitespace(t *testing.T) {
	l := New("   \n\t  ")
	assert.Equal(t, token.EOF, l.NextToken().Type)
}

func TestLexer_OnlyComment(t *testing.T) {
	l := New("-- just a comment\n")
	assert.Equal(t, token.EOF, l.NextToken().Type)
}
