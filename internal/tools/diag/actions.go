package diag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NONERUN/zapman/internal/tools/command"
)

// ErrUnknownAction is the error of Do for an action that is not one of the
// Action* constants.
var ErrUnknownAction = errors.New("unknown diagnostics action")

// ActionError is the error of Do when a step failed. Lines are all lines
// of the action; the FAIL lines start with "FAIL: ".
type ActionError struct {
	Lines []string
}

func (e *ActionError) Error() string {
	var fails []string
	for _, l := range e.Lines {
		if after, ok := strings.CutPrefix(l, "FAIL: "); ok {
			fails = append(fails, after)
		}
	}
	return strings.Join(fails, "\n")
}

// Do does the action of it and gives the log lines ("OK: ...", "WARN:
// ...", "FAIL: ..."). When a line is a FAIL line, the error is an
// *ActionError. It needs administrator rights for the services.
func (c *Checker) Do(ctx context.Context, it Item) ([]string, error) {
	var lines []string
	var err error
	switch it.Action {
	case ActionDiscordCache:
		lines = c.clearDiscordCache()
	case ActionRemoveServices:
		lines, err = c.removeServices(ctx, it.ActionNames)
	case ActionRemoveWinDivert:
		lines, err = c.removeServices(ctx, winDivert)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownAction, it.Action)
	}
	if err != nil {
		return lines, err
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "FAIL: ") {
			return lines, &ActionError{Lines: lines}
		}
	}
	return lines, nil
}

// discordApp is a Discord client with a cache.
type discordApp struct {
	exe, dir, label string
}

var discordClients = []struct{ exe, dir, label string }{
	{"Discord.exe", "discord", "Discord"},
	{"DiscordPTB.exe", "discordptb", "Discord PTB"},
	{"DiscordCanary.exe", "discordcanary", "Discord Canary"},
	{"DiscordDevelopment.exe", "discorddevelopment", "Discord Development"},
}

var cacheDirs = []string{"Cache", "Code Cache", "GPUCache"}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// discordApps gives the Discord clients in %APPDATA% that have a cache
// folder.
func (c *Checker) discordApps() []discordApp {
	if c.AppData == "" {
		return nil
	}
	var out []discordApp
	for _, d := range discordClients {
		dir := filepath.Join(c.AppData, d.dir)
		for _, sub := range cacheDirs {
			if exists(filepath.Join(dir, sub)) {
				out = append(out, discordApp{d.exe, dir, d.label})
				break
			}
		}
	}
	return out
}

// clearDiscordCache stops each Discord client with a cache and deletes its
// cache folders.
func (c *Checker) clearDiscordCache() []string {
	apps := c.discordApps()
	if len(apps) == 0 {
		return []string{"WARN: No Discord cache folders found."}
	}
	var lines []string
	for _, a := range apps {
		_ = c.Sys.KillProcesses(a.exe) // as Stop-Process -ErrorAction SilentlyContinue
		for _, sub := range cacheDirs {
			p := filepath.Join(a.dir, sub)
			if !exists(p) {
				continue
			}
			_ = os.RemoveAll(p) // the check below tells the result
			if exists(p) {
				lines = append(lines, "FAIL: Failed to delete "+p)
			} else {
				lines = append(lines, "OK: Deleted "+p)
			}
		}
	}
	return lines
}

// serviceCheckDelay is the wait between "sc delete" and the check.
const serviceCheckDelay = 300 * time.Millisecond

// removeServices stops and deletes each service, then the WinDivert
// drivers, as Remove-ZapretNamedServices.
func (c *Checker) removeServices(ctx context.Context, names []string) ([]string, error) {
	sleep := c.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	stopDelete := func(name string) error {
		if _, err := c.Run(ctx, command.System32("net.exe"), "stop", name); err != nil && ctx.Err() != nil {
			return err
		}
		if _, err := c.Run(ctx, command.System32("sc.exe"), "delete", name); err != nil && ctx.Err() != nil {
			return err
		}
		return nil
	}
	var lines []string
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return lines, err
		}
		if err := stopDelete(n); err != nil {
			return lines, err
		}
		sleep(serviceCheckDelay)
		if c.exists(n) {
			lines = append(lines, "FAIL: Failed to remove service: "+n)
		} else {
			lines = append(lines, "OK: Removed service: "+n)
		}
	}
	for _, d := range winDivert {
		if err := ctx.Err(); err != nil {
			return lines, err
		}
		if err := stopDelete(d); err != nil {
			return lines, err
		}
	}
	return lines, nil
}
