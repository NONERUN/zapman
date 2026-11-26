// Package service installs, removes, starts and stops the Windows service
// "zapret", which runs winws.exe or winws2.exe directly. The PowerShell
// version uses the same service name, so each version sees and replaces
// the service of the other.
//
// The Manager has the logic. All calls to the SCM, the registry and the
// processes go through the Host interface; the tests use a fake Host.
//
// The service keeps a record of the installed strategy in values of its
// registry key (HKLM\SYSTEM\CurrentControlSet\Services\zapret):
//
//	zapman         REG_SZ        strategy name (the PowerShell version also writes it)
//	zapmanVersion  REG_SZ        strategy version, for example "Flowseal 1.10.4 r3"
//	zapmanGames    REG_MULTI_SZ  games with versions, for example "sky 2"
//	zapmanHash     REG_SZ        SHA256 (hex) of the ImagePath
//
// The values go away with the key when the service is deleted. A user
// without administrator rights can read them.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/spec"
)

// The names of the service. They are the names of the PowerShell version.
const (
	Name        = "zapret"
	DisplayName = "zapret"
	Description = "Zapret DPI bypass software"
)

// The registry values of the record in the service key.
const (
	ValueStrategy = "zapman"
	ValueVersion  = "zapmanVersion"
	ValueGames    = "zapmanGames"
	ValueHash     = "zapmanHash"
	// ValueLegacy is the strategy value of old versions. WriteRecord
	// deletes it. Nothing reads it.
	ValueLegacy = "zapret-discord-youtube"
)

// DriverServices are the WinDivert driver services. Remove stops and
// deletes them after the engines stop.
var DriverServices = []string{"WinDivert", "WinDivert14"}

// The errors of the Manager.
var (
	ErrNotInstalled = errors.New("the zapret service is not installed")
	ErrNotElevated  = errors.New("administrator rights are necessary to change the zapret service. Start zapman as administrator")
)

// State is the state of a service. The values are the SERVICE_* values of
// Windows.
type State uint32

// The service states.
const (
	Stopped         State = 1
	StartPending    State = 2
	StopPending     State = 3
	Running         State = 4
	ContinuePending State = 5
	PausePending    State = 6
	Paused          State = 7
)

func (s State) String() string {
	switch s {
	case Stopped:
		return "stopped"
	case StartPending:
		return "start pending"
	case StopPending:
		return "stop pending"
	case Running:
		return "running"
	case ContinuePending:
		return "continue pending"
	case PausePending:
		return "pause pending"
	case Paused:
		return "paused"
	}
	return fmt.Sprintf("State(%d)", uint32(s))
}

// ServiceStatus is the status of a service from the SCM.
type ServiceStatus struct {
	State State
	// ExitCode is the Win32 exit code of the service, or its service
	// specific exit code when the Win32 code is
	// ERROR_SERVICE_SPECIFIC_ERROR.
	ExitCode uint32
}

// Record is the data in the registry key of the service.
type Record struct {
	ImagePath string // not expanded
	Strategy  string
	Version   string
	Games     []string
	Hash      string
}

// Host is the system for the Manager.
type Host interface {
	// Elevated tells if the process has administrator rights.
	Elevated() bool
	// Query gives the status of the service name. It gives an error that
	// is ErrNotInstalled (errors.Is) when the service does not exist. It
	// does not need administrator rights.
	Query(name string) (ServiceStatus, error)
	// Create makes an auto-start service that runs imagePath as it is
	// (no change of quotes).
	Create(name, displayName, description, imagePath string) error
	// Delete marks the service for deletion.
	Delete(name string) error
	// Start starts the service. A service that runs is not an error.
	Start(name string) error
	// Stop sends the stop control. A service that is stopped is not an
	// error.
	Stop(name string) error
	// ReadRecord reads the record of the service. A missing value gives
	// an empty field. It does not need administrator rights.
	ReadRecord(name string) (Record, error)
	// WriteRecord writes the strategy values of r (not the ImagePath) and
	// deletes ValueLegacy. An empty field deletes its value.
	WriteRecord(name string, r *Record) error
	// KillEngines stops all winws.exe and winws2.exe processes. It gives
	// the number of processes that it stopped.
	KillEngines() (int, error)
}

// Plan is the service that Install makes.
type Plan struct {
	Strategy  string
	Version   string
	Games     []string
	ImagePath string
}

// PlanFor gives the plan for a build.
func PlanFor(b *engine.Build) Plan {
	return Plan{
		Strategy:  b.Strategy.Name,
		Version:   b.Strategy.Version(),
		Games:     b.GameVersions(),
		ImagePath: b.CommandLine(),
	}
}

