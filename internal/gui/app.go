// Package gui is the logic of the windows of the GUI. It does not use Wails:
// cmd/zapman binds App to the Wails runtime and implements Shell.
//
// The frontend calls the exported methods of App. The methods return
// quickly. A method that must show a dialog or call the backend starts a
// flow on a new goroutine. When the state changes, App sends an event with
// no data; the frontend then reads the state with a view method.
package gui

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/gui/i18n"
	"github.com/NONERUN/zapman/internal/spec"
)

// The window names. The frontend has the page <name>.html for each window.
const (
	WinMain       = "main"
	WinStrategy   = "strategy"
	WinStatus     = "status"
	WinTests      = "tests"
	WinTestsSetup = "testssetup"
	WinHosts      = "hosts"
	WinFake       = "fake"
	WinGames      = "games"
	WinDownload   = "download"
	WinDiag       = "diag"
)

// The events. They have no data.
const (
	// EvLang: the UI language changed. All windows read the strings again.
	EvLang = "lang"
	// EvMain: the state of the main window changed.
	EvMain = "main"
	// EvStrategy: the marks of the strategy list changed.
	EvStrategy = "strategy"
	// EvTests: the state of the tests window changed.
	EvTests = "tests"
	// EvDownload: the state of the download window changed.
	EvDownload = "download"
	// EvDiag: the diagnostics items changed.
	EvDiag = "diag"
)

// Shell is the window system. All methods are safe for concurrent use.
type Shell interface {
	// OpenWindow shows the window with the title. A dialog window opens on
	// top of the windows that are open and blocks them until it closes.
	// For the main window, OpenWindow shows it and puts it in front.
	OpenWindow(name, title string)
	// CloseWindow closes the window. It does not call App.WindowClosing.
	CloseWindow(name string)
	// HideWindow hides the window and keeps it.
	HideWindow(name string)
	// SetTitle sets the title of an open window.
	SetTitle(name, title string)
	// Emit sends the event to all windows.
	Emit(event string)
	// Ask shows a Yes / No question and waits for the answer.
	Ask(text string, defaultYes bool) bool
	// Info shows a message and waits until the user closes it.
	Info(text string)
	// Error shows an error and waits until the user closes it.
	Error(text string)
	// OpenURL opens the address in the browser.
	OpenURL(url string) error
	// SetTray sets the tray icon and its menu. visible false removes the icon.
	SetTray(v TrayView, visible bool)
	// SetIcon sets the icon of the tray and of all windows, also of the
	// windows that open later.
	SetIcon(icon Icon)
}

// Options are the options of New.
type Options struct {
	// Culture is the Windows UI culture, for example "ru-RU". It selects the
	// language when the settings have none.
	Culture string
	// Tick is the time between two status refreshes. Zero is 2 s.
	Tick time.Duration
	// VersionTimeout is the time limit of the version check. Zero is 8 s.
	VersionTimeout time.Duration
}

// App is the state of the GUI.
type App struct {
	be    backend.Backend
	shell Shell
	opt   Options

	ctx    context.Context
	cancel context.CancelFunc
	// wg counts the flows and the tasks; tick counts the timer.
	wg   sync.WaitGroup
	tick sync.WaitGroup

	// iconMu puts the calls of syncIcon in sequence. icon is the last icon
	// that went to the shell; iconSet is false before the first one.
	iconMu  sync.Mutex
	icon    Icon
	iconSet bool

	mu         sync.Mutex
	started    bool
	lang       string
	settings   backend.Settings
	configErr  string
	status     backend.Status
	ipset      backend.IPSetMode
	strategies []backend.StrategyInfo
	games      []backend.GameInfo
	lastError  string
	journal    journal
	busy       bool
	flow       bool
	versionRun bool
	refreshErr bool

	dialogs  map[string]*dialog
	stratDlg stratDialog
	setup    setupDialog
	tests    testsState
	dl       downloadState
	fakes    fakesDialog
	diag     diagDialog
	hosts    backend.HostsCheck
}

