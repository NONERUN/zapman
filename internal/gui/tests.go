package gui

import (
	"context"
	"slices"
	"strings"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
)

// setupDialog is the state of the Run Tests dialog.
type setupDialog struct {
	engine spec.Engine
	names  []string
	picked string
}

// setupChoice is the result of the Run Tests dialog.
type setupChoice struct {
	typ   backend.TestType
	names []string
}

// SetupView is the state of the Run Tests dialog.
type SetupView struct {
	Engine string
	// Names are the strategies that the engine can build.
	Names []string
	// Checked are the names that are checked at the start.
	Checked []string
}

// testsState is the state of the tests window.
type testsState struct {
	cancel    context.CancelFunc
	running   bool
	cancelled bool
	phase     string
	view      TestsView
}

// TestsView is the state of the tests window.
type TestsView struct {
	Status string
	// Value and Max are the progress bar. Max 0 shows a bar with no value.
	Value int
	Max   int
	// Head is the log of the setup, before the first strategy.
	Head string
	Tabs []StrategyTab
	// Wait tells to show the "waiting for the first strategy" text.
	Wait    bool
	Summary Table
	Best    string
	// Done tells that the tests ended. The button is Close, not Cancel.
	Done bool
}

// testsFlow runs the tests, as Invoke-GuiTestsFromMain does: the setup
// dialog, the service suspend, the tests window, the service restore.
func (a *App) testsFlow() {
	choice, err := a.testsSetup()
	if err != nil {
		a.shell.Error(a.errText(err))
		return
	}
	if choice == nil {
		return
	}
	var snap backend.ServiceSnapshot
	if a.hasService() {
		// The tests cannot run while the service runs. When the service
		// does not stop, the action shows the error and the flow stops.
		ok := a.action("TestsNeedNoService", func(ctx context.Context) error {
			var err error
			snap, err = a.be.SuspendService(ctx)
			return err
		})
		if !ok {
			return
		}
	}
	a.testsRun(*choice)
	a.logLine(a.t("TestsFinished"))
	if !snap.Installed {
		return
	}
	a.action("BtnInstall", func(ctx context.Context) error {
		if err := a.be.RestoreService(ctx, snap); err != nil {
			return err
		}
		name := snap.Strategy
		if name == "" {
			name = "zapret"
		}
		a.logLine(a.t("InstallDone", name))
		return nil
	})
}

// testsSetup shows the Run Tests dialog, as Show-GuiTestsSetup does. It
// gives nil when the user cancels.
func (a *App) testsSetup() (*setupChoice, error) {
	e := a.engine()
	if err := a.be.CheckEngine(a.ctx, e); err != nil {
		if e == spec.Winws2 {
			return nil, backend.Msg("EngineNoWinws2")
		}
		return nil, backend.Msg("EngineNoWinws")
	}
	a.mu.Lock()
	var names []string
	for _, s := range a.strategies {
		if s.Supports(e) {
			names = append(names, s.Name)
		}
	}
	picked := a.status.Running
	if picked == "" {
		picked = a.status.Installed
	}
	a.setup = setupDialog{engine: e, names: names, picked: picked}
	a.mu.Unlock()
	if len(names) == 0 {
		return nil, backend.Msg("EngineNoFlags", string(e))
	}
	res := a.showDialog(WinTestsSetup, "TestsSetupTitle")
	c, ok := res.(setupChoice)
	if !ok {
		return nil, nil
	}
	if len(c.names) == 0 {
		return nil, backend.Msg("TestsNeedOne")
	}
	return &c, nil
}

// SetupView gives the state of the Run Tests dialog.
func (a *App) SetupView() SetupView {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := SetupView{Engine: string(a.setup.engine), Names: slices.Clone(a.setup.names)}
	if slices.Contains(v.Names, a.setup.picked) {
		v.Checked = []string{a.setup.picked}
	} else if len(v.Names) > 0 {
		v.Checked = []string{v.Names[0]}
	}
	return v
}

