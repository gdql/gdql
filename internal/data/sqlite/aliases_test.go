package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestLoadAliasesFromFile(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := Open(path)
	require.NoError(t, err)
	defer db.Close()

	dir := t.TempDir()
	aliasPath := filepath.Join(dir, "aliases.json")
	err = os.WriteFile(aliasPath, []byte(`[
		{"alias": "Scarlet Begonias-", "canonical": "Scarlet Begonias"},
		{"alias": "Fire On The Mountain", "canonical": "Fire on the Mountain"},
		{"alias": "Nonexistent Canonical", "canonical": "No Such Song In DB"}
	]`), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	loaded, skipped, err := LoadAliasesFromFile(ctx, db.DB(), aliasPath)
	require.NoError(t, err)
	require.Equal(t, 2, loaded, "two entries match songs in fixture")
	require.Equal(t, 1, skipped, "one canonical not found")

	// Resolve via alias
	song, err := db.GetSong(ctx, "Scarlet Begonias-")
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, "Scarlet Begonias", song.Name)

	song2, err := db.GetSong(ctx, "Fire On The Mountain")
	require.NoError(t, err)
	require.NotNil(t, song2)
	require.Equal(t, "Fire on the Mountain", song2.Name)
}
