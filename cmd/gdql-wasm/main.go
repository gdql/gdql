//go:build js && wasm

// Package main is the GDQL WASM entrypoint. Built for GOOS=js GOARCH=wasm,
// it exposes a single JavaScript function — globalThis.gdqlQuery(query) —
// that runs a GDQL query against the embedded default database and returns
// a result object: { json } on success, { error } on failure.
//
// The embedded database is mounted via ncruces/go-sqlite3's memdb VFS
// (bytes-backed, in-memory) rather than extracted to a temp file, because
// os.MkdirTemp / os.WriteFile are not implemented on GOOS=js.
//
// Build:
//
//	GOOS=js GOARCH=wasm go build -o gdql.wasm ./cmd/gdql-wasm
package main

import (
	"context"
	"sync"
	"syscall/js"

	"github.com/ncruces/go-sqlite3/vfs/memdb"

	"github.com/gdql/gdql/internal/data/sqlite"
	"github.com/gdql/gdql/internal/executor"
	"github.com/gdql/gdql/internal/formatter"
	"github.com/gdql/gdql/run"
)

const memDBName = "gdql-embedded"

var (
	dbOnce sync.Once
	dbRef  *sqlite.DB
	dbErr  error
)

func openMemDB() (*sqlite.DB, error) {
	dbOnce.Do(func() {
		memdb.Create(memDBName, run.EmbeddedDB())
		// The memdb VFS is cross-platform (native + wasm). Open the connection
		// via gdql's own sqlite.Open so migrations + helpers are intact.
		dbRef, dbErr = sqlite.Open("file:/" + memDBName + "?vfs=memdb")
	})
	return dbRef, dbErr
}

func runQuery(query string) (string, error) {
	db, err := openMemDB()
	if err != nil {
		return "", err
	}
	ex := executor.New(db)
	fmtr := formatter.New()

	stmts := run.SplitStatements(query)
	if len(stmts) == 0 {
		return "{}", nil
	}
	if len(stmts) == 1 {
		result, err := ex.Execute(context.Background(), stmts[0])
		if err != nil {
			return "", err
		}
		return fmtr.Format(result, formatter.FormatJSON)
	}
	// Multiple statements — concatenate JSON results into an array.
	var b []byte
	b = append(b, '[')
	for i, s := range stmts {
		result, err := ex.Execute(context.Background(), s)
		if err != nil {
			return "", err
		}
		j, err := fmtr.Format(result, formatter.FormatJSON)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, j...)
	}
	b = append(b, ']')
	return string(b), nil
}

func main() {
	js.Global().Set("gdqlQuery", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return map[string]any{"error": "gdqlQuery: missing query argument"}
		}
		out, err := runQuery(args[0].String())
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"json": out}
	}))

	js.Global().Set("gdqlReady", js.ValueOf(true))

	// Keep the Go runtime alive so the registered callback remains callable.
	select {}
}
