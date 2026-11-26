package engine

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: the console process has no window.
const createNoWindow = 0x08000000

// StartProcess is the StartFunc of Windows. It gives cmdLine to
// CreateProcess as it is: the argv lines already have their quotes.
func StartProcess(exe, cmdLine, dir string, out io.Writer) (Process, error) {
	// The command line is the argv of the strategy; Build makes it from the
	// catalog, not from the input of a user.
	cmd := exec.Command(exe) //nolint:gosec // G204: exe is bin\winws(2).exe of the layout.
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       cmdLine,
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd}, nil
}

type osProcess struct {
	cmd *exec.Cmd
	// done is true after Wait. Then the process handle is closed, and Kill
	// has nothing to do.
	done atomic.Bool
}

func (p *osProcess) Pid() int { return p.cmd.Process.Pid }

func (p *osProcess) Kill() error {
	if p.done.Load() {
		return nil
	}
	err := p.cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) || p.done.Load() {
		return nil
	}
	return err
}

func (p *osProcess) Wait() (uint32, error) {
	err := p.cmd.Wait()
	p.done.Store(true)
	if p.cmd.ProcessState == nil {
		return 0, err
	}
	if ws, ok := p.cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
		return ws.ExitCode, nil
	}
	var ee *exec.ExitError
	if err == nil || errors.As(err, &ee) {
		return uint32(p.cmd.ProcessState.ExitCode()), nil //nolint:gosec // G115: Windows exit codes are uint32.
	}
	return 0, err
}