// TestsStart closes the Run Tests dialog with the test type ("standard" or
// "dpi") and the checked names.
func (a *App) TestsStart(typ string, names []string) {
	t := backend.TestStandard
	if typ == string(backend.TestDPI) {
		t = backend.TestDPI
	}
	a.mu.Lock()
	var keep []string
	for _, n := range names {
		if slices.Contains(a.setup.names, n) && !slices.Contains(keep, n) {
			keep = append(keep, n)
		}
	}
	a.mu.Unlock()
	a.closeDialog(WinTestsSetup, setupChoice{typ: t, names: keep})
}

// testsRun opens the tests window, runs the tests and waits until the user
// closes the window.
func (a *App) testsRun(c setupChoice) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.mu.Lock()
	a.tests = testsState{
		cancel: cancel, running: true, phase: "head",
		view: TestsView{Status: a.tLocked("TestsStarting"), Wait: true},
	}
	a.lastError = ""
	e := a.settings.Engine
	a.mu.Unlock()
	if e == "" {
		e = spec.Winws
	}
	d := a.openDialog(WinTests, "TestsTitle")
	if d == nil {
		return
	}
	err := a.be.RunTests(ctx, backend.TestRequest{Type: c.typ, Strategies: c.names, Engine: e}, testSink{a})
	a.mu.Lock()
	ts := &a.tests
	if err != nil && !isCancel(err) {
		ts.view.Head += a.errText(err) + "\n"
	}
	ts.running = false
	ts.view.Done = true
	if ts.view.Max < 1 {
		ts.view.Max = 1
	}
	ts.view.Value = ts.view.Max
	if ts.cancelled {
		ts.view.Status = a.tLocked("TestsCancelledShort")
	} else {
		ts.view.Status = a.tLocked("TestsFinishedLog")
	}
	a.mu.Unlock()
	a.emit(EvTests)
	a.waitDialog(WinTests, d)
	a.refresh()
}

// testsCancel cancels the tests when they run. It gives false when they do
// not run.
func (a *App) testsCancel() bool {
	a.mu.Lock()
	ts := &a.tests
	if !ts.running {
		a.mu.Unlock()
		return false
	}
	ts.cancelled = true
	ts.view.Status = a.tLocked("TestsCancelling")
	cancel := ts.cancel
	a.mu.Unlock()
	cancel()
	a.emit(EvTests)
	return true
}

// TestsView gives the state of the tests window.
func (a *App) TestsView() TestsView {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := a.tests.view
	v.Tabs = slices.Clone(v.Tabs)
	return v
}

// testSink puts the output of the tests into the tests window, as
// Add-GuiTestLine, Add-GuiTestStrategyTab and Add-GuiTestSummary do.
type testSink struct{ a *App }

func (s testSink) Line(text string) {
	a := s.a
	a.mu.Lock()
	ts := &a.tests
	n, m, step := testStep(text)
	if step {
		ts.phase = "mid"
	}
	if ts.phase == "head" {
		ts.view.Head += strings.TrimRight(text, "\r\n") + "\n"
	}
	if step {
		ts.view.Max = m
		ts.view.Value = min(max(n, 0), m)
		ts.view.Status = a.tLocked("TestsProgress", n, m)
	}
	a.mu.Unlock()
	a.emit(EvTests)
}

func (s testSink) Strategy(r backend.TestResult) {
	a := s.a
	a.mu.Lock()
	ts := &a.tests
	ts.view.Tabs = append(ts.view.Tabs, strategyTab(a.lang, r))
	ts.view.Wait = false
	ts.phase = "mid"
	a.mu.Unlock()
	a.emit(EvTests)
}

func (s testSink) Summary(sum backend.TestSummary) {
	a := s.a
	a.mu.Lock()
	ts := &a.tests
	ts.phase = "sum"
	ts.view.Best = ""
	if sum.Best != "" {
		ts.view.Best = a.tLocked("TestsBest", sum.Best)
	}
	ts.view.Summary = summaryTable(a.lang, sum)
	a.mu.Unlock()
	a.emit(EvTests)
}
