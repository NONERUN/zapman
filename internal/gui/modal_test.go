package gui

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
)

func TestModal(t *testing.T) {
	var m Modal
	check := func(top string, dialogs ...string) {
		t.Helper()
		if got := m.Top(); got != top {
			t.Errorf("top = %q, want %q", got, top)
		}
		if got := m.Dialogs(); !slices.Equal(got, dialogs) {
			t.Errorf("dialogs = %v, want %v", got, dialogs)
		}
		for _, w := range []string{WinMain, WinStrategy, WinTestsSetup, WinTests} {
			if m.Enabled(w) != (w == top) {
				t.Errorf("Enabled(%s) = %v with top %q", w, m.Enabled(w), top)
			}
		}
	}
	check(WinMain)
	m.Open(WinMain)
	check(WinMain)
	m.Open(WinStrategy)
	m.Open(WinTestsSetup)
	m.Open(WinStrategy)
	check(WinTestsSetup, WinStrategy, WinTestsSetup)
	// A dialog under the top closes first: the top stays.
	m.Close(WinStrategy)
	check(WinTestsSetup, WinTestsSetup)
	m.Close(WinTests)
	check(WinTestsSetup, WinTestsSetup)
	m.Close(WinTestsSetup)
	check(WinMain)

	// A message box disables all windows until the last box closes.
	m.Open(WinTests)
	m.BoxOpen()
	m.BoxOpen()
	m.BoxClose()
	if m.Enabled(WinTests) || m.Enabled(WinMain) {
		t.Error("a window is enabled while a message box is open")
	}
	m.BoxClose()
	m.BoxClose()
	m.Close(WinTests)
	check(WinMain)
}

// flowEnv is the data of a case of TestModalPaths.
type flowEnv struct {
	t  *testing.T
	a  *App
	f  *backend.Fake
	sh *fakeShell
	bg sync.WaitGroup
}

// later runs fn on a new goroutine, as a later click of the user.
func (e *flowEnv) later(fn func()) {
	e.bg.Add(1)
	go func() {
		defer e.bg.Done()
		fn()
	}()
}

// oneError waits until the shell showed one error.
func (e *flowEnv) oneError() {
	eventually(e.t, func() bool { return len(e.sh.get(&e.sh.errors)) == 1 })
}

