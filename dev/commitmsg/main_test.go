package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFix(t *testing.T) {
	cases := map[string]string{
		"Fix x\n\n" + coAuthored + "\n":             "Fix x\n\n" + assisted + "\n",
		"Fix x\r\n\r\n" + coAuthored + "\r\n":       "Fix x\r\n\r\n" + assisted + "\r\n",
		"Fix x\n\nCo-authored-by: A <a@b.c>\n":      "Fix x\n\nCo-authored-by: A <a@b.c>\n",
		"Fix x\n\n" + assisted + "\n":               "Fix x\n\n" + assisted + "\n",
		coAuthored + "\n" + coAuthored:              assisted + "\n" + assisted,
		"Co-authored-by: Cursor <other@cursor.com>": "Co-authored-by: Cursor <other@cursor.com>",
	}
	for in, want := range cases {
		if got := fix(in); got != want {
			t.Errorf("fix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	if err := run(filepath.Join(dir, "missing")); err != nil {
		t.Errorf("missing file: %v", err)
	}
	path := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(path, []byte("Fix x\n\n"+coAuthored+"\n"), 0o644)
	if err := run(path); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "Fix x\n\n"+assisted+"\n" {
		t.Errorf("file is %q", b)
	}
}
