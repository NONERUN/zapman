// Package app is the real backend of the GUI. It implements
// backend.Backend over internal/config, internal/engine, internal/service
// and internal/spec.
//
// The package is not in the tree of internal/gui: the GUI packages have no
// system code, and the backend package keeps only the interface and the
// fake. cmd/zapman makes the Backend and gives it to the GUI. An other
// front end (for example a CLI) can use the same Backend.
//
// All calls to the SCM, the registry, the processes and schtasks go through
// interfaces of Options. The tests use fakes for them.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/spec"
)

// ReleasesURL is the page of the releases.
const ReleasesURL = "https://github.com/NONERUN/zapman/releases"

// divertSys is the driver file of WinDivert in bin.
const divertSys = "WinDivert64.sys"

// Options are the parts of a Backend. A nil field gets the real part of
// Windows.
type Options struct {
	// Layout is the zapman folder.
	Layout engine.Layout
	// AppVersion is the tag of the build, for example v1.2.0.
	AppVersion string
	// Manager does the service operations. Nil is a Manager with
	// service.NewHost.
	Manager *service.Manager
	// Runner runs the engine in the foreground. Nil is the zero Runner.
	Runner *engine.Runner
	// Engines gives the winws.exe and winws2.exe processes. Nil is
	// service.EngineProcesses.
	Engines func() ([]service.EngineProcess, error)
	// Tray makes and removes the logon task of the tray. Nil is Schtasks.
	Tray TrayTask
	// TrayCommand is the command of the logon task. Empty is
	// TrayCommand(os.Executable()).
	TrayCommand string
	// Tools does the actions of internal/tools. Nil is NotWired.
	Tools Tools
}

// Backend is the real backend.Backend.
type Backend struct {
	l       engine.Layout
	version string
	mgr     *service.Manager
	runner  *engine.Runner
	engines func() ([]service.EngineProcess, error)
	tray    TrayTask
	trayCmd string
	tools   Tools

	// op makes the operations that change the engine or the service run
	// one at a time.
	op sync.Mutex
	// cfg makes the changes of user\config.json run one at a time.
	cfg sync.Mutex
	// mu protects run, runName and runEngine.
	mu        sync.Mutex
	run       *engine.Run
	runName   string
	runEngine spec.Engine
}

var _ backend.Backend = (*Backend)(nil)

// New gives a Backend.
func New(o Options) *Backend {
	b := &Backend{
		l: o.Layout, version: o.AppVersion,
		mgr: o.Manager, runner: o.Runner, engines: o.Engines,
		tray: o.Tray, trayCmd: o.TrayCommand, tools: o.Tools,
	}
	if b.mgr == nil {
		b.mgr = &service.Manager{Host: service.NewHost()}
	}
	if b.runner == nil {
		b.runner = &engine.Runner{}
	}
	if b.engines == nil {
		b.engines = service.EngineProcesses
	}
	if b.tray == nil {
		b.tray = Schtasks{}
	}
	if b.tools == nil {
		b.tools = NotWired{}
	}
	return b
}

// mapErr gives the UI text of the errors that the GUI knows.
func mapErr(err error) error {
	if errors.Is(err, service.ErrNotElevated) {
		return backend.Msg("AdminRequired")
	}
	return err
}

// mapErrKey is mapErr, but an other error gets the UI text key with the
// error text as {0}.
func mapErrKey(err error, key string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrNotElevated) {
		return backend.Msg("AdminRequired")
	}
	return backend.Msg(key, err.Error())
}

// Version implements backend.Backend.
func (b *Backend) Version() backend.Version {
	v := b.version
	if v == "" {
		v = "dev"
	}
	return backend.Version{App: v, Spec: strconv.Itoa(spec.Version), ReleasesURL: ReleasesURL}
}

// loadConfig reads user\config.json. On an error it gives the defaults
// (or the fields that it could read) and the error.
func (b *Backend) loadConfig() (config.Config, error) {
	return config.Load(b.l.ConfigPath())
}

func toSettings(c *config.Config) backend.Settings {
	return backend.Settings{
		Language:        c.Language,
		Engine:          c.Engine,
		Strategy:        c.Strategy,
		Games:           slices.Clone(c.Games),
		GameFilter:      backend.GameFilterMode(c.GameFilter),
		AutoUpdateCheck: c.AutoUpdateCheck,
		TrayWatch:       c.TrayWatch,
	}
}

// LoadSettings implements backend.Backend.
func (b *Backend) LoadSettings(context.Context) (backend.Settings, error) {
	b.cfg.Lock()
	defer b.cfg.Unlock()
	c, err := b.loadConfig()
	s := toSettings(&c)
	if err != nil {
		return s, &backend.ConfigError{Err: err}
	}
	return s, nil
}

