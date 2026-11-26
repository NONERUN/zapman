// Package assets holds the icons of the program for the Go code.
package assets

// The go:embed directive needs the embed package.
import _ "embed"

//go:generate go run ../dev/grayicon -in app.ico -out app-idle.ico

// AppIcon is the blue icon of the program (app.ico). The exe file has it.
// The windows and the tray show it while the bypass works.
//
//go:embed app.ico
var AppIcon []byte

// IdleIcon is the gray icon (app-idle.ico): AppIcon with gray pixels. The
// windows and the tray show it while the bypass does not work. go generate
// makes it from app.ico.
//
//go:embed app-idle.ico
var IdleIcon []byte
