// Package diag is the diagnostics report of zapman and its actions. It is
// the port of Get-ZapmanDiagnosticReport and Invoke-ZapmanDiagnosticAction
// of Tools.ps1: the same checks in the same order, with the same IDs and
// texts. The checks read the system through System, so the tests do not
// read the real services, processes or registry.
package diag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/tools/command"
)

// Status is the result of a check.
type Status string

// The statuses.
const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
)

// The actions of the items.
const (
	ActionRemoveServices  = "remove-services"
	ActionDiscordCache    = "discord-cache"
	ActionRemoveWinDivert = "remove-windivert"
)

// Item is one line of the report.
type Item struct {
	ID     string
	Status Status
	Text   string
	// Action is "" or one of the Action* constants.
	Action string
	// ActionNames are the services of ActionRemoveServices.
	ActionNames []string
}

// Service is a Win32 service of the list.
type Service struct {
	Name, DisplayName string
}

// System reads and changes the computer. NewSystem gives the real one.
type System interface {
	// Services lists the Win32 services, not the drivers, as Get-Service.
	Services() ([]Service, error)
	// ServiceState gives the state of the service or the driver name. When
	// it does not exist, the error is service.ErrNotInstalled (errors.Is).
	ServiceState(name string) (service.State, error)
	// ProcessRunning tells if a process with one of the exe names (for
	// example "winws.exe") runs.
	ProcessRunning(exes ...string) (bool, error)
	// KillProcesses stops all processes with the exe name.
	KillProcesses(exe string) error
	// Proxy gives ProxyEnable == 1 and ProxyServer of the Internet
	// Settings of the current user.
	Proxy() (on bool, server string, err error)
	// SecureDNS tells if a key under Dnscache\InterfaceSpecificParameters
	// has DohFlags > 0.
	SecureDNS() (bool, error)
	// TCPTimestamps is service.Manager.TCPTimestamps.
	TCPTimestamps() (bool, error)
}

// Checker makes the report and does the actions.
type Checker struct {
	Sys System
	// Run runs net.exe and sc.exe for the actions.
	Run command.Func
	// Root is the zapman folder; Bin is its bin folder.
	Root, Bin string
	// HostsPath is the system hosts file.
	HostsPath string
	// AppData and OneDrive are the values of %APPDATA% and %OneDrive%.
	AppData, OneDrive string
	// Sleep waits between the delete of a service and the check; nil is
	// time.Sleep.
	Sleep func(time.Duration)
}

// New gives the Checker of this computer for the zapman folder root with
// the bin folder bin (engine.Layout.Root and .Bin).
func New(root, bin string) *Checker {
	return &Checker{
		Sys:       NewSystem(),
		Run:       command.Run,
		Root:      root,
		Bin:       bin,
		HostsPath: filepath.Join(windowsDir(), "System32", "drivers", "etc", "hosts"),
		AppData:   os.Getenv("APPDATA"),
		OneDrive:  os.Getenv("OneDrive"),
	}
}

func windowsDir() string {
	if d := os.Getenv("SystemRoot"); d != "" {
		return d
	}
	return `C:\Windows`
}

// The services of other bypass tools that conflict with zapret.
var conflictServices = []string{"GoodbyeDPI", "discordfix_zapret", "winws1", "winws2"}

// The WinDivert drivers.
var winDivert = []string{"WinDivert", "WinDivert14"}

var (
	reKiller    = regexp.MustCompile(`(?i)Killer`)
	reIntel     = []*regexp.Regexp{regexp.MustCompile(`(?i)Intel`), regexp.MustCompile(`(?i)Connectivity`), regexp.MustCompile(`(?i)Network`)}
	reCheck     = regexp.MustCompile(`(?i)TracSrvWrapper|EPWD`)
	reSmartByte = regexp.MustCompile(`(?i)SmartByte`)
	reVPN       = regexp.MustCompile(`(?i)VPN|HappService`)
	reCyrillic  = regexp.MustCompile(`[\x{0400}-\x{04FF}]`)
	reYouTube   = regexp.MustCompile(`(?i)youtube\.com|youtu\.be`)
)

func item(id string, ok bool, okText string, st Status, badText string) Item {
	if ok {
		return Item{ID: id, Status: OK, Text: okText}
	}
	return Item{ID: id, Status: st, Text: badText}
}

func anyService(svcs []Service, f func(Service) bool) bool {
	for _, s := range svcs {
		if f(s) {
			return true
		}
	}
	return false
}

// exists tells if the service or driver name exists. An error of the query
// counts as "does not exist", as Get-Service -ErrorAction SilentlyContinue.
func (c *Checker) exists(name string) bool {
	_, err := c.Sys.ServiceState(name)
	return err == nil
}