// Hash gives the hash of the ImagePath of the plan.
func (p *Plan) Hash() string { return HashImagePath(p.ImagePath) }

// HashImagePath gives the SHA256 (hex) of an ImagePath.
func HashImagePath(imagePath string) string {
	sum := sha256.Sum256([]byte(imagePath))
	return hex.EncodeToString(sum[:])
}

func (p *Plan) check() error {
	if p.Strategy == "" {
		return errors.New("the service plan has no strategy")
	}
	if !strings.HasPrefix(p.ImagePath, `"`) || strings.Count(p.ImagePath, `"`)%2 != 0 {
		return fmt.Errorf("the service ImagePath is not correct: %.80s", p.ImagePath)
	}
	if n := len(utf16.Encode([]rune(p.ImagePath))); n > engine.MaxCommandLine {
		return fmt.Errorf("the service ImagePath has %d characters; the limit of Windows is %d", n, engine.MaxCommandLine)
	}
	return nil
}

func (p *Plan) record() Record {
	return Record{ImagePath: p.ImagePath, Strategy: p.Strategy, Version: p.Version, Games: p.Games, Hash: p.Hash()}
}

// Reason tells why the installed service is outdated.
type Reason string

// The reasons. Status gives the first one that applies, in this order.
const (
	ReasonNone Reason = ""
	// ReasonForeign: the service has no strategy name. An other program
	// made it (for example service.bat of Flowseal).
	ReasonForeign Reason = "foreign"
	// ReasonStrategy: an other strategy is selected.
	ReasonStrategy Reason = "strategy"
	// ReasonVersion: the strategy or a game has a new version.
	ReasonVersion Reason = "version"
	// ReasonLegacy: the PowerShell version made the service; it wrote no
	// version and no hash, and the ImagePath is not the new one.
	ReasonLegacy Reason = "legacy"
	// ReasonEdited: an other program changed the ImagePath after Install.
	ReasonEdited Reason = "edited"
	// ReasonArgv: the command line changed with the same versions: the
	// engine, the Game Filter, an active fake or the folder of zapman.
	ReasonArgv Reason = "argv"
)

// Status is the state of the zapret service.
type Status struct {
	Installed bool
	State     State
	ExitCode  uint32
	// Engine is the engine from the ImagePath, or "".
	Engine spec.Engine
	Record Record
	// Edited is true when the stored hash is not the hash of the
	// ImagePath.
	Edited bool
	// Outdated is true when the service does not run the plan of Status.
	Outdated bool
	Reason   Reason
}

// Manager does the service operations.
type Manager struct {
	Host Host
	// Sleep and Now are the clock. Nil is the real clock.
	Sleep func(time.Duration)
	Now   func() time.Time
	// The times. Zero is the default.
	StopTimeout  time.Duration // default 8 s
	GoneTimeout  time.Duration // default 8 s
	StartTimeout time.Duration // default 15 s
	Settle       time.Duration // default 2 s: the service must run this time after Running
	Poll         time.Duration // default 250 ms
	RetryDelay   time.Duration // default 1 s
}

// The create retries: a deleted service can stay for a short time.
const (
	createTries = 8
	createDelay = 400 * time.Millisecond
)