// New gives an App. Call Start before the first window opens.
func New(be backend.Backend, shell Shell, opt Options) *App {
	if opt.Tick <= 0 {
		opt.Tick = 2 * time.Second
	}
	if opt.VersionTimeout <= 0 {
		opt.VersionTimeout = 8 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		be: be, shell: shell, opt: opt,
		ctx: ctx, cancel: cancel,
		lang:    i18n.Pick("", opt.Culture),
		dialogs: map[string]*dialog{},
	}
}

// Start reads the settings and the catalog, gets the status and starts the
// status timer and the auto version check. It returns an error only when
// the catalog cannot be read. A second call does nothing.
//
// Start, Stop and Closing are functions, not methods: Wails binds all
// exported methods of App to the frontend, and the frontend must not call
// them.
func Start(a *App) error {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}
	a.started = true
	a.mu.Unlock()
	s, err := a.be.LoadSettings(a.ctx)
	var cfgErr *backend.ConfigError
	switch {
	case errors.As(err, &cfgErr):
		a.mu.Lock()
		a.configErr = i18n.T(i18n.Pick(s.Language, a.opt.Culture), "ConfigParseFail")
		a.mu.Unlock()
	case err != nil:
		return err
	}
	strats, err := a.be.Strategies(a.ctx)
	if err != nil {
		return err
	}
	games, err := a.be.Games(a.ctx)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.lang = i18n.Pick(s.Language, a.opt.Culture)
	a.strategies = strats
	a.games = games
	cfg := a.configErr
	a.mu.Unlock()
	if cfg != "" {
		a.logLine(cfg)
	}
	a.refresh()
	a.tick.Add(1)
	go a.timer()
	// As Sync-ZapmanTrayWatch at the start of the PS GUI: the logon task
	// follows the Tray icon box. An error goes only to the journal.
	trayOn := s.TrayWatch
	a.goTask(func() {
		if err := a.be.SetTrayAtLogon(a.ctx, trayOn); err != nil {
			a.logLine(a.t("TrayTaskFail", a.errText(err)))
		}
	})
	if s.AutoUpdateCheck {
		a.goTask(func() { a.versionCheck(false) })
	}
	return nil
}

// Stop ends the timer and the flows. It cancels the long actions and waits
// for the goroutines.
func Stop(a *App) {
	a.cancel()
	a.wg.Wait()
	a.tick.Wait()
}

// wait waits until the flows and the tasks end. It does not wait for the
// timer. It is for the tests.
func (a *App) wait() { a.wg.Wait() }

func (a *App) timer() {
	defer a.tick.Done()
	t := time.NewTicker(a.opt.Tick)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			a.mu.Lock()
			busy := a.busy
			a.mu.Unlock()
			if !busy {
				a.refresh()
			}
		}
	}
}

// t gives the string of the key in the UI language.
func (a *App) t(key string, args ...any) string {
	a.mu.Lock()
	lang := a.lang
	a.mu.Unlock()
	return i18n.T(lang, key, args...)
}

// tLocked is t for a caller that holds a.mu.
func (a *App) tLocked(key string, args ...any) string {
	return i18n.T(a.lang, key, args...)
}

// errText gives the text of an error in the UI language.
func (a *App) errText(err error) string {
	var m *backend.MsgError
	if errors.As(err, &m) {
		return a.t(m.Key, m.Args...)
	}
	return err.Error()
}

func (a *App) logLine(msg string) {
	a.mu.Lock()
	a.journal.add(time.Now(), msg)
	a.mu.Unlock()
}

func (a *App) emit(ev string) { a.shell.Emit(ev) }

