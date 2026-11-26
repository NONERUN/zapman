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

// Version is the product version of a release build: the git tag, for
// example v1.2.0. The release workflow sets it with
// -ldflags "-X github.com/NONERUN/zapman/internal/app.Version=<tag>".
// It must be a variable: -X does not change a constant.
var Version string

// BuildVersion gives the product version. Version comes first. Without it,
// it gives the module version of the build info. A build from a checkout
// without a module version gives "dev" with the short commit hash.
func BuildVersion() string {
	bi, _ := debug.ReadBuildInfo() // nil without build info
	return buildVersion(Version, bi)
}

// buildVersion gives the version from the value of -X and the build info.
// A nil bi is a build without build info.
func buildVersion(set string, bi *debug.BuildInfo) string {
	if set = strings.TrimSpace(set); set != "" {
		return set
	}
	if bi == nil {
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
