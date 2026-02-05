// Program build_embed_db creates cmd/gdql/embeddb/default.db for embedding in the gdql binary.
// Run from repo root:
//
//	go run ./cmd/build_embed_db              # create from schema+seed (small default)
//	go run ./cmd/build_embed_db --from path  # copy existing DB (e.g. after gdql import json)
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gdql/gdql/internal/data/sqlite"
)

func main() {
	_, self, _, _ := runtime.Caller(0)
	cmdDir := filepath.Dir(filepath.Dir(self))
	outPath := filepath.Join(cmdDir, "gdql", "embeddb", "default.db")

	var from string
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--from" && i+1 < len(os.Args) {
			from = os.Args[i+1]
			break
		}
	}

	if from != "" {
		if err := copyFile(from, outPath); err != nil {
			fmt.Fprintf(os.Stderr, "copy %s -> %s: %v\n", from, outPath, err)
			os.Exit(1)
		}
		fmt.Println(outPath, "(copied from", from+")")
		return
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := sqlite.Init(outPath); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(outPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
