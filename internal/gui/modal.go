package gui

import (
	"slices"
	"sync"
)

// Modal is the order of the open windows and of the message boxes. It is
// the only data that tells which window takes the input of the user. Wails
// has no modal windows, thus the shell sets the enabled state of each window
// from Enabled after each change. It does not change the state in a
// different place.
//
// The rule: when no message box is open, only the window in front is
// enabled. The window in front is the last dialog that opened, else the main
// window. When a message box is open, all windows are disabled. When the
// last dialog and the last message box close, the main window is enabled
// again.
//
// Modal is safe for concurrent use.
type Modal struct {
	mu    sync.Mutex
	stack []string
	boxes int
}

// Open puts the dialog in front. It does nothing for the main window and
// for a dialog that is open.
func (m *Modal) Open(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if name != WinMain && !slices.Contains(m.stack, name) {
		m.stack = append(m.stack, name)
	}
}

// Close removes the dialog. It does nothing for a window that is not open.
func (m *Modal) Close(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stack = slices.DeleteFunc(m.stack, func(n string) bool { return n == name })
}

// BoxOpen counts a message box that opens. Call BoxClose when it closes.
func (m *Modal) BoxOpen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.boxes++
}

// BoxClose counts a message box that closed.
func (m *Modal) BoxClose() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.boxes > 0 {
		m.boxes--
	}
}

// Top gives the window in front: the last dialog, else the main window.
func (m *Modal) Top() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.topLocked()
}

func (m *Modal) topLocked() string {
	if n := len(m.stack); n > 0 {
		return m.stack[n-1]
	}
	return WinMain
}

// Enabled tells if the window takes the input of the user.
func (m *Modal) Enabled(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.boxes == 0 && name == m.topLocked()
}

// Dialogs gives the open dialogs from the back to the front.
func (m *Modal) Dialogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.stack)
}