// SaveSettings implements backend.Backend. It reads the file first, so the
// fields that the GUI does not know stay.
func (b *Backend) SaveSettings(_ context.Context, s backend.Settings) error {
	cat, err := b.l.LoadCatalog()
	if err != nil {
		return err
	}
	for _, id := range s.Games {
		if cat.Game(id) == nil {
			return fmt.Errorf("unknown game %q", id)
		}
	}
	b.cfg.Lock()
	defer b.cfg.Unlock()
	// A file that is not correct gets the new settings: the user saves
	// them now.
	c, _ := b.loadConfig()
	c.Language = s.Language
	c.Engine = s.Engine
	c.Strategy = s.Strategy
	c.GameFilter = config.GameFilterMode(s.GameFilter)
	c.AutoUpdateCheck = s.AutoUpdateCheck
	c.TrayWatch = s.TrayWatch
	games := slices.Clone(s.Games)
	sort.Strings(games)
	c.Games = slices.Compact(games)
	return config.Save(b.l.ConfigPath(), &c)
}

// keepSelection writes the strategy and the engine of a Run or an Install
// to user\config.json. When the file is not correct, it does not write it:
// the user must see the error and not lose the file.
func (b *Backend) keepSelection(name string, e spec.Engine) error {
	b.cfg.Lock()
	defer b.cfg.Unlock()
	c, err := b.loadConfig()
	if err != nil {
		// LoadSettings shows the error of the file.
		return nil
	}
	c.Strategy = name
	c.Engine = e
	return config.Save(b.l.ConfigPath(), &c)
}

