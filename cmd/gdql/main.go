// Package main is the GDQL CLI entrypoint.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/formatter"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/import/canonical"
	"github.com/gdql/gdql/internal/import/setlistfm"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	args := os.Args[1:]
	if args[0] == "init" {
		path := "shows.db"
		if len(args) >= 2 {
			path = args[1]
		}
		if err := sqlite.Init(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Database created: %s\n", path)
		return
	}

	dbPath := getDBPath(args)
	args = stripDBArg(args)
	if len(args) >= 1 && args[0] == "import" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gdql [-db <path>] import setlistfm")
			fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import json <file.json>")
			fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import aliases <file.json>")
			os.Exit(1)
		}
		switch args[1] {
		case "setlistfm":
			apiKey := os.Getenv("SETLISTFM_API_KEY")
			if apiKey == "" {
				fmt.Fprintln(os.Stderr, "Error: SETLISTFM_API_KEY is not set")
				fmt.Fprintln(os.Stderr, "Get an API key at https://www.setlist.fm/settings/api")
				os.Exit(1)
			}
			client := setlistfm.NewClient(apiKey)
			showsAdded, songsAdded, err := setlistfm.Import(context.Background(), dbPath, client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Import error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Import complete: %d shows, %d songs\n", showsAdded, songsAdded)
			return
		case "json":
			jsonPath := ""
			if len(args) >= 3 && args[2] == "-f" {
				if len(args) < 4 {
					fmt.Fprintln(os.Stderr, "Usage: gdql [-db <path>] import json -f <file.json>")
					os.Exit(1)
				}
				jsonPath = args[3]
			} else if len(args) >= 3 {
				jsonPath = args[2]
			} else {
				fmt.Fprintln(os.Stderr, "Usage: gdql [-db <path>] import json <file.json>")
				fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import json -f <file.json>")
				fmt.Fprintln(os.Stderr, "JSON format: see docs/CANONICAL_IMPORT.md")
				os.Exit(1)
			}
			if err := runImportJSON(dbPath, jsonPath); err != nil {
				fmt.Fprintf(os.Stderr, "Import error: %v\n", err)
				os.Exit(1)
			}
			return
		case "aliases":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: gdql [-db <path>] import aliases <file.json>")
				fmt.Fprintln(os.Stderr, "Format: [{\"alias\": \"...\", \"canonical\": \"...\"}, ...] — see SONG_NORMALIZATION.md")
				os.Exit(1)
			}
			aliasPath := args[2]
			if err := runImportAliases(dbPath, aliasPath); err != nil {
				fmt.Fprintf(os.Stderr, "Import error: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintln(os.Stderr, "Usage: gdql [-db <path>] import setlistfm")
			fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import json <file.json>")
			fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import aliases <file.json>")
			os.Exit(1)
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no query or flag")
		printUsage()
		os.Exit(1)
	}

	query, err := readQuery(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// If the shell merged args into one (e.g. Windows: "-db shows.db SHOWS FROM 1977"),
	// strip the leading -db and path from the query.
	if dbPath, query = stripLeadingDBFromQuery(dbPath, query); query == "" {
		fmt.Fprintln(os.Stderr, "Error: no query after -db")
		printUsage()
		os.Exit(1)
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ex := executor.New(db)
	result, err := ex.Execute(context.Background(), query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmtr := formatter.New()
	out, err := fmtr.Format(result, formatter.FromIR(result.OutputFmt))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}

func runImportJSON(dbPath, jsonPath string) error {
	if err := sqlite.InitSchema(dbPath); err != nil {
		return err
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", jsonPath, err)
	}
	var shows []canonical.Show
	if err := json.Unmarshal(data, &shows); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}
	showsAdded, songsAdded, err := canonical.WriteShows(context.Background(), db.DB(), shows)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Import complete: %d shows, %d songs\n", showsAdded, songsAdded)
	return nil
}

func runImportAliases(dbPath, aliasPath string) error {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	loaded, skipped, err := sqlite.LoadAliasesFromFile(context.Background(), db.DB(), aliasPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Aliases: %d loaded, %d skipped (canonical not found)\n", loaded, skipped)
	return nil
}

func getDBPath(args []string) string {
	for i, a := range args {
		if a == "-db" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if p := os.Getenv("GDQL_DB"); p != "" {
		return p
	}
	return "shows.db"
}

func stripDBArg(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-db" {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// stripLeadingDBFromQuery handles the case where the shell passed one arg like "-db shows.db SHOWS FROM 1977".
// Returns (dbPath, query); if the query started with "-db path ", path is used and the rest is the query.
func stripLeadingDBFromQuery(defaultPath, query string) (dbPath, rest string) {
	rest = query
	q := strings.TrimSpace(query)
	if !strings.HasPrefix(q, "-db ") {
		return defaultPath, rest
	}
	// "-db path [rest of query]"
	q = strings.TrimSpace(q[4:]) // after "-db "
	if q == "" {
		return defaultPath, ""
	}
	idx := strings.Index(q, " ")
	if idx < 0 {
		return q, "" // only path, no query
	}
	dbPath = q[:idx]
	rest = strings.TrimSpace(q[idx+1:])
	return dbPath, rest
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gdql [options] <query>")
	fmt.Fprintln(os.Stderr, "       gdql init [path]              create database with schema and sample data (default: shows.db)")
	fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import setlistfm   import from setlist.fm (requires SETLISTFM_API_KEY)")
	fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import json <file>   import from canonical JSON (see docs/CANONICAL_IMPORT.md)")
	fmt.Fprintln(os.Stderr, "       gdql [-db <path>] import aliases <file>  load song alias mappings (see SONG_NORMALIZATION.md)")
	fmt.Fprintln(os.Stderr, "       gdql -f <file>")
	fmt.Fprintln(os.Stderr, "       gdql -   (read query from stdin)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -db <path>   Database path (default: shows.db or GDQL_DB)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  gdql init                 # create shows.db with sample data")
	fmt.Fprintln(os.Stderr, "  gdql -db shows.db SHOWS FROM 1977 LIMIT 5")
	fmt.Fprintln(os.Stderr, "  gdql -f query.gdql")
	fmt.Fprintln(os.Stderr, "  echo 'SHOWS FROM 1977;' | gdql -")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Queries with double-quoted strings often get split by the shell; use -f or stdin for those.")
}

// readQuery returns the query string from args: either a single arg, -f <file>, or - for stdin.
func readQuery(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no query or flag")
	}

	// -f <file>
	if args[0] == "-f" {
		if len(args) < 2 {
			return "", fmt.Errorf("-f requires a filename")
		}
		b, err := os.ReadFile(args[1])
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}

	// - (stdin)
	if args[0] == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return strings.TrimSpace(strings.Join(lines, "\n")), nil
	}

	return strings.TrimSpace(strings.Join(args, " ")), nil
}
