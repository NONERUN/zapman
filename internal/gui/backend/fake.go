package backend

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NONERUN/zapman/internal/spec"
)

// FakeData is the state of a Fake.
type FakeData struct {
	Version    Version
	Settings   Settings
	ConfigErr  error
	Status     Status
	Strategies []StrategyInfo
	Games      []GameInfo
	// EngineMissing are the engines with no files in bin.
	EngineMissing map[spec.Engine]bool
	Fakes         FakeCatalog
	Hosts         HostsCheck
	Latest        string
	Diag          []DiagItem
	// TCPTimestamps tells if the TCP timestamps are on. A start of the
	// bypass turns them on. The item "tcp" of Diag shows it.
	TCPTimestamps bool
	// IPSet is the IPSet Filter mode of user/ipset-all.txt.
	IPSet IPSetMode
	// IPSetBackup tells if the loaded list is in the backup.
	IPSetBackup bool
	// Foreign are the winws processes that zapman did not start. Run and
	// Install clear them.
	Foreign []string
	// TrayTask tells if the scheduled task of the tray is there.
	TrayTask bool
	// Counters of the actions that have no state.
	IPSetDownloads int
	HostsOpened    int
	Reboots        int
	NetworkResets  int
}

// Fake is a Backend in memory. It does not touch the system, the disk or
// the network. Its results are always the same for the same calls.
type Fake struct {
	// Delay is the time of each step of a long action: a download step, a
	// test step. Zero for the tests.
	Delay time.Duration

	mu    sync.Mutex
	d     FakeData
	fail  map[string]error
	calls []string
}

var _ Backend = (*Fake)(nil)

// NewFake gives a Fake with a sample state: no service, nothing runs, both
// engines have files.
func NewFake() *Fake {
	return &Fake{d: FakeData{
		Version: Version{App: "v0.0.0-dev", Spec: "2", ReleasesURL: "https://github.com/NONERUN/zapman/releases"},
		Settings: Settings{
			Engine:          spec.Winws,
			GameFilter:      GameFilterDisabled,
			AutoUpdateCheck: true,
		},
		IPSet:  IPSetLoaded,
		Status: Status{BypassName: ExeName(spec.Winws), DivertSys: true},
		Strategies: []StrategyInfo{
			{Name: "general", Engines: []spec.Engine{spec.Winws, spec.Winws2}},
			{Name: "general (ALT)", Engines: []spec.Engine{spec.Winws, spec.Winws2}},
			{Name: "general (ALT2)", Engines: []spec.Engine{spec.Winws}},
			{Name: "general (FAKE TLS AUTO)", Engines: []spec.Engine{spec.Winws, spec.Winws2}},
		},
		Games: []GameInfo{{ID: "sky", Title: "Sky"}, {ID: "sker", Title: "Sker Ritual"}},
		Fakes: FakeCatalog{
			Files:        []string{"quic_initial_www_google_com.bin", "stun.bin", "tls_clienthello_www_google_com.bin"},
			Discord:      "stun.bin",
			DiscordState: FakeMatched,
			GameState:    FakeMissing,
		},
		Hosts:  HostsCheck{NeedsUpdate: true, Template: "# zapman\n127.0.0.1 example.invalid\n"},
		Latest: "v0.0.0-dev",
		Diag: []DiagItem{
			{ID: "path", Status: DiagOK, Text: `Zapret Manager is installed in: 'C:\zapman'`},
			{ID: "bfe", Status: DiagOK, Text: "Base Filtering Engine is running"},
			{ID: "proxy", Status: DiagOK, Text: "System proxy is off"},
			{ID: "tcp"},
			{ID: "doh", Status: DiagWarn, Text: "Set Secure DNS in the browser (non-default provider) or Windows 11 Settings"},
			{ID: "conflicts", Status: DiagFail, Text: "Conflicting bypass services: GoodbyeDPI", Action: "remove-services", ActionNames: []string{"GoodbyeDPI"}},
			{ID: "discord", Status: DiagWarn, Text: "Discord cache found: Stable", Action: "discord-cache"},
		},
	}}
}

// ExeName gives the exe of the engine.
func ExeName(e spec.Engine) string { return string(e) + ".exe" }

// Data gives a copy of the state.
func (f *Fake) Data() FakeData {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.d.clone()
}

// Update changes the state.
func (f *Fake) Update(fn func(d *FakeData)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(&f.d)
}

