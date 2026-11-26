package gui

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/gui/i18n"
	"github.com/NONERUN/zapman/internal/spec"
)

// fakeShell records the calls of App. The answers of Ask come from a
// queue; an empty queue gives false. onOpen runs in OpenWindow, as a click
// of the user in the new window. modal follows the windows and the message
// boxes as the shell of cmd/zapman does.
type fakeShell struct {
	mu      sync.Mutex
	opened  []string
	closed  []string
	hidden  []string
	asks    []string
	answers []bool
	infos   []string
	errors  []string
	urls    []string
	tray    TrayView
	trayOn  bool
	onOpen  map[string]func()

	modal Modal
	// mainBusy reads the Busy field of the main view. Emit of EvMain keeps
	// its value in lastBusy, as the page of the main window reads it.
	mainBusy func() bool
	lastBusy bool
}

func (s *fakeShell) OpenWindow(name, _ string) {
	s.modal.Open(name)
	s.mu.Lock()
	s.opened = append(s.opened, name)
	fn := s.onOpen[name]
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (s *fakeShell) CloseWindow(name string) {
	s.modal.Close(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = append(s.closed, name)
}

// box counts a message box as the shell does. It gives the function that
// closes it.
func (s *fakeShell) box() func() {
	s.modal.BoxOpen()
	return s.modal.BoxClose
}

// userClose does what the shell does when the user closes the window with
// the close button.
func (s *fakeShell) userClose(a *App, name string) bool {
	if !Closing(a, name) {
		return false
	}
	if name != WinMain {
		s.CloseWindow(name)
	}
	return true
}

func (s *fakeShell) HideWindow(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hidden = append(s.hidden, name)
}

func (s *fakeShell) SetTitle(string, string) {}

func (s *fakeShell) Emit(ev string) {
	if ev != EvMain {
		return
	}
	s.mu.Lock()
	fn := s.mainBusy
	s.mu.Unlock()
	if fn == nil {
		return
	}
	busy := fn()
	s.mu.Lock()
	s.lastBusy = busy
	s.mu.Unlock()
}

func (s *fakeShell) Ask(text string, _ bool) bool {
	defer s.box()()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asks = append(s.asks, text)
	if len(s.answers) == 0 {
		return false
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a
}

func (s *fakeShell) Info(text string) {
	defer s.box()()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infos = append(s.infos, text)
}

func (s *fakeShell) Error(text string) {
	defer s.box()()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, text)
}

func (s *fakeShell) OpenURL(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = append(s.urls, url)
	return nil
}

func (s *fakeShell) SetTray(v TrayView, visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tray, s.trayOn = v, visible
}

// on sets the action of the user for a window.
func (s *fakeShell) on(name string, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.onOpen == nil {
		s.onOpen = map[string]func(){}
	}
	s.onOpen[name] = fn
}

func (s *fakeShell) answer(v ...bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answers = append(s.answers, v...)
}

func (s *fakeShell) get(list *[]string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(*list)
}

// en gives the English string of the key.
func en(key string, args ...any) string { return i18n.T(i18n.EN, key, args...) }

// start gives a started App on a Fake. setup changes the Fake before the
// start. The calls of Start are cleared.
func start(t *testing.T, setup func(f *backend.Fake)) (*App, *backend.Fake, *fakeShell) {
	t.Helper()
	f := backend.NewFake()
	if setup != nil {
		setup(f)
	}
	sh := &fakeShell{}
	a := New(f, sh, Options{Culture: "en-US", Tick: time.Hour})
	sh.mainBusy = func() bool { return a.Main().Busy }
	if err := Start(a); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Stop(a) })
	a.wait()
	f.ResetCalls()
	return a, f, sh
}

// withService installs the "general" service and runs it.
func withService(f *backend.Fake) {
	f.Update(func(d *backend.FakeData) {
		d.Settings.Strategy = "general"
		d.Status.Installed = "general"
		d.Status.Service = backend.ServiceRunning
		d.Status.BypassRunning = true
		d.Status.Running = "general"
		d.Status.BypassExes = []string{"winws.exe"}
		d.Status.Divert = backend.ServiceRunning
	})
}

// eventually waits until cond is true.
func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	for range 500 {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the condition is not true after 5 s")
}

