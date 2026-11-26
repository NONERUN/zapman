package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/spec"
)

// The tests use only fakeHost. They do not touch the services of this
// computer and do not need administrator rights.

type fakeService struct {
	status ServiceStatus
	rec    Record
	legacy string // the value ValueLegacy
	// gone is the number of Query calls after Delete until the service is
	// gone. -1: the service is not deleted.
	gone int
}

type fakeHost struct {
	elevated bool
	services map[string]*fakeService
	log      []string
	// createFails is the number of Create calls that fail.
	createFails int
	// startCodes are the exit codes of the next starts. 0: the service runs.
	startCodes []uint32
	// stopAfterRunning: a new service (with a hash) stops with this code
	// at the first Query after Running.
	stopAfterRunning uint32
	engines          int
	// tcp is the value of the timestamps line of netsh: enabled or disabled.
	tcp      string
	tcpFails bool
	// badReadBack: ReadRecord gives an other ImagePath for this ImagePath.
	badReadBack string
}

func newFake() *fakeHost {
	return &fakeHost{elevated: true, services: map[string]*fakeService{}, tcp: "enabled"}
}

func (h *fakeHost) add(name string, st State, r Record) *fakeService {
	s := &fakeService{status: ServiceStatus{State: st}, rec: r, gone: -1}
	h.services[name] = s
	return s
}

