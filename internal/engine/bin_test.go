package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestCheckBin(t *testing.T) {
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "winws.exe"), "exe1")
	writeFile(t, filepath.Join(bin, "cygwin1.dll"), "changed")
	writeFile(t, filepath.Join(bin, "lua", "a.lua"), "x\r\ny\r\n")
	versions := `{"updated":"2026-11-26","sources":[{"id":"zapret","tag":"v1"}],"files":[
{"path":"winws.exe","sha256":"` + sha("exe1") + `","source":"zapret"},
{"path":"cygwin1.dll","sha256":"` + strings.ToUpper(sha("dll")) + `","source":"zapret"},
{"path":"WinDivert.dll","sha256":"` + sha("wd") + `","source":"zapret"},
{"path":"lua/a.lua","sha256":"` + sha("x\ny\n") + `","source":"zapret2"}]}`
	writeFile(t, filepath.Join(bin, VersionsFile), versions)
	r, err := CheckBin(bin)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range r.Files {
		got = append(got, f.Path+" "+f.State.String())
	}
	want := []string{"winws.exe ok", "cygwin1.dll hash mismatch", "WinDivert.dll missing", "lua/a.lua ok", "winws2.exe not in versions.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if r.OK() || len(r.Problems()) != 3 {
		t.Fatalf("problems: %v", r.Problems())
	}
	if f := r.Files[1]; f.Want != sha("dll") || f.Got != sha("changed") || f.Source != "zapret" {
		t.Errorf("mismatch: %+v", f)
	}
	if err := r.Err(); err == nil || !strings.Contains(err.Error(), "WinDivert.dll: missing") {
		t.Errorf("Err: %v", err)
	}
	if r.Versions.Sources[0].Tag != "v1" {
		t.Errorf("sources: %+v", r.Versions.Sources)
	}
}

func TestCheckBinErrors(t *testing.T) {
	bin := t.TempDir()
	if _, err := CheckBin(bin); err == nil || !strings.Contains(err.Error(), "versions.json is missing") {
		t.Fatalf("got %v", err)
	}
	for _, p := range []string{"../x", `lua\a.lua`, "/x", "a/../b", "C:x", ""} {
		writeFile(t, filepath.Join(bin, VersionsFile), `{"files":[{"path":"`+strings.ReplaceAll(p, `\`, `\\`)+`","sha256":"00"}]}`)
		if _, err := CheckBin(bin); err == nil {
			t.Errorf("%q: no error", p)
		}
	}
	writeFile(t, filepath.Join(bin, VersionsFile), `{`)
	if _, err := CheckBin(bin); err == nil {
		t.Error("not JSON: no error")
	}
}

// TestCheckBinRepo examines the bin folder of the repository.
func TestCheckBinRepo(t *testing.T) {
	bin := filepath.Join(repoRoot, "bin")
	if _, err := os.Stat(filepath.Join(bin, "winws.exe")); err != nil {
		t.Skip("no bin folder")
	}
	r, err := CheckBin(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
}
