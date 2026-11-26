package main

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

// exeID gives the identity of the file path: the volume serial number and
// the file index. A build that replaces the file at the same path gives a
// new file index. The old file of a running exe can only get a new name, so
// its index does not go to the new file.
func exeID(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // read-only handle.
	var fi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &fi); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x-%08x%08x", fi.VolumeSerialNumber, fi.FileIndexHigh, fi.FileIndexLow), nil
}

// waitPID waits until the process pid ends, but not more than timeout. A
// process that is not found has ended.
func waitPID(pid int, timeout time.Duration) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid)) //nolint:gosec // G115: a PID of -after-pid, checked > 0.
	if err != nil {
		return
	}
	defer windows.CloseHandle(h) //nolint:errcheck // wait-only handle.

	ms := uint32(timeout.Milliseconds()) //nolint:gosec // G115: timeout is afterPIDTimeout, some seconds.
	_, _ = windows.WaitForSingleObject(h, ms)
}