func (m *Manager) sleep(d time.Duration) {
	if m.Sleep != nil {
		m.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// EngineOf gives the engine of an ImagePath, or "".
func EngineOf(imagePath string) spec.Engine {
	s := strings.ToLower(imagePath)
	switch {
	case strings.Contains(s, "winws2.exe"):
		return spec.Winws2
	case strings.Contains(s, "winws.exe"):
		return spec.Winws
	}
	return ""
}

// Status gives the state of the service. When want is not nil, Status
// compares the service with it. Status does not need administrator rights.
func (m *Manager) Status(want *Plan) (Status, error) {
	var st Status
	ss, err := m.Host.Query(Name)
	if errors.Is(err, ErrNotInstalled) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Installed = true
	st.State = ss.State
	st.ExitCode = ss.ExitCode
	if st.Record, err = m.Host.ReadRecord(Name); err != nil {
		return st, err
	}
	rec := &st.Record
	st.Engine = EngineOf(rec.ImagePath)
	got := HashImagePath(rec.ImagePath)
	st.Edited = rec.Hash != "" && rec.Hash != got
	if want == nil {
		return st, nil
	}
	switch {
	case rec.Strategy == "":
		st.Reason = ReasonForeign
	case rec.Strategy != want.Strategy:
		st.Reason = ReasonStrategy
	case got == want.Hash():
		st.Reason = ReasonNone
	case rec.Version != "" && (rec.Version != want.Version || !slices.Equal(rec.Games, want.Games)):
		st.Reason = ReasonVersion
	case rec.Hash == "":
		st.Reason = ReasonLegacy
	case st.Edited:
		st.Reason = ReasonEdited
	default:
		st.Reason = ReasonArgv
	}
	st.Outdated = st.Reason != ReasonNone
	return st, nil
}

// Install replaces the service with the plan and starts it. The steps are
// the steps of the PowerShell version:
//  1. Keep a copy of the old record.
//  2. Stop the service and the engine processes.
//  3. Delete the old service and wait until it is gone.
//  4. Make the new service, write the record and read it again.
//  5. Start the service and make sure that it runs.
//
// When a step after 3 fails, Install deletes the new service and makes the
// old service again (not started).
func (m *Manager) Install(p Plan) error {
	if err := p.check(); err != nil {
		return err
	}
	if !m.Host.Elevated() {
		return ErrNotElevated
	}
	old, hadOld, err := m.snapshot()
	if err != nil {
		return err
	}
	if err := m.stopAll(); err != nil {
		return err
	}
	if hadOld {
		if err := m.deleteAndWait(Name); err != nil && !errors.Is(err, ErrNotInstalled) {
			return err
		}
	}
	rec := p.record()
	err = m.create(&rec)
	if err == nil {
		err = m.start()
	}
	if err == nil {
		return nil
	}
	if rerr := m.restore(&old, hadOld); rerr != nil {
		return errors.Join(err, fmt.Errorf("restore the old service: %w", rerr))
	}
	return err
}

func (m *Manager) snapshot() (Record, bool, error) {
	if _, err := m.Host.Query(Name); errors.Is(err, ErrNotInstalled) {
		return Record{}, false, nil
	} else if err != nil {
		return Record{}, false, err
	}
	r, err := m.Host.ReadRecord(Name)
	return r, true, err
}

// restore deletes the new service and makes the old service again.
func (m *Manager) restore(old *Record, hadOld bool) error {
	_ = m.Host.Stop(Name)
	_ = m.waitState(Name, Stopped, or(m.StopTimeout, 8*time.Second))
	if err := m.deleteAndWait(Name); err != nil && !errors.Is(err, ErrNotInstalled) {
		return err
	}
	if !hadOld || old.ImagePath == "" {
		return nil
	}
	return m.create(old)
}

// create makes the service with r.ImagePath and writes the record.
func (m *Manager) create(r *Record) error {
	var err error
	for try := range createTries {
		if try > 0 {
			m.sleep(createDelay)
		}
		if err = m.Host.Create(Name, DisplayName, Description, r.ImagePath); err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("make the zapret service: %w", err)
	}
	if err := m.Host.WriteRecord(Name, r); err != nil {
		return fmt.Errorf("write the record of the zapret service: %w", err)
	}
	got, err := m.Host.ReadRecord(Name)
	if err != nil {
		return fmt.Errorf("read the record of the zapret service: %w", err)
	}
	if got.ImagePath != r.ImagePath {
		return errors.New("the ImagePath of the zapret service is not the ImagePath that zapman wrote")
	}
	if got.Strategy != r.Strategy || got.Version != r.Version || got.Hash != r.Hash || !slices.Equal(got.Games, r.Games) {
		return errors.New("the record of the zapret service is not the record that zapman wrote")
	}
	return nil
}

// start starts the service and waits until it runs for the Settle time.
// When the engine stops with engine.StatusDLLInitFailed, start waits for
// RetryDelay and starts it one more time.
func (m *Manager) start() error {
	var err error
	for try := range 2 {
		if try > 0 {
			m.sleep(or(m.RetryDelay, time.Second))
		}
		var code uint32
		code, err = m.startOnce()
		if err == nil || code != engine.StatusDLLInitFailed {
			break
		}
	}
	return err
}

// startOnce gives the exit code of the service when it stops.
func (m *Manager) startOnce() (uint32, error) {
	if err := m.Host.Start(Name); err != nil {
		ss, qerr := m.Host.Query(Name)
		if qerr == nil && ss.ExitCode != 0 {
			return ss.ExitCode, startError(ss.ExitCode, err)
		}
		return 0, fmt.Errorf("start the zapret service: %w", err)
	}
	if err := m.waitState(Name, Running, or(m.StartTimeout, 15*time.Second)); err != nil {
		return m.exitCode(), err
	}
	m.sleep(or(m.Settle, 2*time.Second))
	ss, err := m.Host.Query(Name)
	if err != nil {
		return 0, err
	}
	if ss.State != Running {
		return ss.ExitCode, startError(ss.ExitCode, nil)
	}
	return 0, nil
}

func (m *Manager) exitCode() uint32 {
	ss, err := m.Host.Query(Name)
	if err != nil {
		return 0
	}
	return ss.ExitCode
}

func startError(code uint32, err error) error {
	msg := fmt.Sprintf("the zapret service stopped at start with code 0x%08X", code)
	if code == engine.StatusDLLInitFailed {
		msg += " (STATUS_DLL_INIT_FAILED), also after a second start. A DLL of the engine did not start. An antivirus can cause this: add the bin folder to its exclusions"
	}
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return errors.New(msg)
}

// Remove stops and deletes the zapret service, stops the engine processes
// and then stops and deletes the WinDivert drivers. The order prevents a
// driver that stays in STOP_PENDING. A missing service is not an error.
func (m *Manager) Remove() error {
	if !m.Host.Elevated() {
		return ErrNotElevated
	}
	var errs []error
	if err := m.stopAll(); err != nil {
		errs = append(errs, err)
	}
	for _, name := range append([]string{Name}, DriverServices...) {
		if name != Name {
			if err := m.stopAndWait(name); err != nil {
				errs = append(errs, err)
			}
		}
		if err := m.deleteAndWait(name); err != nil && !errors.Is(err, ErrNotInstalled) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Start starts the installed service. A service that runs is not an
// error. Start does not change the ImagePath: use Status to find an
// outdated service.
func (m *Manager) Start() error {
	if !m.Host.Elevated() {
		return ErrNotElevated
	}
	ss, err := m.Host.Query(Name)
	if err != nil {
		return err
	}
	if ss.State == Running {
		return nil
	}
	return m.start()
}

// Stop stops the service and waits until it is stopped. It does not stop
// an engine of a foreground run.
func (m *Manager) Stop() error {
	if !m.Host.Elevated() {
		return ErrNotElevated
	}
	return m.stopAndWait(Name)
}

// stopAll stops the zapret service and then the engine processes.
func (m *Manager) stopAll() error {
	if err := m.stopAndWait(Name); err != nil && !errors.Is(err, ErrNotInstalled) {
		return err
	}
	if _, err := m.Host.KillEngines(); err != nil {
		return fmt.Errorf("stop the engine processes: %w", err)
	}
	return nil
}

// stopAndWait stops the service name. A missing service is not an error.
func (m *Manager) stopAndWait(name string) error {
	ss, err := m.Host.Query(name)
	if errors.Is(err, ErrNotInstalled) {
		return nil
	}
	if err != nil {
		return err
	}
	if ss.State == Stopped {
		return nil
	}
	if err := m.Host.Stop(name); err != nil {
		return fmt.Errorf("stop the %s service: %w", name, err)
	}
	return m.waitState(name, Stopped, or(m.StopTimeout, 8*time.Second))
}

// deleteAndWait deletes the service name and waits until it is gone.
func (m *Manager) deleteAndWait(name string) error {
	if err := m.Host.Delete(name); err != nil {
		if errors.Is(err, ErrNotInstalled) {
			return err
		}
		return fmt.Errorf("delete the %s service: %w", name, err)
	}
	end := m.now().Add(or(m.GoneTimeout, 8*time.Second))
	for {
		_, err := m.Host.Query(name)
		if errors.Is(err, ErrNotInstalled) {
			return nil
		}
		if err != nil {
			return err
		}
		if !m.now().Before(end) {
			return fmt.Errorf("the %s service is still there after the delete. Close the Services window and try again", name)
		}
		m.sleep(or(m.Poll, 250*time.Millisecond))
	}
}

// waitState waits until the service name has the state want.
func (m *Manager) waitState(name string, want State, timeout time.Duration) error {
	end := m.now().Add(timeout)
	for {
		ss, err := m.Host.Query(name)
		if err != nil {
			return err
		}
		if ss.State == want {
			return nil
		}
		if want == Running && ss.State == Stopped {
			return startError(ss.ExitCode, nil)
		}
		if !m.now().Before(end) {
			return fmt.Errorf("the %s service is %s, not %s, after %s", name, ss.State, want, timeout)
		}
		m.sleep(or(m.Poll, 250*time.Millisecond))
	}
}
