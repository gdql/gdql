// verify_queries.go runs hand-curated GDQL queries against the real
// embedded DB and asserts known facts about Grateful Dead history.
//
// This catches data-integrity bugs (like the duplicate song ID issue)
// that the fixture-based unit tests can't catch because the fixture
// is too small.
//
// Usage:
//   go run scripts/verify_queries.go
//   go run scripts/verify_queries.go -db /path/to/shows.db
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/executor"
)

type check struct {
	name      string
	query     string
	mustHave  []string // dates that MUST appear in the result
	mustMiss  []string // dates that MUST NOT appear in the result
	minRows   int      // result must have at least this many rows (0 = no check)
	maxRows   int      // result must have at most this many rows (0 = no check)
	exactRows int      // result must have exactly this many rows (0 = no check)
	countMin  int      // for COUNT queries: minimum count value
	countMax  int      // for COUNT queries: maximum count value
}

var checks = []check{
	{
		name:     "Cornell '77 Scarlet > Fire shows up in 1977 segue search",
		query:    `SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain";`,
		mustHave: []string{"1977-05-08"},
		minRows:  10,
	},
	{
		name:     "Cornell '77 found by venue",
		query:    `SHOWS AT "Barton Hall";`,
		mustHave: []string{"1977-05-08"},
	},
	{
		name:     "FIRST Dark Star is November 14, 1967",
		query:    `FIRST "Dark Star";`,
		mustHave: []string{"1967-11-14"},
	},
	{
		name:     "LAST Dark Star is March 30, 1994",
		query:    `LAST "Dark Star";`,
		mustHave: []string{"1994-03-30"},
	},
	{
		name:     "LAST Saint Stephen",
		query:    `LAST "Saint Stephen";`,
		minRows:  1,
	},
	{
		name:     "Help > Slip > Frank + Dark Star: only 9/10/91 MSG",
		query:    `SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin's Tower" AND PLAYED "Dark Star";`,
		mustHave: []string{"1991-09-10"},
	},
	{
		name:     "BUG REGRESSION: 3/30/80 played BOTH Scarlet AND Fire — should be EXCLUDED from NOT PLAYED",
		query:    `SHOWS AFTER 1980 WHERE PLAYED "Scarlet Begonias" AND NOT PLAYED "Fire on the Mountain";`,
		mustMiss: []string{"1980-03-30"},
	},
	{
		name:     "Two-digit year 69 = 1969 (not 2069)",
		query:    `SHOWS FROM 69 LIMIT 5;`,
		mustHave: []string{"1969-01-17"}, // Santa Barbara opener of 1969 in our DB
	},
	{
		name:     "BEFORE 1970 returns 60s shows",
		query:    `SHOWS BEFORE 1970 LIMIT 100;`,
		minRows:  100,
		mustHave: []string{"1965-11-01"},
	},
	{
		name:     "AFTER 1990 returns 90s shows",
		query:    `SHOWS AFTER 1994 LIMIT 50;`,
		minRows:  10,
	},
	{
		name:     "Fillmore West venue search",
		query:    `SHOWS AT "Fillmore West" FROM 1969;`,
		minRows:  10,
		mustHave: []string{"1969-02-27"}, // known Fillmore West Feb '69 run
	},
	{
		name:     "COUNT SHOWS FROM 1977 around 60",
		query:    `COUNT SHOWS FROM 1977;`,
		countMin: 50,
		countMax: 100,
	},
	{
		name:     "COUNT Dark Star total at least 200",
		query:    `COUNT "Dark Star";`,
		countMin: 200,
	},
}

func main() {
	dbPath := flag.String("db", "", "path to GDQL database (default: ~/Library/Application Support/gdql/shows.db on macOS)")
	flag.Parse()

	if *dbPath == "" {
		home, _ := os.UserHomeDir()
		*dbPath = home + "/Library/Application Support/gdql/shows.db"
	}

	db, err := sqlite.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open DB at %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer db.Close()

	ex := executor.New(db)

	pass, fail := 0, 0
	for _, c := range checks {
		ok, msg := runCheck(ex, c)
		mark := "✓"
		if !ok {
			mark = "✗"
			fail++
		} else {
			pass++
		}
		fmt.Printf("%s %s\n", mark, c.name)
		if !ok {
			fmt.Printf("  %s\n  query: %s\n", msg, c.query)
		}
	}
	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func runCheck(ex executor.Executor, c check) (bool, string) {
	result, err := ex.Execute(context.Background(), c.query)
	if err != nil {
		return false, fmt.Sprintf("error: %v", err)
	}

	// Count check
	if c.countMin > 0 || c.countMax > 0 {
		if result.Count == nil {
			return false, "expected COUNT result, got nothing"
		}
		if c.countMin > 0 && result.Count.Count < c.countMin {
			return false, fmt.Sprintf("count %d below minimum %d", result.Count.Count, c.countMin)
		}
		if c.countMax > 0 && result.Count.Count > c.countMax {
			return false, fmt.Sprintf("count %d above maximum %d", result.Count.Count, c.countMax)
		}
		return true, ""
	}

	dates := extractDates(result)

	if c.exactRows > 0 && len(dates) != c.exactRows {
		return false, fmt.Sprintf("expected exactly %d rows, got %d", c.exactRows, len(dates))
	}
	if c.minRows > 0 && len(dates) < c.minRows {
		return false, fmt.Sprintf("expected at least %d rows, got %d", c.minRows, len(dates))
	}
	if c.maxRows > 0 && len(dates) > c.maxRows {
		return false, fmt.Sprintf("expected at most %d rows, got %d", c.maxRows, len(dates))
	}
	for _, want := range c.mustHave {
		found := false
		for _, d := range dates {
			if strings.HasPrefix(d, want) {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Sprintf("expected date %s in results, missing from %d returned dates", want, len(dates))
		}
	}
	for _, miss := range c.mustMiss {
		for _, d := range dates {
			if strings.HasPrefix(d, miss) {
				return false, fmt.Sprintf("date %s should NOT be in results but was found", miss)
			}
		}
	}
	return true, ""
}

func extractDates(r *executor.Result) []string {
	var dates []string
	for _, s := range r.Shows {
		dates = append(dates, s.Date.Format("2006-01-02"))
	}
	if r.Setlist != nil {
		dates = append(dates, r.Setlist.Date.Format("2006-01-02"))
	}
	return dates
}
