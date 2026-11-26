package app

import (
	"os"
	"path/filepath"
	"runtime/debug"
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

func TestBuildVersion(t *testing.T) {
	vcs := func(rev, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: modified}}
	}
	tagged := &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0+dirty"}, Settings: vcs("0123456789abcdef", "true")}
	devel := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs("0123456789abcdef", "false")}
	for _, c := range []struct {
		name, set string
		bi        *debug.BuildInfo
		want      string
	}{
		{"ldflags before module version", "v0.3.0", tagged, "v0.3.0"},
		{"ldflags without build info", "v0.3.0", nil, "v0.3.0"},
		{"ldflags with spaces", " v0.3.0\n", devel, "v0.3.0"},
		{"module version", "", tagged, "v0.2.0+dirty"},
		{"devel with commit", "", devel, "dev-0123456"},
		{"devel dirty", "", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs("abc", "true")}, "dev-abc-dirty"},
		{"devel without vcs", "", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"no build info", "", nil, ""},
	} {
		if got := buildVersion(c.set, c.bi); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
