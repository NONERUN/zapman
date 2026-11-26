package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/spec"
)

const repoRoot = `..\..`

const testStrategy = "general"

// fakeHost is a service.Host in memory.
type fakeHost struct {
	mu       sync.Mutex
	elevated bool
	svcs     map[string]*fakeSvc
	procs    []service.EngineProcess
	tcp      bool
	nextPid  uint32
	calls    []string
	// startErr is the error of Start of the zapret service.
	startErr error
}

type fakeSvc struct {
	st  service.ServiceStatus
	rec service.Record
	cfg service.Config
}

func newHost() *fakeHost {
	return &fakeHost{elevated: true, svcs: map[string]*fakeSvc{}, nextPid: 100}
}

func (h *fakeHost) call(s string) { h.calls = append(h.calls, s) }

func (h *fakeHost) Calls() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.calls)
}

func (h *fakeHost) Elevated() bool { return h.elevated }

func (h *fakeHost) Query(name string) (service.ServiceStatus, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return service.ServiceStatus{}, fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	return s.st, nil
}

func (h *fakeHost) ReadConfig(name string) (service.Config, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return service.Config{}, fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	return s.cfg, nil
}

func (h *fakeHost) Create(name, imagePath string, c *service.Config) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.call("Create " + name)
	h.svcs[name] = &fakeSvc{st: service.ServiceStatus{State: service.Stopped}, rec: service.Record{ImagePath: imagePath}, cfg: *c}
	return nil
}

func (h *fakeHost) Delete(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.svcs[name]; !ok {
		return fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	h.call("Delete " + name)
	delete(h.svcs, name)
	return nil
}

func (h *fakeHost) Start(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	h.call("Start " + name)
	if name == service.Name && h.startErr != nil {
		return h.startErr
	}
	h.nextPid++
	s.st = service.ServiceStatus{State: service.Running, Pid: h.nextPid}
	if name == service.Name {
		h.procs = append(h.procs, service.EngineProcess{Exe: filepath.Base(strings.Split(s.rec.ImagePath, `"`)[1]), Pid: h.nextPid})
		for _, d := range service.DriverServices[:1] {
			h.svcs[d] = &fakeSvc{st: service.ServiceStatus{State: service.Running}}
		}
	}
	return nil
}

func (h *fakeHost) Stop(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	h.call("Stop " + name)
	pid := s.st.Pid
	s.st = service.ServiceStatus{State: service.Stopped}
	h.procs = slices.DeleteFunc(h.procs, func(p service.EngineProcess) bool { return p.Pid == pid })
	return nil
}

func (h *fakeHost) ReadRecord(name string) (service.Record, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return service.Record{}, fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	return s.rec, nil
}

func (h *fakeHost) WriteRecord(name string, r *service.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.svcs[name]
	if !ok {
		return fmt.Errorf("%s: %w", name, service.ErrNotInstalled)
	}
	img := s.rec.ImagePath
	s.rec = *r
	s.rec.ImagePath = img
	return nil
}

func (h *fakeHost) KillEngines() (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.call("KillEngines")
	n := len(h.procs)
	h.procs = nil
	return n, nil
}

func (h *fakeHost) TCPShowGlobal() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := "disabled"
	if h.tcp {
		v = "enabled"
	}
	return []byte("RFC 1323 Timestamps                 : " + v + "\n"), nil
}

func (h *fakeHost) TCPEnableTimestamps() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.call("TCPEnableTimestamps")
	h.tcp = true
	return nil
}

func (h *fakeHost) engines() ([]service.EngineProcess, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.procs), nil
}

func (h *fakeHost) addProc(exe string) uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextPid++
	h.procs = append(h.procs, service.EngineProcess{Exe: exe, Pid: h.nextPid})
	return h.nextPid
}

func (h *fakeHost) removeProc(pid uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.procs = slices.DeleteFunc(h.procs, func(p service.EngineProcess) bool { return p.Pid == pid })
}

// fakeProc is an engine.Process that runs until Kill. exit makes it stop
// at once with the code.
type fakeProc struct {
	h    *fakeHost
	pid  uint32
	code uint32
	stop chan struct{}
	once sync.Once
}

func (p *fakeProc) Wait() (uint32, error) {
	<-p.stop
	p.h.removeProc(p.pid)
	return p.code, nil
}

func (p *fakeProc) Kill() error {
	p.once.Do(func() { close(p.stop) })
	return nil
}

func (p *fakeProc) Pid() int { return int(p.pid) }

