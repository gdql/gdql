package run

import (
	"context"
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestRunWithDB_Shows(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	out, err := RunWithDB(context.Background(), path, "SHOWS FROM 1977;")
	require.NoError(t, err)
	require.Contains(t, out, `"type": "shows"`)
	require.Contains(t, out, "1977")
}

func TestRunWithDB_Count(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	out, err := RunWithDB(context.Background(), path, `COUNT "Dark Star";`)
	require.NoError(t, err)
	require.Contains(t, out, `"type": "count"`)
}

func TestRunWithDB_ParseError(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	_, err := RunWithDB(context.Background(), path, "BANANA;")
	require.Error(t, err)
}

func TestRunWithDB_BadDBPath(t *testing.T) {
	// SQLite will create the file if it doesn't exist, but querying empty schema fails
	_, err := RunWithDB(context.Background(), "/tmp/gdql-nonexistent-test.db", "SHOWS;")
	require.Error(t, err) // Either open or query fails
}
