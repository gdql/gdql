package errors

import (
	"testing"

	"github.com/gdql/gdql/internal/token"
	"github.com/stretchr/testify/assert"
)

func TestSuggestKeyword_ExactMatch(t *testing.T) {
	// Exact case-insensitive matches return the canonical keyword.
	// (In practice, the parser only calls this when the token wasn't recognized,
	// so this case shouldn't happen — but the function should still work.)
	keywords := []string{"SHOWS", "SONGS", "FROM"}
	assert.Equal(t, "SHOWS", SuggestKeyword("SHOWS", keywords))
	assert.Equal(t, "SHOWS", SuggestKeyword("shows", keywords))
}

func TestSuggestKeyword_CommonTypos(t *testing.T) {
	keywords := []string{"SHOWS", "SONGS", "FROM", "WHERE", "PRIMAL", "EUROPE72"}
	cases := []struct {
		input, want string
	}{
		{"HOWS", "SHOWS"},
		{"SHOW", "SHOWS"},
		{"FORM", "FROM"},
		{"WHEEL", "WHERE"},
		{"PRIMOL", "PRIMAL"},
		{"PRIMEL", "PRIMAL"},
		{"EUROP72", "EUROPE72"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := SuggestKeyword(tc.input, keywords)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSuggestKeyword_NoMatch(t *testing.T) {
	keywords := []string{"SHOWS", "SONGS"}
	// Way too different — no suggestion
	assert.Equal(t, "", SuggestKeyword("BANANA", keywords))
}

func TestSuggestKeyword_Empty(t *testing.T) {
	assert.Equal(t, "", SuggestKeyword("", []string{"SHOWS"}))
}

func TestLevenshtein_KnownDistances(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "abcd", 1},
		{"abcd", "abc", 1},
		{"FORM", "FROM", 2}, // F-O-R-M → F-R-O-M needs 2 swaps
		{"abc", "xyz", 3},
	}
	for _, tc := range cases {
		t.Run(tc.a+"->"+tc.b, func(t *testing.T) {
			assert.Equal(t, tc.want, levenshtein(tc.a, tc.b))
		})
	}
}

func TestParseError_Format(t *testing.T) {
	e := &ParseError{
		Pos:        token.Position{Line: 1, Column: 8, Offset: 7},
		Message:    "expected FROM",
		Query:      "SHOWS WHEN 1977;",
		DidYouMean: "FROM",
		Hint:       "Try: SHOWS FROM 1977;",
	}
	out := e.Error()
	assert.Contains(t, out, "line 1, column 8")
	assert.Contains(t, out, "expected FROM")
	assert.Contains(t, out, "SHOWS WHEN 1977;")
	assert.Contains(t, out, "Did you mean: FROM")
	assert.Contains(t, out, "Hint: Try: SHOWS FROM 1977;")
}

func TestQueryError_WithSuggestions(t *testing.T) {
	e := &QueryError{
		Type:        ErrSongNotFound,
		Message:     "Bertha",
		Suggestions: []string{"Bertha (live)", "Bertha #2"},
	}
	out := e.Error()
	assert.Contains(t, out, "song not found: Bertha")
	assert.Contains(t, out, "Did you mean")
	assert.Contains(t, out, "Bertha (live)")
}
