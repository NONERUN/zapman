package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaFiles checks that strategies/schema has the output of Schemas.
// Run go generate ./internal/spec after a change of the types.
func TestSchemaFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "strategies", SchemaDir)
	for name, want := range Schemas() {
		if !json.Valid(want) {
			t.Errorf("%s: not valid JSON", name)
		}
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%v (run go generate ./internal/spec)", err)
			continue
		}
		got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not up to date (run go generate ./internal/spec)", name)
		}
	}
}
