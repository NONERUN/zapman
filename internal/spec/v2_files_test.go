package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCatalogFiles checks that the files list of strategies/catalog.json
// has the hash of each file of the catalog. Run go generate ./internal/spec
// after a change of a catalog file.
func TestCatalogFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "strategies")
	cat, err := LoadCatalog(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CatalogHashes(dir)
	if err != nil {
		t.Fatal(err)
	}
	const fix = " (run go generate ./internal/spec)"
	for _, rel := range sortedKeys(got) {
		want, ok := cat.Files[rel]
		switch {
		case !ok:
			t.Errorf("catalog.json: files: %s is missing%s", rel, fix)
		case want != got[rel]:
			t.Errorf("catalog.json: files: %s has a different hash%s", rel, fix)
		}
	}
	for _, rel := range sortedKeys(cat.Files) {
		if _, ok := got[rel]; !ok {
			t.Errorf("catalog.json: files: %s is not in the catalog%s", rel, fix)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, CatalogFile))
	if err != nil {
		t.Fatal(err)
	}
	want, err := FormatCatalogInfo(&cat.CatalogInfo)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Errorf("catalog.json is not in the generator format%s", fix)
	}
}

// TestCatalogSelfContained checks that the stock catalog refers to no file
// in bin/: its fakes are in the catalog (cat:, active:).
func TestCatalogSelfContained(t *testing.T) {
	dir := filepath.Join("..", "..", "strategies")
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == SchemaDir {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), `"`+PrefixBin) {
			t.Errorf("%s: has a bin: ref. Put the file in %s/ and use cat:.", path, FakesDir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestActiveRefs(t *testing.T) {
	dir := v2Tree(t, map[string]string{
		CatalogFile:       `{"id":"stock","title":"Z","spec":2,"active":{"GAME_UDP":"cat:fakes/q.bin"}}`,
		ActionsFile:       `{"fake-quic":{"desync":["fake"],"repeats":6,"fake":{"quic":["cat:fakes/q.bin"]}},"fake-game":{"desync":["fake"],"cutoff":4,"fake":{"unknownUdp":["active:GAME_UDP"]}}}`,
		"fakes/q.bin":     "q",
		"flowseal/s.json": okStrategy,
	})
	c, err := LoadCatalog(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Strategy("s")
	if err != nil {
		t.Fatal(err)
	}
	gf := GameFilter{TCP: "12", UDP: "13"}
	p := goldenPaths
	for _, w := range []struct {
		name   string
		active map[string]bool
		z1, z2 string
	}{
		{"default", nil, `--dpi-desync-fake-unknown-udp="C:\zapman\strategies\fakes\q.bin"`, `--blob=q:@"C:\zapman\strategies\fakes\q.bin" --blob=ACTIVE_GAME_UDP:@"C:\zapman\strategies\fakes\q.bin"`},
		{"user file", map[string]bool{"GAME_UDP": true}, `--dpi-desync-fake-unknown-udp="C:\zapman\user\fakes\ACTIVE_GAME_UDP.bin"`, `ACTIVE_GAME_UDP:@"C:\zapman\user\fakes\ACTIVE_GAME_UDP.bin"`},
	} {
		p.ActiveFiles = w.active
		z1, err := Build(st, nil, Winws, p, gf)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(z1, "\n"), w.z1) {
			t.Errorf("%s: winws argv has no %s\n%s", w.name, w.z1, strings.Join(z1, "\n"))
		}
		z2, err := Build(st, nil, Winws2, p, gf)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(z2, "\n"), w.z2) {
			t.Errorf("%s: winws2 argv has no %s\n%s", w.name, w.z2, strings.Join(z2, "\n"))
		}
	}
	// The v0.1 build has no catalog defaults.
	s, err := Parse([]byte(specWith(`"udp":"443","desync":["fake"],"fake":{"unknownUdp":["active:GAME_UDP"]}`)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildV01(s, Winws, goldenPaths, gf); err == nil || !strings.Contains(err.Error(), "active fake active:GAME_UDP has no user file and no catalog default") {
		t.Errorf("no default: %v", err)
	}

	for _, c := range []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{"unknown active", map[string]string{ActionsFile: `{"a":{"desync":["fake"],"fake":{"quic":["active:X"]}}}`}, `actions.json: "a": active fake active:X has no default in catalog.json`},
		{"bad active name", map[string]string{ActionsFile: `{"a":{"desync":["fake"],"fake":{"quic":["active:game"]}}}`}, `Fake value is not valid: 'active:game'`},
		{"missing cat file", map[string]string{ActionsFile: `{"a":{"desync":["fake"],"fake":{"quic":["cat:fakes/x.bin"]}}}`}, `actions.json: "a": catalog file not found: cat:fakes/x.bin`},
		{"cat path out of catalog", map[string]string{ActionsFile: `{"a":{"desync":["fake"],"fake":{"quic":["cat:../x.bin"]}}}`}, `cat: path must be a relative path`},
		{"active default not cat", map[string]string{CatalogFile: `{"id":"stock","title":"Z","spec":2,"active":{"X":"bin:x.bin"}}`}, `catalog.json: active: "X" must be a cat: file`},
		{"active default missing", map[string]string{CatalogFile: `{"id":"stock","title":"Z","spec":2,"active":{"X":"cat:fakes/x.bin"}}`}, `catalog.json: active: "X": catalog file not found`},
		{"active key", map[string]string{CatalogFile: `{"id":"stock","title":"Z","spec":2,"active":{"x":"cat:fakes/x.bin"}}`}, `catalog.json: active: name "x" is not valid`},
		{"files path", map[string]string{CatalogFile: `{"id":"stock","title":"Z","spec":2,"files":{"a\\b":"` + strings.Repeat("0", 64) + `"}}`}, `catalog.json: files: path "a\\b" is not valid`},
		{"files hash", map[string]string{CatalogFile: `{"id":"stock","title":"Z","spec":2,"files":{"a.json":"00"}}`}, `is not a SHA256`},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.files["flowseal/s.json"] = okStrategy
			_, err := LoadCatalog(v2Tree(t, c.files), "")
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %v, want %q", err, c.wantErr)
			}
		})
	}
}
