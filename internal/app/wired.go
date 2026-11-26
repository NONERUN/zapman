package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/spec"
	"github.com/NONERUN/zapman/internal/tools/command"
	"github.com/NONERUN/zapman/internal/tools/diag"
	"github.com/NONERUN/zapman/internal/tools/netreset"
	"github.com/NONERUN/zapman/internal/tools/stratest"
	"github.com/NONERUN/zapman/internal/tools/web"
)

// TestIPSetBackup is the copy of user\ipset-all.txt while the DPI tests
// run, as in Tests.ps1. When the copy is there before the tests, the tests
// of an earlier run did not end; RunTests puts the copy back first.
const TestIPSetBackup = "ipset-all.test-backup.txt"

// testRunner runs the strategy tests: *stratest.Tester.
type testRunner interface {
	Run(ctx context.Context, req stratest.Request, sink stratest.Sink) error
}

// wired is the Tools of internal/tools.
type wired struct {
	b   *Backend
	run command.Func
	web *web.Client
	// hosts gives the path of the system hosts file.
	hosts func() string
	// notepad opens a file in Notepad.
	notepad func(path string) error
	// diag gives the Checker of the diagnostics.
	diag func() *diag.Checker
	// tester gives the tests for the folder, the catalog and the config.
	tester func(l engine.Layout, cat *spec.Catalog, cfg config.Config) testRunner
}

var _ Tools = (*wired)(nil)

// NewTools gives the Tools of internal/tools for b. Give it to
// b.SetTools.
func NewTools(b *Backend) Tools {
	return &wired{
		b:       b,
		run:     command.Run,
		web:     &web.Client{},
		hosts:   web.HostsPath,
		notepad: web.OpenNotepad,
		diag:    func() *diag.Checker { return diag.New(b.l.Root, b.l.Bin) },
		tester:  newTester,
	}
}

// newTester is stratest.New with only the strategies that have flags for
// the engine of the tests.
func newTester(l engine.Layout, cat *spec.Catalog, cfg config.Config) testRunner {
	t := stratest.New(l, cat, cfg)
	t.Engine = supportedEngine{Engine: t.Engine, cat: cat}
	return t
}

// supportedEngine is a stratest.Engine whose Names gives only the
// strategies that support the engine (see supports). The strategy list of
// the GUI uses the same rule.
type supportedEngine struct {
	stratest.Engine
	cat *spec.Catalog
}

// Names implements stratest.Engine.
func (s supportedEngine) Names(e spec.Engine) ([]string, error) {
	all, err := s.Engine.Names(e)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range all {
		if st, err := s.cat.Strategy(n); err == nil && supports(st, e) {
			out = append(out, n)
		}
	}
	return out, nil
}

// NetworkReset implements Tools.
func (w *wired) NetworkReset(ctx context.Context) (string, error) {
	return netreset.Reset(ctx, w.run)
}

// Reboot implements Tools.
func (w *wired) Reboot(ctx context.Context) error { return netreset.Reboot(ctx, w.run) }

// DownloadIPSet implements Tools. The list goes to user\ipset-all.txt.
func (w *wired) DownloadIPSet(ctx context.Context, p backend.Progress) error {
	return w.web.DownloadIPSet(ctx, filepath.Join(w.b.l.User, engine.IPSetFile), web.Progress(p))
}

// CheckHosts implements Tools.
func (w *wired) CheckHosts(ctx context.Context, p backend.Progress) (backend.HostsCheck, error) {
	hc, err := w.web.CheckHosts(ctx, w.hosts(), web.Progress(p))
	if err != nil {
		return backend.HostsCheck{}, err
	}
	return backend.HostsCheck{NeedsUpdate: hc.NeedsUpdate, Template: hc.Template}, nil
}

// OpenHosts implements Tools.
func (w *wired) OpenHosts(context.Context) error { return w.notepad(w.hosts()) }

// OpenFile implements Tools.
func (w *wired) OpenFile(_ context.Context, path string) error { return w.notepad(path) }

// LatestVersion implements Tools. An answer with no version tag gives ""
// and no error; the GUI tells that the check failed.
func (w *wired) LatestVersion(ctx context.Context) (string, error) {
	v, err := w.web.LatestVersion(ctx)
	if errors.Is(err, web.ErrNoVersion) {
		return "", nil
	}
	return v, err
}

// Diagnostics implements Tools.
func (w *wired) Diagnostics(ctx context.Context) ([]backend.DiagItem, error) {
	items, err := w.diag().Report(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backend.DiagItem, len(items))
	for i, it := range items {
		out[i] = backend.DiagItem{
			ID: it.ID, Status: backend.DiagStatus(it.Status), Text: it.Text,
			Action: it.Action, ActionNames: it.ActionNames,
		}
	}
	return out, nil
}