func (h *fakeHost) get(name string) (*fakeService, error) {
	s := h.services[name]
	if s == nil {
		return nil, fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	return s, nil
}

func (h *fakeHost) Elevated() bool { return h.elevated }

func (h *fakeHost) Query(name string) (ServiceStatus, error) {
	s, err := h.get(name)
	if err != nil {
		return ServiceStatus{}, err
	}
	if s.gone >= 0 {
		if s.gone == 0 {
			delete(h.services, name)
			return ServiceStatus{}, fmt.Errorf("%s: %w", name, ErrNotInstalled)
		}
		s.gone--
	}
	if s.status.State == Running && h.stopAfterRunning != 0 && s.rec.Hash != "" {
		s.status = ServiceStatus{State: Stopped, ExitCode: h.stopAfterRunning}
		h.stopAfterRunning = 0
	}
	return s.status, nil
}

func (h *fakeHost) Create(name, displayName, description, imagePath string) error {
	h.log = append(h.log, "create "+name)
	if h.createFails > 0 {
		h.createFails--
		return errors.New("ERROR_SERVICE_MARKED_FOR_DELETE")
	}
	if _, ok := h.services[name]; ok {
		return errors.New("ERROR_SERVICE_EXISTS")
	}
	if displayName != DisplayName || description != Description {
		return fmt.Errorf("bad names %q %q", displayName, description)
	}
	h.add(name, Stopped, Record{ImagePath: imagePath})
	return nil
}

func (h *fakeHost) Delete(name string) error {
	h.log = append(h.log, "delete "+name)
	s, err := h.get(name)
	if err != nil {
		return err
	}
	if s.gone < 0 {
		s.gone = 2
	}
	return nil
}

func (h *fakeHost) Start(name string) error {
	h.log = append(h.log, "start "+name)
	s, err := h.get(name)
	if err != nil {
		return err
	}
	code := uint32(0)
	if len(h.startCodes) > 0 {
		code, h.startCodes = h.startCodes[0], h.startCodes[1:]
	}
	if code != 0 {
		s.status = ServiceStatus{State: Stopped, ExitCode: code}
		return nil
	}
	s.status = ServiceStatus{State: Running}
	return nil
}

func (h *fakeHost) Stop(name string) error {
	h.log = append(h.log, "stop "+name)
	s, err := h.get(name)
	if err != nil {
		return err
	}
	s.status.State = Stopped
	return nil
}

func (h *fakeHost) ReadRecord(name string) (Record, error) {
	s, err := h.get(name)
	if err != nil {
		return Record{}, err
	}
	r := s.rec
	r.Games = slices.Clone(r.Games)
	if h.badReadBack != "" && r.ImagePath == h.badReadBack {
		r.ImagePath += " --edited"
	}
	return r, nil
}

func (h *fakeHost) WriteRecord(name string, r *Record) error {
	h.log = append(h.log, "write "+name)
	s, err := h.get(name)
	if err != nil {
		return err
	}
	s.rec.Strategy, s.rec.Version, s.rec.Hash = r.Strategy, r.Version, r.Hash
	s.rec.Games = slices.Clone(r.Games)
	s.legacy = ""
	return nil
}

func (h *fakeHost) TCPShowGlobal() ([]byte, error) {
	return []byte("TCP Global Parameters\r\n------\r\nECN Capability : disabled\r\nRFC 1323 Timestamps : " + h.tcp + " \r\n"), nil
}

func (h *fakeHost) TCPEnableTimestamps() error {
	h.log = append(h.log, "tcp on")
	if h.tcpFails {
		return errors.New("access denied")
	}
	h.tcp = "enabled"
	return nil
}

func (h *fakeHost) KillEngines() (int, error) {
	h.log = append(h.log, "kill")
	n := h.engines
	h.engines = 0
	return n, nil
}

// fakeClock is a clock that goes forward only in Sleep.
type fakeClock struct{ t time.Time }

func (c *fakeClock) sleep(d time.Duration) { c.t = c.t.Add(d) }
func (c *fakeClock) now() time.Time        { return c.t }

func newManager(h *fakeHost) *Manager {
	c := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return &Manager{Host: h, Sleep: c.sleep, Now: c.now}
}

const exe = `"C:\Program Files\zapman\bin\winws.exe"`

func testPlan() Plan {
	return Plan{
		Strategy:  "general (ALT11)",
		Version:   "Flowseal 1.10.4 r3",
		Games:     []string{"sky 2"},
		ImagePath: exe + ` --wf-tcp=80,443 --filter-tcp=443 --dpi-desync=fake`,
	}
}

func oldRecord() Record {
	return Record{ImagePath: exe + ` --wf-tcp=443 --old`, Strategy: "general"}
}

func TestInstallNew(t *testing.T) {
	h := newFake()
	m := newManager(h)
	p := testPlan()
	if err := m.Install(p); err != nil {
		t.Fatal(err)
	}
	s := h.services[Name]
	if s == nil || s.status.State != Running {
		t.Fatalf("service: %+v", s)
	}
	want := Record{ImagePath: p.ImagePath, Strategy: p.Strategy, Version: p.Version, Games: p.Games, Hash: p.Hash()}
	if !reflect.DeepEqual(s.rec, want) {
		t.Fatalf("record:\n got %+v\nwant %+v", s.rec, want)
	}
	if want := []string{"kill", "create zapret", "write zapret", "start zapret"}; !reflect.DeepEqual(h.log, want) {
		t.Errorf("log: %v", h.log)
	}
	st, err := m.Status(&p)
	if err != nil || !st.Installed || st.State != Running || st.Outdated || st.Engine != spec.Winws || st.Edited {
		t.Fatalf("status %+v, %v", st, err)
	}
}

func TestInstallReplaces(t *testing.T) {
	h := newFake()
	old := h.add(Name, Running, oldRecord())
	old.legacy = "old name"
	h.engines = 1
	m := newManager(h)
	if err := m.Install(testPlan()); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop zapret", "kill", "delete zapret", "create zapret", "write zapret", "start zapret"}
	if !reflect.DeepEqual(h.log, want) {
		t.Errorf("log: %v", h.log)
	}
	s := h.services[Name]
	if s == old || s.rec.Strategy != testPlan().Strategy || s.legacy != "" {
		t.Errorf("service: %+v", s)
	}
}

func TestInstallCreateRetry(t *testing.T) {
	h := newFake()
	h.createFails = 3
	m := newManager(h)
	if err := m.Install(testPlan()); err != nil {
		t.Fatal(err)
	}
	h2 := newFake()
	h2.createFails = createTries
	m2 := newManager(h2)
	if err := m2.Install(testPlan()); err == nil || !strings.Contains(err.Error(), "make the zapret service") {
		t.Fatalf("got %v", err)
	}
}

func TestInstallRestore(t *testing.T) {
	for name, set := range map[string]func(h *fakeHost){
		"start":     func(h *fakeHost) { h.startCodes = []uint32{1, 1} },
		"settle":    func(h *fakeHost) { h.stopAfterRunning = 5 },
		"read-back": func(h *fakeHost) { h.badReadBack = testPlan().ImagePath },
	} {
		t.Run(name, func(t *testing.T) {
			h := newFake()
			h.add(Name, Running, oldRecord())
			set(h)
			m := newManager(h)
			err := m.Install(testPlan())
			if err == nil || strings.Contains(err.Error(), "restore") {
				t.Fatalf("got %v", err)
			}
			s := h.services[Name]
			if s == nil || s.rec.ImagePath != oldRecord().ImagePath || s.rec.Strategy != "general" {
				t.Fatalf("the old service is not back: %+v (%v)", s, err)
			}
			if s.status.State != Stopped {
				t.Errorf("the old service runs: %v", s.status.State)
			}
		})
	}
}

func TestInstallRestoreNone(t *testing.T) {
	h := newFake()
	h.startCodes = []uint32{1}
	m := newManager(h)
	if err := m.Install(testPlan()); err == nil || !strings.Contains(err.Error(), "0x00000001") {
		t.Fatalf("got %v", err)
	}
	if _, ok := h.services[Name]; ok {
		t.Fatal("the failed service is still there")
	}
}

func TestInstallDLLRetry(t *testing.T) {
	h := newFake()
	h.startCodes = []uint32{engine.StatusDLLInitFailed}
	m := newManager(h)
	if err := m.Install(testPlan()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join(h.log, ","), "start zapret"); n != 2 {
		t.Errorf("%d starts: %v", n, h.log)
	}

	h = newFake()
	h.startCodes = []uint32{engine.StatusDLLInitFailed, engine.StatusDLLInitFailed, 0}
	m = newManager(h)
	err := m.Install(testPlan())
	if err == nil || !strings.Contains(err.Error(), "STATUS_DLL_INIT_FAILED") || !strings.Contains(err.Error(), "antivirus") {
		t.Fatalf("got %v", err)
	}
}

func TestInstallChecks(t *testing.T) {
	long := testPlan()
	long.ImagePath = exe + " " + strings.Repeat("a", engine.MaxCommandLine)
	noQuote := testPlan()
	noQuote.ImagePath = `C:\Program Files\zapman\bin\winws.exe --x`
	noName := testPlan()
	noName.Strategy = ""
	for name, p := range map[string]Plan{"long": long, "quote": noQuote, "strategy": noName} {
		h := newFake()
		m := newManager(h)
		if err := m.Install(p); err == nil {
			t.Errorf("%s: no error", name)
		}
		if len(h.log) != 0 {
			t.Errorf("%s: calls %v", name, h.log)
		}
	}
}

func TestNotElevated(t *testing.T) {
	h := newFake()
	h.elevated = false
	h.add(Name, Running, oldRecord())
	m := newManager(h)
	for name, f := range map[string]func() error{
		"install": func() error { return m.Install(testPlan()) },
		"remove":  m.Remove,
		"start":   m.Start,
		"stop":    m.Stop,
	} {
		if err := f(); !errors.Is(err, ErrNotElevated) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if len(h.log) != 0 {
		t.Errorf("calls: %v", h.log)
	}
	// Status does not need administrator rights.
	if st, err := m.Status(nil); err != nil || !st.Installed {
		t.Errorf("status: %+v, %v", st, err)
	}
}

func TestRemove(t *testing.T) {
	h := newFake()
	h.add(Name, Running, oldRecord())
	h.add("WinDivert", Running, Record{})
	h.engines = 2
	m := newManager(h)
	if err := m.Remove(); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop zapret", "kill", "delete zapret", "stop WinDivert", "delete WinDivert", "delete WinDivert14"}
	if !reflect.DeepEqual(h.log, want) {
		t.Errorf("log: %v", h.log)
	}
	if len(h.services) != 0 {
		t.Errorf("services: %v", h.services)
	}
	// Nothing installed is not an error.
	if err := m.Remove(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteTimeout(t *testing.T) {
	h := newFake()
	h.add(Name, Stopped, oldRecord()).gone = 1 << 30
	m := newManager(h)
	if err := m.Remove(); err == nil || !strings.Contains(err.Error(), "still there") {
		t.Fatalf("got %v", err)
	}
}

func TestStartStop(t *testing.T) {
	h := newFake()
	m := newManager(h)
	if err := m.Start(); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("start, not installed: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("stop, not installed: %v", err)
	}
	h.add(Name, Running, oldRecord())
	if err := m.Start(); err != nil || len(h.log) != 0 {
		t.Fatalf("start, running: %v %v", err, h.log)
	}
	if err := m.Stop(); err != nil || h.services[Name].status.State != Stopped {
		t.Fatalf("stop: %v", err)
	}
	if err := m.Start(); err != nil || h.services[Name].status.State != Running {
		t.Fatalf("start: %v", err)
	}
	// Start does not change the service.
	if !reflect.DeepEqual(h.services[Name].rec, oldRecord()) {
		t.Errorf("record: %+v", h.services[Name].rec)
	}
}

func TestStatus(t *testing.T) {
	p := testPlan()
	cur := Record{ImagePath: p.ImagePath, Strategy: p.Strategy, Version: p.Version, Games: p.Games, Hash: p.Hash()}
	other := p.ImagePath + " --new"
	for _, c := range []struct {
		name   string
		edit   func(r *Record)
		reason Reason
		edited bool
	}{
		{"current", func(*Record) {}, ReasonNone, false},
		{"foreign", func(r *Record) { *r = Record{ImagePath: other} }, ReasonForeign, false},
		{"strategy", func(r *Record) { r.Strategy = "general" }, ReasonStrategy, false},
		{"version", func(r *Record) {
			r.Version = "Flowseal 1.10.4 r2"
			r.ImagePath, r.Hash = other, HashImagePath(other)
		}, ReasonVersion, false},
		{"game version", func(r *Record) {
			r.Games = []string{"sky 1"}
			r.ImagePath, r.Hash = other, HashImagePath(other)
		}, ReasonVersion, false},
		// The PowerShell version wrote only the strategy name.
		{"legacy", func(r *Record) { *r = Record{ImagePath: other, Strategy: p.Strategy} }, ReasonLegacy, false},
		{"legacy, same argv", func(r *Record) { *r = Record{ImagePath: p.ImagePath, Strategy: p.Strategy} }, ReasonNone, false},
		{"edited", func(r *Record) { r.ImagePath = other }, ReasonEdited, true},
		{"argv", func(r *Record) { r.ImagePath, r.Hash = other, HashImagePath(other) }, ReasonArgv, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newFake()
			r := cur
			c.edit(&r)
			h.add(Name, Stopped, r)
			m := newManager(h)
			st, err := m.Status(&p)
			if err != nil {
				t.Fatal(err)
			}
			if st.Reason != c.reason || st.Outdated != (c.reason != ReasonNone) || st.Edited != c.edited {
				t.Errorf("got reason %q outdated %v edited %v", st.Reason, st.Outdated, st.Edited)
			}
		})
	}
	m := newManager(newFake())
	if st, err := m.Status(&p); err != nil || st.Installed || st.Outdated {
		t.Errorf("not installed: %+v, %v", st, err)
	}
}

func TestEngineOf(t *testing.T) {
	for in, want := range map[string]spec.Engine{
		`"C:\z\bin\winws.exe" --x`:              spec.Winws,
		`"C:\z\bin\WINWS2.EXE" --chdir="C:\z"`:  spec.Winws2,
		`"C:\winws.exe dir\bin\winws2.exe" --x`: spec.Winws2,
		`C:\other.exe`:                          "",
	} {
		if got := EngineOf(in); got != want {
			t.Errorf("%s: got %q", in, got)
		}
	}
}

func TestPlanFor(t *testing.T) {
	cat, err := spec.LoadCatalog(filepath.Join("..", "..", "strategies"), "")
	if err != nil {
		t.Fatal(err)
	}
	st, err := cat.Strategy("general (ALT11)")
	if err != nil {
		t.Fatal(err)
	}
	sky := cat.Game("sky")
	if sky == nil {
		t.Fatal("no stock strategy or game")
	}
	b := &engine.Build{Strategy: st, Games: []*spec.Game{sky}, Engine: spec.Winws, Exe: `C:\z\bin\winws.exe`, Bin: `C:\z\bin`, Lines: []string{"--wf-tcp=80"}}
	p := PlanFor(b)
	want := Plan{Strategy: st.Name, Version: st.Version(), Games: b.GameVersions(), ImagePath: `"C:\z\bin\winws.exe" --wf-tcp=80`}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("got %+v", p)
	}
	if len(p.Hash()) != 64 || p.Hash() != HashImagePath(p.ImagePath) {
		t.Errorf("hash %q", p.Hash())
	}
}
