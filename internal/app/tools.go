package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
)

// Tools are the actions of the network, the hosts file, the downloads, the
// diagnostics and the strategy tests. The methods have the contracts of the
// methods of backend.Backend with the same names. NewTools gives the Tools
// of internal/tools; cmd/zapman gives it to SetTools.
type Tools interface {
	NetworkReset(ctx context.Context) (text string, err error)
	Reboot(ctx context.Context) error
	DownloadIPSet(ctx context.Context, progress backend.Progress) error
	CheckHosts(ctx context.Context, progress backend.Progress) (backend.HostsCheck, error)
	OpenHosts(ctx context.Context) error
	// OpenFile opens the file at path in Notepad. OpenGameLists uses it.
	OpenFile(ctx context.Context, path string) error
	LatestVersion(ctx context.Context) (string, error)
	Diagnostics(ctx context.Context) ([]backend.DiagItem, error)
	RunDiagAction(ctx context.Context, item backend.DiagItem) error
	RunTests(ctx context.Context, req backend.TestRequest, sink backend.TestSink) error
}

// NotWired is the Tools of a build with no internal/tools. Each method
// gives the error NotWired with the name of the method. A Tools that does
// only some actions can embed NotWired for the others.
type NotWired struct{}

var _ Tools = NotWired{}

func notWired(name string) error { return backend.Msg("NotWired", name) }

// SetTools sets the Tools. A Tools that needs the Backend (for example the
// tests, which use SuspendService) gets it after New. Call SetTools before
// the first call of a method of the Backend; it is not safe for concurrent
// use. Nil is NotWired.
func (b *Backend) SetTools(t Tools) {
	if t == nil {
		t = NotWired{}
	}
	b.tools = t
}

// NetworkReset implements Tools.
func (NotWired) NetworkReset(context.Context) (string, error) {
	return "", notWired("NetworkReset")
}

// Reboot implements Tools.
func (NotWired) Reboot(context.Context) error { return notWired("Reboot") }

// DownloadIPSet implements Tools.
func (NotWired) DownloadIPSet(context.Context, backend.Progress) error {
	return notWired("DownloadIPSet")
}

// CheckHosts implements Tools.
func (NotWired) CheckHosts(context.Context, backend.Progress) (backend.HostsCheck, error) {
	return backend.HostsCheck{}, notWired("CheckHosts")
}

// OpenHosts implements Tools.
func (NotWired) OpenHosts(context.Context) error { return notWired("OpenHosts") }

// OpenFile implements Tools.
func (NotWired) OpenFile(context.Context, string) error { return notWired("OpenFile") }

// LatestVersion implements Tools.
func (NotWired) LatestVersion(context.Context) (string, error) {
	return "", notWired("LatestVersion")
}

// Diagnostics implements Tools.
func (NotWired) Diagnostics(context.Context) ([]backend.DiagItem, error) {
	return nil, notWired("Diagnostics")
}

// RunDiagAction implements Tools.
func (NotWired) RunDiagAction(context.Context, backend.DiagItem) error {
	return notWired("RunDiagAction")
}

// RunTests implements Tools.
func (NotWired) RunTests(context.Context, backend.TestRequest, backend.TestSink) error {
	return notWired("RunTests")
}

// NetworkReset implements backend.Backend with Options.Tools.
func (b *Backend) NetworkReset(ctx context.Context) (string, error) {
	return b.tools.NetworkReset(ctx)
}

// Reboot implements backend.Backend with Options.Tools.
func (b *Backend) Reboot(ctx context.Context) error { return b.tools.Reboot(ctx) }

// DownloadIPSet implements backend.Backend with Options.Tools.
func (b *Backend) DownloadIPSet(ctx context.Context, progress backend.Progress) error {
	return b.tools.DownloadIPSet(ctx, progress)
}

// CheckHosts implements backend.Backend with Options.Tools.
func (b *Backend) CheckHosts(ctx context.Context, progress backend.Progress) (backend.HostsCheck, error) {
	return b.tools.CheckHosts(ctx, progress)
}

// OpenHosts implements backend.Backend with Options.Tools.
func (b *Backend) OpenHosts(ctx context.Context) error { return b.tools.OpenHosts(ctx) }

// OpenGameLists implements backend.Backend. It makes the missing user
// files of the game (EnsureUserFiles), then opens each user list of the
// game with Options.Tools.
func (b *Backend) OpenGameLists(ctx context.Context, id string) error {
	cat, err := b.l.LoadCatalog()
	if err != nil {
		return err
	}
	g := cat.Game(id)
	if g == nil {
		return fmt.Errorf("unknown game %q", id)
	}
	names := g.UserLists()
	if len(names) == 0 {
		return fmt.Errorf("game %q has no user lists", id)
	}
	if err := b.l.EnsureUserFiles(cat, []*spec.Game{g}); err != nil {
		return fmt.Errorf("make the user files: %w", err)
	}
	var errs []error
	for _, name := range names {
		path, err := b.l.UserFile(name)
		if err == nil {
			err = b.tools.OpenFile(ctx, path)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// LatestVersion implements backend.Backend with Options.Tools.
func (b *Backend) LatestVersion(ctx context.Context) (string, error) {
	return b.tools.LatestVersion(ctx)
}

// Diagnostics implements backend.Backend with Options.Tools.
func (b *Backend) Diagnostics(ctx context.Context) ([]backend.DiagItem, error) {
	return b.tools.Diagnostics(ctx)
}

// RunDiagAction implements backend.Backend with Options.Tools.
func (b *Backend) RunDiagAction(ctx context.Context, item backend.DiagItem) error {
	return b.tools.RunDiagAction(ctx, item)
}

// RunTests implements backend.Backend with Options.Tools.
func (b *Backend) RunTests(ctx context.Context, req backend.TestRequest, sink backend.TestSink) error {
	return b.tools.RunTests(ctx, req, sink)
}
