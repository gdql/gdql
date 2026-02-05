package resolver

import (
	"context"
	"strings"
)

// SongResolver resolves song names to canonical IDs.
type SongResolver interface {
	Resolve(ctx context.Context, name string) (int, error)
	ResolveFuzzy(ctx context.Context, name string) ([]SongMatch, error)
	Suggest(ctx context.Context, name string) []string
}

// SongMatch is a fuzzy match result.
type SongMatch struct {
	ID    int
	Name  string
	Score float64
}

// StaticResolver resolves names from a fixed map (for tests or small catalogs).
type StaticResolver struct {
	ByName map[string]int
	ByID   map[int]string
}

// NewStaticResolver builds a resolver from name -> id. ByID is filled from ByName.
func NewStaticResolver(byName map[string]int) *StaticResolver {
	byID := make(map[int]string)
	for name, id := range byName {
		byID[id] = name
	}
	return &StaticResolver{ByName: byName, ByID: byID}
}

// Resolve returns the ID for an exact or case-insensitive match.
func (s *StaticResolver) Resolve(ctx context.Context, name string) (int, error) {
	if id, ok := s.ByName[name]; ok {
		return id, nil
	}
	lower := strings.ToLower(name)
	for n, id := range s.ByName {
		if strings.ToLower(n) == lower {
			return id, nil
		}
	}
	return 0, &ErrSongNotFound{Name: name}
}

// ErrSongNotFound is returned when a song name cannot be resolved.
type ErrSongNotFound struct {
	Name string
}

func (e *ErrSongNotFound) Error() string {
	return "song not found: " + e.Name
}

// ResolveFuzzy returns matches containing the name (for typos); not implemented in stub.
func (s *StaticResolver) ResolveFuzzy(ctx context.Context, name string) ([]SongMatch, error) {
	var out []SongMatch
	lower := strings.ToLower(name)
	for n, id := range s.ByName {
		if strings.Contains(strings.ToLower(n), lower) || strings.Contains(lower, strings.ToLower(n)) {
			score := 0.5
			if strings.EqualFold(n, name) {
				score = 1.0
			}
			out = append(out, SongMatch{ID: id, Name: n, Score: score})
		}
	}
	return out, nil
}

// Suggest returns names that might match (for "did you mean?").
func (s *StaticResolver) Suggest(ctx context.Context, name string) []string {
	matches, _ := s.ResolveFuzzy(ctx, name)
	out := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		if !seen[m.Name] {
			seen[m.Name] = true
			out = append(out, m.Name)
		}
	}
	return out
}
