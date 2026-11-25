package spec

import (
	"errors"
	"flag"
	"fmt"
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
				got := goldenHeader(s.st, s.games) + strings.Join(lines, "\n") + "\n"
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
				if err := checkGolden(got, string(want)); err != nil {
					t.Errorf("%s %s %s: %v", s.name, eng, c.name, err)
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

// versionPrefix starts the first line of a golden file.
const versionPrefix = "# version: "

// errBump tells that the argv changed, but the version did not change.
var errBump = errors.New("argv changed: bump rev/version")

// goldenHeader gives the first line of a golden file: the version of the
// strategy and of each game, for example
// "# version: flowseal 1.10.4 r2; sky 1; sker 1".
func goldenHeader(st *Strategy, games []*Game) string {
	h := versionPrefix + strings.ToLower(st.Version())
	for _, g := range games {
		h += "; " + g.ID + " " + strconv.Itoa(g.GameFile.Version)
	}
	return h + "\n"
}

// splitGolden gives the version line and the argv of a golden text. The
// version line is empty when the text has no version line.
func splitGolden(text string) (header, argv string) {
	if !strings.HasPrefix(text, versionPrefix) {
		return "", text
	}
	header, argv, _ = strings.Cut(text, "\n")
	return header, argv
}

// checkGolden compares the Go output got with the golden text want. When
// the argv is different and the version line is the same, the error is
// errBump: a person must increase rev or version. Other differences are
// also an error.
func checkGolden(got, want string) error {
	if got == want {
		return nil
	}
	gh, ga := splitGolden(got)
	wh, wa := splitGolden(want)
	switch {
	case wh == "":
		return errors.New("the golden file has no version line; run the test with -update")
	case ga != wa && gh == wh:
		return fmt.Errorf("%w\n%s", errBump, firstDiff(got, want))
	case ga != wa:
		return fmt.Errorf("argv and version changed; run the test with -update and examine the diff\n%s", firstDiff(got, want))
	default:
		return fmt.Errorf("version changed (%s, golden %s); run the test with -update", gh, wh)
	}
}

func TestCheckGolden(t *testing.T) {
	const (
		v1 = versionPrefix + "flowseal 1.10.4 r1\n"
		v2 = versionPrefix + "flowseal 1.10.4 r2\n"
		a  = "--wf-udp=443\n--filter-udp=443\n"
		b  = "--wf-udp=443\n--filter-udp=444\n"
	)
	cases := []struct {
		name, got, want string
		wantErr         string // empty: no error
	}{
		{"same", v1 + a, v1 + a, ""},
		{"argv changed, version same", v1 + b, v1 + a, errBump.Error()},
		{"argv and version changed", v2 + b, v1 + a, "argv and version changed"},
		{"version changed only", v2 + a, v1 + a, "version changed"},
		{"no version line", v1 + a, a, "no version line"},
	}
	for _, c := range cases {
		err := checkGolden(c.got, c.want)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: error %v, want %q", c.name, err, c.wantErr)
		}
	}
	if err := checkGolden(v1+b, v1+a); !errors.Is(err, errBump) {
		t.Errorf("error %v is not errBump", err)
	}
	if !strings.Contains(checkGolden(v1+b, v1+a).Error(), "line 3") {
		t.Error("the bump error does not show the line")
	}
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