func TestStartNoService(t *testing.T) {
	a, _, sh := start(t, nil)
	v := a.Main()
	if v.Lang != i18n.EN {
		t.Errorf("lang = %q", v.Lang)
	}
	if v.Status.Banner.Text != en("BannerNoService") {
		t.Errorf("banner = %q", v.Status.Banner.Text)
	}
	if v.HasSvc || !v.HasStrt || v.Busy {
		t.Errorf("view = %+v", v)
	}
	if v.IPSet != slices.Index(backend.IPSetModes, backend.IPSetLoaded) {
		t.Errorf("ipset index = %d", v.IPSet)
	}
	if sh.trayOn {
		t.Error("the tray icon is on with TrayWatch off")
	}
}

func TestStartConfigError(t *testing.T) {
	a, _, _ := start(t, func(f *backend.Fake) {
		f.Update(func(d *backend.FakeData) { d.ConfigErr = errSample })
	})
	if !strings.Contains(a.StatusText(), en("ConfigParseFail")) {
		t.Errorf("status text has no config error:\n%s", a.StatusText())
	}
}

type sampleError struct{}

func (sampleError) Error() string { return "sample" }

var errSample = sampleError{}

func TestInstallFromStrategyDialog(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinStrategy, func() {
		v := a.StrategyView("")
		if v.Engine != string(spec.Winws) || !v.EngineOK || len(v.Items) != 4 || v.Selected != 0 {
			t.Errorf("strategy view = %+v", v)
		}
		a.StrategyInstall("general (ALT)", string(spec.Winws2))
	})
	a.ClickStrategy()
	a.wait()
	d := f.Data()
	if d.Status.Installed != "general (ALT)" || d.Settings.Engine != spec.Winws2 {
		t.Errorf("installed %q with %q", d.Status.Installed, d.Settings.Engine)
	}
	if got := sh.get(&sh.closed); !slices.Contains(got, WinStrategy) {
		t.Errorf("the dialog did not close: %v", got)
	}
	if !strings.Contains(a.StatusText(), en("InstallDone", "general (ALT)")) {
		t.Error("the journal has no InstallDone")
	}
	// The settings in memory have the strategy of Install: a later save
	// must not lose it.
	a.SetAutoUpdate(false)
	a.wait()
	if s := f.Data().Settings; s.Strategy != "general (ALT)" || s.AutoUpdateCheck {
		t.Errorf("settings = %+v", s)
	}
	v := a.Main()
	if v.Status.Banner.Text != "" || !v.HasSvc {
		t.Errorf("main view = %+v", v)
	}
	sv := a.StrategyView(string(spec.Winws2))
	if sv.Items[sv.Selected].Name != "general (ALT)" || !strings.HasSuffix(sv.Items[sv.Selected].Label, en("StratMarkRun")) {
		t.Errorf("selected item = %+v", sv.Items[sv.Selected])
	}
	if sv.Items[2].Supported {
		t.Error("general (ALT2) is supported by winws2")
	}
}

func TestActionErrorKeepsLastError(t *testing.T) {
	a, f, sh := start(t, withService)
	f.FailNext("Stop", backend.Msg("NoStrategies"))
	a.ClickStop()
	a.wait()
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{en("NoStrategies")}) {
		t.Errorf("errors = %q", got)
	}
	if b := a.Main().Status.Banner.Text; b != en("BannerLastError") {
		t.Errorf("banner = %q", b)
	}
	a.ClickStart()
	a.wait()
	if b := a.Main().Status.Banner.Text; b != "" {
		t.Errorf("banner after start = %q", b)
	}
}

func TestRunAsksWhenLive(t *testing.T) {
	a, f, sh := start(t, withService)
	a.StrategyRun("general (ALT)", string(spec.Winws))
	a.wait()
	if slices.Contains(f.Calls(), "Run") {
		t.Fatal("Run ran with the answer No")
	}
	if got := sh.get(&sh.asks); !slices.Equal(got, []string{en("ConfirmRunOver")}) {
		t.Errorf("asks = %q", got)
	}
	sh.answer(true)
	a.StrategyRun("general (ALT)", string(spec.Winws))
	a.wait()
	if d := f.Data(); d.Status.Running != "general (ALT)" {
		t.Errorf("running = %q", d.Status.Running)
	}
}

