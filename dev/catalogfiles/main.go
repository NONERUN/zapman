// Command catalogfiles writes the "files" list of catalog.json: the SHA256
// of each file of the catalog (see spec.CatalogHashes).
//
// Run it with go generate ./internal/spec, or:
//
//	go run ./dev/catalogfiles -dir strategies
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NONERUN/zapman/internal/spec"
)

func main() {
	dir := flag.String("dir", "strategies", "catalog folder")
	flag.Parse()
	n, err := run(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d files\n", filepath.Join(*dir, spec.CatalogFile), n)
}

// run writes the files list of the catalog.json in dir. It gives the
// number of files.
func run(dir string) (int, error) {
	path := filepath.Join(dir, spec.CatalogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var info spec.CatalogInfo
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&info); err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	if info.Files, err = spec.CatalogHashes(dir); err != nil {
		return 0, err
	}
	out, err := spec.FormatCatalogInfo(&info)
	if err != nil {
		return 0, err
	}
	return len(info.Files), os.WriteFile(path, out, 0o644)
}
