package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticResolver_Resolve_Exact(t *testing.T) {
	r := NewStaticResolver(map[string]int{
		"Scarlet Begonias": 1,
		"Fire on the Mountain": 2,
	})
	id, err := r.Resolve(context.Background(), "Scarlet Begonias")
	require.NoError(t, err)
	require.Equal(t, 1, id)
}

func TestStaticResolver_Resolve_CaseInsensitive(t *testing.T) {
	r := NewStaticResolver(map[string]int{"Dark Star": 10})
	id, err := r.Resolve(context.Background(), "dark star")
	require.NoError(t, err)
	require.Equal(t, 10, id)
}

func TestStaticResolver_Resolve_NotFound(t *testing.T) {
	r := NewStaticResolver(map[string]int{"Scarlet Begonias": 1})
	_, err := r.Resolve(context.Background(), "Unknown Song")
	require.Error(t, err)
	var nf *ErrSongNotFound
	require.ErrorAs(t, err, &nf)
	require.Equal(t, "Unknown Song", nf.Name)
}

func TestStaticResolver_Suggest(t *testing.T) {
	r := NewStaticResolver(map[string]int{
		"Scarlet Begonias": 1,
		"Fire on the Mountain": 2,
	})
	sug := r.Suggest(context.Background(), "Scarlet")
	require.Contains(t, sug, "Scarlet Begonias")
}
