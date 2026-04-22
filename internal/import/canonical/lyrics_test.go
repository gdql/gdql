package canonical

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestImportLyrics(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer conn.Close()

	// Write test lyrics JSON
	lyricsJSON := `[
		{"song": "Scarlet Begonias", "lyrics": "As I was walkin round Grosvenor Square"},
		{"song": "Fire on the Mountain", "lyrics": "Long distance runner what you standin there for"},
		{"song": "Nonexistent Song", "lyrics": "should be skipped"},
		{"song": "", "lyrics": "empty song name skipped"}
	]`
	lyricsPath := filepath.Join(t.TempDir(), "lyrics.json")
	require.NoError(t, os.WriteFile(lyricsPath, []byte(lyricsJSON), 0644))

	ctx := context.Background()
	loaded, skipped, err := ImportLyrics(ctx, conn, lyricsPath)
	require.NoError(t, err)
	require.Equal(t, 2, loaded)
	require.Equal(t, 2, skipped)

	// Verify lyrics are queryable
	var lyrics string
	err = conn.QueryRowContext(ctx, "SELECT lyrics FROM lyrics WHERE song_id = 1").Scan(&lyrics)
	require.NoError(t, err)
	require.Contains(t, lyrics, "Grosvenor Square")

	// Verify FTS content is lowercase
	var fts string
	err = conn.QueryRowContext(ctx, "SELECT lyrics_fts FROM lyrics WHERE song_id = 2").Scan(&fts)
	require.NoError(t, err)
	require.Contains(t, fts, "long distance runner")
}

func TestImportLyrics_CaseInsensitiveMatch(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	conn, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer conn.Close()

	lyricsJSON := `[{"song": "fire on the mountain", "lyrics": "Long distance runner"}]`
	lyricsPath := filepath.Join(t.TempDir(), "lyrics.json")
	require.NoError(t, os.WriteFile(lyricsPath, []byte(lyricsJSON), 0644))

	loaded, _, err := ImportLyrics(context.Background(), conn, lyricsPath)
	require.NoError(t, err)
	require.Equal(t, 1, loaded, "should match case-insensitively")
}
