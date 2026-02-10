// Package run exposes a single entry point for executing GDQL queries
// (e.g. from the sandbox API). It uses the internal executor and formatter.
package run

import (
	"context"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/formatter"
	"github.com/gdql/gdql/internal/data/sqlite"
)

// RunWithDB executes a GDQL query against the SQLite database at dbPath
// and returns the result as JSON. Output format is JSON.
func RunWithDB(ctx context.Context, dbPath, query string) (jsonResult string, err error) {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()
	ex := executor.New(db)
	result, err := ex.Execute(ctx, query)
	if err != nil {
		return "", err
	}
	fmtr := formatter.New()
	return fmtr.Format(result, formatter.FormatJSON)
}
