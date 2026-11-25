package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/spec"
)

func TestLoadMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "user", FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Fatalf("got %+v, want defaults", c)
	}
	if !c.AutoUpdateBricks || !c.AutoUpdateCheck || !c.TrayWatch || c.Engine != spec.Winws || c.GameFilter != GameFilterOff {
		t.Fatalf("defaults: %+v", c)
	}
}

// TestLoadPowerShell reads a file of the PowerShell version (with a BOM).
func TestLoadPowerShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	ps := "\xef\xbb\xbf" + `{
    "trayWatch":  false,
    "gameFilter":  "udp",
    "language":  "ru",
    "autoUpdateCheck":  false,
    "engine":  "winws2"
}`
	if err := os.WriteFile(path, []byte(ps), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.TrayWatch = false
	want.GameFilter = GameFilterUDP
	want.Language = "ru"
	want.AutoUpdateCheck = false
	want.Engine = spec.Winws2
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v\nwant %+v", c, want)
	}
}

func TestLoadFieldErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	data := `{"engine":"winws3","gameFilter":true,"language":"de","games":["sky","sky"],"strategy":"a/b","trayWatch":"yes","autoUpdateCheck":false}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err == nil {
		t.Fatal("no error")
	}
	var names []string
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var fe *FieldError
		if !errors.As(e, &fe) {
			t.Fatalf("not a FieldError: %v", e)
		}
		names = append(names, fe.Field)
		if !strings.Contains(e.Error(), fe.Field) {
			t.Errorf("error does not name the field: %v", e)
		}
	}
	want := []string{"language", "engine", "strategy", "games", "gameFilter", "trayWatch"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("fields %v, want %v", names, want)
	}
	// The incorrect fields keep the defaults; the correct field applies.
	d := Default()
	d.AutoUpdateCheck = false
	if !reflect.DeepEqual(c, d) {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadNotJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err == nil || !reflect.DeepEqual(c, Default()) {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user", FileName)
	src := `{"engine":"winws2","future":{"a":[1,2]},"zeta":"z","games":["sky"]}`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.SetGame("photon", true)
	c.SetGame("sky", false)
	c.SetGame("akey", true)
	c.Strategy = "general (ALT11)"
	if err := Save(path, &c); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "language": "",
  "engine": "winws2",
  "strategy": "general (ALT11)",
  "games": [
    "akey",
    "photon"
  ],
  "gameFilter": "disabled",
  "autoUpdateCheck": true,
  "trayWatch": true,
  "autoUpdateBricks": true,
  "future": {
    "a": [
      1,
      2
    ]
  },
  "zeta": "z"
}
`
	if string(got) != want {
		t.Fatalf("got\n%s", got)
	}
	c2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c2.marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Fatalf("round trip:\n%s", again)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary file is left: %v", entries)
	}
}

func TestSaveValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	c := Default()
	c.Engine = "x"
	err := Save(path, &c)
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "engine" {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Save wrote a file with an incorrect field")
	}
}

func TestGameFilterPorts(t *testing.T) {
	for mode, want := range map[GameFilterMode]spec.GameFilter{
		GameFilterOff: {TCP: "12", UDP: "12"},
		GameFilterAll: {TCP: "1024-65535", UDP: "1024-65535"},
		GameFilterTCP: {TCP: "1024-65535", UDP: "12"},
		GameFilterUDP: {TCP: "12", UDP: "1024-65535"},
	} {
		c := Default()
		c.GameFilter = mode
		if got := c.GameFilterPorts(); got != want {
			t.Errorf("%s: got %+v, want %+v", mode, got, want)
		}
	}
}
