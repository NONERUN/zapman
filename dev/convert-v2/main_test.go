package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/dev/internal/upstream"
)

// revRe finds the rev value of a strategy file.
var revRe = regexp.MustCompile(`(?m)^(\s*"rev":\s*)[0-9]+`)

// baseCommit has the last v0.1 strategy files.
const baseCommit = "3ff33d8"

// TestConvert converts the v0.1 files of baseCommit. The conversion must
// pass the parity check. When the repository has the v2 files, the output
// must be the same as them, with rev 1. The test is skipped when git, the
// commit or the upstream ref is not available.
func TestConvert(t *testing.T) {
	root := filepath.Join("..", "..")
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", upstream.DefaultRef).Run(); err != nil {
		t.Skipf("no %s: %v", upstream.DefaultRef, err)
	}
	out, err := exec.Command("git", "-C", root, "ls-tree", "--name-only", baseCommit, "strategies/").Output()
	if err != nil {
		t.Skipf("git ls-tree %s: %v", baseCommit, err)
	}
	dir := t.TempDir()
	n := 0
	// Names have spaces: read the lines.
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := exec.Command("git", "-C", root, "show", baseCommit+":"+name).Output()
		if err != nil {
			t.Skipf("git show %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(name)), data, 0o644); err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n == 0 {
		t.Skipf("no strategies in %s", baseCommit)
	}
	var log bytes.Buffer
	if err := run(options{Dir: dir, Bin: filepath.Join(root, "bin"), Lists: filepath.Join(root, "lists"), Git: root, Ref: upstream.DefaultRef}, &log); err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	t.Log(log.String())
	if _, err := os.Stat(filepath.Join(dir, photonName+".json")); err == nil {
		t.Errorf("%s.json is not deleted", photonName)
	}

	repo := filepath.Join(root, "strategies")
	if _, err := os.Stat(filepath.Join(repo, "lists.json")); err != nil {
		return
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		got, _ := os.ReadFile(path)
		want, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		// The converter writes rev 1. The repository can have a higher rev.
		want = revRe.ReplaceAll(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), []byte(`${1}1`))
		if !bytes.Equal(got, want) {
			t.Errorf("%s: the converter output is not the same as the file in strategies", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
