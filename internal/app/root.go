package app

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// RootFolder gives the folder of the exe. When the exe is in a temporary
// go-build folder ("go run"), it gives the working folder.
func RootFolder(exe string) (string, error) {
	if exe, err := filepath.EvalSymlinks(exe); err == nil {
		dir := filepath.Dir(exe)
		if !isGoBuildDir(dir) {
			return dir, nil
		}
	}
	return os.Getwd()
}

// isGoBuildDir tells if dir is in a temporary folder of "go run".
func isGoBuildDir(dir string) bool {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		tmp = os.TempDir()
	}
	rel, err := filepath.Rel(tmp, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	return strings.HasPrefix(strings.ToLower(rel), "go-build")
}

// BuildVersion gives the module version of the build. A build from a
// checkout gives "dev" with the short commit hash.
func BuildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if rev == "" {
		return "dev"
	}
	return "dev-" + rev + dirty
}
