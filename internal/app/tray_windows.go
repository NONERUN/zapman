package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: the console process has no window.
const createNoWindow = 0x08000000

// Schtasks is the TrayTask of Windows. It runs System32\schtasks.exe with
// no window, as Invoke-ZapmanSchtasks does.
type Schtasks struct{}

var _ TrayTask = Schtasks{}

// Register implements TrayTask.
func (Schtasks) Register(ctx context.Context, cmd string) error {
	code, out, err := schtasks(ctx, createArgs(cmd))
	if err != nil {
		return err
	}
	if code != 0 {
		return schtasksError("create", code, out)
	}
	return nil
}

// Unregister implements TrayTask.
func (Schtasks) Unregister(ctx context.Context) error {
	code, _, err := schtasks(ctx, queryArgs())
	if err != nil {
		return err
	}
	if code != 0 {
		// The task is not there.
		return nil
	}
	code, out, err := schtasks(ctx, deleteArgs())
	if err != nil {
		return err
	}
	if code != 0 {
		return schtasksError("delete", code, out)
	}
	return nil
}

// schtasks runs schtasks.exe with the arguments as they are. It gives the
// exit code and the output (stdout, then stderr). The error is for a
// failure of the start.
func schtasks(ctx context.Context, args string) (int, string, error) {
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "schtasks.exe")
	if _, err := os.Stat(exe); err != nil {
		exe = "schtasks.exe"
	}
	cmd := exec.CommandContext(ctx, exe) //nolint:gosec // G204: exe is schtasks.exe of System32.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `"` + exe + `" ` + args,
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), stdout.String() + stderr.String(), nil
	}
	if err != nil {
		return 0, "", err
	}
	return 0, stdout.String() + stderr.String(), nil
}
