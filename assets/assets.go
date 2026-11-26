// Package assets holds the icon of the program for the Go code.
package assets

// The go:embed directive needs the embed package.
import _ "embed"

// AppIcon is the icon of the program (app.ico): the windows, the tray and
// the dialogs use it.
//
//go:embed app.ico
var AppIcon []byte