func TestRunBadName(t *testing.T) {
	a, f, sh := start(t, nil)
	a.StrategyRun("nope", string(spec.Winws))
	a.wait()
	if slices.Contains(f.Calls(), "Run") || len(sh.get(&sh.errors)) != 1 {
		t.Errorf("calls %v, errors %q", f.Calls(), sh.get(&sh.errors))
	}
}

func TestTestsFlowSuspendsService(t *testing.T) {
	a, f, sh := start(t, withService)
	sh.on(WinStrategy, func() { a.StrategyTests(string(spec.Winws)) })
	sh.on(WinTestsSetup, func() {
		v := a.SetupView()
		if !slices.Equal(v.Checked, []string{"general"}) || len(v.Names) != 4 {
			t.Errorf("setup view = %+v", v)
		}
		a.TestsStart("standard", []string{"general", "general (ALT)", "nope"})
	})
	sh.on(WinTests, func() {
		go func() {
			eventually(t, func() bool { return a.TestsView().Done })
			a.CloseDialog(WinTests)
		}()
	})
	a.ClickStrategy()
	a.wait()
	calls := f.Calls()
	want := []string{"SaveSettings", "SuspendService", "RunTests", "RestoreService"}
	if !slices.Equal(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
	v := a.TestsView()
	if len(v.Tabs) != 2 || v.Tabs[0].HasError || !v.Tabs[1].HasError {
		t.Errorf("tabs = %+v", v.Tabs)
	}
	if v.Best != en("TestsBest", "general") || v.Status != en("TestsFinishedLog") || v.Value != 2 || v.Max != 2 {
		t.Errorf("view = %+v", v)
	}
	if !strings.Contains(v.Head, "Engine: winws.exe") {
		t.Errorf("head = %q", v.Head)
	}
	if d := f.Data(); d.Status.Installed != "general" || d.Status.Service != backend.ServiceRunning {
		t.Errorf("status after restore = %+v", d.Status)
	}
}

func TestTestsCloseCancels(t *testing.T) {
	a, f, sh := start(t, nil)
	f.Delay = time.Hour
	sh.on(WinTestsSetup, func() { a.TestsStart("dpi", []string{"general"}) })
	sh.on(WinTests, func() {
		go func() {
			if Closing(a, WinTests) {
				t.Error("the tests window closed while the tests run")
			}
			eventually(t, func() bool { return a.TestsView().Done })
			if !Closing(a, WinTests) {
				t.Error("the tests window did not close at the end")
			}
		}()
	})
	sh.on(WinStrategy, func() { a.StrategyTests(string(spec.Winws)) })
	a.ClickStrategy()
	a.wait()
	if v := a.TestsView(); v.Status != en("TestsCancelledShort") || len(v.Tabs) != 0 {
		t.Errorf("view = %+v", v)
	}
	if slices.Contains(f.Calls(), "SuspendService") {
		t.Error("SuspendService ran with no service")
	}
}

func TestTestsNeedOne(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinStrategy, func() { a.StrategyTests(string(spec.Winws)) })
	sh.on(WinTestsSetup, func() { a.TestsStart("standard", nil) })
	a.ClickStrategy()
	a.wait()
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{en("TestsNeedOne")}) {
		t.Errorf("errors = %q", got)
	}
	if slices.Contains(f.Calls(), "RunTests") {
		t.Error("RunTests ran with no strategy")
	}
}

func TestDownloadCancel(t *testing.T) {
	a, f, sh := start(t, nil)
	f.Delay = time.Hour
	sh.on(WinDownload, func() {
		go func() {
			if v := a.DownloadView(); v.Status != en("DownloadConnecting") || v.Percent != -1 {
				t.Errorf("download view = %+v", v)
			}
			if Closing(a, WinDownload) {
				t.Error("the download window closed while it runs")
			}
		}()
	})
	a.ClickIPSet()
	a.wait()
	if f.Data().IPSetDownloads != 0 {
		t.Error("the download ended")
	}
	if !strings.Contains(a.StatusText(), en("IpsetCancel")) {
		t.Error("the journal has no IpsetCancel")
	}
	if len(sh.get(&sh.errors)) != 0 {
		t.Errorf("errors = %q", sh.get(&sh.errors))
	}
}

