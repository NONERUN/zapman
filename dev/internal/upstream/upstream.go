// Package upstream finds the upstream tag of a Flowseal strategy: the
// first tag of Flowseal/zapret-discord-youtube that has the last change of
// the .bat file of the strategy.
//
// The dev tools use this package. The product does not use it.
package upstream

import (
	"errors"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// DefaultRef is the git ref of the upstream branch.
const DefaultRef = "upstream/main"

// ErrNoBat tells that the ref has no commit that changes the .bat file.
var ErrNoBat = errors.New("no .bat upstream")

// ErrNoTag tells that no upstream tag has the last change of the .bat file
// yet.
var ErrNoTag = errors.New("no upstream tag has the last change")

// Repo is a git repository with the upstream ref.
type Repo struct {
	Dir string // git folder; empty: the current folder
	Ref string // upstream ref; empty: DefaultRef
}

// Tag gives the upstream tag of the strategy name and the last commit that
// changes <name>.bat in the ref. The tag is the first tag by creator date
// that has the commit and that is in the ref. A tag that is not in the
// ref (for example a zapman release tag) is not an upstream tag.
func (r Repo) Tag(name string) (tag, commit string, err error) {
	ref := r.ref()
	commit, err = r.git("log", "-1", "--format=%H", ref, "--", name+".bat")
	if err != nil {
		return "", "", err
	}
	if commit == "" {
		return "", "", ErrNoBat
	}
	out, err := r.git("tag", "--contains", commit, "--merged", ref, "--sort=creatordate")
	if err != nil {
		return "", commit, err
	}
	if out == "" {
		return "", commit, ErrNoTag
	}
	tag, _, _ = strings.Cut(out, "\n")
	return strings.TrimSpace(tag), commit, nil
}

// ref gives r.Ref, or DefaultRef when it is empty.
func (r Repo) ref() string {
	if r.Ref == "" {
		return DefaultRef
	}
	return r.Ref
}

// Bats gives the strategy names of the general*.bat files in the root of
// the ref, sorted. A name has no .bat extension.
func (r Repo) Bats() ([]string, error) {
	// ls-tree has no glob pathspecs: list the root and select the names.
	out, err := r.git("ls-tree", "--name-only", r.ref())
	if err != nil {
		return nil, err
	}
	var names []string
	// Names have spaces: read the lines.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if ok, _ := path.Match("general*.bat", line); !ok {
			continue
		}
		names = append(names, strings.TrimSuffix(line, ".bat"))
	}
	sort.Strings(names)
	return names, nil
}

// Show gives the bytes of the file (a path with /) in the ref.
func (r Repo) Show(file string) ([]byte, error) {
	args := []string{"show", r.ref() + ":" + file}
	if r.Dir != "" {
		args = append([]string{"-C", r.Dir}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, gitError(args, err)
	}
	return out, nil
}

// git runs git in r.Dir and gives the output without the last line break.
func (r Repo) git(args ...string) (string, error) {
	if r.Dir != "" {
		args = append([]string{"-C", r.Dir}, args...)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", gitError(args, err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// gitError gives the error of a git run with the stderr text of git.
func gitError(args []string, err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
	}
	return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}
