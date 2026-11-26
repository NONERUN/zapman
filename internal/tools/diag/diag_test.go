package diag

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/tools/command"
)

// fakeSys is a System in memory.
type fakeSys struct {
	services []Service
	// states are the services and drivers that exist.
	states  map[string]service.State
	running map[string]bool
	killed  []string
	proxy   bool
	server  string
	doh     bool
	ts      bool
	tsErr   error
	// onDelete is called by the fake "sc delete".
	deleted []string
}

func (f *fakeSys) Services() ([]Service, error) { return f.services, nil }
func (f *fakeSys) ServiceState(name string) (service.State, error) {
	st, ok := f.states[name]
	if !ok {
		return 0, service.ErrNotInstalled
	}
	return st, nil
}

func (f *fakeSys) ProcessRunning(exes ...string) (bool, error) {
	for _, x := range exes {
		if f.running[x] {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeSys) KillProcesses(exe string) error {
	f.killed = append(f.killed, exe)
	return nil
}
func (f *fakeSys) Proxy() (bool, string, error) { return f.proxy, f.server, nil }
func (f *fakeSys) SecureDNS() (bool, error)     { return f.doh, nil }
func (f *fakeSys) TCPTimestamps() (bool, error) { return f.ts, f.tsErr }

// run is the fake command.Func: "sc delete" removes the service from
// states, except the names in keep.
func (f *fakeSys) run(keep ...string) command.Func {
	return func(_ context.Context, exe string, args ...string) (command.Result, error) {
		f.deleted = append(f.deleted, filepath.Base(exe)+" "+strings.Join(args, " "))
		if filepath.Base(exe) == "sc.exe" && args[0] == "delete" && !slices.Contains(keep, args[1]) {
			delete(f.states, args[1])
		}
		return command.Result{}, nil
	}
}

func checker(t *testing.T, sys *fakeSys) *Checker {
	t.Helper()
	dir := t.TempDir()
	return &Checker{
		Sys:       sys,
		Run:       sys.run(),
		Root:      `C:\zapman`,
		Bin:       filepath.Join(dir, "bin"),
		HostsPath: filepath.Join(dir, "hosts"),
		AppData:   filepath.Join(dir, "AppData"),
		Sleep:     func(time.Duration) {},
	}
}

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

var order = []string{"path", "bfe", "proxy", "tcp", "adguard", "killer", "intel", "checkpoint", "smartbyte",
	"cyrillic", "onedrive", "sys", "vpn", "doh", "hosts", "windivert", "conflicts", "discord"}

func TestReportClean(t *testing.T) {
	sys := &fakeSys{states: map[string]service.State{"BFE": service.Running}, ts: true, doh: true,
		services: []Service{{"BFE", "Base Filtering Engine"}, {"Dnscache", "DNS Client"}}}
	c := checker(t, sys)
	if err := os.MkdirAll(c.Bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Bin, "WinDivert64.sys"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.HostsPath, []byte("127.0.0.1 localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := c.Report(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(items), order) {
		t.Fatalf("ids = %v", ids(items))
	}
	want := []string{
		`OK: Zapret Manager is installed in: 'C:\zapman'`,
		"OK: Base Filtering Engine is running",
		"OK: System proxy is off",
		"OK: TCP timestamps are enabled",
		"OK: Adguard is not running",
		"OK: No Killer services",
		"OK: No Intel Connectivity Network Service",
		"OK: No Check Point services",
		"OK: No SmartByte services",
		"OK: Path has no Cyrillic characters",
		"OK: Zapret is not in OneDrive",
		"OK: WinDivert64.sys is present",
		"OK: No VPN services",
		"OK: Secure DNS is configured",
		"OK: hosts has no YouTube overrides",
		"OK: No leftover WinDivert without winws",
		"OK: No conflicting bypass services",
		"OK: No Discord cache folders to clear",
	}
	if got := Lines(items); !slices.Equal(got, want) {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
	for _, it := range items {
		if it.Action != "" {
			t.Errorf("%s has action %q", it.ID, it.Action)
		}
	}
}

func TestReportProblems(t *testing.T) {
	sys := &fakeSys{
		states: map[string]service.State{
			"BFE": service.Stopped, "WinDivert": service.StopPending, "GoodbyeDPI": service.Stopped, "winws1": service.Running,
		},
		running: map[string]bool{"AdguardSvc.exe": true},
		proxy:   true, server: "127.0.0.1:8080",
		tsErr: errors.New("netsh failed"),
		services: []Service{
			{"KillerNetworkService", "Killer Network Service"},
			{"jhi_service", "Intel(R) Connectivity Network Service"},
			{"TracSrvWrapper", "Check Point"},
			{"SmartByteSvc", "SmartByte"},
			{"ProtonVPN Service", "Proton VPN"},
			{"HappService", "Happ"},
		},
	}
	c := checker(t, sys)
	c.Root = `C:\Users\Иван\OneDrive\zapman`
	c.OneDrive = `C:\Users\Иван\onedrive\`
	if err := os.WriteFile(c.HostsPath, []byte("1.2.3.4 www.YouTube.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.AppData, "discordptb", "GPUCache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.AppData, "discord", "Code Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	items, err := c.Report(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Status{
		"path": OK, "bfe": Fail, "proxy": Warn, "tcp": Fail, "adguard": Fail, "killer": Fail, "intel": Fail,
		"checkpoint": Fail, "smartbyte": Fail, "cyrillic": Warn, "onedrive": Fail, "sys": Fail, "vpn": Warn,
		"doh": Warn, "hosts": Warn, "windivert": Warn, "conflicts": Fail, "discord": Warn,
	}
	by := map[string]Item{}
	for _, it := range items {
		by[it.ID] = it
		if it.Status != want[it.ID] {
			t.Errorf("%s: status %s, want %s (%s)", it.ID, it.Status, want[it.ID], it.Text)
		}
	}
	for id, text := range map[string]string{
		"proxy":     "System proxy is enabled: 127.0.0.1:8080. Disable it if you do not use a proxy",
		"vpn":       "VPN services found: ProtonVPN Service, HappService. Disable VPN if bypass fails",
		"conflicts": "Conflicting bypass services: GoodbyeDPI, winws1",
		"discord":   "Discord cache found: Discord, Discord PTB",
	} {
		if by[id].Text != text {
			t.Errorf("%s: %q", id, by[id].Text)
		}
	}
	if it := by["conflicts"]; it.Action != ActionRemoveServices || !slices.Equal(it.ActionNames, []string{"GoodbyeDPI", "winws1"}) {
		t.Errorf("conflicts: %+v", it)
	}
	if by["windivert"].Action != ActionRemoveWinDivert || by["discord"].Action != ActionDiscordCache {
		t.Errorf("actions: %+v %+v", by["windivert"], by["discord"])
	}
}

func TestReportWinDivertWithEngine(t *testing.T) {
	sys := &fakeSys{states: map[string]service.State{"WinDivert": service.Running}, running: map[string]bool{"winws2.exe": true}}
	items, err := checker(t, sys).Report(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == "windivert" && it.Status != OK {
			t.Errorf("windivert: %+v", it)
		}
	}
}

func TestReportCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := checker(t, &fakeSys{}).Report(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestInOneDrive(t *testing.T) {
	for _, tc := range []struct {
		root, od string
		want     bool
	}{
		{`C:\Users\a\OneDrive\z`, `C:\Users\a\OneDrive`, true},
		{`C:\Users\a\OneDrive`, `C:\Users\a\OneDrive`, false},
		{`C:\Users\a\OneDriveX\z`, `C:\Users\a\OneDrive`, false},
		{`C:\z`, ``, false},
	} {
		if got := inOneDrive(tc.root, tc.od); got != tc.want {
			t.Errorf("inOneDrive(%q, %q) = %v", tc.root, tc.od, got)
		}
	}
}

func TestRemoveServices(t *testing.T) {
	sys := &fakeSys{states: map[string]service.State{"GoodbyeDPI": service.Running, "winws1": service.Stopped, "WinDivert": service.Running}}
	c := checker(t, sys)
	c.Run = sys.run("winws1")
	lines, err := c.Do(t.Context(), Item{Action: ActionRemoveServices, ActionNames: []string{"GoodbyeDPI", " ", "winws1"}})
	want := []string{"OK: Removed service: GoodbyeDPI", "FAIL: Failed to remove service: winws1"}
	if !slices.Equal(lines, want) {
		t.Errorf("lines = %q", lines)
	}
	var ae *ActionError
	if !errors.As(err, &ae) || err.Error() != "Failed to remove service: winws1" {
		t.Errorf("err = %v", err)
	}
	wantCalls := []string{
		"net.exe stop GoodbyeDPI", "sc.exe delete GoodbyeDPI", "net.exe stop winws1", "sc.exe delete winws1",
		"net.exe stop WinDivert", "sc.exe delete WinDivert", "net.exe stop WinDivert14", "sc.exe delete WinDivert14",
	}
	if !slices.Equal(sys.deleted, wantCalls) {
		t.Errorf("calls = %q", sys.deleted)
	}
}

func TestRemoveWinDivert(t *testing.T) {
	sys := &fakeSys{states: map[string]service.State{"WinDivert": service.Running}}
	lines, err := checker(t, sys).Do(t.Context(), Item{Action: ActionRemoveWinDivert})
	if err != nil || !slices.Equal(lines, []string{"OK: Removed service: WinDivert", "OK: Removed service: WinDivert14"}) {
		t.Errorf("lines = %q, err = %v", lines, err)
	}
}

func TestClearDiscordCache(t *testing.T) {
	sys := &fakeSys{}
	c := checker(t, sys)
	lines, err := c.Do(t.Context(), Item{Action: ActionDiscordCache})
	if err != nil || !slices.Equal(lines, []string{"WARN: No Discord cache folders found."}) {
		t.Errorf("no cache: %q, %v", lines, err)
	}
	cache := filepath.Join(c.AppData, "discordcanary", "Cache")
	if err := os.MkdirAll(filepath.Join(cache, "Cache_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	gpu := filepath.Join(c.AppData, "discordcanary", "GPUCache")
	if err := os.MkdirAll(gpu, 0o755); err != nil {
		t.Fatal(err)
	}
	lines, err = c.Do(t.Context(), Item{Action: ActionDiscordCache})
	if err != nil || !slices.Equal(lines, []string{"OK: Deleted " + cache, "OK: Deleted " + gpu}) {
		t.Errorf("lines = %q, %v", lines, err)
	}
	if !slices.Equal(sys.killed, []string{"DiscordCanary.exe"}) {
		t.Errorf("killed = %q", sys.killed)
	}
	if _, err := os.Stat(filepath.Join(c.AppData, "discordcanary")); err != nil {
		t.Errorf("the app folder is deleted: %v", err)
	}
}

func TestUnknownAction(t *testing.T) {
	if _, err := checker(t, &fakeSys{}).Do(t.Context(), Item{Action: "x"}); !errors.Is(err, ErrUnknownAction) {
		t.Errorf("err = %v", err)
	}
}