// Report gives the items in the order of the PowerShell version. As there,
// an error of one check does not stop the report: the check gives its
// "not found" result. Report gives an error only when ctx stops.
func (c *Checker) Report(ctx context.Context) ([]Item, error) {
	var items []Item
	add := func(it Item) { items = append(items, it) }

	add(Item{ID: "path", Status: OK, Text: fmt.Sprintf("Zapret Manager is installed in: '%s'", c.Root)})

	st, err := c.Sys.ServiceState("BFE")
	add(item("bfe", err == nil && st == service.Running,
		"Base Filtering Engine is running", Fail, "Base Filtering Engine is not running. This service is required"))

	if on, server, err := c.Sys.Proxy(); err == nil && on {
		add(Item{ID: "proxy", Status: Warn, Text: fmt.Sprintf("System proxy is enabled: %s. Disable it if you do not use a proxy", server)})
	} else {
		add(Item{ID: "proxy", Status: OK, Text: "System proxy is off"})
	}

	ts, err := c.Sys.TCPTimestamps()
	add(item("tcp", err == nil && ts, "TCP timestamps are enabled", Fail, "TCP timestamps are disabled"))
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	adguard, _ := c.Sys.ProcessRunning("AdguardSvc.exe")
	add(item("adguard", !adguard, "Adguard is not running", Fail, "Adguard process found. Adguard may block Discord"))

	svcs, _ := c.Sys.Services()
	add(item("killer", !anyService(svcs, func(s Service) bool { return reKiller.MatchString(s.DisplayName) || reKiller.MatchString(s.Name) }),
		"No Killer services", Fail, "Killer services found. Killer conflicts with zapret"))
	add(item("intel", !anyService(svcs, func(s Service) bool {
		for _, re := range reIntel {
			if !re.MatchString(s.DisplayName) {
				return false
			}
		}
		return true
	}), "No Intel Connectivity Network Service", Fail, "Intel Connectivity Network Service found. It conflicts with zapret"))
	add(item("checkpoint", !anyService(svcs, func(s Service) bool { return reCheck.MatchString(s.Name) }),
		"No Check Point services", Fail, "Check Point services found. Uninstall Check Point"))
	add(item("smartbyte", !anyService(svcs, func(s Service) bool { return reSmartByte.MatchString(s.DisplayName) || reSmartByte.MatchString(s.Name) }),
		"No SmartByte services", Fail, "SmartByte services found. Disable SmartByte in services.msc"))

	add(item("cyrillic", !reCyrillic.MatchString(c.Root),
		"Path has no Cyrillic characters", Warn, `The path contains Cyrillic characters. Move Zapret to C:\zapret if bypass fails`))
	add(item("onedrive", !inOneDrive(c.Root, c.OneDrive),
		"Zapret is not in OneDrive", Fail, `Zapret is in a OneDrive folder. Move it to C:\zapret`))

	sys, _ := filepath.Glob(filepath.Join(c.Bin, "*.sys"))
	add(item("sys", len(sys) > 0, "WinDivert64.sys is present", Fail, "WinDivert64.sys file NOT found"))

	var vpn []string
	for _, s := range svcs {
		if reVPN.MatchString(s.DisplayName) || reVPN.MatchString(s.Name) {
			vpn = append(vpn, s.Name)
		}
	}
	add(item("vpn", len(vpn) == 0, "No VPN services", Warn,
		fmt.Sprintf("VPN services found: %s. Disable VPN if bypass fails", strings.Join(vpn, ", "))))

	doh, _ := c.Sys.SecureDNS()
	add(item("doh", doh, "Secure DNS is configured", Warn, "Set Secure DNS in the browser (non-default provider) or Windows 11 Settings"))

	if data, err := os.ReadFile(c.HostsPath); err != nil {
		add(Item{ID: "hosts", Status: OK, Text: "hosts file not found"})
	} else {
		add(item("hosts", !reYouTube.Match(data), "hosts has no YouTube overrides", Warn, "hosts contains youtube.com or youtu.be. This may break YouTube"))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var conflicts []string
	engines, _ := c.Sys.ProcessRunning("winws.exe", "winws2.exe")
	wd, err := c.Sys.ServiceState("WinDivert")
	if !engines && err == nil && (wd == service.Running || wd == service.StopPending) {
		add(Item{ID: "windivert", Status: Warn, Text: "winws.exe is not running but WinDivert is active", Action: ActionRemoveWinDivert})
		if c.exists("GoodbyeDPI") {
			conflicts = append(conflicts, "GoodbyeDPI")
		}
	} else {
		add(Item{ID: "windivert", Status: OK, Text: "No leftover WinDivert without winws"})
	}

	for _, n := range conflictServices {
		if !slices.Contains(conflicts, n) && c.exists(n) {
			conflicts = append(conflicts, n)
		}
	}
	if len(conflicts) > 0 {
		add(Item{ID: "conflicts", Status: Fail, Text: "Conflicting bypass services: " + strings.Join(conflicts, ", "),
			Action: ActionRemoveServices, ActionNames: conflicts})
	} else {
		add(Item{ID: "conflicts", Status: OK, Text: "No conflicting bypass services"})
	}

	if apps := c.discordApps(); len(apps) > 0 {
		labels := make([]string, len(apps))
		for i, a := range apps {
			labels[i] = a.label
		}
		add(Item{ID: "discord", Status: Warn, Text: "Discord cache found: " + strings.Join(labels, ", "), Action: ActionDiscordCache})
	} else {
		add(Item{ID: "discord", Status: OK, Text: "No Discord cache folders to clear"})
	}
	return items, nil
}

// inOneDrive is "root -like <OneDrive>\*": root is in the OneDrive folder.
// The compare ignores the case.
func inOneDrive(root, oneDrive string) bool {
	oneDrive = strings.TrimRight(oneDrive, `\`)
	if oneDrive == "" {
		return false
	}
	prefix := strings.ToLower(oneDrive + `\`)
	return strings.HasPrefix(strings.ToLower(root), prefix)
}

// Lines gives the text form of the report: "OK: ...", "WARN: ...",
// "FAIL: ...".
func Lines(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		p := "OK"
		switch it.Status {
		case Fail:
			p = "FAIL"
		case Warn:
			p = "WARN"
		}
		out[i] = p + ": " + it.Text
	}
	return out
}
