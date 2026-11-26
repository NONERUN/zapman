package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

//go:generate go run ../../dev/catalogfiles -dir ../../strategies

// FakesDir is the folder of the fake and pattern files in a catalog.
const FakesDir = "fakes"

// CatalogHashes gives the SHA256 (lowercase hex) of each file in the
// catalog folder dir, by the path from dir with /. catalog.json and the
// schema folder in the root are not in the result. The hash is of the file
// bytes on the disk. .gitattributes keeps these bytes the same as the bytes
// in git.
func CatalogHashes(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == SchemaDir {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == CatalogFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FormatCatalogInfo gives the catalog.json text of info: JSON with an
// indent of two spaces, keys of maps in sort order, no HTML escapes, and a
// line end at the end.
func FormatCatalogInfo(info *CatalogInfo) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(info); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
