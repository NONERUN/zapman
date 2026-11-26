// Command commitmsg is the commit-msg hook of the repository. It replaces
// the Cursor Co-authored-by trailer with Assisted-by in the message file.
//
// Use: go run ./dev/commitmsg <message file>
// .githooks/commit-msg runs it. Enable: git config core.hooksPath .githooks
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const (
	coAuthored = "Co-authored-by: Cursor <cursoragent@cursor.com>"
	assisted   = "Assisted-by: Cursor <cursoragent@cursor.com>"
)

// fix gives the message with each coAuthored replaced by assisted.
func fix(msg string) string {
	return strings.ReplaceAll(msg, coAuthored, assisted)
}

// run changes the message file. A missing file is not an error.
func run(path string) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	msg := fix(string(b))
	if msg == string(b) {
		return nil
	}
	return os.WriteFile(path, []byte(msg), 0o644)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: commitmsg <message file>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "commitmsg:", err)
		os.Exit(1)
	}
}
