package engine

import (
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/spec"
)

const testStrategy = "general (ALT11)"

func TestBuild(t *testing.T) {
	l := testLayout(t)
	writeFile(t, filepath.Join(l.Bin, "winws.exe"), "")
	writeFile(t, filepath.Join(l.Bin, "winws2.exe"), "")
	cat := stockCatalog(t, l)
	cfg := config.Default()
	cfg.Strategy = testStrategy
	cfg.Games = []string{"photon", "sky"}
	cfg.GameOptions = map[string]map[string]bool{"photon": {"ipset": true}}
	cfg.GameFilter = config.GameFilterUDP
	cfg.GameFilterUDP = "1024-1934,1936-65535"

	b, err := l.Build(cat, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if b.Exe != filepath.Join(l.Bin, "winws.exe") || b.Bin != l.Bin || b.Strategy.Name != testStrategy {
		t.Fatalf("got %+v", b)
	}
	if got := b.GameVersions(); !reflect.DeepEqual(got, []string{
		"photon " + strconv.Itoa(cat.Game("photon").GameFile.Version) + " ipset=on",
		"sky " + strconv.Itoa(cat.Game("sky").GameFile.Version),
	}) {
		t.Errorf("games: %v", got)
	}
	cl := b.CommandLine()
	if !strings.HasPrefix(cl, `"`+b.Exe+`" --wf-tcp=`) || strings.Contains(cl, "--chdir") {
		t.Errorf("winws: %s", cl)
	}
	if !strings.Contains(cl, ",1024-1934,1936-65535 ") || !strings.Contains(cl, `user\ipset-sky.txt`) || !strings.Contains(cl, `user\ipset-photon.txt`) {
		t.Errorf("Game Filter or game is not in the argv: %s", cl)
	}
	if strings.Contains(cl, "  ") || strings.ContainsAny(cl, "\r\n") {
		t.Errorf("bad spaces: %q", cl)
	}

	cfg.Engine = spec.Winws2
	b2, err := l.Build(cat, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"` + filepath.Join(l.Bin, "winws2.exe") + `" --chdir="` + l.Bin + `" --wf-tcp-out=`; !strings.HasPrefix(b2.CommandLine(), want) {
		t.Errorf("winws2: %s", b2.CommandLine())
	}
}

func TestBuildErrors(t *testing.T) {
	l := testLayout(t)
	writeFile(t, filepath.Join(l.Bin, "winws.exe"), "")
	cat := stockCatalog(t, l)
	cfg := config.Default()
	if _, err := l.Build(cat, &cfg); !errors.Is(err, ErrNoStrategy) {
		t.Errorf("no strategy: %v", err)
	}
	cfg.Strategy = "no such strategy"
	if _, err := l.Build(cat, &cfg); err == nil {
		t.Error("unknown strategy: no error")
	}
	cfg.Strategy = testStrategy
	cfg.Games = []string{"nogame"}
	var fe *config.FieldError
	if _, err := l.Build(cat, &cfg); !errors.As(err, &fe) || fe.Field != "games" {
		t.Errorf("unknown game: %v", err)
	}
	cfg.Games = []string{"photon"}
	for _, opts := range []map[string]map[string]bool{
		{"nogame": {"x": true}},
		{"photon": {"nope": true}},
		// A game that is off is checked too.
		{"sky": {"ipset": true}},
	} {
		cfg.GameOptions = opts
		if _, err := l.Build(cat, &cfg); !errors.As(err, &fe) || fe.Field != "gameOptions" {
			t.Errorf("game options %v: %v", opts, err)
		}
	}
	cfg.GameOptions = nil
	cfg.Games = nil
	cfg.Engine = spec.Winws2
	if _, err := l.Build(cat, &cfg); err == nil || !strings.Contains(err.Error(), "winws2.exe is missing") {
		t.Errorf("missing exe: %v", err)
	}
}

func TestArgs(t *testing.T) {
	b := &Build{Engine: spec.Winws2, Exe: `C:\z\bin\winws2.exe`, Bin: `C:\z\bin\`, Lines: []string{" --a=1 --new ", "", "--b=\"C:\\x y\\f.txt\""}}
	if got := b.CommandLine(); got != `"C:\z\bin\winws2.exe" --chdir="C:\z\bin" --a=1 --new --b="C:\x y\f.txt"` {
		t.Errorf("got %s", got)
	}
	b.Lines = []string{`--chdir="D:\l" --a=1`}
	if got := b.Args(); got != `--chdir="D:\l" --a=1` {
		t.Errorf("own chdir: %s", got)
	}
	b.Engine = spec.Winws
	b.Lines = []string{"--a=1"}
	if got := b.Args(); got != "--a=1" {
		t.Errorf("winws: %s", got)
	}
}

func TestCheckLength(t *testing.T) {
	if err := checkLength(strings.Repeat("a", MaxCommandLine)); err != nil {
		t.Error(err)
	}
	if err := checkLength(strings.Repeat("a", MaxCommandLine+1)); err == nil {
		t.Error("no error")
	}
	// U+1F600 is two UTF-16 units.
	if err := checkLength(strings.Repeat("a", MaxCommandLine-1) + "\U0001F600"); err == nil {
		t.Error("no error for a surrogate pair")
	}
}
