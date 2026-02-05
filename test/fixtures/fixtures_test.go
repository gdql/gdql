package fixtures

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/stretchr/testify/require"
)

func TestCreateTestDB_AndQuery(t *testing.T) {
	path, cleanup := CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	rs, err := db.ExecuteQuery(context.Background(), "SELECT COUNT(*) FROM shows")
	require.NoError(t, err)
	require.Len(t, rs.Rows, 1)
	require.Len(t, rs.Columns, 1)

	song, err := db.GetSong(context.Background(), "Scarlet Begonias")
	require.NoError(t, err)
	require.NotNil(t, song)
	require.Equal(t, "Scarlet Begonias", song.Name)
}
