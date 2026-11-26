//go:build !windows

package web

import "errors"

// OpenNotepad gives an error: Notepad is a program of Windows.
func OpenNotepad(string) error {
	return errors.New("notepad is available only on Windows")
}