// FailNext makes the next call of the method return err.
func (f *Fake) FailNext(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == nil {
		f.fail = map[string]error{}
	}
	f.fail[method] = err
}

// Calls gives the names of the methods that changed the state or the
// system, in call order.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// ResetCalls clears the list of Calls.
func (f *Fake) ResetCalls() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

// begin locks f, records the call and gives the injected error. The caller
// must call f.mu.Unlock.
func (f *Fake) begin(method string, record bool) error {
	f.mu.Lock()
	if record {
		f.calls = append(f.calls, method)
	}
	if err, ok := f.fail[method]; ok {
		delete(f.fail, method)
		return err
	}
	return nil
}

func (d *FakeData) clone() FakeData {
	c := *d
	c.Settings.Games = slices.Clone(d.Settings.Games)
	c.Status.BypassExes = slices.Clone(d.Status.BypassExes)
	c.Strategies = slices.Clone(d.Strategies)
	c.Games = slices.Clone(d.Games)
	c.Fakes.Files = slices.Clone(d.Fakes.Files)
	c.Diag = slices.Clone(d.Diag)
	c.Foreign = slices.Clone(d.Foreign)
	c.EngineMissing = map[spec.Engine]bool{}
	for k, v := range d.EngineMissing {
		c.EngineMissing[k] = v
	}
	return c
}

// Version implements Backend.
func (f *Fake) Version() Version {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.d.Version
}

// LoadSettings implements Backend.
func (f *Fake) LoadSettings(context.Context) (Settings, error) {
	err := f.begin("LoadSettings", false)
	defer f.mu.Unlock()
	if err != nil {
		return Settings{}, err
	}
	s := f.d.clone().Settings
	if f.d.ConfigErr != nil {
		return s, &ConfigError{Err: f.d.ConfigErr}
	}
	return s, nil
}

