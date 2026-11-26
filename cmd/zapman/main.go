// Command zapman is the GUI of Zapret Manager: the windows of the PS GUI and
// the tray icon in one process.
//
// The frontend is plain HTML, CSS and JS in frontend/, embedded in the
// binary. The Wails asset server gives these files and also the Wails
// runtime at /wails/runtime.js; the pages import it as an ES module.
//
// The backend is internal/app over the folder of zapman.exe (bin,
// strategies, lists, user). The flag -root sets an other folder. With
// "go run", the exe is in a temporary go-build folder, so the root is the
// working folder. The flag -fake gives the in-memory fake
// (backend.NewFake), which needs no admin rights and changes nothing.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/NONERUN/zapman/assets"
	"github.com/NONERUN/zapman/internal/app"
	"github.com/NONERUN/zapman/internal/engine"
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
	root := flag.String("root", "", "the folder with bin, strategies, lists and user (default: the folder of zapman.exe)")
	useFake := flag.Bool("fake", false, "use the in-memory fake backend: no admin rights, no changes to the system")
	flag.Parse()

	pages, err := fs.Sub(frontend, "frontend")
	if err != nil {
		return err
	}

	be, err := newBackend(*root, *useFake)
	if err != nil {
		return err
	}

	sh := &shell{}
	wapp := application.New(application.Options{
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
	ui := gui.New(be, sh, gui.Options{Culture: uiCulture()})
	sh.init(wapp, ui)
	wapp.RegisterService(application.NewService(ui))

	wapp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go sh.start(*tray)
	})
	return wapp.Run()
}

// newBackend gives the fake, or the real backend over the root folder.
func newBackend(root string, useFake bool) (backend.Backend, error) {
	if useFake {
		f := backend.NewFake()
		f.Delay = 300 * time.Millisecond
		return f, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if root == "" {
		if root, err = rootFolder(exe); err != nil {
			return nil, err
		}
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}
	o := app.Options{Layout: engine.NewLayout(root), AppVersion: appVersion()}
	// A bad exe path is an error of SetTrayAtLogon only.
	o.TrayCommand, _ = app.TrayCommand(exe)
	return app.New(o), nil
}

// rootFolder gives the folder of the exe. When the exe is in a temporary
// go-build folder ("go run"), it gives the working folder.
func rootFolder(exe string) (string, error) {
	if exe, err := filepath.EvalSymlinks(exe); err == nil {
		dir := filepath.Dir(exe)
		if !isGoBuildDir(dir) {
			return dir, nil
		}
	}
	return os.Getwd()
}

// isGoBuildDir tells if dir is in a temporary folder of "go run".
func isGoBuildDir(dir string) bool {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		tmp = os.TempDir()
	}
	rel, err := filepath.Rel(tmp, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	return strings.HasPrefix(strings.ToLower(rel), "go-build")
}

// appVersion gives the module version of the build. A build from a
// checkout gives "dev" with the short commit hash.
func appVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if rev == "" {
		return "dev"
	}
	return "dev-" + rev + dirty
}
