// Package backend is the interface between the GUI and the system part of
// zapman: config, engine, services, tests, diagnostics and downloads.
//
// The GUI uses only this interface. Fake is an in-memory implementation for
// the development and the tests. The real implementation comes from the
// packages internal/config, internal/engine and internal/service.
package backend

import (
	"context"
	"fmt"

	"github.com/NONERUN/zapman/internal/spec"
)

// Backend is the system part of the GUI. All methods are safe for
// concurrent use. A method that fails returns an error; it does not log
// the error and it does not show it.
type Backend interface {
	// Version gives the versions for the title and the status lines.
	Version() Version

	// LoadSettings reads user/config.json. When the file is not valid, it
	// returns the defaults and a *ConfigError.
	LoadSettings(ctx context.Context) (Settings, error)
	// SaveSettings writes user/config.json.
	SaveSettings(ctx context.Context, s Settings) error

	// Status gives the state of the bypass, the services and WinDivert.
	Status(ctx context.Context) (Status, error)
	// Strategies gives the strategies of the catalog, sorted by name.
	Strategies(ctx context.Context) ([]StrategyInfo, error)
	// Games gives the game modules of the catalog, in build order.
	Games(ctx context.Context) ([]GameInfo, error)
	// CheckEngine returns nil when bin has the files of the engine.
	CheckEngine(ctx context.Context, e spec.Engine) error

	// StartService starts the installed zapret service.
	StartService(ctx context.Context) error
	// Stop stops winws / winws2 and the zapret service.
	Stop(ctx context.Context) error
	// RemoveServices removes the zapret and WinDivert services.
	RemoveServices(ctx context.Context) error
	// Run starts the strategy with the engine as a process, not as a service.
	Run(ctx context.Context, strategy string, e spec.Engine) error
	// Install installs and starts the zapret service with the strategy.
	Install(ctx context.Context, strategy string, e spec.Engine) error
	// SuspendService removes the service for the tests. Restore puts it back.
	SuspendService(ctx context.Context) (ServiceSnapshot, error)
	// RestoreService installs the service of the snapshot again.
	RestoreService(ctx context.Context, snap ServiceSnapshot) error

	// NetworkReset resets Winsock, IP, the WinHTTP proxy and the DNS cache.
	// The text is the output of the commands. It is not empty on an error.
	NetworkReset(ctx context.Context) (text string, err error)
	// Reboot restarts Windows.
	Reboot(ctx context.Context) error

	// Fakes gives the fake files of bin and the active fakes.
	Fakes(ctx context.Context) (FakeCatalog, error)
	// SetFake copies the fake file to the slot.
	SetFake(ctx context.Context, slot FakeSlot, file string) error

	// DownloadIPSet downloads ipset-all.txt to lists/.
	DownloadIPSet(ctx context.Context, progress Progress) error
	// CheckHosts downloads the hosts template and compares it with the
	// system hosts file.
	CheckHosts(ctx context.Context, progress Progress) (HostsCheck, error)
	// OpenHosts opens the system hosts file in Notepad.
	OpenHosts(ctx context.Context) error
	// LatestVersion gives the tag of the latest release.
	LatestVersion(ctx context.Context) (string, error)

	// Diagnostics gives the items of the diagnostics report.
	Diagnostics(ctx context.Context) ([]DiagItem, error)
	// RunDiagAction does the action of the item.
	RunDiagAction(ctx context.Context, item DiagItem) error

	// RunTests tests the strategies. It stops when ctx is done. It sends the
	// lines and the results to sink, on the goroutine of the call.
	RunTests(ctx context.Context, req TestRequest, sink TestSink) error
}

// Version holds the versions of the program.
type Version struct {
	// App is the tag of the build, for example v1.2.0.
	App string
	// Spec is the version of the strategy catalog.
	Spec string
	// ReleasesURL is the page of the releases.
	ReleasesURL string
}

// GameFilterMode is the Game Filter mode. The values are the values of
// internal/config.
type GameFilterMode string

// The Game Filter modes, in the order of the combo box.
const (
	GameFilterDisabled GameFilterMode = "disabled"
	GameFilterAll      GameFilterMode = "all"
	GameFilterTCP      GameFilterMode = "tcp"
	GameFilterUDP      GameFilterMode = "udp"
)

// GameFilterModes are the Game Filter modes in the order of the combo box.
var GameFilterModes = []GameFilterMode{GameFilterDisabled, GameFilterAll, GameFilterTCP, GameFilterUDP}

// IPSetMode is the IPSet Filter mode.
type IPSetMode string

// The IPSet Filter modes, in the order of the combo box.
const (
	IPSetNone   IPSetMode = "none"
	IPSetLoaded IPSetMode = "loaded"
	IPSetAny    IPSetMode = "any"
)

// IPSetModes are the IPSet Filter modes in the order of the combo box.
var IPSetModes = []IPSetMode{IPSetNone, IPSetLoaded, IPSetAny}

// Settings are the user settings. The field names are the names of
// internal/config.Config. IPSet is not in internal/config yet.
type Settings struct {
	// Language is "ru", "en" or empty (the language of Windows).
	Language        string
	Engine          spec.Engine
	Strategy        string
	Games           []string
	GameFilter      GameFilterMode
	IPSet           IPSetMode
	AutoUpdateCheck bool
	TrayWatch       bool
}

// ConfigError tells that user/config.json is not valid.
type ConfigError struct {
	Err error
}

func (e *ConfigError) Error() string { return "user/config.json: " + e.Err.Error() }

func (e *ConfigError) Unwrap() error { return e.Err }

// ServiceState is the state of a Windows service. Empty means that the
// service is not installed.
type ServiceState string

