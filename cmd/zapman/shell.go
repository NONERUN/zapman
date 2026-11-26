package main

import (
	"maps"
	"os"
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
// dialog is open, as ShowDialog of WPF does. modal has the order of the
// windows and sync is the only function that enables and disables them. A
// window that closes is destroyed, so the shell makes it again at the next
// open.
type shell struct {
	app *application.App
	ui  *gui.App

	// openMu puts the open and close operations in sequence.
	openMu sync.Mutex
	// syncMu puts the calls of sync in sequence.
	syncMu sync.Mutex
	modal  gui.Modal

	// mu keeps the fields below. Do not call Wails while you hold it.
	mu       sync.Mutex
	wins     map[string]*application.WebviewWindow
	appClose map[*application.WebviewWindow]bool
	quitting bool

	// iconMu puts the icon changes of the windows in sequence. icon is the
	// icon of the windows.
	iconMu sync.Mutex
	icon   gui.Icon

	trayMu   sync.Mutex
	tray     *application.SystemTray
	trayView gui.TrayView
	trayOn   bool
	trayIcon gui.Icon
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

// quitTimeout is the time limit of the Wails quit. After it the process
// ends. It is more than stopTimeout, thus shutdown can end first.
const quitTimeout = stopTimeout + 10*time.Second

// quit ends the program. Normally app.Quit ends the main loop and run
// returns. If the main loop does not end (for example a message box keeps
// it), the timer ends the process after quitTimeout. The winws of the
// foreground start is not a part of the process, thus it continues to run,
// as in the PS GUI.
func (s *shell) quit() {
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	time.AfterFunc(quitTimeout, func() { os.Exit(0) })
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
		// Not HiddenOnTaskbar: it makes a tool window with a small close
		// button. The owner window (setOwner) keeps the dialog off the taskbar.
		opt.Windows.ExStyle = dialogExStyle
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

// userClose asks the GUI logic if the window can close. For a dialog the
// GUI logic closes the window itself; CloseWindow here is for a window that
// the GUI logic does not know.
func (s *shell) userClose(name string) {
	if !gui.Closing(s.ui, name) {
		s.sync()
		return
	}
	if name == gui.WinMain {
		s.quit()
		return
	}
	s.CloseWindow(name)
}

// sync sets the enabled state of each window from modal. It is the only
// function that calls SetEnabled. It sets all windows, not only the windows
// that changed, thus a state that went wrong (for example after a message
// box) becomes correct again at the next call.
func (s *shell) sync() {
	if s.isQuitting() {
		return
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	s.mu.Lock()
	wins := maps.Clone(s.wins)
	s.mu.Unlock()
	for name, w := range wins {
		w.SetEnabled(s.modal.Enabled(name))
	}
}

// top gives the window in front: the last dialog, else the main window. It
// can be nil.
func (s *shell) top() *application.WebviewWindow {
	name := s.modal.Top()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wins[name]
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
	mainWin := s.wins[gui.WinMain]
	s.wins[name] = w
	s.mu.Unlock()
	s.modal.Open(name)
	s.sync()
	top := s.top()

	w.SetTitle(title)
	w.Show()
	// The owner of each dialog is the main window, not the dialog below:
	// a dialog below can close or hide, and a hidden owner does not bring
	// its dialogs to the front. The native window can be made at Show, thus
	// set the owner after it.
	if name != gui.WinMain {
		setOwner(w, mainWin)
	}
	// The native window is made now: give it the icon of now.
	s.iconMu.Lock()
	setWindowIcon(w, s.icon)
	s.iconMu.Unlock()
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
	s.mu.Unlock()
	s.modal.Close(name)
	// The window in front is enabled before the dialog closes, thus Windows
	// gives the activation to it.
	s.sync()
	top := s.top()

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
	w := s.top()
	if w != nil && w.IsVisible() {
		d.AttachToWindow(w)
	}
	return d.SetTitle(dialogTitle)
}

// box counts a message box in modal. Call the function that it gives when
// the box closes. The box disables its owner and enables it again itself;
// a sync while the box is open keeps the owner disabled. When the box
// closes, sync sets all windows again: the box can enable a window that
// must stay disabled, or not enable a window that sync disabled.
func (s *shell) box() func() {
	s.modal.BoxOpen()
	return func() {
		s.modal.BoxClose()
		s.sync()
	}
}

// Ask implements gui.Shell.
func (s *shell) Ask(text string, defaultYes bool) bool {
	if s.isQuitting() {
		return false
	}
	defer s.box()()
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
	defer s.box()()
	s.owner(s.app.Dialog.Info()).SetMessage(text).Show()
}

// Error implements gui.Shell.
func (s *shell) Error(text string) {
	if s.isQuitting() {
		return
	}
	defer s.box()()
	s.owner(s.app.Dialog.Error()).SetMessage(text).Show()
}

// OpenURL implements gui.Shell.
func (s *shell) OpenURL(url string) error {
	return s.app.Browser.OpenURL(url)
}

// iconData gives the ICO file of the icon.
func iconData(icon gui.Icon) []byte {
	if icon == gui.IconLive {
		return assets.AppIcon
	}
	return assets.IdleIcon
}

// SetIcon implements gui.Shell. It sets the icon of the tray and of the
// windows that are open. OpenWindow gives the icon to a window that opens
// later.
func (s *shell) SetIcon(icon gui.Icon) {
	if s.isQuitting() {
		return
	}
	s.trayMu.Lock()
	s.trayIcon = icon
	if s.tray != nil {
		s.tray.SetIcon(iconData(icon))
	}
	s.trayMu.Unlock()

	s.iconMu.Lock()
	defer s.iconMu.Unlock()
	s.icon = icon
	s.mu.Lock()
	wins := maps.Clone(s.wins)
	s.mu.Unlock()
	for _, w := range wins {
		setWindowIcon(w, icon)
	}
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
	m.AddSeparator()
	// Exit ends the program also when the main window hides in the tray.
	// The winws of the foreground start continues to run.
	m.Add(v.Exit).OnClick(func(*application.Context) { go s.quit() })
	s.tray.SetMenu(m)
	s.tray.SetTooltip(v.Tooltip)
}
