// Script to scrape Grateful Dead lyrics and output lyrics.json.
// Usage: go run scripts/scrape_lyrics.go -db shows.db -out lyrics.json
//
// Uses Genius web search + HTML scraping. Rate-limited to 1 req/sec.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SongLyrics struct {
	Song   string `json:"song"`
	Lyrics string `json:"lyrics"`
}

func main() {
	dbPath := flag.String("db", "shows.db", "path to GDQL database")
	outPath := flag.String("out", "lyrics.json", "output JSON file")
	minPlays := flag.Int("min-plays", 50, "minimum performances to include a song")
	limit := flag.Int("limit", 0, "max songs to scrape (0 = all)")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Get songs ordered by play count
	query := `SELECT s.name, count(*) as plays
		FROM performances p JOIN songs s ON p.song_id = s.id
		GROUP BY s.id HAVING plays >= ?
		ORDER BY plays DESC`
	rows, err := db.Query(query, *minPlays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query error: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var songs []string
	for rows.Next() {
		var name string
		var plays int
		rows.Scan(&name, &plays)
		// Skip non-songs
		lower := strings.ToLower(name)
		if strings.Contains(lower, "drums") || strings.Contains(lower, "space") ||
			strings.Contains(lower, "tuning") || strings.Contains(lower, "jam") ||
			strings.Contains(lower, "comment") || strings.Contains(lower, "discussion") ||
			strings.Contains(lower, "dead air") || strings.Contains(lower, "step back") {
			continue
		}
		songs = append(songs, name)
	}

	if *limit > 0 && len(songs) > *limit {
		songs = songs[:*limit]
	}

	fmt.Fprintf(os.Stderr, "Scraping lyrics for %d songs...\n", len(songs))

	// Load existing lyrics if output file exists
	var results []SongLyrics
	existing := make(map[string]bool)
	if data, err := os.ReadFile(*outPath); err == nil {
		json.Unmarshal(data, &results)
		for _, r := range results {
			existing[r.Song] = true
		}
		fmt.Fprintf(os.Stderr, "Loaded %d existing lyrics, resuming...\n", len(existing))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	found := 0
	skipped := 0

	for i, song := range songs {
		if existing[song] {
			skipped++
			continue
		}

		fmt.Fprintf(os.Stderr, "[%d/%d] %s... ", i+1, len(songs), song)
		lyrics, err := fetchLyrics(client, song)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SKIP (%v)\n", err)
			skipped++
		} else if lyrics == "" {
			fmt.Fprintf(os.Stderr, "SKIP (no lyrics found)\n")
			skipped++
		} else {
			fmt.Fprintf(os.Stderr, "OK (%d chars)\n", len(lyrics))
			results = append(results, SongLyrics{Song: song, Lyrics: lyrics})
			found++

			// Save progress every 10 songs
			if found%10 == 0 {
				saveJSON(results, *outPath)
			}
		}

		time.Sleep(1500 * time.Millisecond) // rate limit
	}

	saveJSON(results, *outPath)
	fmt.Fprintf(os.Stderr, "\nDone: %d lyrics found, %d skipped. Saved to %s\n", found, skipped, *outPath)
}

func saveJSON(results []SongLyrics, path string) {
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(path, data, 0644)
}

func fetchLyrics(client *http.Client, song string) (string, error) {
	// Search Genius for "Grateful Dead <song>"
	searchQuery := "Grateful Dead " + song
	searchURL := "https://genius.com/api/search/multi?per_page=5&q=" + url.QueryEscape(searchQuery)

	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "GDQL-lyrics-scraper/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("search returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	// Parse the search response to find the song URL
	songURL := extractSongURL(body, song)
	if songURL == "" {
		return "", fmt.Errorf("no Genius match")
	}

	time.Sleep(500 * time.Millisecond)

	// Fetch the song page and extract lyrics
	req2, _ := http.NewRequest("GET", songURL, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp2, err := client.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		return "", fmt.Errorf("page returned %d", resp2.StatusCode)
	}

	pageBody, _ := io.ReadAll(resp2.Body)
	return extractLyricsFromPage(string(pageBody)), nil
}

func extractSongURL(body []byte, songName string) string {
	// The Genius search API returns JSON with hits containing url fields
	// Look for a URL matching "grateful-dead" in the path
	type hit struct {
		Result struct {
			URL             string `json:"url"`
			Title           string `json:"title"`
			PrimaryArtist   struct {
				Name string `json:"name"`
			} `json:"primary_artist"`
		} `json:"result"`
	}
	type section struct {
		Type string `json:"type"`
		Hits []hit  `json:"hits"`
	}
	type searchResp struct {
		Response struct {
			Sections []section `json:"sections"`
		} `json:"response"`
	}

	var sr searchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return ""
	}

	songLower := strings.ToLower(songName)
	for _, sec := range sr.Response.Sections {
		for _, h := range sec.Hits {
			artistLower := strings.ToLower(h.Result.PrimaryArtist.Name)
			titleLower := strings.ToLower(h.Result.Title)
			if strings.Contains(artistLower, "grateful dead") &&
				(strings.Contains(titleLower, songLower) || strings.Contains(songLower, titleLower)) {
				return h.Result.URL
			}
		}
	}
	// Fallback: any Grateful Dead result
	for _, sec := range sr.Response.Sections {
		for _, h := range sec.Hits {
			if strings.Contains(strings.ToLower(h.Result.PrimaryArtist.Name), "grateful dead") {
				return h.Result.URL
			}
		}
	}
	return ""
}

