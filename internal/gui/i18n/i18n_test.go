package i18n

import (
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
