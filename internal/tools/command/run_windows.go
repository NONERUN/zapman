package command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

// Run is the Func of Windows. It starts the program without a window. The
// console programs write in the OEM code page; Run changes the output to
// UTF-8.
func Run(ctx context.Context, exe string, args ...string) (Result, error) {
	// The tools give a fixed program of System32 and fixed arguments, or a
	// service name of the diagnostics report.
	cmd := exec.CommandContext(ctx, exe, args...) //nolint:gosec // G204: exe and args come from the tools, not from a user.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{Output: decodeOEM(out)}, fmt.Errorf("%s: %w", exe, ctxErr)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return Result{Output: decodeOEM(out), Code: ee.ExitCode()}, nil
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Output: decodeOEM(out)}, nil
}

// cpOEM is CP_OEMCP: the OEM code page of the system.
const cpOEM = 1

// decodeOEM changes b from the OEM code page to a UTF-8 string. Text that
// is already valid UTF-8 and only ASCII stays as it is.
func decodeOEM(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	ascii := true
	for _, c := range b {
		if c >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return string(b)
	}
	n, err := windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), nil, 0) //nolint:gosec // G115: the output of a console program is less than 2 GB.
	if err != nil || n <= 0 {
		return string(b)
	}
	w := make([]uint16, n)
	if _, err := windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), &w[0], n); err != nil { //nolint:gosec // G115: as above.
		return string(b)
	}
	return windows.UTF16ToString(w)
}
