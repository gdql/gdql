// Package main is the GDQL data import tool.
// Separate from the query engine (gdql) to keep concerns clean.
//
// Usage:
//
//	gdql-import [-db path] setlistfm          Import from setlist.fm API
//	gdql-import [-db path] json <file>        Import from canonical JSON
//	gdql-import [-db path] lyrics <file>      Import lyrics JSON
//	gdql-import [-db path] aliases <file>     Import song alias mappings
//	gdql-import [-db path] fix-sets           Re-infer set numbers from song order
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/import/canonical"
	"github.com/gdql/gdql/internal/import/deadlists"
	"github.com/gdql/gdql/internal/import/setlistfm"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func main() {
	args := os.Args[1:]
	dbPath := "shows.db"

	// Parse -db flag
	for i := 0; i < len(args); i++ {
		if args[i] == "-db" && i+1 < len(args) {
			dbPath = args[i+1]
			args = append(args[:i], args[i+2:]...)
			break
		}
	}
	if p := os.Getenv("GDQL_DB"); p != "" && dbPath == "shows.db" {
		dbPath = p
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "setlistfm":
		apiKey := os.Getenv("SETLISTFM_API_KEY")
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "Error: SETLISTFM_API_KEY is not set")
			fmt.Fprintln(os.Stderr, "Get an API key at https://www.setlist.fm/settings/api")
			os.Exit(1)
		}
		if err := sqlite.InitSchema(dbPath); err != nil {
			fatal(err)
		}
		client := setlistfm.NewClient(apiKey)
		showsAdded, songsAdded, err := setlistfm.Import(context.Background(), dbPath, client)
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "Import complete: %d shows, %d songs\n", showsAdded, songsAdded)

	case "json":
		path := argOrFlag(args[1:])
		if path == "" {
			fmt.Fprintln(os.Stderr, "Usage: gdql-import [-db path] json <file.json>")
			os.Exit(1)
		}
		if err := sqlite.InitSchema(dbPath); err != nil {
			fatal(err)
		}
		db, err := sqlite.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer db.Close()
		data, err := os.ReadFile(path)
		if err != nil {
			fatal(err)
		}
		var shows []canonical.Show
		if err := json.Unmarshal(data, &shows); err != nil {
			fatal(fmt.Errorf("parsing JSON: %w", err))
		}
		showsAdded, songsAdded, err := canonical.WriteShows(context.Background(), db.DB(), shows)
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "Import complete: %d shows, %d songs\n", showsAdded, songsAdded)

	case "lyrics":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gdql-import [-db path] lyrics <file.json>")
			os.Exit(1)
		}
		db, err := sqlite.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer db.Close()
		loaded, skipped, err := canonical.ImportLyrics(context.Background(), db.DB(), args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "Lyrics: %d loaded, %d skipped\n", loaded, skipped)

	case "aliases":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gdql-import [-db path] aliases <file.json>")
			os.Exit(1)
		}
		db, err := sqlite.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer db.Close()
		loaded, skipped, err := sqlite.LoadAliasesFromFile(context.Background(), db.DB(), args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "Aliases: %d loaded, %d skipped\n", loaded, skipped)

	case "relations":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gdql-import [-db path] relations <file.json>")
			os.Exit(1)
		}
		db, err := sqlite.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer db.Close()
		loaded, skipped, err := sqlite.LoadRelationsFromFile(context.Background(), db.DB(), args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "Relations: %d loaded, %d skipped\n", loaded, skipped)

	case "deadlists":
		firstYear, lastYear := 1965, 1995
		if len(args) >= 2 {
			firstYear, _ = strconv.Atoi(args[1])
		}
		if len(args) >= 3 {
			lastYear, _ = strconv.Atoi(args[2])
		}
		if err := importDeadlists(dbPath, firstYear, lastYear); err != nil {
			fatal(err)
		}

	case "fix-sets":
		if err := fixSets(dbPath); err != nil {
			fatal(err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

// importDeadlists crawls setlists.net for shows with proper set structure.
func importDeadlists(dbPath string, firstYear, lastYear int) error {
	if err := sqlite.InitSchema(dbPath); err != nil {
		return err
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	client := deadlists.NewClient()
	var allShows []canonical.Show
	var allIDs []int

	for year := firstYear; year <= lastYear; year++ {
		ids, err := client.FetchShowIDs(year)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %d: %v\n", year, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%d: %d shows\n", year, len(ids))
		allIDs = append(allIDs, ids...)
	}

	fmt.Fprintf(os.Stderr, "Fetching %d shows (10 concurrent)...\n", len(allIDs))
	shows := client.FetchShowsConcurrent(allIDs, 10)
	for _, s := range shows {
		allShows = append(allShows, *s)
	}
	fmt.Fprintf(os.Stderr, "Fetched %d shows successfully\n", len(allShows))

	if len(allShows) == 0 {
		fmt.Fprintln(os.Stderr, "No shows fetched.")
		return nil
	}

	showsAdded, songsAdded, err := canonical.WriteShows(context.Background(), db.DB(), allShows)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Import complete: %d shows, %d songs (from %d fetched)\n", showsAdded, songsAdded, len(allShows))
	return nil
}

// fixSets re-infers set numbers for all shows that have everything in set 1.
func fixSets(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Find shows where all performances are in set 1 and there are >8 songs
	rows, err := db.Query(`
		SELECT show_id, count(*) as cnt
		FROM performances
		GROUP BY show_id
		HAVING cnt > 8 AND min(set_number) = max(set_number) AND min(set_number) = 1
	`)
	if err != nil {
		return err
	}

	type showInfo struct {
		id    int
		count int
	}
	var shows []showInfo
	for rows.Next() {
		var s showInfo
		if err := rows.Scan(&s.id, &s.count); err != nil {
			rows.Close()
			return err
		}
		shows = append(shows, s)
	}
	rows.Close()

	if len(shows) == 0 {
		fmt.Fprintln(os.Stderr, "No shows need set number fixes.")
		return nil
	}
	fmt.Fprintf(os.Stderr, "Fixing set numbers for %d shows...\n", len(shows))

	fixed := 0
	for _, show := range shows {
		// Get all performances for this show in order
		perfRows, err := db.Query(`
			SELECT p.id, s.name
			FROM performances p
			JOIN songs s ON p.song_id = s.id
			WHERE p.show_id = ?
			ORDER BY p.position
		`, show.id)
		if err != nil {
			return err
		}

		type perfInfo struct {
			id   int
			name string
		}
		var perfs []perfInfo
		for perfRows.Next() {
			var p perfInfo
			if err := perfRows.Scan(&p.id, &p.name); err != nil {
				perfRows.Close()
				return err
			}
			perfs = append(perfs, p)
		}
		perfRows.Close()

		if len(perfs) <= 8 {
			continue
		}

		// Build fake Song slice for inferSetBreaks
		songs := make([]setlistfm.Song, len(perfs))
		for i, p := range perfs {
			songs[i] = setlistfm.Song{Name: p.name}
		}
		sets := setlistfm.InferSetBreaks(songs)
		if len(sets) <= 1 {
			continue
		}

		// Apply set numbers
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		perfIdx := 0
		for setIdx, set := range sets {
			setNum := setIdx + 1
			if set.Encore > 0 {
				setNum = 3 + set.Encore
			}
			pos := 0
			for range set.Songs {
				if perfIdx >= len(perfs) {
					break
				}
				pos++
				isOpener := 0
				if pos == 1 && setNum == 1 {
					isOpener = 1
				}
				isCloser := 0
				if pos == len(set.Songs) {
					isCloser = 1
				}
				_, err := tx.Exec(
					"UPDATE performances SET set_number = ?, position = ?, is_opener = ?, is_closer = ? WHERE id = ?",
					setNum, pos, isOpener, isCloser, perfs[perfIdx].id,
				)
				if err != nil {
					tx.Rollback()
					return err
				}
				perfIdx++
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		fixed++
	}

	fmt.Fprintf(os.Stderr, "Fixed %d of %d shows.\n", fixed, len(shows))
	return nil
}

func argOrFlag(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if args[0] == "-f" && len(args) >= 2 {
		return args[1]
	}
	return args[0]
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func printUsage() {
	w := os.Stderr
	fmt.Fprintln(w, "Usage: gdql-import [-db <path>] <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  deadlists [first] [last]   Crawl setlists.net for proper set data (default: 1965-1995)")
	fmt.Fprintln(w, "  setlistfm                  Import shows from setlist.fm (requires SETLISTFM_API_KEY)")
	fmt.Fprintln(w, "  json <file>                Import from canonical JSON")
	fmt.Fprintln(w, "  lyrics <file>              Import lyrics from JSON")
	fmt.Fprintln(w, "  aliases <file>             Import song alias mappings")
	fmt.Fprintln(w, "  relations <file>           Import song-to-song relations (variant_of, merge_into, pairs_with)")
	fmt.Fprintln(w, "  fix-sets                   Re-infer set numbers for shows with flat set data")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -db <path>         Database path (default: shows.db, or GDQL_DB env)")
}