// TestModalPaths runs each dialog and each message box of the GUI to an end
// with an error, a cancel or the close button of the window. After each
// path the main window must be in front and enabled, no dialog is open, and
// the last state of the main page is not busy.
func TestModalPaths(t *testing.T) {
	fail := func(method string) func(*flowEnv) {
		return func(e *flowEnv) { e.f.FailNext(method, errSample) }
	}
	cases := []struct {
		name string
		// svc installs and runs a service before the start.
		svc     bool
		prepare func(e *flowEnv)
		run     func(a *App)
		// errors is the number of error boxes that the path must show.
		errors int
	}{
		{name: "status close", prepare: func(e *flowEnv) {
			e.sh.on(WinStatus, func() { e.sh.userClose(e.a, WinStatus) })
		}, run: (*App).ClickStatus},
		{name: "status after action error and status error", svc: true, prepare: func(e *flowEnv) {
			e.f.FailNext("Stop", errSample)
			e.f.FailNext("Status", errSample)
		}, run: (*App).ClickStop, errors: 1},
		{name: "strategy close", prepare: func(e *flowEnv) {
			e.sh.on(WinStrategy, func() { e.sh.userClose(e.a, WinStrategy) })
		}, run: (*App).ClickStrategy},
		{name: "strategy cancel", prepare: func(e *flowEnv) {
			e.sh.on(WinStrategy, func() { e.a.CloseDialog(WinStrategy) })
		}, run: (*App).ClickStrategy},
		{name: "strategy install error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("Install", errSample)
			e.sh.on(WinStrategy, func() {
				e.a.StrategyInstall("general", string(spec.Winws))
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinStrategy)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "strategy run error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("Run", errSample)
			e.sh.on(WinStrategy, func() {
				e.a.StrategyRun("general", string(spec.Winws))
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinStrategy)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "strategy bad name, then close", prepare: func(e *flowEnv) {
			e.sh.on(WinStrategy, func() {
				e.a.StrategyRun("nope", string(spec.Winws))
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinStrategy)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "network reset error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("NetworkReset", errSample)
			e.sh.answer(true)
			e.sh.on(WinStrategy, func() {
				e.a.StrategyNone()
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinStrategy)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "reboot error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("Reboot", errSample)
			e.sh.answer(true, true)
			e.sh.on(WinStrategy, func() {
				e.a.StrategyNone()
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinStrategy)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "tests setup close", prepare: func(e *flowEnv) {
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.sh.userClose(e.a, WinTestsSetup) })
		}, run: (*App).ClickStrategy},
		{name: "tests with no strategy", prepare: func(e *flowEnv) {
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.a.TestsStart("standard", nil) })
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "tests with no engine", prepare: func(e *flowEnv) {
			e.f.Update(func(d *backend.FakeData) {
				d.EngineMissing = map[spec.Engine]bool{spec.Winws: true}
			})
		}, run: func(a *App) { a.goFlow(a.testsFlow) }, errors: 1},
		{name: "tests suspend error", svc: true, prepare: func(e *flowEnv) {
			e.f.FailNext("SuspendService", errSample)
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.a.TestsStart("standard", []string{"general"}) })
			e.sh.on(WinTests, func() {
				e.t.Error("the tests window opened after the suspend failed")
				e.later(func() {
					eventually(e.t, func() bool { return e.a.TestsView().Done })
					e.a.CloseDialog(WinTests)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "tests error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("RunTests", errSample)
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.a.TestsStart("standard", []string{"general"}) })
			e.sh.on(WinTests, func() {
				e.later(func() {
					eventually(e.t, func() bool { return e.a.TestsView().Done })
					e.sh.userClose(e.a, WinTests)
				})
			})
		}, run: (*App).ClickStrategy},
		{name: "tests close while they run", prepare: func(e *flowEnv) {
			e.f.Delay = time.Hour
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.a.TestsStart("dpi", []string{"general"}) })
			e.sh.on(WinTests, func() {
				e.later(func() {
					if e.sh.userClose(e.a, WinTests) {
						e.t.Error("the tests window closed while the tests run")
					}
					eventually(e.t, func() bool { return e.a.TestsView().Done })
					e.sh.userClose(e.a, WinTests)
				})
			})
		}, run: (*App).ClickStrategy},
		{name: "tests restore error", svc: true, prepare: func(e *flowEnv) {
			e.f.FailNext("RestoreService", errSample)
			e.sh.on(WinStrategy, func() { e.a.StrategyTests(string(spec.Winws)) })
			e.sh.on(WinTestsSetup, func() { e.a.TestsStart("standard", []string{"general"}) })
			e.sh.on(WinTests, func() {
				e.later(func() {
					eventually(e.t, func() bool { return e.a.TestsView().Done })
					e.a.CloseDialog(WinTests)
				})
			})
		}, run: (*App).ClickStrategy, errors: 1},
		{name: "fakes list error", prepare: fail("Fakes"), run: (*App).ClickFakes, errors: 1},
		{name: "fakes close", prepare: func(e *flowEnv) {
			e.sh.on(WinFake, func() { e.sh.userClose(e.a, WinFake) })
		}, run: (*App).ClickFakes},
		{name: "fakes set error", prepare: func(e *flowEnv) {
			e.f.FailNext("SetFake", errSample)
			e.sh.on(WinFake, func() { e.a.FakesApply(2, 1) })
		}, run: (*App).ClickFakes, errors: 1},
		{name: "games close", prepare: func(e *flowEnv) {
			e.sh.on(WinGames, func() { e.sh.userClose(e.a, WinGames) })
		}, run: (*App).ClickGames},
		{name: "games save error", prepare: func(e *flowEnv) {
			e.f.FailNext("SaveSettings", errSample)
			e.sh.on(WinGames, func() { e.a.GamesApply([]string{"sky"}, nil) })
		}, run: (*App).ClickGames, errors: 1},
		{name: "diagnostics error", prepare: fail("Diagnostics"), run: (*App).ClickDiag, errors: 1},
		{name: "diagnostics action error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("RunDiagAction", errSample)
			e.sh.on(WinDiag, func() {
				e.a.DiagRun(5)
				e.later(func() {
					e.oneError()
					eventually(e.t, func() bool { return !e.a.DiagView().Busy })
					e.sh.userClose(e.a, WinDiag)
				})
			})
		}, run: (*App).ClickDiag, errors: 1},
		{name: "download error", prepare: fail("DownloadIPSet"), run: (*App).ClickIPSet, errors: 1},
		{name: "download close while it runs", prepare: func(e *flowEnv) {
			e.f.Delay = time.Hour
			e.sh.on(WinDownload, func() {
				e.later(func() {
					if e.sh.userClose(e.a, WinDownload) {
						e.t.Error("the download window closed while it runs")
					}
				})
			})
		}, run: (*App).ClickIPSet},
		{name: "hosts error", prepare: fail("CheckHosts"), run: (*App).ClickHosts, errors: 1},
		{name: "hosts close", prepare: func(e *flowEnv) {
			e.sh.on(WinHosts, func() { e.sh.userClose(e.a, WinHosts) })
		}, run: (*App).ClickHosts},
		{name: "hosts open error, then close", prepare: func(e *flowEnv) {
			e.f.FailNext("OpenHosts", errSample)
			e.sh.on(WinHosts, func() {
				e.a.HostsOpen()
				e.later(func() {
					e.oneError()
					e.sh.userClose(e.a, WinHosts)
				})
			})
		}, run: (*App).ClickHosts, errors: 1},
		{name: "start error", svc: true, prepare: func(e *flowEnv) {
			e.f.Update(func(d *backend.FakeData) { d.Status.Service = backend.ServiceStopped })
			e.f.FailNext("StartService", errSample)
		}, run: (*App).ClickStart, errors: 1},
		{name: "remove error", svc: true, prepare: fail("RemoveServices"), run: (*App).ClickRemove, errors: 1},
		{name: "ipset filter error", prepare: fail("SetIPSet"), run: func(a *App) { a.SetIPSet(0) }, errors: 1},
		{name: "game filter error", prepare: fail("SaveSettings"), run: func(a *App) { a.SetGameFilter(1) }, errors: 1},
		{name: "tray task error", prepare: fail("SetTrayAtLogon"), run: func(a *App) { a.SetTrayWatch(true) }, errors: 1},
	}
	opened := map[string]bool{}
	ran := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ran++
			var setup func(f *backend.Fake)
			if c.svc {
				setup = withService
			}
			a, f, sh := start(t, setup)
			e := &flowEnv{t: t, a: a, f: f, sh: sh}
			c.prepare(e)
			c.run(a)
			a.wait()
			e.bg.Wait()
			if got := sh.get(&sh.errors); len(got) != c.errors {
				t.Errorf("errors = %q, want %d", got, c.errors)
			}
			checkFree(t, a, sh)
			for _, w := range sh.get(&sh.opened) {
				opened[w] = true
			}
		})
	}
	// Each dialog window must have a case. Each window has a page in the
	// frontend, thus a new window with no case fails here. With -run only
	// some cases run, and the check does not apply.
	if ran != len(cases) {
		return
	}
	for _, w := range dialogPages(t) {
		if !opened[w] {
			t.Errorf("no case opens the window %q", w)
		}
	}
}

// dialogPages gives the names of the dialog windows from the pages of the
// frontend. index.html is the main window.
func dialogPages(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "cmd", "zapman", "frontend", "*.html"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no frontend pages: %v", err)
	}
	var names []string
	for _, f := range files {
		n := strings.TrimSuffix(filepath.Base(f), ".html")
		if n != "index" {
			names = append(names, n)
		}
	}
	return names
}

// checkFree checks that the main window takes the input of the user: no
// dialog and no message box is open, no flow and no action runs, and the
// main page got its last state with Busy false.
func checkFree(t *testing.T, a *App, sh *fakeShell) {
	t.Helper()
	if d := sh.modal.Dialogs(); len(d) != 0 {
		t.Errorf("dialogs are open: %v", d)
	}
	if !sh.modal.Enabled(WinMain) {
		t.Error("the main window is not enabled")
	}
	a.mu.Lock()
	n, busy, flow := len(a.dialogs), a.busy, a.flow
	a.mu.Unlock()
	if n != 0 || busy || flow {
		t.Errorf("dialogs %d, busy %v, flow %v", n, busy, flow)
	}
	sh.mu.Lock()
	last := sh.lastBusy
	sh.mu.Unlock()
	if last {
		t.Error("the last state of the main page is busy")
	}
}