// The service states that the GUI knows. Other states show as text.
const (
	ServiceNone         ServiceState = ""
	ServiceRunning      ServiceState = "Running"
	ServiceStopped      ServiceState = "Stopped"
	ServiceStartPending ServiceState = "StartPending"
	ServiceStopPending  ServiceState = "StopPending"
)

// Status is the state of the bypass.
type Status struct {
	// BypassRunning tells if winws.exe or winws2.exe runs.
	BypassRunning bool
	// BypassName is the exe that runs. When none runs, it is the exe of the
	// service, else the exe of the engine in the settings.
	BypassName string
	// BypassExes are the exes that run: winws.exe, winws2.exe or both.
	BypassExes []string
	// Service is the state of the zapret service.
	Service ServiceState
	// Divert is the state of the WinDivert service.
	Divert ServiceState
	// DivertSys tells if bin has WinDivert64.sys.
	DivertSys bool
	// Running is the strategy of the process that runs, or empty.
	Running string
	// Installed is the strategy of the service, or empty.
	Installed string
}

// HasService tells if the zapret service is installed.
func (s Status) HasService() bool { return s.Service != ServiceNone }

// Live tells if the bypass runs or the service runs.
func (s Status) Live() bool { return s.BypassRunning || s.Service == ServiceRunning }

// StrategyInfo is a strategy of the catalog.
type StrategyInfo struct {
	Name string
	// Engines are the engines that can build the strategy.
	Engines []spec.Engine
}

// Supports tells if the engine can build the strategy.
func (s StrategyInfo) Supports(e spec.Engine) bool {
	for _, x := range s.Engines {
		if x == e {
			return true
		}
	}
	return false
}

// GameInfo is a game module of the catalog.
type GameInfo struct {
	ID    string
	Title string
}

// ServiceSnapshot is the service that SuspendService removed.
type ServiceSnapshot struct {
	// Strategy is the strategy of the service. Empty means no service.
	Strategy string
	Engine   spec.Engine
	// Running tells if the service ran.
	Running bool
}

// FakeSlot is a slot of an active fake.
type FakeSlot string

// The fake slots.
const (
	FakeDiscord FakeSlot = "discord"
	FakeGame    FakeSlot = "game"
)

// FakeState tells how the active fake of a slot relates to the files.
type FakeState string

// The fake states.
const (
	// FakeMatched: the active fake is a copy of a file of the list.
	FakeMatched FakeState = "matched"
	// FakeMissing: the slot has no active fake.
	FakeMissing FakeState = "missing"
	// FakeUnlisted: the active fake is not a copy of a file of the list.
	FakeUnlisted FakeState = "unlisted"
)

// FakeCatalog is the list of the fake files and the active fakes.
type FakeCatalog struct {
	// Files are the names of the .bin files.
	Files        []string
	Discord      string
	DiscordState FakeState
	Game         string
	GameState    FakeState
}

// Progress gets the state of a download. Total is -1 when the size is not
// known.
type Progress func(done, total int64)

// HostsCheck is the result of CheckHosts.
type HostsCheck struct {
	// NeedsUpdate tells if the hosts file does not have the template block.
	NeedsUpdate bool
	// Template is the block for the hosts file.
	Template string
}

// DiagStatus is the result of a diagnostics item.
type DiagStatus string

// The diagnostics results.
const (
	DiagOK   DiagStatus = "ok"
	DiagWarn DiagStatus = "warn"
	DiagFail DiagStatus = "fail"
)

// DiagItem is an item of the diagnostics report.
type DiagItem struct {
	ID     string
	Status DiagStatus
	Text   string
	// Action is the fix for the item, or empty.
	Action string
	// ActionNames are the arguments of the action.
	ActionNames []string
}

// TestType is the type of the strategy tests.
type TestType string

// The test types.
const (
	TestStandard TestType = "standard"
	TestDPI      TestType = "dpi"
)

// TestRequest selects the tests.
type TestRequest struct {
	Type       TestType
	Strategies []string
	Engine     spec.Engine
}

// TestSink gets the output of RunTests.
type TestSink interface {
	// Line is a line of the test log.
	Line(text string)
	// Strategy is the result of a strategy.
	Strategy(r TestResult)
	// Summary is the result of all strategies.
	Summary(s TestSummary)
}

// TestResult is the result of a strategy.
type TestResult struct {
	Name string
	// Kind is the TestType, or "fail" when the strategy did not start.
	Kind string
	// Error is the text of the start failure.
	Error   string
	Targets []TestTarget
}

// TestTarget is the result of a target: a URL for the standard tests, a
// DPI target for the DPI tests. A value is a status ("OK", "ERROR",
// "UNSUP", "-" ...).
type TestTarget struct {
	Name  string
	HTTP  string
	TLS12 string
	TLS13 string
	// Ping is the ping result. The DPI tests have no ping.
	Ping string
}

// TestSummary is the result of all strategies.
type TestSummary struct {
	Type TestType
	Best string
	Rows []TestSumRow
}

// TestSumRow is the result of a strategy in the summary. The standard tests
// use OK, Err, Unsup, PingOK and PingFail. The DPI tests use OK, Fail,
// Unsup and Blocked.
type TestSumRow struct {
	Name     string
	OK       int
	Err      int
	Unsup    int
	PingOK   int
	PingFail int
	Fail     int
	Blocked  int
}

// MsgError is an error with a text of the UI tables. The GUI shows the
// text in the language of the UI.
type MsgError struct {
	Key  string
	Args []any
}

func (e *MsgError) Error() string {
	if len(e.Args) == 0 {
		return e.Key
	}
	return fmt.Sprint(e.Key, e.Args)
}

// Msg gives a *MsgError.
func Msg(key string, args ...any) error { return &MsgError{Key: key, Args: args} }
