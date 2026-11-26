package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/spec"
)

// repoRoot is the root of the repository with the stock catalog.
const repoRoot = `..\..`

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// testLayout gives a layout with the stock catalog and lists, and a new
// bin and user folder.
func testLayout(t *testing.T) Layout {
	t.Helper()
	root := t.TempDir()
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	l := NewLayout(root)
	l.Strategies = filepath.Join(repo, "strategies")
	l.Lists = filepath.Join(repo, "lists")
	return l
}

func stockCatalog(t *testing.T, l Layout) *spec.Catalog {
	t.Helper()
	cat, err := spec.LoadCatalog(l.Strategies, "")
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestNewLayout(t *testing.T) {
	l := NewLayout(`C:\zapman\`)
	want := Layout{Root: `C:\zapman`, Bin: `C:\zapman\bin`, Strategies: `C:\zapman\strategies`, Lists: `C:\zapman\lists`, User: `C:\zapman\user`}
	if l != want {
		t.Fatalf("got %+v", l)
	}
	if l.ConfigPath() != `C:\zapman\user\config.json` || l.VersionsPath() != `C:\zapman\bin\versions.json` {
		t.Fatal(l.ConfigPath(), l.VersionsPath())
	}
}

func TestPaths(t *testing.T) {
	l := NewLayout(t.TempDir())
	p, err := l.Paths()
	if err != nil || p.ActiveFiles != nil {
		t.Fatalf("no fakes folder: %v, %v", p.ActiveFiles, err)
	}
	fakes := filepath.Join(l.User, spec.ActiveFakesDir)
	for _, n := range []string{"ACTIVE_DISCORD_UDP.bin", "active_GAME_UDP.BIN", "ACTIVE_bad.bin", "ACTIVE_.bin", "ACTIVE__X.bin", "other.bin", "ACTIVE_A.txt"} {
		writeFile(t, filepath.Join(fakes, n), "x")
	}
	if err := os.Mkdir(filepath.Join(fakes, "ACTIVE_DIR.bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err = l.Paths()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"DISCORD_UDP": true, "GAME_UDP": true}
	if !reflect.DeepEqual(p.ActiveFiles, want) {
		t.Fatalf("got %v", p.ActiveFiles)
	}
	if p.Bin != l.Bin || p.Lists != l.Lists || p.User != l.User || p.Catalog != l.Strategies {
		t.Fatalf("got %+v", p)
	}
}

func TestExe(t *testing.T) {
	l := NewLayout(t.TempDir())
	writeFile(t, filepath.Join(l.Bin, "winws.exe"), "")
	if p, err := l.Exe(spec.Winws); err != nil || p != filepath.Join(l.Bin, "winws.exe") {
		t.Fatal(p, err)
	}
	// No fallback to the other engine.
	if _, err := l.Exe(spec.Winws2); err == nil || !strings.Contains(err.Error(), "winws2.exe is missing") {
		t.Fatalf("got %v", err)
	}
	if _, err := l.Exe("winws3"); err == nil {
		t.Fatal("no error")
	}
}

func TestEnsureUserFiles(t *testing.T) {
	l := testLayout(t)
	cat := stockCatalog(t, l)
	old := filepath.Join(l.User, "list-general-user.txt")
	writeFile(t, old, "my.example\r\n")
	if err := l.EnsureUserFiles(cat, []*spec.Game{cat.Game("sky"), cat.Game("photon")}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, old); got != "my.example\r\n" {
		t.Errorf("an old file changed: %q", got)
	}
	for name, want := range map[string]string{
		"list-exclude-user.txt":  newHostlist,
		"ipset-exclude-user.txt": newIpset,
		"ipset-all.txt":          readFile(t, filepath.Join(l.Lists, "ipset-all.default.txt")),
		"ipset-sky.txt":          newGameIpset,
		// The ipset option of photon is off: the file is also made.
		"ipset-photon.txt": newGameIpset,
	} {
		if got := readFile(t, filepath.Join(l.User, name)); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	// The games that are off get no files.
	l2 := testLayout(t)
	if err := l2.EnsureUserFiles(stockCatalog(t, l2), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l2.User, "ipset-photon.txt")); err == nil {
		t.Error("ipset-photon.txt is made, but photon is off")
	}
	// The active fakes are copies of the catalog defaults.
	if len(cat.Active) == 0 {
		t.Fatal("the stock catalog has no active fake defaults")
	}
	for name, ref := range cat.Active {
		src, err := l.CatalogFile(ref)
		if err != nil {
			t.Fatal(err)
		}
		got := readFile(t, filepath.Join(l.User, spec.ActiveFakesDir, "ACTIVE_"+name+".bin"))
		if got != readFile(t, src) {
			t.Errorf("ACTIVE_%s.bin is not a copy of %s", name, ref)
		}
	}
}

func TestEnsureActiveFakes(t *testing.T) {
	l := testLayout(t)
	cat := stockCatalog(t, l)
	// A file that the user selected does not change, also with an other
	// case of the prefix and of the extension.
	own := filepath.Join(l.User, spec.ActiveFakesDir, "active_GAME_UDP.BIN")
	writeFile(t, own, "own fake")
	if err := l.EnsureActiveFakes(cat); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, own); got != "own fake" {
		t.Errorf("the user file changed: %q", got)
	}
	discord := filepath.Join(l.User, spec.ActiveFakesDir, "ACTIVE_DISCORD_UDP.bin")
	src, err := l.CatalogFile(cat.Active["DISCORD_UDP"])
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, discord) != readFile(t, src) {
		t.Fatal("ACTIVE_DISCORD_UDP.bin is not the default")
	}
	// A second call does not write over a file that exists.
	writeFile(t, discord, "changed")
	if err := l.EnsureActiveFakes(cat); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, discord); got != "changed" {
		t.Errorf("the second call changed the file: %q", got)
	}
	p, err := l.Paths()
	if err != nil || !p.ActiveFiles["DISCORD_UDP"] || !p.ActiveFiles["GAME_UDP"] {
		t.Errorf("Paths does not find the fakes: %v, %v", p.ActiveFiles, err)
	}
}

func TestCatalogFile(t *testing.T) {
	l := NewLayout(`C:\z`)
	if p, err := l.CatalogFile("cat:fakes/stun.bin"); err != nil || p != `C:\z\strategies\fakes\stun.bin` {
		t.Errorf("got %q, %v", p, err)
	}
	for _, ref := range []string{"fakes/stun.bin", "cat:../x.bin", "cat:", "lists:a.txt"} {
		if _, err := l.CatalogFile(ref); err == nil {
			t.Errorf("%q: no error", ref)
		}
	}
}

func TestIsInside(t *testing.T) {
	for p, want := range map[string]bool{
		`C:\u\a.txt`:    true,
		`C:\u\d\a.txt`:  true,
		`C:\u`:          false,
		`C:\u\..\a.txt`: false,
		`C:\x\a`:        false,
		`C:\u..\a`:      false,
	} {
		if got := isInside(`C:\u`, filepath.Clean(p)); got != want {
			t.Errorf("%s: got %v", p, got)
		}
	}
}
