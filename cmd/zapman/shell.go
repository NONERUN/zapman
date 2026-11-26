package main

import (
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/NONERUN/zapman/assets"
	"github.com/NONERUN/zapman/internal/gui"
	"github.com/NONERUN/zapman/internal/gui/i18n"
)

// winSpec is the size of a window. The sizes are the outer sizes of the
// windows of the PS GUI (the Width and Height of the XAML).
type winSpec struct {
	w, h       int
	minW, minH int
	resize     bool
}

var winSpecs = map[string]winSpec{
	gui.WinMain:       {w: 416, h: 604},
	gui.WinStrategy:   {w: 436, h: 394},
	gui.WinStatus:     {w: 536, h: 458},
	gui.WinTests:      {w: 780, h: 860, minW: 640, minH: 640, resize: true},
	gui.WinTestsSetup: {w: 456, h: 458},
	gui.WinHosts:      {w: 436, h: 228},
	gui.WinFake:       {w: 480, h: 234},
	gui.WinGames:      {w: 480, h: 360},
	gui.WinDownload:   {w: 436, h: 156},
	gui.WinDiag:       {w: 656, h: 758, minW: 496, minH: 518, resize: true},
}

// dialogTitle is the title of the message boxes, as in the PS GUI.
const dialogTitle = "Zapret"

// stopTimeout is the time limit of gui.Stop at the end of the program.
const stopTimeout = 5 * time.Second

// shell implements gui.Shell with Wails windows, message boxes and the tray
// icon.
//
// Wails has no modal windows. The shell disables the other windows while a
// dialog is open, as ShowDialog of WPF does. A window that closes is
// destroyed, so the shell makes it again at the next open.
type shell struct {
	app *application.App
	ui  *gui.App

	// openMu puts the open and close operations in sequence.
	openMu sync.Mutex

	// mu keeps the fields below. Do not call Wails while you hold it.
	mu       sync.Mutex
	wins     map[string]*application.WebviewWindow
	stack    []string
	appClose map[*application.WebviewWindow]bool
	quitting bool

	trayMu   sync.Mutex
	tray     *application.SystemTray
	trayView gui.TrayView
	trayOn   bool
}

func (s *shell) init(app *application.App, ui *gui.App) {
	s.app, s.ui = app, ui
	s.wins = map[string]*application.WebviewWindow{}
	s.appClose = map[*application.WebviewWindow]bool{}
}

// start starts the GUI logic and shows the main window. With tray true the
// main window stays hidden when the tray icon is on.
func (s *shell) start(tray bool) {
	if err := gui.Start(s.ui); err != nil {
		s.Error(err.Error())
		s.quit()
		return
	}
	if !tray || !s.ui.Main().TrayWatch {
		s.openMain()
	}
}

func (s *shell) openMain() { go s.ui.TrayOpen() }

// quit ends the program.
func (s *shell) quit() {
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	s.app.Quit()
}

// shutdown stops the GUI logic. After it the shell does nothing.
func (s *shell) shutdown() {
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		gui.Stop(s.ui)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(stopTimeout):
	}
}

func (s *shell) isQuitting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// pageURL gives the page of the window. The asset server of Wails needs an
// index.html, so the main window has index.html.
func pageURL(name string) string {
	if name == gui.WinMain {
		return "/index.html"
	}
	return "/" + name + ".html"
}

// newWindow makes the window. The window is hidden.
func (s *shell) newWindow(name, title string) *application.WebviewWindow {
	sp := winSpecs[name]
	opt := application.WebviewWindowOptions{
		Name:                       name,
		Title:                      title,
		Width:                      sp.w,
		Height:                     sp.h,
		MinWidth:                   sp.minW,
		MinHeight:                  sp.minH,
		DisableResize:              !sp.resize,
		URL:                        pageURL(name),
		Hidden:                     true,
		InitialPosition:            application.WindowCentered,
		BackgroundColour:           application.NewRGB(240, 240, 240),
		DefaultContextMenuDisabled: true,
		MaximiseButtonState:        application.ButtonDisabled,
	}
	if name != gui.WinMain {
		opt.MinimiseButtonState = application.ButtonHidden
		opt.Windows.HiddenOnTaskbar = true
		if sp.resize {
			opt.MaximiseButtonState = application.ButtonEnabled
		} else {
			opt.MaximiseButtonState = application.ButtonHidden
		}
	}
	w := s.app.Window.NewWithOptions(opt)
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.mu.Lock()
		own := s.appClose[w]
		delete(s.appClose, w)
		quit := s.quitting
		s.mu.Unlock()
		if own || quit {
			return
		}
		// The user closes the window. The GUI logic can block, so it runs
		// on a new goroutine, and the shell closes the window later.
		e.Cancel()
		go s.userClose(name)
	})
	return w
}

// userClose asks the GUI logic if the window can close.
func (s *shell) userClose(name string) {
	if !gui.Closing(s.ui, name) {
		return
	}
	if name == gui.WinMain {
		s.quit()
		return
	}
	s.CloseWindow(name)
}

// top gives the window in front: the last dialog, else the main window.
func (s *shell) topLocked() *application.WebviewWindow {
	if n := len(s.stack); n > 0 {
		return s.wins[s.stack[n-1]]
	}
	return s.wins[gui.WinMain]
}

