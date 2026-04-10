// Package run exposes a single entry point for executing GDQL queries
// (e.g. from the sandbox API). It uses the internal executor and formatter.
package run

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/formatter"
)

// RunWithDB executes one or more semicolon-separated GDQL queries against
// the SQLite database at dbPath and returns the result as JSON.
// Multiple statements produce a JSON array of results.
func RunWithDB(ctx context.Context, dbPath, query string) (jsonResult string, err error) {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	stmts := SplitStatements(query)
	if len(stmts) == 0 {
		return "{}", nil
	}

	// Single statement — return a single result object (backwards compatible).
	if len(stmts) == 1 {
		return runOne(ctx, db, stmts[0])
	}

	// Multiple statements — return a JSON array.
	results := make([]json.RawMessage, 0, len(stmts))
	for _, s := range stmts {
		j, err := runOne(ctx, db, s)
		if err != nil {
			return "", err
		}
		results = append(results, json.RawMessage(j))
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func runOne(ctx context.Context, db *sqlite.DB, query string) (string, error) {
	ex := executor.New(db)
	result, err := ex.Execute(ctx, query)
	if err != nil {
		return "", err
	}
	fmtr := formatter.New()
	return fmtr.Format(result, formatter.FormatJSON)
}

// SplitStatements splits input on semicolons, ignoring semicolons inside strings.
func SplitStatements(input string) []string {
	var stmts []string
	var current strings.Builder
	inString := false
	quote := byte(0)

	for i := 0; i < len(input); i++ {
		c := input[i]
		if inString {
			current.WriteByte(c)
			if c == quote {
				inString = false
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = true
			quote = c
			current.WriteByte(c)
			continue
		}
		if c == ';' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				stmts = append(stmts, s+";")
			}
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	// Trailing statement without semicolon.
	s := strings.TrimSpace(current.String())
	if s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}
