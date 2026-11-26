package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGoBuildDir(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]bool{
		filepath.Join(tmp, "go-build123456", "b001", "exe"): true,
		filepath.Join(tmp, "Go-Build9", "exe"):              true,
		filepath.Join(tmp, "zapman"):                        false,
		`C:\zapman`:                                         false,
	} {
		if got := isGoBuildDir(dir); got != want {
			t.Errorf("%s: got %v", dir, got)
		}
	}
}

func TestRootFolder(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "zapman.exe")
	if err := os.WriteFile(exe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := RootFolder(exe)
	want, _ := filepath.EvalSymlinks(dir)
	if err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

func TestAppVersion(t *testing.T) {
	if BuildVersion() == "" {
		t.Error("empty version")
	}
}