var tagRe = regexp.MustCompile(`<[^>]+>`)
var multiNewline = regexp.MustCompile(`\n{3,}`)
var contributorsRe = regexp.MustCompile(`^\d+\s+Contributor.*Lyrics`)

func extractLyricsFromPage(html string) string {
	// Genius puts lyrics in divs with data-lyrics-container="true"
	// These divs can contain nested HTML, so we can't use a simple regex.
	// Instead, find each container start and extract until the closing </div> at the right depth.
	var parts []string
	marker := `data-lyrics-container="true"`
	searchFrom := 0

	for {
		idx := strings.Index(html[searchFrom:], marker)
		if idx < 0 {
			break
		}
		idx += searchFrom

		// Find the > that closes this opening tag
		tagClose := strings.Index(html[idx:], ">")
		if tagClose < 0 {
			break
		}
		contentStart := idx + tagClose + 1

		// Find matching </div> by tracking depth
		depth := 1
		pos := contentStart
		for pos < len(html) && depth > 0 {
			nextOpen := strings.Index(html[pos:], "<div")
			nextClose := strings.Index(html[pos:], "</div>")
			if nextClose < 0 {
				break
			}
			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				pos += nextOpen + 4
			} else {
				depth--
				if depth == 0 {
					content := html[contentStart : pos+nextClose]
					text := cleanLyricsHTML(content)
					if text != "" {
						parts = append(parts, text)
					}
				}
				pos += nextClose + 6
			}
		}
		searchFrom = pos
	}

	lyrics := strings.Join(parts, "\n\n")
	lyrics = multiNewline.ReplaceAllString(lyrics, "\n\n")

	// Remove the "N Contributors...Lyrics" header that sometimes appears
	lyrics = contributorsRe.ReplaceAllString(lyrics, "")
	lyrics = multiNewline.ReplaceAllString(lyrics, "\n\n")
	return strings.TrimSpace(lyrics)
}

func cleanLyricsHTML(html string) string {
	// Replace <br/> with newlines
	text := strings.ReplaceAll(html, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br >", "\n")
	// Strip HTML tags
	text = tagRe.ReplaceAllString(text, "")
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&#x27;", "'")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	return strings.TrimSpace(text)
}
