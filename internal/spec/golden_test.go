package spec

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// update makes TestGolden and TestEdge write the Go output to the golden
// files. Use: go test ./internal/spec -run "Golden|Edge" -update
var update = flag.Bool("update", false, "write testdata/golden and testdata/edge.*.txt from the Go builder")

// writeGolden writes text to path and makes the folder if necessary.
func writeGolden(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goldenPaths are the fixed folders of the golden files.
var goldenPaths = Paths{Bin: `C:\zapman\bin`, Lists: `C:\zapman\lists`, User: `C:\zapman\user`}

var goldenCases = []struct {
	name string
	gf   GameFilter
}{
	{"gf-off", GameFilter{TCP: "12", UDP: "12"}},
	{"gf-all", GameFilter{TCP: "1024-65535", UDP: "1024-65535"}},
}

// gamesGolden is the golden name of ALT11 with all games.
const gamesGolden = "general (ALT11)+games"

// TestGolden compares the argv of each strategy in strategies/ with
// testdata/golden. The strategies are built without games. "general
// (ALT11)" is also built with all games, as gamesGolden. ZAPMAN_SPEC_ROOT
// can point to another tree with strategies/, bin/ and testdata/golden/.
func TestGolden(t *testing.T) {
	root := os.Getenv("ZAPMAN_SPEC_ROOT")
	if root == "" {
		root = filepath.Join("..", "..")
	}
	cat, err := LoadCatalog(filepath.Join(root, "strategies"), filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := cat.StrategyNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatalf("no strategies in %s", root)
	}
	type goldenStrategy struct {
		name  string
		st    *Strategy
		games []*Game
	}
	var all []goldenStrategy
	for _, name := range names {
		st, err := cat.Strategy(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		all = append(all, goldenStrategy{name, st, nil})
		if name == "general (ALT11)" {
			all = append(all, goldenStrategy{gamesGolden, st, cat.Games})
		}
	}
	n := 0
	for _, s := range all {
		for _, eng := range []Engine{Winws, Winws2} {
			for _, c := range goldenCases {
				golden := filepath.Join(root, "testdata", "golden", string(eng), c.name, s.name+".txt")
				lines, err := Build(s.st, s.games, eng, goldenPaths, c.gf)
				if err != nil {
					t.Errorf("%s %s %s: %v", s.name, eng, c.name, err)
					continue
				}
				got := strings.Join(lines, "\n") + "\n"
				if *update {
					writeGolden(t, golden, got)
					n++
					continue
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Errorf("%s: %v", golden, err)
					continue
				}
				if got != string(want) {
					t.Errorf("%s %s %s: argv differs\n%s", s.name, eng, c.name, firstDiff(got, string(want)))
				}
				n++
			}
		}
	}
	if *update {
		t.Logf("%d golden files written", n)
		return
	}
	t.Logf("%d golden files match", n)
}
func firstDiff(got, want string) string {
	g := strings.Split(got, "\n")
	w := strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			return "line " + strconv.Itoa(i+1) + "\n got: " + a + "\nwant: " + b
		}
	}
	return ""
}
