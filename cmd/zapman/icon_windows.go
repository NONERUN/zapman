package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"

	"github.com/NONERUN/zapman/internal/gui"
)

// winIcon is the small and the big HICON of an icon.
type winIcon struct {
	small, big w32.HICON
}

// winIcons keeps the HICONs of each icon. There are only four icons, and the
// program keeps the handles until it ends, thus it does not destroy them.
var (
	winIconsMu sync.Mutex
	winIcons   = map[gui.Icon]winIcon{}
)

// hicons gives the HICONs of the icon. It makes them at the first call.
func hicons(icon gui.Icon) winIcon {
	winIconsMu.Lock()
	defer winIconsMu.Unlock()
	if ic, ok := winIcons[icon]; ok {
		return ic
	}
	data := iconData(icon)
	var ic winIcon
	ic.small, _ = w32.CreateSmallHIconFromImage(data)
	ic.big, _ = w32.CreateLargeHIconFromImage(data)
	winIcons[icon] = ic
	return ic
}

// setWindowIcon sets the title bar and taskbar icon of w. Wails sets the
// icon only when it makes the window, thus send WM_SETICON. Do nothing when
// the native window is not made yet.
func setWindowIcon(w *application.WebviewWindow, icon gui.Icon) {
	if w == nil {
		return
	}
	hwnd := w.NativeWindow()
	if hwnd == nil {
		return
	}
	ic := hicons(icon)
	application.InvokeSync(func() {
		if ic.small != 0 {
			w32.SendMessage(w32.HWND(hwnd), w32.WM_SETICON, w32.ICON_SMALL, ic.small)
		}
		if ic.big != 0 {
			w32.SendMessage(w32.HWND(hwnd), w32.WM_SETICON, w32.ICON_BIG, ic.big)
		}
	})
}
