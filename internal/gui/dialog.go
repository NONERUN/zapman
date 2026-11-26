package gui

// dialog is an open dialog window. A flow opens it and waits for its
// result, as ShowDialog of the PS GUI does.
type dialog struct {
	done   chan struct{}
	result any
}

// openDialog opens the window and gives the dialog. When the window is
// open already, it gives nil.
func (a *App) openDialog(name, titleKey string) *dialog {
	a.mu.Lock()
	if _, ok := a.dialogs[name]; ok {
		a.mu.Unlock()
		return nil
	}
	d := &dialog{done: make(chan struct{})}
	a.dialogs[name] = d
	a.mu.Unlock()
	a.shell.OpenWindow(name, a.t(titleKey))
	return d
}

// showDialog opens the window and waits until it closes. It gives the
// result of the dialog, or nil when the user closed the window. When the
// App stops, it closes the window.
func (a *App) showDialog(name, titleKey string) any {
	d := a.openDialog(name, titleKey)
	if d == nil {
		return nil
	}
	return a.waitDialog(name, d)
}

func (a *App) waitDialog(name string, d *dialog) any {
	select {
	case <-d.done:
	case <-a.ctx.Done():
		a.closeDialog(name, nil)
		<-d.done
	}
	return d.result
}

// dropDialog removes the dialog and wakes its flow. It gives false when the
// dialog is not open.
func (a *App) dropDialog(name string, result any) bool {
	a.mu.Lock()
	d, ok := a.dialogs[name]
	if ok {
		delete(a.dialogs, name)
	}
	a.mu.Unlock()
	if !ok {
		return false
	}
	d.result = result
	close(d.done)
	return true
}

// closeDialog closes the window of the dialog with the result.
func (a *App) closeDialog(name string, result any) {
	if a.dropDialog(name, result) {
		a.shell.CloseWindow(name)
	}
}

// WindowClosing is called when the user closes a window. It gives false
// when the window must stay open.
//
// The main window hides when the tray icon is on. The tests window and the
// download window stay open while their work runs: the close cancels the
// work, as in the PS GUI.
func (a *App) WindowClosing(name string) bool {
	switch name {
	case WinMain:
		a.mu.Lock()
		tray := a.settings.TrayWatch
		a.mu.Unlock()
		if tray {
			a.shell.HideWindow(WinMain)
			return false
		}
		return true
	case WinTests:
		if a.testsCancel() {
			return false
		}
	case WinDownload:
		if a.downloadCancel() {
			return false
		}
	}
	a.dropDialog(name, nil)
	return true
}

// CloseDialog closes a dialog window with no result, as a Cancel or Close
// button does.
func (a *App) CloseDialog(name string) {
	switch name {
	case WinTests:
		if a.testsCancel() {
			return
		}
	case WinDownload:
		if a.downloadCancel() {
			return
		}
	}
	a.closeDialog(name, nil)
}
