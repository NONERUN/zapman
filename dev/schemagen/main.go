// Command schemagen writes the JSON Schema files of the strategy format v2.
// The schemas come from the Go types in internal/spec (see spec.Schemas).
//
// Run it with go generate ./internal/spec, or:
//
//	go run ./dev/schemagen -out strategies/schema
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/NONERUN/zapman/internal/spec"
)

func main() {
	out := flag.String("out", filepath.Join("strategies", spec.SchemaDir), "output folder")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	files := spec.Schemas()
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(*out, n), files[n], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(filepath.Join(*out, n))
	}
}
