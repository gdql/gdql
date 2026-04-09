package shared

import (
	"testing"

	"github.com/gdql/gdql/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaxID_KnownTables(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	cases := []string{"venues", "shows", "songs", "performances"}
	for _, table := range cases {
		t.Run(table, func(t *testing.T) {
			id, err := MaxID(db, table)
			require.NoError(t, err)
			require.GreaterOrEqual(t, id, int64(0))
		})
	}
}

func TestMaxID_UnknownTableRejected(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	cases := []string{"foo", "users; DROP TABLE shows;--", "", "lyrics"}
	for _, table := range cases {
		t.Run(table, func(t *testing.T) {
			_, err := MaxID(db, table)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown table")
		})
	}
}

func TestMaxID_FixtureValues(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	// Fixture has 3 shows, 6 songs, 12 performances, 3 venues
	id, err := MaxID(db, "shows")
	require.NoError(t, err)
	require.Equal(t, int64(3), id)

	id, err = MaxID(db, "songs")
	require.NoError(t, err)
	require.Equal(t, int64(6), id)
}

func TestLoadSongByName(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	m, err := LoadSongByName(db)
	require.NoError(t, err)
	// 6 songs + 1 alias ("Scarlet Begonias-")
	require.Equal(t, int64(1), m["Scarlet Begonias"])
	require.Equal(t, int64(2), m["Fire on the Mountain"])
	require.Equal(t, int64(6), m["Dark Star"])
	require.Equal(t, int64(1), m["Scarlet Begonias-"], "alias should resolve")
	require.GreaterOrEqual(t, len(m), 7)
}

func TestShowExists_True(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	// Cornell '77 from minimal_data.sql
	exists := ShowExists(db, "1977-05-08", "Barton Hall", "Ithaca", "NY", "USA")
	require.True(t, exists)
}

func TestShowExists_False(t *testing.T) {
	db := fixtures.OpenTestDB(t)
	defer db.Close()

	exists := ShowExists(db, "2000-01-01", "Made Up Hall", "Nowhere", "XX", "ZZ")
	require.False(t, exists)
}

func TestNullStr(t *testing.T) {
	assert.Nil(t, NullStr(""))
	assert.Equal(t, "hello", NullStr("hello"))
	assert.Equal(t, " ", NullStr(" "), "whitespace is not empty")
}