// fakeTray is a TrayTask in memory.
type fakeTray struct {
	mu  sync.Mutex
	cmd string
	on  bool
	err error
}

func (t *fakeTray) Register(_ context.Context, cmd string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return t.err
	}
	t.on, t.cmd = true, cmd
	return nil
}

func (t *fakeTray) Unregister(context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.on, t.cmd = false, ""
	return t.err
}

type env struct {
	b    *Backend
	h    *fakeHost
	tray *fakeTray
	l    engine.Layout
	// exitCode is the exit code of the next engine process; zero runs.
	exitCode uint32
	cmdLines []string
}

// newEnv gives a Backend on the stock catalog, the lists and bin of the
// repository, a new user folder, a fake host, a fake runner and a fake
// tray task.
func newEnv(t *testing.T) *env {
	t.Helper()
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	l := engine.NewLayout(t.TempDir())
	l.Bin = filepath.Join(repo, "bin")
	l.Strategies = filepath.Join(repo, "strategies")
	l.Lists = filepath.Join(repo, "lists")
	e := &env{h: newHost(), tray: &fakeTray{}, l: l}
	runner := &engine.Runner{
		StartWindow: 20 * time.Millisecond,
		Sleep:       func(time.Duration) {},
		Start: func(exe, cmdLine, _ string, _ io.Writer) (engine.Process, error) {
			e.cmdLines = append(e.cmdLines, cmdLine)
			p := &fakeProc{h: e.h, pid: e.h.addProc(filepath.Base(exe)), code: e.exitCode, stop: make(chan struct{})}
			if e.exitCode != 0 {
				close(p.stop)
			}
			return p, nil
		},
	}
	e.b = New(Options{
		Layout:      l,
		AppVersion:  "v9.9.9",
		Manager:     &service.Manager{Host: e.h, Sleep: func(time.Duration) {}},
		Runner:      runner,
		Engines:     e.h.engines,
		Tray:        e.tray,
		TrayCommand: `"C:\zapman\zapman.exe" --tray`,
	})
	return e
}

func msgKey(err error) string {
	var m *backend.MsgError
	if errors.As(err, &m) {
		return m.Key
	}
	return ""
}

var ctx = context.Background()

func TestVersion(t *testing.T) {
	e := newEnv(t)
	v := e.b.Version()
	if v.App != "v9.9.9" || v.Spec != "2" || v.ReleasesURL != ReleasesURL {
		t.Fatalf("got %+v", v)
	}
}