// Strategies implements backend.Backend.
func (b *Backend) Strategies(context.Context) ([]backend.StrategyInfo, error) {
	list, _, err := backend.LoadCatalogInfo(b.l.Strategies)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// Games implements backend.Backend.
func (b *Backend) Games(context.Context) ([]backend.GameInfo, error) {
	_, games, err := backend.LoadCatalogInfo(b.l.Strategies)
	return games, err
}

// CheckEngine implements backend.Backend. winws2 also needs bin\lua. Then
// the files of the engine must have the SHA256 of bin\versions.json, as
// Confirm-ZapretEngineReady does: winws does not need winws2.exe and
// lua/*, winws2 does not need winws.exe.
func (b *Backend) CheckEngine(_ context.Context, e spec.Engine) error {
	switch e {
	case spec.Winws:
		if _, err := b.l.Exe(e); err != nil {
			return backend.Msg("EngineNoWinws")
		}
	case spec.Winws2:
		_, err := b.l.Exe(e)
		st, lerr := os.Stat(filepath.Join(b.l.Bin, "lua"))
		if err != nil || lerr != nil || !st.IsDir() {
			return backend.Msg("EngineNoWinws2")
		}
	default:
		_, err := engine.ExeName(e)
		return err
	}
	return checkBin(b.l.Bin, e)
}

// checkBin gives EngineBinBad when a file of the engine e is not the file
// of bin\versions.json.
func checkBin(bin string, e spec.Engine) error {
	r, err := engine.CheckBin(bin)
	if err != nil {
		return backend.Msg("EngineBinBad", err.Error())
	}
	var lines []string
	for _, f := range r.Problems() {
		p := strings.ToLower(f.Path)
		if e == spec.Winws && (p == "winws2.exe" || strings.HasPrefix(p, "lua/")) ||
			e == spec.Winws2 && p == "winws.exe" {
			continue
		}
		lines = append(lines, f.Path+": "+f.State.String())
	}
	if len(lines) > 0 {
		return backend.Msg("EngineBinBad", strings.Join(lines, "\n"))
	}
	return nil
}

// IPSet implements backend.Backend.
func (b *Backend) IPSet(context.Context) (backend.IPSetMode, error) {
	m, err := b.l.IPSetMode()
	return backend.IPSetMode(m), err
}

// SetIPSet implements backend.Backend.
func (b *Backend) SetIPSet(_ context.Context, mode backend.IPSetMode) error {
	if !slices.Contains(backend.IPSetModes, mode) {
		return fmt.Errorf("unknown IPSet Filter mode %q", mode)
	}
	err := b.l.SetIPSetMode(engine.IPSetMode(mode), engine.IPSetBackup)
	if errors.Is(err, engine.ErrNoIPSetBackup) {
		return backend.Msg("IpsetNoBackup")
	}
	return err
}

// liveRun gives the run of Run when its process runs, else nil.
func (b *Backend) liveRun() (*engine.Run, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.run == nil {
		return nil, ""
	}
	select {
	case <-b.run.Done():
		b.run, b.runName, b.runEngine = nil, "", ""
		return nil, ""
	default:
		return b.run, b.runName
	}
}

// stopRun stops the process of Run.
func (b *Backend) stopRun() error {
	b.mu.Lock()
	r := b.run
	b.run, b.runName, b.runEngine = nil, "", ""
	b.mu.Unlock()
	if r == nil {
		return nil
	}
	return r.Stop()
}

// stateText gives the name of the service state, as the PS version shows
// it (ServiceControllerStatus).
func stateText(installed bool, s service.State) backend.ServiceState {
	if !installed {
		return backend.ServiceNone
	}
	switch s {
	case service.NotInstalled:
		return backend.ServiceNone
	case service.Running:
		return backend.ServiceRunning
	case service.Stopped:
		return backend.ServiceStopped
	case service.StartPending:
		return backend.ServiceStartPending
	case service.StopPending:
		return backend.ServiceStopPending
	case service.ContinuePending:
		return "ContinuePending"
	case service.PausePending:
		return "PausePending"
	case service.Paused:
		return "Paused"
	}
	return backend.ServiceState(s.String())
}

// divertState gives the state of the first WinDivert driver service that
// is installed.
func (b *Backend) divertState() (backend.ServiceState, error) {
	for _, name := range service.DriverServices {
		ss, err := b.mgr.Host.Query(name)
		if errors.Is(err, service.ErrNotInstalled) {
			continue
		}
		if err != nil {
			return backend.ServiceNone, err
		}
		return stateText(true, ss.State), nil
	}
	return backend.ServiceNone, nil
}

// engineExes gives the names of the engine exes that run, winws.exe first.
func engineExes(procs []service.EngineProcess) []string {
	var exes []string
	for _, e := range []spec.Engine{spec.Winws, spec.Winws2} {
		name := backend.ExeName(e)
		if slices.ContainsFunc(procs, func(p service.EngineProcess) bool { return strings.EqualFold(p.Exe, name) }) {
			exes = append(exes, name)
		}
	}
	return exes
}

// Status implements backend.Backend. It does not need administrator
// rights.
func (b *Backend) Status(context.Context) (backend.Status, error) {
	var st backend.Status
	procs, err := b.engines()
	if err != nil {
		return st, fmt.Errorf("list the engine processes: %w", err)
	}
	svc, err := b.mgr.Status(nil)
	if err != nil {
		return st, err
	}
	if st.Divert, err = b.divertState(); err != nil {
		return st, err
	}
	st.Service = stateText(svc.Installed, svc.State)
	st.Installed = svc.Record.Strategy
	if fi, err := os.Stat(filepath.Join(b.l.Bin, divertSys)); err == nil && fi.Mode().IsRegular() {
		st.DivertSys = true
	}
	st.BypassExes = engineExes(procs)
	st.BypassRunning = len(st.BypassExes) > 0
	switch {
	case st.BypassRunning:
		st.BypassName = st.BypassExes[0]
	case svc.Engine != "":
		st.BypassName = backend.ExeName(svc.Engine)
	default:
		c, _ := b.loadConfig()
		e := c.Engine
		if e == "" {
			e = spec.Winws
		}
		st.BypassName = backend.ExeName(e)
	}
	if r, name := b.liveRun(); r != nil {
		st.Running = name
	} else if svc.Installed && svc.State == service.Running {
		st.Running = svc.Record.Strategy
	}
	return st, nil
}

// ForeignEngines implements backend.Backend.
func (b *Backend) ForeignEngines(context.Context) ([]string, error) {
	procs, err := b.engines()
	if err != nil {
		return nil, fmt.Errorf("list the engine processes: %w", err)
	}
	var own []uint32
	if r, _ := b.liveRun(); r != nil {
		own = append(own, uint32(r.Pid())) //nolint:gosec // G115: a Windows PID is a DWORD.
	}
	ss, err := b.mgr.Host.Query(service.Name)
	switch {
	case err == nil:
		if ss.Pid != 0 {
			own = append(own, ss.Pid)
		}
	case !errors.Is(err, service.ErrNotInstalled):
		return nil, err
	}
	var list []string
	for _, p := range procs {
		if !slices.Contains(own, p.Pid) {
			list = append(list, fmt.Sprintf("%s (PID %d)", p.Exe, p.Pid))
		}
	}
	return list, nil
}

// fileExists tells if p is a file.
func fileExists(p string) (bool, error) {
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}
