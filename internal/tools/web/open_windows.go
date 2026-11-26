package web

import (
	"os"
	"os/exec"
	"path/filepath"
)

// OpenNotepad starts Notepad with the file at path and does not wait for
// it. The app runs elevated, thus Notepad can save the system hosts file.
func OpenNotepad(path string) error {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	cmd := exec.Command(filepath.Join(root, "System32", "notepad.exe"), path) //nolint:gosec // G204: path is the hosts path of HostsPath or a user list of the app.
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
