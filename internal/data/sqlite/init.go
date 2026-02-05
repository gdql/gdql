package sqlite

import (
	_ "embed"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed seed.sql
var seedSQL string

// Init creates a new database at path with the schema and optional seed data.
// If path exists and has tables, Init is a no-op (safe to call multiple times).
func Init(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if _, err := db.Exec(seedSQL); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}

// InitSchema creates the database with schema only (no seed). Use for import-from-API flows.
func InitSchema(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}
