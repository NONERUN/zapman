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
	ref := r.Ref
	if ref == "" {
		ref = DefaultRef
	}
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

// git runs git in r.Dir and gives the output without the last line break.
func (r Repo) git(args ...string) (string, error) {
	if r.Dir != "" {
		args = append([]string{"-C", r.Dir}, args...)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}