func TestDownloadIPSet(t *testing.T) {
	a, f, sh := start(t, nil)
	a.ClickIPSet()
	a.wait()
	if f.Data().IPSetDownloads != 1 || !strings.Contains(a.StatusText(), en("IpsetOk")) {
		t.Errorf("downloads %d", f.Data().IPSetDownloads)
	}
	if v := a.DownloadView(); v.Percent != 100 {
		t.Errorf("download view = %+v", v)
	}
	if got := sh.get(&sh.closed); !slices.Equal(got, []string{WinDownload}) {
		t.Errorf("closed = %v", got)
	}
}

func TestHostsNeedsUpdate(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinHosts, func() {
		if !strings.Contains(a.HostsView(), "example.invalid") {
			t.Errorf("template = %q", a.HostsView())
		}
		a.HostsOpen()
		a.CloseDialog(WinHosts)
	})
	a.ClickHosts()
	a.wait()
	if f.Data().HostsOpened != 1 || !strings.Contains(a.StatusText(), en("HostsNeed")) {
		t.Errorf("opened %d", f.Data().HostsOpened)
	}
	f.FailNext("CheckHosts", errSample)
	a.ClickHosts()
	a.wait()
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{en("HostsFail")}) {
		t.Errorf("errors = %q", got)
	}
}

func TestFakesApply(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinFake, func() {
		v := a.FakesView()
		if v.DiscordSelected != 1 || v.GameSelected != 0 || len(v.Game) != 4 || v.Game[0] != en("FakesMissing") {
			t.Errorf("fakes view = %+v", v)
		}
		a.FakesApply(2, 1)
	})
	a.ClickFakes()
	a.wait()
	d := f.Data().Fakes
	if d.Discord != "tls_clienthello_www_google_com.bin" || d.Game != "quic_initial_www_google_com.bin" {
		t.Errorf("fakes = %+v", d)
	}
	if got := sh.get(&sh.infos); !slices.Equal(got, []string{en("FakeNeedRerun")}) {
		t.Errorf("infos = %q", got)
	}
}

func TestFakesRestartService(t *testing.T) {
	a, f, sh := start(t, withService)
	sh.on(WinFake, func() { a.FakesApply(1, 0) })
	a.ClickFakes()
	a.wait()
	if c := f.Calls(); len(c) != 0 {
		t.Errorf("calls with no change = %v", c)
	}
	sh.on(WinFake, func() { a.FakesApply(0, 2) })
	sh.answer(true)
	a.ClickFakes()
	a.wait()
	want := []string{"SetFake", "SetFake", "Stop", "StartService"}
	if c := f.Calls(); !slices.Equal(c, want) {
		t.Errorf("calls = %v, want %v", c, want)
	}
}

func TestGameFilterOpensStrategy(t *testing.T) {
	a, f, sh := start(t, withService)
	sh.answer(true)
	sh.on(WinStrategy, func() { a.CloseDialog(WinStrategy) })
	a.SetGameFilter(2)
	a.wait()
	if g := f.Data().Settings.GameFilter; g != backend.GameFilterModes[2] {
		t.Errorf("game filter = %q", g)
	}
	if got := sh.get(&sh.asks); !slices.Equal(got, []string{en("FilterNeedInstall")}) {
		t.Errorf("asks = %q", got)
	}
	if got := sh.get(&sh.opened); !slices.Equal(got, []string{WinStrategy}) {
		t.Errorf("opened = %v", got)
	}
	if a.Main().GameFilter != 2 {
		t.Error("the combo box index did not change")
	}
}

func TestGamesApply(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinGames, func() {
		if v := a.GamesView(); len(v) != 2 || v[0].Checked {
			t.Errorf("games view = %+v", v)
		}
		a.GamesApply([]string{"sker", "nope"})
	})
	a.ClickGames()
	a.wait()
	if g := f.Data().Settings.Games; !slices.Equal(g, []string{"sker"}) {
		t.Errorf("games = %v", g)
	}
	if got := sh.get(&sh.infos); !slices.Equal(got, []string{en("FilterNeedRerun")}) {
		t.Errorf("infos = %q", got)
	}
}

func TestMainCloseWithTray(t *testing.T) {
	a, _, sh := start(t, nil)
	if !Closing(a, WinMain) {
		t.Error("main did not close with the tray off")
	}
	a.SetTrayWatch(true)
	a.wait()
	if Closing(a, WinMain) {
		t.Error("main closed with the tray on")
	}
	if got := sh.get(&sh.hidden); !slices.Equal(got, []string{WinMain}) {
		t.Errorf("hidden = %v", got)
	}
	sh.mu.Lock()
	on, tray := sh.trayOn, sh.tray
	sh.mu.Unlock()
	if !on || tray.StartEnabled || tray.StopEnabled {
		t.Errorf("tray %v %+v", on, tray)
	}
}

