package fixtures

import (
	_ "embed"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed minimal_data.sql
var minimalDataSQL string

// CreateTestDB creates a temporary SQLite database with schema and minimal_data applied.
// Returns the file path and a cleanup function. The database is closed before return.
func CreateTestDB(t *testing.T) (path string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	if len(minimalDataSQL) > 0 {
		if _, err := db.Exec(minimalDataSQL); err != nil {
			t.Fatalf("exec minimal_data: %v", err)
		}
	}

	cleanup = func() { os.RemoveAll(dir) }
	return path, cleanup
}

// OpenTestDB opens a temporary database and returns a *sql.DB (caller must Close).
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path, cleanup := CreateTestDB(t)
	t.Cleanup(cleanup)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen test db: %v", err)
	}
	return db
}
