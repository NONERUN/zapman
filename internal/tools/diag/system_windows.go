package diag

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/NONERUN/zapman/internal/service"
)

// NewSystem gives the System of this computer. The tests must not use it.
func NewSystem() System {
	h := service.NewHost()
	return winSystem{host: h, mgr: &service.Manager{Host: h}}
}

type winSystem struct {
	host service.Host
	mgr  *service.Manager
}

func (s winSystem) ServiceState(name string) (service.State, error) {
	st, err := s.host.Query(name)
	return st.State, err
}

func (s winSystem) TCPTimestamps() (bool, error) { return s.mgr.TCPTimestamps() }

// Services calls EnumServicesStatusEx with SERVICE_WIN32: it gives the
// names and the display names in one call.
func (winSystem) Services() ([]Service, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, fmt.Errorf("open the service manager: %w", err)
	}
	defer windows.CloseServiceHandle(scm) //nolint:errcheck // a close of a read handle.
	var buf []byte
	var size, needed, count uint32
	for {
		var p *byte
		if len(buf) > 0 {
			p = &buf[0]
		}
		err = windows.EnumServicesStatusEx(scm, windows.SC_ENUM_PROCESS_INFO, windows.SERVICE_WIN32,
			windows.SERVICE_STATE_ALL, p, size, &needed, &count, nil, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_MORE_DATA) || needed <= size {
			return nil, err
		}
		buf = make([]byte, needed)
		size = needed
	}
	if count == 0 {
		return nil, nil
	}
	ents := unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0])), count) //nolint:gosec // G103: the API writes an array of ENUM_SERVICE_STATUS_PROCESS in buf.
	out := make([]Service, len(ents))
	for i, e := range ents {
		out[i] = Service{Name: windows.UTF16PtrToString(e.ServiceName), DisplayName: windows.UTF16PtrToString(e.DisplayName)}
	}
	return out, nil
}

// processes gives the process IDs of each exe name in lower case.
func processes() (map[string][]uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a close of a snapshot.
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	out := map[string][]uint32{}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		n := strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))
		out[n] = append(out[n], e.ProcessID)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return out, nil
}

func (winSystem) ProcessRunning(exes ...string) (bool, error) {
	ps, err := processes()
	if err != nil {
		return false, err
	}
	for _, x := range exes {
		if len(ps[strings.ToLower(x)]) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// killWait is the time in milliseconds that KillProcesses waits for each
// process to stop.
const killWait = 5000

func (winSystem) KillProcesses(exe string) error {
	ps, err := processes()
	if err != nil {
		return err
	}
	var errs []error
	for _, pid := range ps[strings.ToLower(exe)] {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue // the process stopped
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("open process %d: %w", pid, err))
			continue
		}
		if err := windows.TerminateProcess(h, 1); err != nil {
			errs = append(errs, fmt.Errorf("stop process %d: %w", pid, err))
		} else {
			_, _ = windows.WaitForSingleObject(h, killWait)
		}
		_ = windows.CloseHandle(h)
	}
	return errors.Join(errs...)
}

const internetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func (winSystem) Proxy() (bool, string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE)
	if err != nil {
		return false, "", err
	}
	defer k.Close() //nolint:errcheck // a close of a read key.
	on, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || on != 1 {
		return false, "", nil // a missing value is "proxy off", as in the PowerShell version
	}
	server, _, _ := k.GetStringValue("ProxyServer")
	return true, server, nil
}

const dohRoot = `SYSTEM\CurrentControlSet\Services\Dnscache\InterfaceSpecificParameters`

// maxDohDepth limits the walk of the keys. The real tree has 3 levels:
// interface, DohInterfaceSettings, Doh (or Doh6), server.
const maxDohDepth = 8

func (winSystem) SecureDNS() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, dohRoot, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer k.Close() //nolint:errcheck // a close of a read key.
	return dohIn(k, 0), nil
}

// dohIn tells if a subkey of k (at all levels) has DohFlags > 0.
func dohIn(k registry.Key, depth int) bool {
	if depth >= maxDohDepth {
		return false
	}
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}
	for _, n := range names {
		sub, err := registry.OpenKey(k, n, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		v, _, err := sub.GetIntegerValue("DohFlags")
		found := (err == nil && v > 0) || dohIn(sub, depth+1)
		_ = sub.Close()
		if found {
			return true
		}
	}
	return false
}
