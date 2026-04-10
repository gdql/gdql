// Package main is the GDQL CLI entrypoint.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/formatter"
	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/run"
)

func main() {
	args := os.Args[1:]

	// No args → interactive REPL (gdql>>)
	if len(args) == 0 {
		runREPL(defaultDBPathSentinel)
		return
	}
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

	// Only -db (no query) → REPL
	if len(args) == 0 {
		runREPL(dbPath)
		return
	}

	if len(args) >= 1 && args[0] == "import" {
		fmt.Fprintln(os.Stderr, "Import commands have moved to gdql-import.")
		fmt.Fprintln(os.Stderr, "Usage: gdql-import [-db <path>] setlistfm|json|lyrics|aliases|fix-sets")
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

	dbPath, err = ensureDefaultDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ex := executor.New(db)
	fmtr := formatter.New()

	stmts := run.SplitStatements(query)
	for i, stmt := range stmts {
		result, err := ex.Execute(context.Background(), stmt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		out, err := fmtr.Format(result, formatter.FromIR(result.OutputFmt))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
		if i < len(stmts)-1 {
			fmt.Println()
		}
	}
}

func runREPL(dbPath string) {
	dbPath, err := ensureDefaultDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ex := executor.New(db)
	fmtr := formatter.New()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Fprintln(os.Stderr, "GDQL — type a query and press Enter. End with ; to run. .quit to exit.")
	for {
		fmt.Fprint(os.Stderr, "gdql>> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			break
		}
		line := strings.TrimSpace(scanner.Text())

		// Commands to exit
		if line == "" || line == ".quit" || line == ".exit" || strings.ToLower(line) == "\\q" {
			if line == ".quit" || line == ".exit" || strings.ToLower(line) == "\\q" {
				break
			}
			continue
		}

		// Accumulate until ; (allow multi-line)
		query := line
		for !strings.HasSuffix(strings.TrimSpace(query), ";") {
			fmt.Fprint(os.Stderr, "    -> ")
			if !scanner.Scan() {
				break
			}
			query += "\n" + scanner.Text()
		}
		query = strings.TrimSpace(sanitizeQuery(query))
		if query == "" {
			continue
		}

		result, err := ex.Execute(context.Background(), query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		out, err := fmtr.Format(result, formatter.FromIR(result.OutputFmt))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting: %v\n", err)
			continue
		}
		fmt.Println(out)
	}
}


// defaultDBPathSentinel means "use embedded default"; only -db overrides.
const defaultDBPathSentinel = ""

func getDBPath(args []string) string {
	for i, a := range args {
		if a == "-db" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if p := os.Getenv("GDQL_DB"); p != "" {
		return p
	}
	return defaultDBPathSentinel
}

// ensureDefaultDB returns the path to use. When no -db was given (path is empty), it always uses
// the embedded DB, unpacked to the config dir (e.g. ~/.config/gdql/shows.db). Use -db <path> to
// override and use a different database.
func ensureDefaultDB(path string) (string, error) {
	if path != defaultDBPathSentinel {
		return path, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot use config dir for default database: %w", err)
	}
	gdqlDir := filepath.Join(configDir, "gdql")
	dbPath := filepath.Join(gdqlDir, "shows.db")
	if err := os.MkdirAll(gdqlDir, 0755); err != nil {
		return "", fmt.Errorf("creating config dir %s: %w", gdqlDir, err)
	}
	if len(run.EmbeddedDB()) > 0 {
		if err := os.WriteFile(dbPath, run.EmbeddedDB(), 0644); err != nil {
			return "", fmt.Errorf("writing database to %s: %w", dbPath, err)
		}
	} else {
		if err := sqlite.Init(dbPath); err != nil {
			return "", fmt.Errorf("initializing database at %s: %w", dbPath, err)
		}
	}
	return dbPath, nil
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
	fmt.Fprintln(os.Stderr, "Usage: gdql [options] [query]")
	fmt.Fprintln(os.Stderr, "       gdql                              interactive mode (gdql>>)")
	fmt.Fprintln(os.Stderr, "       gdql init [path]                  create database with schema and sample data")
	fmt.Fprintln(os.Stderr, "       gdql -f <file>                    run queries from a file")
	fmt.Fprintln(os.Stderr, "       gdql -                            read query from stdin")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -db <path>   Database path (default: embedded DB in config dir)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  gdql SHOWS FROM 1977 LIMIT 5")
	fmt.Fprintln(os.Stderr, "  gdql -db shows.db SHOWS FROM 1977 LIMIT 5")
	fmt.Fprintln(os.Stderr, "  gdql -f query.gdql")
	fmt.Fprintln(os.Stderr, "  echo 'SHOWS FROM 1977;' | gdql -")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "For data import, use gdql-import. See https://docs.gdql.dev")
}

// decodeFileToUTF8 converts file bytes to a UTF-8 string. Handles UTF-8, UTF-16 LE/BE (with BOM)
// so Windows-saved "Unicode" files work.
func decodeFileToUTF8(b []byte) string {
	if len(b) >= 2 {
		if b[0] == 0xFF && b[1] == 0xFE {
			// UTF-16 LE
			return decodeUTF16LE(b[2:])
		}
		if b[0] == 0xFE && b[1] == 0xFF {
			// UTF-16 BE
			return decodeUTF16BE(b[2:])
		}
	}
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return string(b[3:])
	}
	if utf8.Valid(b) {
		return string(b)
	}
	// Invalid UTF-8: replace bad runes with space so we don't pass garbage to parser
	return strings.ToValidUTF8(string(b), " ")
}

func decodeUTF16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

func decodeUTF16BE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(u))
}

// sanitizeQuery removes BOM, normalizes line endings, and forces ASCII so the parser
// never sees Unicode lookalikes (e.g. fullwidth ＞ from Windows editors).
func sanitizeQuery(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0xFEFF {
			continue
		}
		// Fullwidth block U+FF01–FF5E → ASCII U+0021–007E (so ＞ U+FF1E → '>' 0x3E)
		if r >= 0xFF01 && r <= 0xFF5E {
			b.WriteRune(rune(r - 0xFF01 + 0x21))
			continue
		}
		// Halfwidth variants (e.g. small form ＞ U+FE65) and other lookalikes
		switch r {
		case 0x02C3, 0x203A, 0x22F1, 0x2E2B, 0xFE65:
			b.WriteRune('>')
			continue
		case 0x02C2, 0x2039, 0x22F0, 0x2E2A, 0xFE64:
			b.WriteRune('<')
			continue
		case 0x201C, 0x201D, 0x201E, 0x201F:
			b.WriteRune('"')
			continue
		case 0x2018, 0x2019, 0x201A, 0x201B:
			b.WriteRune('\'')
			continue
		}
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		if r == 0x200B || r == 0x200C || r == 0x200D {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
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
		s := decodeFileToUTF8(b)
		return strings.TrimSpace(sanitizeQuery(s)), nil
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
		return strings.TrimSpace(sanitizeQuery(strings.Join(lines, "\n"))), nil
	}

	return strings.TrimSpace(strings.Join(args, " ")), nil
}
