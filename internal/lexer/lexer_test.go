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

func tokenWithoutPos(t token.Token) token.Token {
	t.Pos = token.Position{}
	return t
}
