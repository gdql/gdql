package errors

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/token"
)

// ParseError is a parsing error with position, message, suggestions, and hint.
type ParseError struct {
	Pos         token.Position
	Message     string
	Query       string
	Hint        string
	DidYouMean  string // closest-match suggestion (e.g. "FROM" for "FORM")
}

func (e *ParseError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "parse error at line %d, column %d: %s\n", e.Pos.Line, e.Pos.Column, e.Message)
	if e.Query != "" {
		fmt.Fprintf(&b, "  %s\n", e.Query)
		pad := e.Pos.Offset
		if pad > len(e.Query) {
			pad = len(e.Query)
		}
		fmt.Fprintf(&b, "  %s^\n", strings.Repeat(" ", pad))
	}
	if e.DidYouMean != "" {
		fmt.Fprintf(&b, "\nDid you mean: %s", e.DidYouMean)
	}
	if e.Hint != "" {
		fmt.Fprintf(&b, "\nHint: %s", e.Hint)
	}
	return b.String()
}

// SuggestKeyword returns the closest matching keyword from the candidates list,
// or "" if no good match exists. Uses Levenshtein distance with a max threshold.
func SuggestKeyword(input string, candidates []string) string {
	if input == "" {
		return ""
	}
	upper := strings.ToUpper(input)
	best := ""
	bestDist := len(input)/2 + 2 // tolerance grows with input length
	for _, c := range candidates {
		d := levenshtein(upper, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// QueryError is an execution/planning error with optional suggestions and hint.
type QueryError struct {
	Type        ErrorType
	Message     string
	Cause       error
	Suggestions []string
	Hint        string // Shown when no suggestions (e.g. empty DB)
}

type ErrorType int

const (
	ErrSongNotFound ErrorType = iota
	ErrDateInvalid
	ErrVenueNotFound
	ErrAmbiguousSong
	ErrNoDatabase
)

func (e *QueryError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", e.Type.String(), e.Message)
	if e.Cause != nil {
		fmt.Fprintf(&b, " (%v)", e.Cause)
	}
	if len(e.Suggestions) > 0 {
		fmt.Fprint(&b, "\nDid you mean:\n")
		for _, s := range e.Suggestions {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	if e.Hint != "" {
		fmt.Fprintf(&b, "\nHint: %s", e.Hint)
	}
	return b.String()
}

func (t ErrorType) String() string {
	switch t {
	case ErrSongNotFound:
		return "song not found"
	case ErrDateInvalid:
		return "invalid date"
	case ErrVenueNotFound:
		return "venue not found"
	case ErrAmbiguousSong:
		return "ambiguous song"
	case ErrNoDatabase:
		return "no database"
	default:
		return "query error"
	}
}
