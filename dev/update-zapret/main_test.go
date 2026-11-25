package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestZipAsset(t *testing.T) {
	rel := &release{TagName: "v1", Assets: []asset{
		{Name: "zapret2-v1-openwrt-embedded.zip"},
		{Name: "zapret-v1-openwrt-embedded.zip"},
		{Name: "zapret-v1.tar.gz"},
		{Name: "zapret2-v1.zip"},
		{Name: "zapret-v1.zip"},
	}}
	for prefix, want := range map[string]string{"zapret": "zapret-v1.zip", "zapret2": "zapret2-v1.zip"} {
		a, err := zipAsset(rel, prefix)
		if err != nil || a.Name != want {
			t.Errorf("%s: %q, %v; want %q", prefix, a.Name, err, want)
		}
	}
	if _, err := zipAsset(&release{TagName: "v1", Assets: rel.Assets[:3]}, "zapret"); err == nil {
		t.Error("no error without a zip asset")
	}
}

// testZip makes a zip with the files (name → text).
func testZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(f, files[n])
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// TestExtract checks that only the engine files go to bin/.
func TestExtract(t *testing.T) {
	dir := t.TempDir()
	z1 := filepath.Join(dir, "zapret.zip")
	z2 := filepath.Join(dir, "zapret2.zip")
	testZip(t, z1, map[string]string{
		"zapret-v1/binaries/windows-x86/winws.exe":             "x86",
		"zapret-v1/binaries/windows-x86_64/":                   "",
		"zapret-v1/binaries/windows-x86_64/WINWS.EXE":          "winws",
		"zapret-v1/binaries/windows-x86_64/cygwin1.dll":        "cygwin",
		"zapret-v1/binaries/windows-x86_64/WinDivert.dll":      "dll",
		"zapret-v1/binaries/windows-x86_64/WinDivert64.sys":    "sys",
		"zapret-v1/binaries/windows-x86_64/killall.exe":        "other",
		"zapret-v1/files/fake/quic_initial_www_google_com.bin": "fake",
	})
	testZip(t, z2, map[string]string{
		"zapret2-v1/binaries/windows-x86_64/winws2.exe": "winws2",
		"zapret2-v1/lua/zapret-lib.lua":                 "lib\n",
		"zapret2-v1/lua/zapret-antidpi.lua":             "antidpi\n",
		"zapret2-v1/lua/zapret-auto.lua":                "auto\n",
		"zapret2-v1/files/fake/stun.bin":                "fake",
	})
	bin := filepath.Join(dir, "bin")
	zips := map[string]string{"zapret": z1, "zapret2": z2}
	sums := map[string]map[string]string{"zapret": parseSums([]byte(
		sum("winws") + "  zapret-v1/binaries/windows-x86_64/winws.exe\r\n" +
			sum("cygwin") + " *zapret-v1/binaries/windows-x86_64/cygwin1.dll\n" +
			sum("other") + "  zapret-v1/binaries/windows-x86_64/killall.exe\n"))}
	files, err := extract(zips, sums, bin, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{"winws.exe", sum("winws"), "zapret"},
		{"cygwin1.dll", sum("cygwin"), "zapret"},
		{"WinDivert.dll", sum("dll"), "zapret"},
		{"WinDivert64.sys", sum("sys"), "zapret"},
		{"winws2.exe", sum("winws2"), "zapret2"},
		{"lua/zapret-lib.lua", sum("lib\n"), "zapret2"},
		{"lua/zapret-antidpi.lua", sum("antidpi\n"), "zapret2"},
	}
	if len(files) != len(want) {
		t.Fatalf("files %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("file %d: %v, want %v", i, files[i], want[i])
		}
	}
	got := binFiles(t, bin)
	if len(got) != len(want) {
		t.Errorf("bin has %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(bin, "winws.exe")); string(b) != "winws" {
		t.Errorf("winws.exe is %q", b)
	}

	// A wrong SHA256 is an error, and bin does not change.
	bad := map[string]map[string]string{"zapret2": {"zapret2-v1/binaries/windows-x86_64/winws2.exe": sum("x")}}
	os.Remove(filepath.Join(bin, "winws.exe"))
	if _, err := extract(zips, bad, bin, io.Discard); err == nil || !strings.Contains(err.Error(), "winws2.exe") {
		t.Errorf("bad SHA256: error %v", err)
	}
	if _, err := os.Stat(filepath.Join(bin, "winws.exe")); err == nil {
		t.Error("bad SHA256: bin changed")
	}

	testZip(t, z2, map[string]string{"zapret2-v1/lua/zapret-lib.lua": "lib\n"})
	if _, err := extract(zips, nil, bin, io.Discard); err == nil || !strings.Contains(err.Error(), "winws2.exe") {
		t.Errorf("missing entry: error %v", err)
	}
}

func TestParseSums(t *testing.T) {
	got := parseSums([]byte("ABC  Dir/A.exe\r\ndef *dir\\b.dll\n\nbad line here\n"))
	want := map[string]string{"dir/a.exe": "abc", "dir/b.dll": "def"}
	if len(got) != len(want) {
		t.Fatalf("%v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
}

func TestCacheHit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.zip")
	if cacheHit(path, asset{Size: 3}) {
		t.Error("hit without a file")
	}
	os.WriteFile(path, []byte("abc"), 0o644)
	for size, want := range map[int64]bool{3: true, 0: true, 4: false} {
		if got := cacheHit(path, asset{Size: size}); got != want {
			t.Errorf("size %d: %v, want %v", size, got, want)
		}
	}
}

func TestManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versions.json")
	if m := readManifest(path); m.tag("zapret") != "" {
		t.Error("tag without a file")
	}
	os.WriteFile(path, []byte("{bad"), 0o644)
	if m := readManifest(path); m.tag("zapret") != "" {
		t.Error("tag from a bad file")
	}
	m := Manifest{
		Updated: "2026-01-02",
		Sources: []Source{
			{"zapret", "https://github.com/bol-van/zapret", "v1", "2026-01-01T00:00:00Z", "https://github.com/bol-van/zapret/releases/tag/v1"},
			{"zapret2", "https://github.com/bol-van/zapret2", "v2", "2026-01-01T00:00:00Z", "https://github.com/bol-van/zapret2/releases/tag/v2"},
		},
		Files: []File{{"lua/zapret-lib.lua", sum("x"), "zapret2"}},
	}
	if err := writeManifest(path, m); err != nil {
		t.Fatal(err)
	}
	got := readManifest(path)
	if got.tag("zapret") != "v1" || got.tag("zapret2") != "v2" || got.tag("other") != "" {
		t.Errorf("tags %q %q", got.tag("zapret"), got.tag("zapret2"))
	}
	if got.Files[0] != m.Files[0] || got.Updated != m.Updated {
		t.Errorf("read %+v", got)
	}
}

// TestManifestFormat checks that the tool writes bin/versions.json of the
// repository in the same form.
func TestManifestFormat(t *testing.T) {
	path := filepath.Join("..", "..", "bin", "versions.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(path)
	if m == nil {
		t.Fatal("bin/versions.json is not valid")
	}
	got, err := marshalManifest(*m)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("bin/versions.json is not in the form of the tool:\n%s", got)
	}
}

// binFiles gives the files under bin, with /, without versions.json.
func binFiles(t *testing.T, bin string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(bin, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(bin, p)
		if rel = filepath.ToSlash(rel); rel != "versions.json" {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// TestRepoBin checks bin/ of the repository: it has only the engine files,
// each one is in versions.json with the same SHA256, and each source of a
// file is in sources.
func TestRepoBin(t *testing.T) {
	bin := filepath.Join("..", "..", "bin")
	m := readManifest(filepath.Join(bin, "versions.json"))
	if m == nil {
		t.Fatal("bin/versions.json is not found or not valid")
	}
	engine := map[string]bool{}
	for _, ef := range engineFiles {
		engine[ef.Dest] = true
	}
	listed := map[string]bool{}
	for _, f := range m.Files {
		listed[f.Path] = true
		if !engine[f.Path] {
			t.Errorf("versions.json: %s is not an engine file", f.Path)
		}
		if m.tag(f.Source) == "" {
			t.Errorf("versions.json: %s: source %q has no tag", f.Path, f.Source)
		}
		got, err := fileSHA256(filepath.Join(bin, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("versions.json: %v", err)
			continue
		}
		if got != f.SHA256 {
			t.Errorf("bin/%s: SHA256 %s, versions.json has %s", f.Path, got, f.SHA256)
		}
	}
	for p := range engine {
		if !listed[p] {
			t.Errorf("versions.json has no %s", p)
		}
	}
	for _, p := range binFiles(t, bin) {
		if !listed[p] {
			t.Errorf("bin/%s is not in versions.json: bin/ has only engine files", p)
		}
	}
}