// refresh gets the status and the IPSet Filter mode and updates the main
// window and the tray. When the mode cannot be read, the old mode stays.
// It always sends EvMain: the main window must show the busy state of now
// also when the status cannot be read.
func (a *App) refresh() {
	st, err := a.be.Status(a.ctx)
	mode, ipErr := a.be.IPSet(a.ctx)
	a.mu.Lock()
	if err != nil {
		warn := !a.refreshErr
		a.refreshErr = true
		a.mu.Unlock()
		if warn {
			a.logLine(a.t("StatusRefreshFail") + " " + a.errText(err))
		}
		a.emit(EvMain)
		return
	}
	a.status = st
	if ipErr == nil {
		a.ipset = mode
	}
	lang := a.lang
	tray := a.settings.TrayWatch
	a.mu.Unlock()
	a.syncIcon()
	a.shell.SetTray(trayView(lang, st), tray)
	a.emit(EvMain)
}

// syncIcon sends the icon of the last status to the shell when it is not
// the icon that the shell has. Thus the icon does not flicker. Two refreshes
// can run at the same time: iconMu puts the calls in sequence, and each call
// reads the status of now, thus the last call sends the icon of the last
// status.
func (a *App) syncIcon() {
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	icon := iconOf(a.statusNow())
	if a.iconSet && icon == a.icon {
		return
	}
	a.shell.SetIcon(icon)
	a.icon, a.iconSet = icon, true
}

// goTask runs fn on a new goroutine.
func (a *App) goTask(fn func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		fn()
	}()
}

// goFlow runs fn on a new goroutine when no other flow of the main window
// runs. In the PS GUI the dialogs are modal, so two flows cannot run at the
// same time.
func (a *App) goFlow(fn func()) {
	a.mu.Lock()
	if a.flow {
		a.mu.Unlock()
		return
	}
	a.flow = true
	a.mu.Unlock()
	a.goTask(func() {
		defer func() {
			a.mu.Lock()
			a.flow = false
			a.mu.Unlock()
		}()
		fn()
	})
}

// action runs fn as Invoke-GuiAction of the PS GUI does: it writes the busy
// text to the journal, blocks the controls of the main window, and on an
// error keeps the error as the last error and shows it. It returns true
// when fn did not fail. When an action runs, action does not run fn.
func (a *App) action(busyKey string, fn func(ctx context.Context) error) bool {
	busyText := a.t(busyKey)
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		a.logLine(busyText)
		return false
	}
	a.busy = true
	a.mu.Unlock()
	a.emit(EvMain)
	a.logLine(busyText)
	err := fn(a.ctx)
	if err != nil {
		text := a.errText(err)
		a.mu.Lock()
		a.lastError = text
		a.mu.Unlock()
		a.shell.Error(text)
		a.logLine(text)
	}
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
	a.refresh()
	return err == nil
}

// fail shows the error and refreshes the status, as the catch blocks of the
// PS handlers do.
func (a *App) fail(err error) {
	a.shell.Error(a.errText(err))
	a.refresh()
}

// saveSettings changes the settings and writes them. On an error the
// settings in memory do not change.
func (a *App) saveSettings(fn func(s *backend.Settings)) error {
	a.mu.Lock()
	s := a.settings
	s.Games = slices.Clone(s.Games)
	s.GameOptions = backend.CloneGameOptions(s.GameOptions)
	a.mu.Unlock()
	fn(&s)
	if err := a.be.SaveSettings(a.ctx, s); err != nil {
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.configErr = ""
	a.mu.Unlock()
	return nil
}

// reloadSettings reads the settings again after a backend call that writes
// them (Run and Install keep the strategy). On an error it writes the error
// to the journal and keeps the settings in memory.
func (a *App) reloadSettings() {
	s, err := a.be.LoadSettings(a.ctx)
	if err != nil {
		a.logLine(a.errText(err))
		return
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
}

func (a *App) statusNow() backend.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

func (a *App) engine() spec.Engine {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settings.Engine == "" {
		return spec.Winws
	}
	return a.settings.Engine
}

func (a *App) hasService() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status.HasService()
}

// clearLastError clears the last error after an action that worked.
func (a *App) clearLastError() {
	a.mu.Lock()
	a.lastError = ""
	a.mu.Unlock()
}

// Strings gives the UI strings in the UI language.
func (a *App) Strings() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return i18n.Table(a.lang)
}

// Lang gives the UI language.
func (a *App) Lang() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lang
}
