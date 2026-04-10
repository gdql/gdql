package setlistfm

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
}

func TestClient_GetSetlist_Success(t *testing.T) {
	body := `{
		"id": "abc123",
		"versionId": "v1",
		"eventDate": "08-05-1977",
		"venue": {"id":"v1","name":"Barton Hall","city":{"name":"Ithaca","stateCode":"NY","country":{"code":"US","name":"USA"}}},
		"sets": {"set": [
			{"name":"Set 1","encore":0,"song":[{"name":"Minglewood Blues"},{"name":"Loser"}]},
			{"name":"","encore":1,"song":[{"name":"One More Saturday Night"}]}
		]}
	}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		require.Contains(t, r.URL.Path, "/setlist/version/v1")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	sl, err := c.GetSetlist("v1")
	require.NoError(t, err)
	require.Equal(t, "abc123", sl.ID)
	require.Equal(t, "08-05-1977", sl.EventDate)
	require.Equal(t, "Barton Hall", sl.Venue.Name)
	require.Len(t, sl.Set, 2)
	require.Len(t, sl.Set[0].Songs, 2)
	require.Equal(t, "Minglewood Blues", sl.Set[0].Songs[0].Name)
	require.Equal(t, 1, sl.Set[1].Encore)
}

func TestClient_GetSetlist_NoAPIKey(t *testing.T) {
	c := &Client{HTTPClient: &http.Client{}}
	_, err := c.GetSetlist("v1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestClient_GetSetlist_NotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetSetlist("nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}

func TestClient_GetArtistSetlists_Success(t *testing.T) {
	body := `{
		"setlist": [
			{"id":"1","versionId":"v1","eventDate":"08-05-1977","venue":{"name":"Barton Hall","city":{"name":"Ithaca","stateCode":"NY","country":{"code":"US"}}}},
			{"id":"2","versionId":"v2","eventDate":"26-02-1977","venue":{"name":"Winterland","city":{"name":"San Francisco","stateCode":"CA","country":{"code":"US"}}}}
		],
		"total": 2,
		"page": 1,
		"itemsPerPage": 20
	}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		require.Contains(t, r.URL.Path, "/artist/")
		require.Contains(t, r.URL.RawQuery, "p=1")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := c.GetArtistSetlists(GratefulDeadMBID, 1)
	require.NoError(t, err)
	require.Equal(t, 2, resp.Total)
	require.Len(t, resp.Setlist, 2)
	require.Equal(t, "Barton Hall", resp.Setlist[0].Venue.Name)
}

func TestClient_GetArtistSetlists_NoAPIKey(t *testing.T) {
	c := &Client{HTTPClient: &http.Client{}}
	_, err := c.GetArtistSetlists(GratefulDeadMBID, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestClient_GetArtistSetlists_ServerError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := c.GetArtistSetlists(GratefulDeadMBID, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestThrottleTransport_Concurrent(t *testing.T) {
	// Verify mutex protects t.last under concurrent access — should not race.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tt := &throttleTransport{perSec: 100, rt: http.DefaultTransport}
	client := &http.Client{Transport: tt}

	done := make(chan struct{}, 5)
	for i := 0; i < 5; i++ {
		go func() {
			req, _ := http.NewRequest("GET", srv.URL, nil)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 5; i++ {
		<-done
	}
	// If the test runs with -race and there's a data race, it would fail.
}