func TestSettingsKeepUnknownFields(t *testing.T) {
	e := newEnv(t)
	s, err := e.b.LoadSettings(ctx)
	if err != nil || s.Engine != spec.Winws || !s.TrayWatch {
		t.Fatalf("defaults: %+v, %v", s, err)
	}
	if err := os.MkdirAll(e.l.User, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.l.ConfigPath(), []byte(`{"engine":"winws2","future":{"a":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = e.b.LoadSettings(ctx)
	if err != nil || s.Engine != spec.Winws2 {
		t.Fatalf("got %+v, %v", s, err)
	}
	s.Games = []string{"sky", "sky"}
	s.GameFilter = backend.GameFilterUDP
	s.Language = "ru"
	if err := e.b.SaveSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(e.l.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"future"`) {
		t.Errorf("unknown field is lost: %s", data)
	}
	got, err := e.b.LoadSettings(ctx)
	if err != nil || got.Language != "ru" || got.GameFilter != backend.GameFilterUDP || !slices.Equal(got.Games, []string{"sky"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	s.Games = []string{"nogame"}
	if err := e.b.SaveSettings(ctx, s); err == nil {
		t.Error("unknown game: no error")
	}
	s.Games = nil
	s.Engine = "bad"
	if err := e.b.SaveSettings(ctx, s); err == nil {
		t.Error("bad engine: no error")
	}
}

func TestSettingsBadFile(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(e.l.User, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.l.ConfigPath(), []byte(`{bad`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := e.b.LoadSettings(ctx)
	var ce *backend.ConfigError
	if !errors.As(err, &ce) || s.Engine != spec.Winws {
		t.Fatalf("got %+v, %v", s, err)
	}
	if err := e.b.SaveSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := e.b.LoadSettings(ctx); err != nil {
		t.Errorf("after save: %v", err)
	}
}

func TestCatalog(t *testing.T) {
	e := newEnv(t)
	list, err := e.b.Strategies(ctx)
	if err != nil || len(list) == 0 {
		t.Fatal(list, err)
	}
	if !slices.IsSortedFunc(list, func(a, b backend.StrategyInfo) int { return strings.Compare(a.Name, b.Name) }) {
		t.Error("strategies are not sorted")
	}
	games, err := e.b.Games(ctx)
	if err != nil || !slices.ContainsFunc(games, func(g backend.GameInfo) bool { return g.ID == "sky" }) {
		t.Fatal(games, err)
	}
}

func TestCheckEngine(t *testing.T) {
	e := newEnv(t)
	for _, en := range []spec.Engine{spec.Winws, spec.Winws2} {
		if err := e.b.CheckEngine(ctx, en); err != nil {
			t.Errorf("%s: %v", en, err)
		}
	}
	e.b.l.Bin = t.TempDir()
	if k := msgKey(e.b.CheckEngine(ctx, spec.Winws)); k != "EngineNoWinws" {
		t.Errorf("winws: %q", k)
	}
	if err := os.WriteFile(filepath.Join(e.b.l.Bin, "winws2.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if k := msgKey(e.b.CheckEngine(ctx, spec.Winws2)); k != "EngineNoWinws2" {
		t.Errorf("winws2 with no lua: %q", k)
	}
	if err := e.b.CheckEngine(ctx, "bad"); err == nil {
		t.Error("bad engine: no error")
	}
}

// TestCheckEngineHashes: a file of the engine that is not the file of
// versions.json gives EngineBinBad. The files of the other engine do not
// count.
func TestCheckEngineHashes(t *testing.T) {
	e := newEnv(t)
	bin := t.TempDir()
	e.b.l.Bin = bin
	write := func(name, data string) {
		t.Helper()
		p := filepath.Join(bin, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sum := func(s string) string {
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:])
	}
	write("winws.exe", "w1")
	if k := msgKey(e.b.CheckEngine(ctx, spec.Winws)); k != "EngineBinBad" {
		t.Errorf("no versions.json: %q", k)
	}
	write("versions.json", `{"files":[`+
		`{"path":"winws.exe","sha256":"`+sum("w1")+`"},`+
		`{"path":"winws2.exe","sha256":"`+sum("w2")+`"},`+
		`{"path":"lua/zapret-lib.lua","sha256":"`+sum("lua")+`"}]}`)
	if err := e.b.CheckEngine(ctx, spec.Winws); err != nil {
		t.Errorf("winws with no winws2 files: %v", err)
	}
	write("winws2.exe", "w2")
	write("lua/zapret-lib.lua", "lua\r\n")
	if err := e.b.CheckEngine(ctx, spec.Winws2); msgKey(err) != "EngineBinBad" || !strings.Contains(err.Error(), "lua/zapret-lib.lua: hash mismatch") {
		t.Errorf("winws2 with a changed lua file: %v", err)
	}
	write("lua/zapret-lib.lua", "lua")
	write("winws.exe", "changed")
	if err := e.b.CheckEngine(ctx, spec.Winws2); err != nil {
		t.Errorf("winws2 with a changed winws.exe: %v", err)
	}
	if k := msgKey(e.b.CheckEngine(ctx, spec.Winws)); k != "EngineBinBad" {
		t.Errorf("winws with a changed winws.exe: %q", k)
	}
}

func TestStatusNothing(t *testing.T) {
	e := newEnv(t)
	st, err := e.b.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.BypassName != "winws.exe" || !st.DivertSys || st.BypassRunning || st.Service != "" || st.Divert != "" ||
		st.Running != "" || st.Installed != "" || len(st.BypassExes) != 0 {
		t.Fatalf("got %+v", st)
	}
}

func TestRunAndStop(t *testing.T) {
	e := newEnv(t)
	other := e.h.addProc("winws2.exe")
	if err := e.b.Run(ctx, testStrategy, spec.Winws); err != nil {
		t.Fatal(err)
	}
	calls := e.h.Calls()
	if !slices.Equal(calls, []string{"KillEngines", "TCPEnableTimestamps"}) {
		t.Errorf("calls: %v", calls)
	}
	if len(e.cmdLines) != 1 || !strings.Contains(e.cmdLines[0], `winws.exe" `) {
		t.Errorf("command lines: %q", e.cmdLines)
	}
	procs, _ := e.h.engines()
	if slices.ContainsFunc(procs, func(p service.EngineProcess) bool { return p.Pid == other }) {
		t.Error("Run did not stop the other engine")
	}
	st, err := e.b.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.BypassRunning || st.Running != testStrategy || st.BypassName != "winws.exe" || !slices.Equal(st.BypassExes, []string{"winws.exe"}) {
		t.Fatalf("status: %+v", st)
	}
	if list, err := e.b.ForeignEngines(ctx); err != nil || len(list) != 0 {
		t.Errorf("foreign: %v, %v", list, err)
	}
	s, err := e.b.LoadSettings(ctx)
	if err != nil || s.Strategy != testStrategy || s.Engine != spec.Winws {
		t.Errorf("settings: %+v, %v", s, err)
	}
	if _, err := os.Stat(e.l.IPSetPath()); err != nil {
		t.Errorf("user lists: %v", err)
	}

	if err := e.b.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	st, err = e.b.Status(ctx)
	if err != nil || st.BypassRunning || st.Running != "" {
		t.Fatalf("after stop: %+v, %v", st, err)
	}
}

func TestRunErrors(t *testing.T) {
	e := newEnv(t)
	e.h.elevated = false
	for name, err := range map[string]error{
		"Run":     e.b.Run(ctx, testStrategy, spec.Winws),
		"Install": e.b.Install(ctx, testStrategy, spec.Winws),
		"Stop":    e.b.Stop(ctx),
		"Start":   e.b.StartService(ctx),
		"Remove":  e.b.RemoveServices(ctx),
	} {
		if msgKey(err) != "AdminRequired" {
			t.Errorf("%s not elevated: %v", name, err)
		}
	}
	e.h.elevated = true
	e.exitCode = 1
	err := e.b.Run(ctx, testStrategy, spec.Winws)
	var m *backend.MsgError
	if !errors.As(err, &m) || m.Key != "EngineStartFailText" || len(m.Args) != 2 || m.Args[0] != "winws.exe" ||
		!strings.Contains(fmt.Sprint(m.Args[1]), "0x00000001") {
		t.Fatalf("exit at start: %v", err)
	}
	if s, _ := e.b.LoadSettings(ctx); s.Strategy != "" {
		t.Errorf("a failed run kept the strategy: %+v", s)
	}
	if err := e.b.Run(ctx, "no such strategy", spec.Winws); err == nil {
		t.Error("unknown strategy: no error")
	}
}

func TestInstall(t *testing.T) {
	e := newEnv(t)
	if err := e.b.Install(ctx, testStrategy, spec.Winws); err != nil {
		t.Fatal(err)
	}
	st, err := e.b.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Service != backend.ServiceRunning || st.Installed != testStrategy || st.Running != testStrategy ||
		st.Divert != backend.ServiceRunning || !st.BypassRunning {
		t.Fatalf("status: %+v", st)
	}
	if list, err := e.b.ForeignEngines(ctx); err != nil || len(list) != 0 {
		t.Errorf("the service engine is foreign: %v, %v", list, err)
	}
	pid := e.h.addProc("winws2.exe")
	list, err := e.b.ForeignEngines(ctx)
	if err != nil || !slices.Equal(list, []string{fmt.Sprintf("winws2.exe (PID %d)", pid)}) {
		t.Errorf("foreign: %v, %v", list, err)
	}
	if s, _ := e.b.LoadSettings(ctx); s.Strategy != testStrategy {
		t.Errorf("settings: %+v", s)
	}

	if err := e.b.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.b.Status(ctx); st.Service != backend.ServiceStopped || st.Running != "" || st.BypassRunning {
		t.Errorf("after stop: %+v", st)
	}
	if err := e.b.StartService(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.b.Status(ctx); st.Service != backend.ServiceRunning {
		t.Errorf("after start: %+v", st)
	}
	if err := e.b.RemoveServices(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.b.Status(ctx); st.Service != backend.ServiceNone || st.Divert != backend.ServiceNone || st.Installed != "" {
		t.Errorf("after remove: %+v", st)
	}
}

func TestSuspendRestore(t *testing.T) {
	e := newEnv(t)
	snap, err := e.b.SuspendService(ctx)
	if err != nil || snap.Installed || snap.Data != nil {
		t.Fatalf("no service: %+v, %v", snap, err)
	}
	if err := e.b.RestoreService(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := e.b.Install(ctx, testStrategy, spec.Winws2); err != nil {
		t.Fatal(err)
	}
	snap, err = e.b.SuspendService(ctx)
	if err != nil || !snap.Installed || snap.Strategy != testStrategy || snap.Engine != spec.Winws2 || !snap.Running {
		t.Fatalf("got %+v, %v", snap, err)
	}
	if st, _ := e.b.Status(ctx); st.HasService() {
		t.Fatalf("service is still there: %+v", st)
	}
	if err := e.b.RestoreService(ctx, snap); err != nil {
		t.Fatal(err)
	}
	st, _ := e.b.Status(ctx)
	if st.Service != backend.ServiceRunning || st.Installed != testStrategy || st.BypassName != "winws2.exe" {
		t.Errorf("after restore: %+v", st)
	}
}

func TestInstallFailText(t *testing.T) {
	e := newEnv(t)
	e.h.startErr = errors.New("access is denied by the antivirus")
	err := e.b.Install(ctx, testStrategy, spec.Winws)
	var me *backend.MsgError
	if !errors.As(err, &me) || me.Key != "ServiceInstallFail" || !strings.Contains(err.Error(), "access is denied by the antivirus") {
		t.Fatalf("got %v", err)
	}
	if _, ok := e.h.svcs[service.Name]; ok {
		t.Error("the failed service is still there")
	}
	e.h.elevated = false
	if k := msgKey(e.b.Install(ctx, testStrategy, spec.Winws)); k != "AdminRequired" {
		t.Errorf("not elevated: %q", k)
	}
}

// TestSuspendRestoreForeign: a service of an other program comes back with
// the same ImagePath, configuration and registry values, and stays stopped.
func TestSuspendRestoreForeign(t *testing.T) {
	e := newEnv(t)
	cfg := service.Config{DisplayName: "zapret DPI", Description: "service.bat", StartType: service.StartDemand, DelayedAutoStart: true}
	rec := service.Record{ImagePath: `"C:\zapret\bin\winws.exe" --wf-tcp=80`, Legacy: "general (ALT).bat"}
	e.h.svcs[service.Name] = &fakeSvc{st: service.ServiceStatus{State: service.Stopped}, rec: rec, cfg: cfg}
	snap, err := e.b.SuspendService(ctx)
	if err != nil || !snap.Installed || snap.Strategy != "" || snap.Running || snap.Engine != spec.Winws {
		t.Fatalf("got %+v, %v", snap, err)
	}
	if _, ok := e.h.svcs[service.Name]; ok {
		t.Fatal("the service is still there")
	}
	if err := e.b.RestoreService(ctx, snap); err != nil {
		t.Fatal(err)
	}
	s := e.h.svcs[service.Name]
	if s == nil || s.cfg != cfg || !reflect.DeepEqual(s.rec, rec) || s.st.State != service.Stopped {
		t.Fatalf("after restore: %+v", s)
	}
	if err := e.b.RestoreService(ctx, backend.ServiceSnapshot{Installed: true}); err == nil {
		t.Error("a snapshot with no data: no error")
	}
}
func TestIPSet(t *testing.T) {
	e := newEnv(t)
	if m, err := e.b.IPSet(ctx); err != nil || m != backend.IPSetNone {
		t.Fatalf("no file: %q, %v", m, err)
	}
	if k := msgKey(e.b.SetIPSet(ctx, backend.IPSetLoaded)); k != "IpsetNoBackup" {
		t.Errorf("loaded with no backup: %q", k)
	}
	if err := e.b.SetIPSet(ctx, backend.IPSetAny); err != nil {
		t.Fatal(err)
	}
	if m, _ := e.b.IPSet(ctx); m != backend.IPSetAny {
		t.Errorf("got %q", m)
	}
	if err := e.b.SetIPSet(ctx, "bad"); err == nil {
		t.Error("bad mode: no error")
	}
}

func TestFakes(t *testing.T) {
	e := newEnv(t)
	c, err := e.b.Fakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(c.Files, "stun.bin") || c.DiscordState != backend.FakeMissing || c.GameState != backend.FakeMissing {
		t.Fatalf("got %+v", c)
	}
	if err := e.b.SetFake(ctx, backend.FakeDiscord, "stun.bin"); err != nil {
		t.Fatal(err)
	}
	if c, _ = e.b.Fakes(ctx); c.Discord != "stun.bin" || c.DiscordState != backend.FakeMatched || c.GameState != backend.FakeMissing {
		t.Fatalf("after set: %+v", c)
	}
	p := filepath.Join(e.l.User, spec.ActiveFakesDir, "ACTIVE_GAME_UDP.bin")
	if err := os.WriteFile(p, []byte("own fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, _ = e.b.Fakes(ctx); c.Game != "" || c.GameState != backend.FakeUnlisted {
		t.Fatalf("own fake: %+v", c)
	}
	paths, err := e.l.Paths()
	if err != nil || !paths.ActiveFiles["DISCORD_UDP"] || !paths.ActiveFiles["GAME_UDP"] {
		t.Errorf("the build does not see the active fakes: %v, %v", paths.ActiveFiles, err)
	}
	for _, f := range []string{"none.bin", `..\x.bin`, ""} {
		if err := e.b.SetFake(ctx, backend.FakeGame, f); err == nil {
			t.Errorf("%q: no error", f)
		}
	}
	if err := e.b.SetFake(ctx, "bad", "stun.bin"); err == nil {
		t.Error("bad slot: no error")
	}
}

func TestTray(t *testing.T) {
	e := newEnv(t)
	if err := e.b.SetTrayAtLogon(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !e.tray.on || e.tray.cmd != `"C:\zapman\zapman.exe" --tray` {
		t.Errorf("got %+v", e.tray)
	}
	if err := e.b.SetTrayAtLogon(ctx, false); err != nil || e.tray.on {
		t.Errorf("off: %v, %+v", err, e.tray)
	}
	e.tray.err = errors.New("access denied")
	if err := e.b.SetTrayAtLogon(ctx, true); err == nil {
		t.Error("no error")
	}
}

func TestTrayArgs(t *testing.T) {
	cmd, err := TrayCommand(`C:\Program Files\zapman\zapman.exe`)
	if err != nil || cmd != `"C:\Program Files\zapman\zapman.exe" --tray` {
		t.Fatal(cmd, err)
	}
	if _, err := TrayCommand(`C:\a"b\zapman.exe`); err == nil {
		t.Error("quote: no error")
	}
	if _, err := TrayCommand(""); err == nil {
		t.Error("empty: no error")
	}
	want := `/Create /TN "zapman-tray" /SC ONLOGON /RL HIGHEST /F /TR "\"C:\Program Files\zapman\zapman.exe\" --tray"`
	if got := createArgs(cmd); got != want {
		t.Errorf("create:\n got %s\nwant %s", got, want)
	}
	if queryArgs() != `/Query /TN "zapman-tray"` || deleteArgs() != `/Delete /TN "zapman-tray" /F` {
		t.Error(queryArgs(), deleteArgs())
	}
	if err := schtasksError("delete", 5, "  \r\n"); err == nil || err.Error() != "schtasks delete exit 5" {
		t.Errorf("no output: %v", err)
	}
	if err := schtasksError("create", 1, "ERROR: Access is denied.\r\n"); err == nil || err.Error() != "ERROR: Access is denied." {
		t.Errorf("output: %v", err)
	}
}

func TestNotWired(t *testing.T) {
	e := newEnv(t)
	_, err := e.b.NetworkReset(ctx)
	var m *backend.MsgError
	if !errors.As(err, &m) || m.Key != "NotWired" || len(m.Args) != 1 || m.Args[0] != "NetworkReset" {
		t.Fatalf("got %v", err)
	}
	for name, err := range map[string]error{
		"Reboot":        e.b.Reboot(ctx),
		"DownloadIPSet": e.b.DownloadIPSet(ctx, nil),
		"OpenHosts":     e.b.OpenHosts(ctx),
		"RunDiagAction": e.b.RunDiagAction(ctx, backend.DiagItem{}),
		"RunTests":      e.b.RunTests(ctx, backend.TestRequest{}, nil),
	} {
		if msgKey(err) != "NotWired" {
			t.Errorf("%s: %v", name, err)
		}
	}
	e.b.SetTools(stubTools{})
	if v, err := e.b.LatestVersion(ctx); err != nil || v != "v1" {
		t.Errorf("SetTools: %q, %v", v, err)
	}
	if _, err := e.b.Diagnostics(ctx); msgKey(err) != "NotWired" {
		t.Errorf("embedded NotWired: %v", err)
	}
}

type stubTools struct{ NotWired }

func (stubTools) LatestVersion(context.Context) (string, error) { return "v1", nil }

func TestStateText(t *testing.T) {
	for s, want := range map[service.State]backend.ServiceState{
		service.NotInstalled: "", service.Running: "Running", service.Stopped: "Stopped",
		service.StartPending: "StartPending", service.StopPending: "StopPending", service.Paused: "Paused",
	} {
		if got := stateText(true, s); got != want {
			t.Errorf("%v: %q", s, got)
		}
	}
	if stateText(false, service.Running) != "" {
		t.Error("not installed")
	}
}
