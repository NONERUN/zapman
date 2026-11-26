package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTablesHaveSameKeys(t *testing.T) {
	for k := range en {
		if _, ok := ru[k]; !ok {
			t.Errorf("key %q: not in RU", k)
		}
	}
	for k := range ru {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q: not in EN", k)
		}
	}
}

func TestPlacesMatch(t *testing.T) {
	for k, e := range en {
		if strings.Count(e, "{") != strings.Count(ru[k], "{") {
			t.Errorf("key %q: EN %q and RU %q have different places", k, e, ru[k])
		}
	}
}

func TestT(t *testing.T) {
	tests := []struct {
		lang, key string
		args      []any
		want      string
	}{
		{RU, "BtnStart", nil, "Старт службы"},
		{EN, "BtnStart", nil, "Start service"},
		{"de", "BtnStart", nil, "Start service"},
		{RU, "GrpStatus", nil, "Status"},
		{RU, "TestsProgress", []any{2, 5}, "Тест 2 / 5"},
		{EN, "DownloadProgress", []any{10, 64, 15}, "Downloaded 10 of 64 KB (15%)"},
		{EN, "AppTitle", nil, "Zapret Manager {0}"},
		{EN, "AppTitle", []any{}, "Zapret Manager {0}"},
		{EN, "StatusStrategy", []any{"a {1} b"}, "Strategy: a {1} b"},
		{EN, "NoSuchKey", nil, "NoSuchKey"},
		{RU, "BtnGames", nil, "Игры..."},
		{RU, "DiagFix", nil, "Исправить"},
		{EN, "DiagFix", nil, "Fix"},
	}
	for _, tt := range tests {
		if got := T(tt.lang, tt.key, tt.args...); got != tt.want {
			t.Errorf("T(%q, %q, %v) = %q, want %q", tt.lang, tt.key, tt.args, got, tt.want)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		s    string
		args []any
		want string
	}{
		{"{0}-{1}-{0}", []any{"a", 1}, "a-1-a"},
		{"{2}", []any{"a"}, "{2}"},
		{"{x} {0}", []any{"a"}, "{x} a"},
		{"open {", []any{"a"}, "open {"},
	}
	for _, tt := range tests {
		if got := format(tt.s, tt.args); got != tt.want {
			t.Errorf("format(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestPick(t *testing.T) {
	tests := []struct{ config, culture, want string }{
		{"en", "ru-RU", EN},
		{"", "ru-RU", RU},
		{"", "RU", RU},
		{"", "en-US", EN},
		{"xx", "", EN},
		{"ru", "en-US", RU},
	}
	for _, tt := range tests {
		if got := Pick(tt.config, tt.culture); got != tt.want {
			t.Errorf("Pick(%q, %q) = %q, want %q", tt.config, tt.culture, got, tt.want)
		}
	}
}

// TestDiagFixIsNotAButton makes sure that the button of a Diagnostics row
// does not have the text of a main window button: the row button does a fix,
// not a run of the bypass.
func TestDiagFixIsNotAButton(t *testing.T) {
	for _, tab := range []map[string]string{en, ru} {
		for k, v := range tab {
			if strings.HasPrefix(k, "Btn") && v == tab["DiagFix"] {
				t.Errorf("DiagFix %q is the same as %s", v, k)
			}
		}
	}
}

// keyRefs finds the keys in t("Key") calls and data-t="Key" attributes.
var keyRefs = regexp.MustCompile(`\bt\("([A-Za-z0-9]+)"|data-t="([A-Za-z0-9]+)"`)

// TestFrontendKeys makes sure that each key of the frontend pages is in the
// tables.
func TestFrontendKeys(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "cmd", "zapman", "frontend")
	files, err := filepath.Glob(filepath.Join(dir, "*.*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no frontend files: %v", err)
	}
	n := 0
	for _, f := range files {
		if ext := filepath.Ext(f); ext != ".js" && ext != ".html" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range keyRefs.FindAllStringSubmatch(string(b), -1) {
			k := m[1] + m[2]
			n++
			if _, ok := en[k]; !ok {
				t.Errorf("%s: key %q is not in the tables", filepath.Base(f), k)
			}
		}
	}
	if n == 0 {
		t.Fatal("no keys in the frontend files")
	}
}

func TestTable(t *testing.T) {
	m := Table(RU)
	if m["BtnStop"] != "Стоп" || len(m) != len(en) {
		t.Fatalf("Table(RU): %d keys, BtnStop %q", len(m), m["BtnStop"])
	}
	m["BtnStop"] = "x"
	if ru["BtnStop"] != "Стоп" {
		t.Fatal("Table gives the table, not a copy")
	}
}
