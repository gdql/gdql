package acceptance

import (
	"context"
	"testing"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/require"
)

func TestE2E_ShowsFromYear(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), "SHOWS FROM 1977 LIMIT 5")
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 2, "seed has 2 shows in 1977 (Winterland, Cornell)")
	dates := make([]string, len(result.Shows))
	for i, s := range result.Shows {
		dates[i] = s.Date.Format("2006-01-02")
	}
	require.Contains(t, dates, "1977-02-26")
	require.Contains(t, dates, "1977-05-08")
}

func TestE2E_SegueScarletFire(t *testing.T) {
	path, cleanup := fixtures.CreateTestDB(t)
	defer cleanup()

	db, err := sqlite.Open(path)
	require.NoError(t, err)
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), `SHOWS FROM 1977-1978 WHERE "Scarlet Begonias" > "Fire on the Mountain"`)
	require.NoError(t, err)
	require.Equal(t, executor.ResultShows, result.Type)
	require.Len(t, result.Shows, 3, "seed has Scarlet > Fire at Cornell, Winterland, Landover")
}