func TestVersionNew(t *testing.T) {
	a, f, sh := start(t, nil)
	f.Update(func(d *backend.FakeData) { d.Version.App = "v0.2.0"; d.Latest = "v0.10.0" })
	sh.answer(true)
	a.ClickVersion()
	a.wait()
	if got := sh.get(&sh.urls); !slices.Equal(got, []string{f.Version().ReleasesURL}) {
		t.Errorf("urls = %v", got)
	}
	f.Update(func(d *backend.FakeData) { d.Latest = d.Version.App })
	a.ClickVersion()
	a.wait()
	if got := sh.get(&sh.infos); !slices.Equal(got, []string{en("VersionLatest", f.Version().App)}) {
		t.Errorf("infos = %q", got)
	}
}

// TestVersionNoOffer: a release that is newer than the latest tag, and a
// dev build, get no offer of an update.
func TestVersionNoOffer(t *testing.T) {
	for _, tc := range []struct{ app, latest, info string }{
		{"v0.2.0", "v0.1.1", en("VersionLatest", "v0.2.0")},
		{"dev-0123abc", "v0.1.1", en("VersionDev", "dev-0123abc", "v0.1.1")},
		{"dev-0123abc-dirty", "v0.1.1", en("VersionDev", "dev-0123abc-dirty", "v0.1.1")},
		{"v0.1.2-0.20261120101010-0123456789ab", "v0.1.1", en("VersionDev", "v0.1.2-0.20261120101010-0123456789ab", "v0.1.1")},
		{"v0.1.1+dirty", "v9.9.9", en("VersionDev", "v0.1.1+dirty", "v9.9.9")},
	} {
		t.Run(tc.app+"_"+tc.latest, func(t *testing.T) {
			a, f, sh := start(t, nil)
			f.Update(func(d *backend.FakeData) { d.Version.App = tc.app; d.Latest = tc.latest })
			a.ClickVersion()
			a.wait()
			if got := sh.get(&sh.asks); len(got) != 0 {
				t.Errorf("asks = %q", got)
			}
			if got := sh.get(&sh.infos); !slices.Equal(got, []string{tc.info}) {
				t.Errorf("infos = %q, want %q", got, tc.info)
			}
		})
	}
}

func TestDiagRun(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.on(WinDiag, func() {
		go func() {
			v := a.DiagView()
			if len(v.Rows) != 7 || !v.Rows[5].HasAction || v.Rows[0].HasAction {
				t.Errorf("diag view = %+v", v)
			}
			a.DiagRun(0)
			a.DiagRun(5)
			eventually(t, func() bool {
				v := a.DiagView()
				return !v.Busy && v.Rows[5].Status == string(backend.DiagOK)
			})
			a.CloseDialog(WinDiag)
		}()
	})
	a.ClickDiag()
	a.wait()
	if c := f.Calls(); !slices.Equal(c, []string{"RunDiagAction"}) {
		t.Errorf("calls = %v", c)
	}
	if !strings.Contains(a.StatusText(), en("DiagDone")) {
		t.Error("the journal has no DiagDone")
	}
}

func TestStrategyNoneReboot(t *testing.T) {
	a, f, sh := start(t, nil)
	sh.answer(true, true)
	a.StrategyNone()
	a.wait()
	if d := f.Data(); d.NetworkResets != 1 || d.Reboots != 1 {
		t.Errorf("resets %d, reboots %d", d.NetworkResets, d.Reboots)
	}
	f.FailNext("NetworkReset", errSample)
	sh.answer(true)
	a.StrategyNone()
	a.wait()
	want := en("StratNoneFail") + "\n\nnetsh winsock reset: FAIL"
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{want}) {
		t.Errorf("errors = %q", got)
	}
}

func TestStopClosesDialog(t *testing.T) {
	a, _, sh := start(t, nil)
	opened := make(chan struct{})
	sh.on(WinStatus, func() { close(opened) })
	a.ClickStatus()
	<-opened
	Stop(a)
	if got := sh.get(&sh.closed); !slices.Equal(got, []string{WinStatus}) {
		t.Errorf("closed = %v", got)
	}
}

