package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// dialogExStyle is the extended style of a dialog. It does not have
// WS_EX_APPWINDOW, thus an owned dialog is not on the taskbar. It does not
// have WS_EX_TOOLWINDOW, which gives a small title bar and a small close
// button.
const dialogExStyle = w32.WS_EX_CONTROLPARENT

// setOwner makes owner the owner window of w. An owned window stays in front
// of its owner and is not on the taskbar.
func setOwner(w, owner *application.WebviewWindow) {
	if w == nil || owner == nil {
		return
	}
	hwnd, own := w.NativeWindow(), owner.NativeWindow()
	if hwnd == nil || own == nil {
		return
	}
	application.InvokeSync(func() {
		w32.SetWindowLongPtr(w32.HWND(hwnd), w32.GWLP_HWNDPARENT, uintptr(own))
	})
}
