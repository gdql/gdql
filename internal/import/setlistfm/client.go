package setlistfm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://api.setlist.fm/rest/1.0"

// Grateful Dead MusicBrainz ID
const GratefulDeadMBID = "6faa7ca7-0d99-4a5e-bfa6-1fd5037520c6"

// Client calls the setlist.fm REST API.
type Client struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewClient returns a client that uses the given API key (x-api-key header).
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &throttleTransport{perSec: 1, rt: http.DefaultTransport},
		},
	}
}

// throttleTransport limits requests to perSec per second.
type throttleTransport struct {
	perSec int
	rt     http.RoundTripper
	last   time.Time
}

func (t *throttleTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	elapsed := time.Since(t.last)
	if elapsed < time.Second/time.Duration(t.perSec) {
		time.Sleep(time.Second/time.Duration(t.perSec) - elapsed)
	}
	t.last = time.Now()
	return t.rt.RoundTrip(req)
}

// SetlistsResponse is the paginated response for artist setlists.
type SetlistsResponse struct {
	Setlist      []Setlist `json:"setlist"`
	Total        int       `json:"total"`
	Page         int       `json:"page"`
	ItemsPerPage int       `json:"itemsPerPage"`
}

// Setlist is a single setlist (one show).
type Setlist struct {
	ID          string `json:"id"`
	VersionID   string `json:"versionId"`
	EventDate   string `json:"eventDate"`   // dd-MM-yyyy
	LastUpdated string `json:"lastUpdated"`
	Info        string `json:"info"`
	URL         string `json:"url"`
	Venue       Venue  `json:"venue"`
	Tour        *Tour  `json:"tour"`
	Set         []Set  `json:"set"`
}

// Venue is the venue of a setlist.
type Venue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	City *City  `json:"city"`
	URL  string `json:"url"`
}

// City has name, state, country.
type City struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	StateCode string   `json:"stateCode"`
	State     string   `json:"state"`
	Country   *Country `json:"country"`
}

// Country has code and name.
type Country struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Tour has optional tour name.
type Tour struct {
	Name string `json:"name"`
}

// Set is one set or encore.
type Set struct {
	Name   string `json:"name"`
	Encore int    `json:"encore"`
	Songs  []Song `json:"song"`
}

// Song is one song in a set.
type Song struct {
	Name string `json:"name"`
	Info string `json:"info"` // e.g. ">" for segue into this song
	Tape bool   `json:"tape"`
}

// GetSetlist fetches a single setlist by version ID (full details including sets/songs).
// On 429 Too Many Requests retries up to 3 times with backoff (respecting Retry-After if present).
func (c *Client) GetSetlist(versionID string) (*Setlist, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("setlist.fm API key required")
	}
	url := fmt.Sprintf("%s/setlist/version/%s", baseURL, versionID)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("Accept", "application/json")
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			var out Setlist
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				resp.Body.Close()
				return nil, err
			}
			resp.Body.Close()
			return &out, nil
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("setlist.fm API: %s", resp.Status)
		if resp.StatusCode != http.StatusTooManyRequests {
			return nil, lastErr
		}
		wait := 60 * time.Second
		if s := resp.Header.Get("Retry-After"); s != "" {
			if sec, err := strconv.Atoi(s); err == nil && sec > 0 && sec <= 3600 {
				wait = time.Duration(sec) * time.Second
			}
		}
		if attempt < 2 {
			time.Sleep(wait)
		}
	}
	return nil, fmt.Errorf("%w (daily limit may be exceeded; run again tomorrow to resume)", lastErr)
}

// GetArtistSetlists fetches a page of setlists for the given artist MBID.
// On 429 Too Many Requests it retries up to 3 times with backoff (respecting Retry-After if present).
func (c *Client) GetArtistSetlists(mbid string, page int) (*SetlistsResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("setlist.fm API key required (SETLISTFM_API_KEY or -api-key)")
	}
	url := fmt.Sprintf("%s/artist/%s/setlists?p=%d", baseURL, mbid, page)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusOK {
			var out SetlistsResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				resp.Body.Close()
				return nil, err
			}
			resp.Body.Close()
			return &out, nil
		}

		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("setlist.fm API: %s", resp.Status)

		if resp.StatusCode != http.StatusTooManyRequests {
			return nil, lastErr
		}

		// 429: backoff then retry
		wait := 60 * time.Second
		if s := resp.Header.Get("Retry-After"); s != "" {
			if sec, err := strconv.Atoi(s); err == nil && sec > 0 && sec <= 3600 {
				wait = time.Duration(sec) * time.Second
			}
		}
		if attempt < 2 {
			time.Sleep(wait)
		}
	}
	return nil, fmt.Errorf("%w (free tier: 1440 requests/day; try again later or request an upgrade at https://www.setlist.fm/settings/api)", lastErr)
}
