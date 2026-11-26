//go:build !windows

package web

import "errors"

// OpenHosts gives an error: Notepad is a program of Windows.
func OpenHosts(string) error {
	return errors.New("the hosts file opens only on Windows")
}
