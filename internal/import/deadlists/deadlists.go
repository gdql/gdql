// Package deadlists crawls setlists.net (Deadlists) for Grateful Dead show data
// with proper set/encore structure.
package deadlists

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdql/gdql/internal/import/canonical"
)

var (
	showIDRe    = regexp.MustCompile(`show_id=(\d+)`)
	dateRe      = regexp.MustCompile(`(\d{1,2}/\d{1,2}/\d{2,4})`)
	setHeaderRe = regexp.MustCompile(`<b>(Set \d+|Encore\d*)\s*:?\s*</b>`)
)

// Client fetches and parses Deadlists pages.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Delay      time.Duration
}

// NewClient creates a Deadlists client with rate limiting.
func NewClient() *Client {
	return &Client{
		BaseURL:    "http://www.setlists.net",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Delay:      300 * time.Millisecond,
	}
}

// FetchShowIDs returns all show IDs for a given year.
func (c *Client) FetchShowIDs(year int) ([]int, error) {
	time.Sleep(c.Delay)
	url := fmt.Sprintf("%s/?search=true&year=%d", c.BaseURL, year)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	matches := showIDRe.FindAllStringSubmatch(body, -1)
	seen := make(map[int]bool)
	var ids []int
	for _, m := range matches {
		id, _ := strconv.Atoi(m[1])
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids, nil
}

// FetchShow fetches and parses a single show page into canonical format.
func (c *Client) FetchShow(showID int) (*canonical.Show, error) {
	time.Sleep(c.Delay)
	url := fmt.Sprintf("%s/?show_id=%d", c.BaseURL, showID)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	return parseShow(body)
}

// FetchShowsConcurrent fetches multiple shows in parallel with a concurrency limit.
func (c *Client) FetchShowsConcurrent(ids []int, workers int) []*canonical.Show {
	type result struct {
		show *canonical.Show
		id   int
		err  error
	}
	results := make(chan result, len(ids))
	sem := make(chan struct{}, workers)

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(showID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			url := fmt.Sprintf("%s/?show_id=%d", c.BaseURL, showID)
			body, err := c.get(url)
			if err != nil {
				results <- result{id: showID, err: err}
				return
			}
			show, err := parseShow(body)
			results <- result{show: show, id: showID, err: err}
		}(id)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var shows []*canonical.Show
	for r := range results {
		if r.err != nil {
			fmt.Fprintf(io.Discard, "  show %d: %v\n", r.id, r.err)
			continue
		}
		if r.show != nil {
			shows = append(shows, r.show)
		}
	}
	return shows
}

func (c *Client) get(url string) (string, error) {
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return "", err
	}
	// Deadlists uses Latin-1; convert to UTF-8 by replacing non-ASCII
	s := string(b)
	s = strings.ReplaceAll(s, "\x92", "'")
	s = strings.ReplaceAll(s, "\x93", "\"")
	s = strings.ReplaceAll(s, "\x94", "\"")
	s = strings.ReplaceAll(s, "\x96", "-")
	return s, nil
}

func parseShow(html string) (*canonical.Show, error) {
	// Extract date: bold date before the first set header
	var dateMatch string
	set1Idx := strings.Index(html, "Set 1:")
	if set1Idx < 0 {
		set1Idx = len(html)
	}
	// Look for bold date in the area before sets
	boldDateRe := regexp.MustCompile(`<b>\s*(\d{1,2}/\d{1,2}/\d{2,4})\s*</b>`)
	before := html[:set1Idx]
	m := boldDateRe.FindStringSubmatch(before)
	if len(m) >= 2 {
		dateMatch = m[1]
	} else {
		// Fallback: any date in the area before sets
		dateMatch = dateRe.FindString(before)
	}
	if dateMatch == "" {
		return nil, fmt.Errorf("no date found")
	}
	date := normalizeDate(dateMatch)
	if date == "" {
		return nil, fmt.Errorf("invalid date: %s", dateMatch)
	}

	// Extract venue/location from the page title or header area
	venue, city, state := parseVenue(html)

	// Parse sets
	sets := parseSets(html)
	if len(sets) == 0 {
		return nil, fmt.Errorf("no sets found")
	}

	return &canonical.Show{
		Date:  date,
		Venue: canonical.Venue{Name: venue, City: city, State: state, Country: "US"},
		Sets:  sets,
	}, nil
}

