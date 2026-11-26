// Command zapman is the GUI of Zapret Manager: the windows of the PS GUI and
// the tray icon in one process.
//
// The frontend is plain HTML, CSS and JS in frontend/, embedded in the
// binary. The Wails asset server gives these files and also the Wails
// runtime at /wails/runtime.js; the pages import it as an ES module.
//
// The backend is the in-memory fake (backend.NewFake) until the engine,
// config and service packages of branch go are ready.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/NONERUN/zapman/assets"
	"github.com/NONERUN/zapman/internal/gui"
	"github.com/NONERUN/zapman/internal/gui/backend"
)

//go:embed all:frontend
var frontend embed.FS

// trayFlag is the flag of a start in the tray: the main window stays
// hidden, as the tray task of the PS version does.
const trayFlag = "--tray"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "zapman:", err)
		os.Exit(1)
	}
}

func run() error {
	tray := flag.Bool(trayFlag[2:], false, "start in the tray, with the main window hidden")
	flag.Parse()

	pages, err := fs.Sub(frontend, "frontend")
	if err != nil {
		return err
	}

	fake := backend.NewFake()
	fake.Delay = 300 * time.Millisecond

	sh := &shell{}
	app := application.New(application.Options{
		Name:        "Zapret Manager",
		Description: "Zapret Manager",
		Icon:        assets.AppIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(pages),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.github.nonerun.zapman",
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				if !slices.Contains(d.Args, trayFlag) {
					sh.openMain()
				}
			},
		},
		Windows: application.WindowsOptions{
			// The tray icon keeps the program on when the main window hides.
			DisableQuitOnLastWindowClosed: true,
		},
		OnShutdown: sh.shutdown,
	})
	ui := gui.New(fake, sh, gui.Options{Culture: uiCulture()})
	sh.init(app, ui)
	app.RegisterService(application.NewService(ui))

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go sh.start(*tray)
	})
	return app.Run()
}
