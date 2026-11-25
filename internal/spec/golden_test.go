package spec

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goldenPaths are the folders of dev/export-golden.ps1.
var goldenPaths = Paths{Bin: `C:\zapman\bin`, Lists: `C:\zapman\lists`, User: `C:\zapman\user`}

var goldenCases = []struct {
	name string
	gf   GameFilter
}{
	{"gf-off", GameFilter{TCP: "12", UDP: "12"}},
	{"gf-all", GameFilter{TCP: "1024-65535", UDP: "1024-65535"}},
}

// TestGolden compares the argv of each strategies/*.json with
// testdata/golden. ZAPMAN_SPEC_ROOT can point to another tree with
// strategies/ and testdata/golden/.
func TestGolden(t *testing.T) {
	root := os.Getenv("ZAPMAN_SPEC_ROOT")
	if root == "" {
		root = filepath.Join("..", "..")
	}
	files, err := filepath.Glob(filepath.Join(root, "strategies", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no strategies in %s", root)
	}
	n := 0
	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), ".json")
		s, err := LoadFile(file)
		if err != nil {
			t.Errorf("%s: %v", base, err)
			continue
		}
		for _, eng := range []Engine{Winws, Winws2} {
			for _, c := range goldenCases {
				golden := filepath.Join(root, "testdata", "golden", string(eng), c.name, base+".txt")
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Errorf("%s: %v", golden, err)
					continue
				}
				lines, err := Build(s, eng, goldenPaths, c.gf)
				if err != nil {
					t.Errorf("%s %s %s: %v", base, eng, c.name, err)
					continue
				}
				got := strings.Join(lines, "\n") + "\n"
				if got != string(want) {
					t.Errorf("%s %s %s: argv differs\n%s", base, eng, c.name, firstDiff(got, string(want)))
				}
				n++
			}
		}
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
