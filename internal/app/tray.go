package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// TrayTaskName is the name of the scheduled task that starts the tray at
// logon. The PS version uses the same name, so each version replaces the
// task of the other.
const TrayTaskName = "zapman-tray"

// TrayFlag is the flag of zapman.exe for a start in the tray.
const TrayFlag = "--tray"

// TrayTask makes and removes the scheduled task TrayTaskName.
type TrayTask interface {
	// Register makes the task, or replaces it, with the command line cmd.
	Register(ctx context.Context, cmd string) error
	// Unregister removes the task. A missing task is not an error.
	Unregister(ctx context.Context) error
}

// TrayCommand gives the command of the logon task for the exe:
// "<exe>" --tray.
func TrayCommand(exe string) (string, error) {
	if exe == "" {
		return "", errors.New("the path of zapman.exe is not known")
	}
	if strings.Contains(exe, `"`) {
		return "", fmt.Errorf("the path of zapman.exe has a quote: %s", exe)
	}
	return `"` + exe + `" ` + TrayFlag, nil
}

// createArgs gives the arguments of schtasks.exe that make the task. The
// quotes of cmd get a backslash: /TR is one quoted argument.
func createArgs(cmd string) string {
	return `/Create /TN "` + TrayTaskName + `" /SC ONLOGON /RL HIGHEST /F /TR "` +
		strings.ReplaceAll(cmd, `"`, `\"`) + `"`
}

func queryArgs() string { return `/Query /TN "` + TrayTaskName + `"` }

func deleteArgs() string { return `/Delete /TN "` + TrayTaskName + `" /F` }

// schtasksError gives the error of a schtasks.exe call that failed: its
// output, or the exit code when there is no output.
func schtasksError(what string, code int, out string) error {
	if s := strings.TrimSpace(out); s != "" {
		return errors.New(s)
	}
	return fmt.Errorf("schtasks %s exit %d", what, code)
}

// SetTrayAtLogon implements backend.Backend, as Sync-ZapmanTrayWatch does.
func (b *Backend) SetTrayAtLogon(ctx context.Context, on bool) error {
	if !on {
		return b.tray.Unregister(ctx)
	}
	cmd := b.trayCmd
	if cmd == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if cmd, err = TrayCommand(exe); err != nil {
			return err
		}
	}
	return b.tray.Register(ctx, cmd)
}
