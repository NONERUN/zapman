// Package netreset resets the network stack of Windows and restarts the
// computer, as Invoke-ZapmanNetworkReset and Restart-Computer -Force of the
// PowerShell version. Both need administrator rights.
package netreset

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NONERUN/zapman/internal/tools/command"
)

// ErrFailed is the error of Reset when a step has a non-zero exit code or
// cannot start.
var ErrFailed = errors.New("network reset failed")

// Step is one command of the reset.
type Step struct {
	// Name is the text of the step in the log.
	Name string
	// Exe is the program name in System32.
	Exe  string
	Args []string
}

// Steps are the commands of the reset, in the order of the PowerShell
// version (README FAQ: if no strategy works, reset the stack, then restart
// Windows).
var Steps = []Step{
	{"netsh winsock reset", "netsh.exe", []string{"winsock", "reset"}},
	{"netsh int ip reset all", "netsh.exe", []string{"int", "ip", "reset", "all"}},
	{"netsh winhttp reset proxy", "netsh.exe", []string{"winhttp", "reset", "proxy"}},
	{"ipconfig /flushdns", "ipconfig.exe", []string{"/flushdns"}},
}

// Reset runs all Steps with run, also after a step that failed. The log has
// "> <name>" for each step, then the output lines of the step, then
// "exit <code>" when the code is not zero. When a step fails, Reset gives
// the log and ErrFailed. When ctx stops, Reset gives the log and the error
// of ctx; the next steps do not run.
func Reset(ctx context.Context, run command.Func) (string, error) {
	var log []string
	failed := false
	for _, s := range Steps {
		if err := ctx.Err(); err != nil {
			return strings.Join(log, "\n"), err
		}
		log = append(log, "> "+s.Name)
		r, err := run(ctx, command.System32(s.Exe), s.Args...)
		log = append(log, r.Lines()...)
		if err != nil {
			if ctx.Err() != nil {
				return strings.Join(log, "\n"), fmt.Errorf("%s: %w", s.Name, err)
			}
			log = append(log, err.Error())
			failed = true
			continue
		}
		if r.Code != 0 {
			log = append(log, fmt.Sprintf("exit %d", r.Code))
			failed = true
		}
	}
	text := strings.Join(log, "\n")
	if failed {
		return text, ErrFailed
	}
	return text, nil
}

// Reboot restarts the computer at once and closes the programs without a
// question, as Restart-Computer -Force.
func Reboot(ctx context.Context, run command.Func) error {
	r, err := run(ctx, command.System32("shutdown.exe"), "/r", "/f", "/t", "0")
	if err != nil {
		return fmt.Errorf("restart the computer: %w", err)
	}
	if r.Code != 0 {
		return fmt.Errorf("restart the computer: shutdown exit %d: %s", r.Code, strings.Join(r.Lines(), " "))
	}
	return nil
}
