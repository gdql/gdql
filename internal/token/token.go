package token

// TokenType identifies the type of lexer token.
type TokenType int

const (
	EOF TokenType = iota
	ILLEGAL

	// Keywords
	SHOWS
	SONGS
	PERFORMANCES
	SETLIST
	FROM
	WHERE
	WITH
	WRITTEN
	ORDER
	BY
	LIMIT
	AS
	AND
	OR
	NOT
	OF
	INTO
	THEN
	TEASE
	SET1
	SET2
	SET3
	ENCORE
	OPENED
	CLOSED
	LYRICS
	LENGTH
	FIRST
	LAST
	COUNT
	DISTINCT
	PLAYED
	GUEST
	FOR
	ASC
	DESC

	// Literals
	STRING
	NUMBER
	DURATION

	// Operators
	GT   // >
	GTGT // >>
	TILDE_GT
	EQ
	LT   // <
	GTEQ
	LTEQ
	NEQ
	MINUS // - for date ranges

	// Delimiters
	LPAREN
	RPAREN
	COMMA
	SEMICOLON
	SLASH // / for dates e.g. 5/8/77
)

var tokens = [...]string{
	ILLEGAL: "<illegal>",
	EOF:     "<eof>",

	SHOWS:        "SHOWS",
	SONGS:        "SONGS",
	PERFORMANCES: "PERFORMANCES",
	SETLIST:      "SETLIST",
	FROM:         "FROM",
	WHERE:        "WHERE",
	WITH:         "WITH",
	WRITTEN:      "WRITTEN",
	ORDER:        "ORDER",
	BY:           "BY",
	LIMIT:        "LIMIT",
	AS:           "AS",
	AND:          "AND",
	OR:           "OR",
	NOT:          "NOT",
	OF:           "OF",
	INTO:         "INTO",
	THEN:         "THEN",
	TEASE:        "TEASE",
	SET1:         "SET1",
	SET2:         "SET2",
	SET3:         "SET3",
	ENCORE:       "ENCORE",
	OPENED:       "OPENED",
	CLOSED:       "CLOSED",
	LYRICS:       "LYRICS",
	LENGTH:       "LENGTH",
	FIRST:        "FIRST",
	LAST:         "LAST",
	COUNT:        "COUNT",
	DISTINCT:     "DISTINCT",
	PLAYED:       "PLAYED",
	GUEST:        "GUEST",
	FOR:          "FOR",
	ASC:          "ASC",
	DESC:         "DESC",

	STRING:   "<string>",
	NUMBER:   "<number>",
	DURATION: "<duration>",

	GT:       ">",
	GTGT:     ">>",
	TILDE_GT: "~>",
	EQ:       "=",
	LT:       "<",
	GTEQ:     ">=",
	LTEQ:     "<=",
	NEQ:      "!=",
	MINUS:    "-",

	LPAREN:   "(",
	RPAREN:   ")",
	COMMA:    ",",
	SEMICOLON: ";",
	SLASH:    "/",
}

func (tt TokenType) String() string {
	if int(tt) < len(tokens) {
		return tokens[tt]
	}
	return "<unknown>"
}

// Token represents a single lexer token.
type Token struct {
	Type    TokenType
	Literal string
	Pos     Position
}

// Position represents a source position.
type Position struct {
	Line   int
	Column int
	Offset int
}