func TestStartTwice(t *testing.T) {
	a, f, _ := start(t, nil)
	before := len(f.Calls())
	if err := Start(a); err != nil {
		t.Fatal(err)
	}
	a.wait()
	if n := len(f.Calls()); n != before {
		t.Errorf("the second Start made %d calls", n-before)
	}
}

func TestSetIPSetChangesFile(t *testing.T) {
	a, f, sh := start(t, nil)
	a.SetIPSet(slices.Index(backend.IPSetModes, backend.IPSetAny))
	a.wait()
	if m := f.Data().IPSet; m != backend.IPSetAny {
		t.Errorf("ipset = %q", m)
	}
	if v := a.Main(); v.IPSet != slices.Index(backend.IPSetModes, backend.IPSetAny) {
		t.Errorf("combo index = %d", v.IPSet)
	}
	if got := sh.get(&sh.infos); !slices.Equal(got, []string{en("FilterNeedRerun")}) {
		t.Errorf("infos = %q", got)
	}
	if slices.Contains(f.Calls(), "SaveSettings") {
		t.Error("SetIPSet saved the settings")
	}
	// With no backup, "loaded" fails and the combo box shows the file.
	f.Update(func(d *backend.FakeData) { d.IPSetBackup = false })
	a.SetIPSet(slices.Index(backend.IPSetModes, backend.IPSetLoaded))
	a.wait()
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{en("IpsetNoBackup")}) {
		t.Errorf("errors = %q", got)
	}
	if v := a.Main(); v.IPSet != slices.Index(backend.IPSetModes, backend.IPSetAny) {
		t.Errorf("combo index after the error = %d", v.IPSet)
	}
}

func TestStartSyncsTrayTask(t *testing.T) {
	_, f, _ := start(t, func(f *backend.Fake) {
		f.Update(func(d *backend.FakeData) { d.Settings.TrayWatch = true })
	})
	if !f.Data().TrayTask {
		t.Error("Start did not make the tray task")
	}
}

func TestTrayWatchTask(t *testing.T) {
	a, f, sh := start(t, nil)
	a.SetTrayWatch(true)
	a.wait()
	if d := f.Data(); !d.TrayTask || !d.Settings.TrayWatch {
		t.Errorf("task %v, setting %v", d.TrayTask, d.Settings.TrayWatch)
	}
	f.FailNext("SetTrayAtLogon", errSample)
	a.SetTrayWatch(false)
	a.wait()
	if d := f.Data(); !d.TrayTask || !d.Settings.TrayWatch {
		t.Errorf("after a fail: task %v, setting %v", d.TrayTask, d.Settings.TrayWatch)
	}
	if got := sh.get(&sh.errors); !slices.Equal(got, []string{en("TrayTaskFail", "sample")}) {
		t.Errorf("errors = %q", got)
	}
	if a.Main().TrayWatch != true {
		t.Error("the box changed after a fail")
	}
}

func TestForeignEngineAsk(t *testing.T) {
	foreign := func(f *backend.Fake) {
		f.Update(func(d *backend.FakeData) { d.Foreign = []string{"winws.exe (PID 7)"} })
	}
	a, f, sh := start(t, foreign)
	a.StrategyRun("general", string(spec.Winws))
	a.wait()
	if slices.Contains(f.Calls(), "Run") {
		t.Fatal("Run ran with the answer No")
	}
	want := en("EngineForeignAsk", "winws.exe (PID 7)")
	if got := sh.get(&sh.asks); !slices.Equal(got, []string{want}) {
		t.Errorf("asks = %q", got)
	}
	if !strings.Contains(a.StatusText(), en("EngineForeignKeep", "winws.exe (PID 7)")) {
		t.Error("the journal has no EngineForeignKeep")
	}
	a.StrategyInstall("general", string(spec.Winws))
	a.wait()
	if slices.Contains(f.Calls(), "Install") {
		t.Fatal("Install ran with the answer No")
	}
	sh.answer(true)
	a.StrategyInstall("general", string(spec.Winws))
	a.wait()
	if d := f.Data(); d.Status.Installed != "general" || len(d.Foreign) != 0 {
		t.Errorf("installed %q, foreign %v", d.Status.Installed, d.Foreign)
	}
	if len(sh.get(&sh.errors)) != 0 {
		t.Errorf("errors = %q", sh.get(&sh.errors))
	}
}
