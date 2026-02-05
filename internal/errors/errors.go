package errors

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/token"
)

// ParseError is a parsing error with position and suggestion.
type ParseError struct {
	Pos     token.Position
	Message string
	Query   string
	Hint    string
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
	if e.Hint != "" {
		fmt.Fprintf(&b, "\nHint: %s", e.Hint)
	}
	return b.String()
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