func parseSets(html string) []canonical.Set {
	// Split on set headers: <b>Set 1:</b>, <b>Set 2:</b>, <b>Encore:</b>
	parts := setHeaderRe.Split(html, -1)
	headers := setHeaderRe.FindAllStringSubmatch(html, -1)

	if len(headers) == 0 || len(parts) < 2 {
		return nil
	}

	var sets []canonical.Set
	for i, header := range headers {
		body := ""
		if i+1 < len(parts) {
			body = parts[i+1]
		}
		// Get text up to next <p> or end
		if idx := strings.Index(body, "<p>"); idx >= 0 {
			body = body[:idx]
		}
		// Also cut at comment sections
		if idx := strings.Index(body, "<table"); idx >= 0 {
			body = body[:idx]
		}
		if idx := strings.Index(body, "<div"); idx >= 0 {
			body = body[:idx]
		}

		songs := parseSongs(body)
		if len(songs) == 0 {
			continue
		}

		set := canonical.Set{Songs: songs}
		_ = header // set label info available if needed
		sets = append(sets, set)
	}
	return sets
}

func parseSongs(body string) []canonical.SongInSet {
	// Songs are separated by <br> or newlines
	body = strings.ReplaceAll(body, "<br>", "\n")
	body = strings.ReplaceAll(body, "<BR>", "\n")
	// Strip remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	body = tagRe.ReplaceAllString(body, "")
	body = html.UnescapeString(body)

	var songs []canonical.SongInSet
	for _, line := range strings.Split(body, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		// Skip non-song lines (comments, notes)
		if strings.HasPrefix(name, "(") || strings.HasPrefix(name, "*") || strings.HasPrefix(name, "[") {
			continue
		}
		// Detect segues: ">" at end of name or between songs
		segue := false
		if strings.HasSuffix(name, ">") {
			name = strings.TrimSpace(strings.TrimSuffix(name, ">"))
			// Next song has segue_before = true
		}
		// Handle "Song1 > Song2" on one line
		if strings.Contains(name, " > ") {
			parts := strings.Split(name, " > ")
			for j, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					songs = append(songs, canonical.SongInSet{Name: p, SegueBefore: j > 0})
				}
			}
			continue
		}
		// Check if previous song had >
		if len(songs) > 0 {
			prevName := songs[len(songs)-1].Name
			if strings.HasSuffix(prevName, ">") {
				songs[len(songs)-1].Name = strings.TrimSpace(strings.TrimSuffix(prevName, ">"))
				segue = true
			}
		}
		songs = append(songs, canonical.SongInSet{Name: name, SegueBefore: segue})
	}
	return songs
}

func parseVenue(html string) (venue, city, state string) {
	// Venue is in a bold tag after the date, before Set 1:
	// <b> The Mosque - Richmond, VA</b>
	set1Idx := strings.Index(html, "Set 1:")
	if set1Idx < 0 {
		set1Idx = len(html)
	}
	before := html[:set1Idx]
	bolds := regexp.MustCompile(`<b>([^<]+)</b>`).FindAllStringSubmatch(before, -1)

	// The venue is typically the last bold item before Set 1 that isn't
	// the date or "N Show(s) Found"
	for i := len(bolds) - 1; i >= 0; i-- {
		text := strings.TrimSpace(bolds[i][1])
		if text == "" {
			continue
		}
		// Skip date patterns and "Show Found"
		if dateRe.MatchString(text) || strings.Contains(text, "Show") {
			continue
		}
		// Parse "Venue - City, ST" or "Venue, City, ST"
		if idx := strings.Index(text, " - "); idx >= 0 {
			venue = strings.TrimSpace(text[:idx])
			location := strings.TrimSpace(text[idx+3:])
			parts := strings.Split(location, ", ")
			if len(parts) >= 1 {
				city = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				state = strings.TrimSpace(parts[1])
			}
			return venue, city, state
		}
		// Try "Venue, City, ST" format
		parts := strings.Split(text, ", ")
		if len(parts) >= 3 {
			venue = strings.TrimSpace(parts[0])
			city = strings.TrimSpace(parts[1])
			state = strings.TrimSpace(parts[2])
			return venue, city, state
		}
		if len(parts) >= 2 {
			venue = strings.TrimSpace(parts[0])
			city = strings.TrimSpace(parts[1])
			return venue, city, ""
		}
		return text, "", ""
	}
	return "Unknown", "", ""
}

func normalizeDate(d string) string {
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return ""
	}
	month, _ := strconv.Atoi(parts[0])
	day, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])
	if year < 100 {
		year += 1900
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || year < 1965 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}