// SaveSettings implements Backend.
func (f *Fake) SaveSettings(_ context.Context, s Settings) error {
	err := f.begin("SaveSettings", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if s.Engine != spec.Winws && s.Engine != spec.Winws2 {
		return fmt.Errorf("unknown engine %q", s.Engine)
	}
	if !slices.Contains(GameFilterModes, s.GameFilter) {
		return fmt.Errorf("unknown Game Filter mode %q", s.GameFilter)
	}
	for _, id := range s.Games {
		if !slices.ContainsFunc(f.d.Games, func(g GameInfo) bool { return g.ID == id }) {
			return fmt.Errorf("unknown game %q", id)
		}
	}
	s.Games = slices.Clone(s.Games)
	sort.Strings(s.Games)
	s.Games = slices.Compact(s.Games)
	f.d.Settings = s
	f.d.ConfigErr = nil
	if !f.d.Status.BypassRunning && !f.d.Status.HasService() {
		f.d.Status.BypassName = ExeName(s.Engine)
	}
	return nil
}

// SetTrayAtLogon implements Backend.
func (f *Fake) SetTrayAtLogon(_ context.Context, on bool) error {
	err := f.begin("SetTrayAtLogon", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	f.d.TrayTask = on
	return nil
}

// IPSet implements Backend.
func (f *Fake) IPSet(context.Context) (IPSetMode, error) {
	err := f.begin("IPSet", false)
	defer f.mu.Unlock()
	if err != nil {
		return "", err
	}
	return f.d.IPSet, nil
}

// SetIPSet implements Backend. It follows the rules of the backup of
// internal/engine.
func (f *Fake) SetIPSet(_ context.Context, mode IPSetMode) error {
	err := f.begin("SetIPSet", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if !slices.Contains(IPSetModes, mode) {
		return fmt.Errorf("unknown IPSet Filter mode %q", mode)
	}
	if mode == f.d.IPSet {
		return nil
	}
	if f.d.IPSet == IPSetLoaded {
		f.d.IPSetBackup = true
	}
	if mode == IPSetLoaded {
		if !f.d.IPSetBackup {
			return Msg("IpsetNoBackup")
		}
		f.d.IPSetBackup = false
	}
	f.d.IPSet = mode
	return nil
}

// ForeignEngines implements Backend.
func (f *Fake) ForeignEngines(context.Context) ([]string, error) {
	err := f.begin("ForeignEngines", false)
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return slices.Clone(f.d.Foreign), nil
}

// Status implements Backend.
func (f *Fake) Status(context.Context) (Status, error) {
	err := f.begin("Status", false)
	defer f.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	return f.d.clone().Status, nil
}

// Strategies implements Backend.
func (f *Fake) Strategies(context.Context) ([]StrategyInfo, error) {
	err := f.begin("Strategies", false)
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	list := f.d.clone().Strategies
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// Games implements Backend.
func (f *Fake) Games(context.Context) ([]GameInfo, error) {
	err := f.begin("Games", false)
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return slices.Clone(f.d.Games), nil
}

// CheckEngine implements Backend.
func (f *Fake) CheckEngine(_ context.Context, e spec.Engine) error {
	err := f.begin("CheckEngine", false)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	return f.checkEngine(e)
}

func (f *Fake) checkEngine(e spec.Engine) error {
	if !f.d.EngineMissing[e] {
		return nil
	}
	if e == spec.Winws2 {
		return Msg("EngineNoWinws2")
	}
	return Msg("EngineNoWinws")
}

func (f *Fake) strategy(name string, e spec.Engine) error {
	i := slices.IndexFunc(f.d.Strategies, func(s StrategyInfo) bool { return s.Name == name })
	if i < 0 {
		return fmt.Errorf("strategy %q is not in the catalog", name)
	}
	if !f.d.Strategies[i].Supports(e) {
		return Msg("EngineNoFlags", string(e))
	}
	return f.checkEngine(e)
}

// startProcess sets the state of a running process of the engine.
func (f *Fake) startProcess(name string, e spec.Engine) {
	f.d.TCPTimestamps = true
	st := &f.d.Status
	st.BypassRunning = true
	st.BypassName = ExeName(e)
	st.BypassExes = []string{ExeName(e)}
	st.Running = name
	st.Divert = ServiceRunning
}

func (f *Fake) stopProcess() {
	st := &f.d.Status
	st.BypassRunning = false
	st.BypassExes = nil
	st.Running = ""
	if st.Service == ServiceRunning {
		st.Service = ServiceStopped
	}
}

// StartService implements Backend.
func (f *Fake) StartService(context.Context) error {
	err := f.begin("StartService", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	st := &f.d.Status
	if !st.HasService() {
		return errors.New("the zapret service is not installed")
	}
	f.startProcess(st.Installed, f.d.Settings.Engine)
	st.Service = ServiceRunning
	return nil
}

// Stop implements Backend.
func (f *Fake) Stop(context.Context) error {
	err := f.begin("Stop", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	f.stopProcess()
	return nil
}

// RemoveServices implements Backend.
func (f *Fake) RemoveServices(context.Context) error {
	err := f.begin("RemoveServices", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	f.removeServices()
	return nil
}

func (f *Fake) removeServices() {
	f.stopProcess()
	st := &f.d.Status
	st.Service = ServiceNone
	st.Installed = ""
	st.Divert = ServiceNone
	st.BypassName = ExeName(f.d.Settings.Engine)
}

// Run implements Backend.
func (f *Fake) Run(_ context.Context, name string, e spec.Engine) error {
	err := f.begin("Run", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if err := f.strategy(name, e); err != nil {
		return err
	}
	f.stopProcess()
	f.d.Foreign = nil
	f.d.Settings.Engine = e
	f.startProcess(name, e)
	return nil
}

// Install implements Backend.
func (f *Fake) Install(_ context.Context, name string, e spec.Engine) error {
	err := f.begin("Install", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if err := f.strategy(name, e); err != nil {
		return err
	}
	f.removeServices()
	f.d.Foreign = nil
	f.d.Settings.Engine = e
	f.d.Settings.Strategy = name
	f.startProcess(name, e)
	f.d.Status.Service = ServiceRunning
	f.d.Status.Installed = name
	return nil
}

// SuspendService implements Backend.
func (f *Fake) SuspendService(context.Context) (ServiceSnapshot, error) {
	err := f.begin("SuspendService", true)
	defer f.mu.Unlock()
	if err != nil {
		return ServiceSnapshot{}, err
	}
	st := f.d.Status
	if !st.HasService() {
		return ServiceSnapshot{}, nil
	}
	snap := ServiceSnapshot{Strategy: st.Installed, Engine: f.d.Settings.Engine, Running: st.Service == ServiceRunning}
	f.removeServices()
	return snap, nil
}

// RestoreService implements Backend.
func (f *Fake) RestoreService(_ context.Context, snap ServiceSnapshot) error {
	err := f.begin("RestoreService", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if snap.Strategy == "" {
		return nil
	}
	if err := f.strategy(snap.Strategy, snap.Engine); err != nil {
		return err
	}
	f.removeServices()
	f.d.Settings.Engine = snap.Engine
	f.d.Status.Installed = snap.Strategy
	f.d.Status.Service = ServiceStopped
	f.d.Status.BypassName = ExeName(snap.Engine)
	if snap.Running {
		f.startProcess(snap.Strategy, snap.Engine)
		f.d.Status.Service = ServiceRunning
	}
	return nil
}

// NetworkReset implements Backend.
func (f *Fake) NetworkReset(context.Context) (string, error) {
	err := f.begin("NetworkReset", true)
	defer f.mu.Unlock()
	text := strings.Join([]string{
		"netsh winsock reset: OK",
		"netsh int ip reset all: OK",
		"netsh winhttp reset proxy: OK",
		"ipconfig /flushdns: OK",
	}, "\n")
	if err != nil {
		return "netsh winsock reset: FAIL", err
	}
	f.d.NetworkResets++
	return text, nil
}

// Reboot implements Backend. The fake only counts the calls.
func (f *Fake) Reboot(context.Context) error {
	err := f.begin("Reboot", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	f.d.Reboots++
	return nil
}

// Fakes implements Backend.
func (f *Fake) Fakes(context.Context) (FakeCatalog, error) {
	err := f.begin("Fakes", false)
	defer f.mu.Unlock()
	if err != nil {
		return FakeCatalog{}, err
	}
	return f.d.clone().Fakes, nil
}

// SetFake implements Backend.
func (f *Fake) SetFake(_ context.Context, slot FakeSlot, file string) error {
	err := f.begin("SetFake", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if !slices.Contains(f.d.Fakes.Files, file) {
		return fmt.Errorf("fake file %q is not in bin", file)
	}
	switch slot {
	case FakeDiscord:
		f.d.Fakes.Discord, f.d.Fakes.DiscordState = file, FakeMatched
	case FakeGame:
		f.d.Fakes.Game, f.d.Fakes.GameState = file, FakeMatched
	default:
		return fmt.Errorf("unknown fake slot %q", slot)
	}
	return nil
}

// download sends the progress of a fake download of 4 steps. The first
// step has no total, as a server with no Content-Length at the start.
func (f *Fake) download(ctx context.Context, progress Progress) error {
	const size = 64 << 10
	for i := range 5 {
		if err := f.wait(ctx); err != nil {
			return err
		}
		if progress == nil {
			continue
		}
		if i == 0 {
			progress(0, -1)
			continue
		}
		progress(int64(i)*size/4, size)
	}
	return nil
}

// wait waits for Delay or for the end of ctx.
func (f *Fake) wait(ctx context.Context) error {
	if f.Delay <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(f.Delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// DownloadIPSet implements Backend.
func (f *Fake) DownloadIPSet(ctx context.Context, progress Progress) error {
	err := f.begin("DownloadIPSet", true)
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if err := f.download(ctx, progress); err != nil {
		return err
	}
	f.Update(func(d *FakeData) { d.IPSetDownloads++ })
	return nil
}

// CheckHosts implements Backend.
func (f *Fake) CheckHosts(ctx context.Context, progress Progress) (HostsCheck, error) {
	err := f.begin("CheckHosts", false)
	h := f.d.Hosts
	f.mu.Unlock()
	if err != nil {
		return HostsCheck{}, err
	}
	if err := f.download(ctx, progress); err != nil {
		return HostsCheck{}, err
	}
	return h, nil
}

// OpenHosts implements Backend.
func (f *Fake) OpenHosts(context.Context) error {
	err := f.begin("OpenHosts", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	f.d.HostsOpened++
	return nil
}

// LatestVersion implements Backend.
func (f *Fake) LatestVersion(ctx context.Context) (string, error) {
	err := f.begin("LatestVersion", false)
	v := f.d.Latest
	f.mu.Unlock()
	if err != nil {
		return "", err
	}
	if err := f.wait(ctx); err != nil {
		return "", err
	}
	return v, nil
}

// Diagnostics implements Backend.
func (f *Fake) Diagnostics(context.Context) ([]DiagItem, error) {
	err := f.begin("Diagnostics", false)
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	items := f.d.clone().Diag
	for i := range items {
		if items[i].ID != "tcp" {
			continue
		}
		if f.d.TCPTimestamps {
			items[i].Status, items[i].Text = DiagOK, "TCP timestamps are enabled"
		} else {
			items[i].Status, items[i].Text = DiagFail, "TCP timestamps are disabled"
		}
	}
	return items, nil
}

// RunDiagAction implements Backend. The action makes the item "ok".
func (f *Fake) RunDiagAction(_ context.Context, item DiagItem) error {
	err := f.begin("RunDiagAction", true)
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	switch item.Action {
	case "remove-services", "discord-cache", "remove-windivert":
	default:
		return fmt.Errorf("unknown diagnostics action %q", item.Action)
	}
	for i := range f.d.Diag {
		d := &f.d.Diag[i]
		if d.ID == item.ID && d.Action == item.Action {
			d.Status, d.Action, d.ActionNames = DiagOK, "", nil
			d.Text = "Fixed: " + d.Text
		}
	}
	return nil
}

// RunTests implements Backend. The results come from the names: the
// strategy at index 1 does not start, the others get fixed statuses.
func (f *Fake) RunTests(ctx context.Context, req TestRequest, sink TestSink) error {
	err := f.begin("RunTests", true)
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if len(req.Strategies) == 0 {
		return Msg("TestsNeedOne")
	}
	if req.Type != TestStandard && req.Type != TestDPI {
		return fmt.Errorf("unknown test type %q", req.Type)
	}
	sink.Line("Engine: " + ExeName(req.Engine))
	sink.Line("Targets: 3")
	sum := TestSummary{Type: req.Type}
	bestOK := -1
	n := len(req.Strategies)
	for i, name := range req.Strategies {
		if err := f.wait(ctx); err != nil {
			return err
		}
		sink.Line(fmt.Sprintf("[%d/%d] %s", i+1, n, name))
		r := fakeResult(req.Type, i, name)
		sink.Strategy(r)
		if r.Kind == "fail" {
			continue
		}
		row := sumRow(r)
		sum.Rows = append(sum.Rows, row)
		if row.OK > bestOK {
			bestOK, sum.Best = row.OK, name
		}
	}
	if err := f.wait(ctx); err != nil {
		return err
	}
	sink.Summary(sum)
	return nil
}

func fakeResult(t TestType, i int, name string) TestResult {
	if i == 1 {
		return TestResult{Name: name, Kind: "fail", Error: ExeName(spec.Winws) + " did not start."}
	}
	r := TestResult{Name: name, Kind: string(t)}
	if t == TestDPI {
		r.Targets = []TestTarget{
			{Name: "US.CF-01", HTTP: "OK", TLS12: "OK", TLS13: "OK"},
			{Name: "DE.HE-01", HTTP: "OK", TLS12: "LIKELY_BLOCKED", TLS13: "OK"},
			{Name: "RU.YA-01", HTTP: "UNSUPPORTED", TLS12: "OK", TLS13: "FAIL"},
		}
		if i == 0 {
			r.Targets[2].TLS13 = "OK"
		}
		return r
	}
	r.Targets = []TestTarget{
		{Name: "Discord", HTTP: "OK", TLS12: "OK", TLS13: "OK", Ping: "24 ms"},
		{Name: "YouTube", HTTP: "OK", TLS12: "ERROR", TLS13: "OK", Ping: "31 ms"},
		{Name: "Google", HTTP: "OK", TLS12: "OK", TLS13: "UNSUP", Ping: "Timeout"},
	}
	if i == 0 {
		r.Targets[1].TLS12 = "OK"
	}
	return r
}

// sumRow counts the statuses of a result as the PS summary does.
func sumRow(r TestResult) TestSumRow {
	row := TestSumRow{Name: r.Name}
	for _, t := range r.Targets {
		for _, v := range []string{t.HTTP, t.TLS12, t.TLS13} {
			switch v {
			case "OK":
				row.OK++
			case "ERROR":
				row.Err++
			case "UNSUP", "UNSUPPORTED":
				row.Unsup++
			case "FAIL":
				row.Fail++
			case "LIKELY_BLOCKED":
				row.Blocked++
			}
		}
		if r.Kind == string(TestStandard) {
			if t.Ping != "Timeout" && t.Ping != "n/a" {
				row.PingOK++
			} else {
				row.PingFail++
			}
		}
	}
	return row
}
