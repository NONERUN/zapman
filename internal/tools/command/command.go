// Package command runs the console programs of Windows (netsh, ipconfig,
// sc, net, shutdown) for the tools. The tools get a Func, so the tests can
// give a fake one and do not start a real process.
package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Result is the result of a program that ran to its end.
type Result struct {
	// Output is stdout and stderr together, as text.
	Output string
	// Code is the exit code.
	Code int
}

// Lines gives the lines of Output without the empty lines and without the
// spaces at the ends.
func (r Result) Lines() []string {
	var out []string
	for l := range strings.Lines(r.Output) {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Func runs exe with args and waits for its end. It gives an error only
// when the program cannot start or ctx stops it; a program with a non-zero
// exit code is not an error. When ctx stops the program, the error wraps
// the error of ctx.
type Func func(ctx context.Context, exe string, args ...string) (Result, error)

// System32 gives the full path of the program name in
// %SystemRoot%\System32. A full path does not let a program with the same
// name in the work folder or in PATH start in its place.
func System32(name string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", name)
}