// RunDiagAction implements Tools. The actions that remove services need
// administrator rights.
func (w *wired) RunDiagAction(ctx context.Context, item backend.DiagItem) error {
	if item.Action != diag.ActionDiscordCache {
		if err := w.b.elevated(); err != nil {
			return err
		}
	}
	_, err := w.diag().Do(ctx, diag.Item{
		ID: item.ID, Status: diag.Status(item.Status), Text: item.Text,
		Action: item.Action, ActionNames: item.ActionNames,
	})
	return err
}

// RunTests implements Tools, as Tests.ps1 does. It removes the zapret
// service and stops the process of Run. For the DPI tests it sets the IPSet
// Filter to "any". After the tests (also after an error or a cancel) it
// puts back the ipset, then the service, then the process of Run.
//
// The strategies are only those that support the engine. The other
// operations of the Backend wait until the tests end.
func (w *wired) RunTests(ctx context.Context, req backend.TestRequest, sink backend.TestSink) (err error) {
	b := w.b
	if err := b.CheckEngine(ctx, req.Engine); err != nil {
		return err
	}
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	// The restores must run also when the user cancels the tests.
	bg := context.WithoutCancel(ctx)
	_, runName := b.liveRun()
	b.mu.Lock()
	runEngine := b.runEngine
	b.mu.Unlock()
	snap, err := b.suspend()
	if err != nil {
		return err
	}
	if runName != "" {
		defer func() { err = errors.Join(err, b.restart(bg, runName, runEngine)) }()
	}
	if s, ok := snap.Data.(service.Snapshot); ok && snap.Installed {
		defer func() { err = errors.Join(err, b.restore(&s)) }()
	}
	// Stop the process of Run also when no service was there.
	if err := b.stopRun(); err != nil {
		return err
	}

	cat, err := b.l.LoadCatalog()
	if err != nil {
		return err
	}
	b.cfg.Lock()
	cfg, _ := b.loadConfig()
	b.cfg.Unlock()
	if err := b.l.EnsureUserFiles(cat, gamesOf(cat, &cfg)); err != nil {
		return fmt.Errorf("make the user files: %w", err)
	}
	if err := b.l.RestoreIPSet(TestIPSetBackup); err != nil {
		return fmt.Errorf("put back the ipset of earlier tests: %w", err)
	}
	if req.Type == backend.TestDPI {
		mode, err := b.l.IPSetMode()
		if err != nil {
			return err
		}
		if mode != engine.IPSetAny {
			if err := b.l.CopyIPSetMode(engine.IPSetAny, TestIPSetBackup); err != nil {
				return fmt.Errorf("set the IPSet Filter to any: %w", err)
			}
			defer func() {
				if rerr := b.l.RestoreIPSet(TestIPSetBackup); rerr != nil {
					err = errors.Join(err, fmt.Errorf("put back the ipset: %w", rerr))
				}
			}()
		}
	}

	t := w.tester(b.l, cat, cfg)
	return t.Run(ctx, stratest.Request{
		Type: stratest.Type(req.Type), Strategies: req.Strategies, Engine: req.Engine,
	}, testSink{sink})
}

// gamesOf gives the games of cfg that are in the catalog, for
// EnsureUserFiles. The option values are not necessary: EnsureUserFiles
// makes the lists of all bundles of a game.
func gamesOf(cat *spec.Catalog, cfg *config.Config) []*spec.Game {
	var out []*spec.Game
	for _, id := range cfg.Games {
		if g := cat.Game(id); g != nil {
			out = append(out, g)
		}
	}
	return out
}

// restart starts the process of Run again with the strategy name and the
// engine e, after the tests. It does not change user\config.json. The
// caller holds b.op.
func (b *Backend) restart(ctx context.Context, name string, e spec.Engine) error {
	bd, err := b.build(ctx, name, e)
	if err != nil {
		return err
	}
	r, err := b.runner.Run(bd)
	if err != nil {
		return backend.Msg("EngineStartFailText", filepath.Base(bd.Exe), err.Error())
	}
	b.mu.Lock()
	b.run, b.runName, b.runEngine = r, name, e
	b.mu.Unlock()
	return nil
}

// testSink is the stratest.Sink of a backend.TestSink.
type testSink struct{ s backend.TestSink }

func (t testSink) Line(text string) { t.s.Line(text) }

func (t testSink) Strategy(r stratest.Result) {
	out := backend.TestResult{Name: r.Name, Kind: r.Kind, Error: r.Error}
	for _, g := range r.Targets {
		out.Targets = append(out.Targets, backend.TestTarget{
			Name: g.Name, HTTP: g.HTTP, TLS12: g.TLS12, TLS13: g.TLS13, Ping: g.Ping,
		})
	}
	t.s.Strategy(out)
}

func (t testSink) Summary(s stratest.Summary) {
	out := backend.TestSummary{Type: backend.TestType(s.Type), Best: s.Best}
	for _, r := range s.Rows {
		out.Rows = append(out.Rows, backend.TestSumRow{
			Name: r.Name, OK: r.OK, Err: r.Err, Unsup: r.Unsup,
			PingOK: r.PingOK, PingFail: r.PingFail, Fail: r.Fail, Blocked: r.Blocked,
		})
	}
	t.s.Summary(out)
}