// OpenWindow implements gui.Shell.
func (s *shell) OpenWindow(name, title string) {
	if _, ok := winSpecs[name]; !ok || s.isQuitting() {
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()

	s.mu.Lock()
	w := s.wins[name]
	s.mu.Unlock()
	if w == nil {
		w = s.newWindow(name, title)
	}

	s.mu.Lock()
	s.wins[name] = w
	var block []*application.WebviewWindow
	if name != gui.WinMain && !slices.Contains(s.stack, name) {
		for n, o := range s.wins {
			if n != name {
				block = append(block, o)
			}
		}
		s.stack = append(s.stack, name)
	}
	top := s.topLocked()
	s.mu.Unlock()

	for _, o := range block {
		o.SetEnabled(false)
	}
	w.SetTitle(title)
	w.Show()
	if w.IsMinimised() {
		w.UnMinimise()
	}
	w.Focus()
	if top != nil && top != w {
		// A dialog is open over the main window: put it in front.
		top.Focus()
	}
}

// CloseWindow implements gui.Shell.
func (s *shell) CloseWindow(name string) {
	if s.isQuitting() {
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()

	s.mu.Lock()
	w := s.wins[name]
	if w == nil {
		s.mu.Unlock()
		return
	}
	delete(s.wins, name)
	s.appClose[w] = true
	s.stack = slices.DeleteFunc(s.stack, func(n string) bool { return n == name })
	top := s.topLocked()
	s.mu.Unlock()

	if top != nil {
		top.SetEnabled(true)
	}
	w.Close()
	if top != nil {
		top.Focus()
	}
}

// HideWindow implements gui.Shell.
func (s *shell) HideWindow(name string) {
	if s.isQuitting() {
		return
	}
	s.mu.Lock()
	w := s.wins[name]
	s.mu.Unlock()
	if w != nil {
		w.Hide()
	}
}

// SetTitle implements gui.Shell.
func (s *shell) SetTitle(name, title string) {
	if s.isQuitting() {
		return
	}
	s.mu.Lock()
	w := s.wins[name]
	s.mu.Unlock()
	if w != nil {
		w.SetTitle(title)
	}
}

// Emit implements gui.Shell.
func (s *shell) Emit(event string) {
	if s.isQuitting() {
		return
	}
	s.app.Event.Emit(event)
}

// owner gives the window of a message box: the window in front when it is
// visible.
func (s *shell) owner(d *application.MessageDialog) *application.MessageDialog {
	s.mu.Lock()
	w := s.topLocked()
	s.mu.Unlock()
	if w != nil && w.IsVisible() {
		d.AttachToWindow(w)
	}
	return d.SetTitle(dialogTitle)
}

// Ask implements gui.Shell.
func (s *shell) Ask(text string, defaultYes bool) bool {
	if s.isQuitting() {
		return false
	}
	lang := s.ui.Lang()
	d := s.owner(s.app.Dialog.Question()).SetMessage(text)
	yes := false
	by := d.AddButton(i18n.T(lang, "BtnYes")).OnClick(func() { yes = true })
	bn := d.AddButton(i18n.T(lang, "BtnNo")).SetAsCancel()
	if defaultYes {
		by.SetAsDefault()
	} else {
		bn.SetAsDefault()
	}
	d.Show()
	return yes
}

// Info implements gui.Shell.
func (s *shell) Info(text string) {
	if s.isQuitting() {
		return
	}
	s.owner(s.app.Dialog.Info()).SetMessage(text).Show()
}

// Error implements gui.Shell.
func (s *shell) Error(text string) {
	if s.isQuitting() {
		return
	}
	s.owner(s.app.Dialog.Error()).SetMessage(text).Show()
}

// OpenURL implements gui.Shell.
func (s *shell) OpenURL(url string) error {
	return s.app.Browser.OpenURL(url)
}

// SetTray implements gui.Shell. The icon is made at the first call with
// visible true. The menu is made again only when the view changes.
func (s *shell) SetTray(v gui.TrayView, visible bool) {
	if s.isQuitting() {
		return
	}
	s.trayMu.Lock()
	defer s.trayMu.Unlock()
	if s.tray == nil {
		if !visible {
			return
		}
		t := s.app.SystemTray.New()
		t.SetIcon(assets.AppIcon)
		// As the NotifyIcon of the PS tray: a double click opens the main
		// window, a right click opens the menu, a left click does nothing.
		t.OnClick(func() {})
		t.OnRightClick(t.OpenMenu)
		t.OnDoubleClick(s.openMain)
		s.tray = t
		s.trayOn = true
	} else if visible != s.trayOn {
		if visible {
			s.tray.Show()
		} else {
			s.tray.Hide()
		}
		s.trayOn = visible
	}
	if !visible || v == s.trayView {
		return
	}
	s.trayView = v
	m := s.app.NewMenu()
	for _, l := range v.Lines {
		m.Add(l).SetEnabled(false)
	}
	m.AddSeparator()
	m.Add(v.Open).OnClick(func(*application.Context) { s.openMain() })
	m.Add(v.Start).SetEnabled(v.StartEnabled).OnClick(func(*application.Context) { go s.ui.TrayStart() })
	m.Add(v.Stop).SetEnabled(v.StopEnabled).OnClick(func(*application.Context) { go s.ui.TrayStop() })
	s.tray.SetMenu(m)
	s.tray.SetTooltip(v.Tooltip)
}
