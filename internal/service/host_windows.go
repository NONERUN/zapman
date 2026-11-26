package service

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// NewHost gives the Host of this computer. The tests must not use it: it
// changes the real services.
func NewHost() Host { return windowsHost{} }

type windowsHost struct{}

const servicesKey = `SYSTEM\CurrentControlSet\Services\`

func (windowsHost) Elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// notInstalled changes ERROR_SERVICE_DOES_NOT_EXIST to ErrNotInstalled.
func notInstalled(name string, err error) error {
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	return err
}

// Query opens the SCM and the service only for a read, thus it does not
// need administrator rights.
func (windowsHost) Query(name string) (ServiceStatus, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return ServiceStatus{}, fmt.Errorf("open the service manager: %w", err)
	}
	defer windows.CloseServiceHandle(scm) //nolint:errcheck // a close of a read handle.
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return ServiceStatus{}, err
	}
	h, err := windows.OpenService(scm, n, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return ServiceStatus{}, notInstalled(name, err)
	}
	defer windows.CloseServiceHandle(h) //nolint:errcheck // a close of a read handle.
	var st windows.SERVICE_STATUS_PROCESS
	var need uint32
	if err := windows.QueryServiceStatusEx(h, windows.SC_STATUS_PROCESS_INFO,
		(*byte)(unsafe.Pointer(&st)), uint32(unsafe.Sizeof(st)), &need); err != nil { //nolint:gosec // G103: the API takes a pointer to SERVICE_STATUS_PROCESS as *byte.
		return ServiceStatus{}, err
	}
	code := st.Win32ExitCode
	if code == uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR) {
		code = st.ServiceSpecificExitCode
	}
	return ServiceStatus{State: State(st.CurrentState), ExitCode: code, Pid: st.ProcessId}, nil
}

func connect() (*mgr.Mgr, error) {
	m, err := mgr.Connect()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil, ErrNotElevated
	}
	return m, err
}

// withService opens the service name with all access and calls f.
func withService(name string, f func(s *mgr.Service) error) error {
	m, err := connect()
	if err != nil {
		return err
	}
	defer m.Disconnect() //nolint:errcheck // the result of f is the result.
	s, err := m.OpenService(name)
	if err != nil {
		return notInstalled(name, err)
	}
	defer s.Close() //nolint:errcheck // the result of f is the result.
	return f(s)
}

// ReadConfig opens the SCM and the service only for a read, thus it does
// not need administrator rights.
func (windowsHost) ReadConfig(name string) (Config, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return Config{}, fmt.Errorf("open the service manager: %w", err)
	}
	defer windows.CloseServiceHandle(scm) //nolint:errcheck // a close of a read handle.
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return Config{}, err
	}
	h, err := windows.OpenService(scm, n, windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return Config{}, notInstalled(name, err)
	}
	s := &mgr.Service{Name: name, Handle: h}
	defer s.Close() //nolint:errcheck // a close of a read handle.
	c, err := s.Config()
	if err != nil {
		return Config{}, err
	}
	return Config{
		DisplayName:      c.DisplayName,
		Description:      c.Description,
		StartType:        c.StartType,
		DelayedAutoStart: c.DelayedAutoStart,
	}, nil
}

// Create calls CreateService directly: mgr.CreateService puts the path and
// the arguments in quotes again.
func (windowsHost) Create(name, imagePath string, c *Config) error {
	m, err := connect()
	if err != nil {
		return err
	}
	defer m.Disconnect() //nolint:errcheck // the result of the create is the result.
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	d, err := windows.UTF16PtrFromString(c.DisplayName)
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(imagePath)
	if err != nil {
		return err
	}
	start := c.StartType
	if start == 0 {
		start = StartAuto
	}
	h, err := windows.CreateService(m.Handle, n, d, windows.SERVICE_ALL_ACCESS,
		windows.SERVICE_WIN32_OWN_PROCESS, start, windows.SERVICE_ERROR_NORMAL,
		p, nil, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(h) //nolint:errcheck // the service is made.
	desc, err := windows.UTF16PtrFromString(c.Description)
	if err != nil {
		return err
	}
	info := windows.SERVICE_DESCRIPTION{Description: desc}
	if err := windows.ChangeServiceConfig2(h, windows.SERVICE_CONFIG_DESCRIPTION, (*byte)(unsafe.Pointer(&info))); err != nil { //nolint:gosec // G103: the API takes a pointer to SERVICE_DESCRIPTION as *byte.
		return err
	}
	if !c.DelayedAutoStart {
		return nil
	}
	delayed := windows.SERVICE_DELAYED_AUTO_START_INFO{IsDelayedAutoStartUp: 1}
	return windows.ChangeServiceConfig2(h, windows.SERVICE_CONFIG_DELAYED_AUTO_START_INFO, (*byte)(unsafe.Pointer(&delayed))) //nolint:gosec // G103: the API takes a pointer to SERVICE_DELAYED_AUTO_START_INFO as *byte.
}

func (windowsHost) Delete(name string) error {
	return withService(name, func(s *mgr.Service) error {
		err := s.Delete()
		if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
			return nil
		}
		return err
	})
}

func (windowsHost) Start(name string) error {
	return withService(name, func(s *mgr.Service) error {
		err := s.Start()
		if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
			return nil
		}
		return err
	})
}

func (windowsHost) Stop(name string) error {
	return withService(name, func(s *mgr.Service) error {
		_, err := s.Control(svc.Stop)
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		return err
	})
}

func (windowsHost) ReadRecord(name string) (Record, error) {
	var r Record
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, servicesKey+name, registry.QUERY_VALUE)
	if errors.Is(err, fs.ErrNotExist) {
		return r, fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	if err != nil {
		return r, err
	}
	defer k.Close() //nolint:errcheck // a close of a read key.
	str := func(v string) (string, error) {
		// GetStringValue does not expand REG_EXPAND_SZ.
		s, _, err := k.GetStringValue(v)
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return s, err
	}
	var errs []error
	for _, f := range []struct {
		value string
		dst   *string
	}{
		{"ImagePath", &r.ImagePath},
		{ValueStrategy, &r.Strategy},
		{ValueVersion, &r.Version},
		{ValueHash, &r.Hash},
		{ValueLegacy, &r.Legacy},
	} {
		s, err := str(f.value)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", f.value, err))
		}
		*f.dst = s
	}
	games, _, err := k.GetStringsValue(ValueGames)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("read %s: %w", ValueGames, err))
	}
	r.Games = games
	return r, errors.Join(errs...)
}

func (windowsHost) WriteRecord(name string, r *Record) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, servicesKey+name, registry.SET_VALUE)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return ErrNotElevated
	}
	if err != nil {
		return err
	}
	defer k.Close() //nolint:errcheck // the values are written.
	del := func(v string) error {
		if err := k.DeleteValue(v); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	var errs []error
	for _, f := range []struct{ value, s string }{
		{ValueStrategy, r.Strategy},
		{ValueVersion, r.Version},
		{ValueHash, r.Hash},
		{ValueLegacy, r.Legacy},
	} {
		if f.s == "" {
			err = del(f.value)
		} else {
			err = k.SetStringValue(f.value, f.s)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("write %s: %w", f.value, err))
		}
	}
	if len(r.Games) == 0 {
		err = del(ValueGames)
	} else {
		err = k.SetStringsValue(ValueGames, r.Games)
	}
	if err != nil {
		errs = append(errs, fmt.Errorf("write %s: %w", ValueGames, err))
	}
	return errors.Join(errs...)
}

// netsh runs System32\netsh.exe with args and no window.
func netsh(args ...string) ([]byte, error) {
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "netsh.exe")
	cmd := exec.Command(exe, args...) //nolint:gosec // G204: the args are constants of this file.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("netsh %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return out, nil
}

func (windowsHost) TCPShowGlobal() ([]byte, error) {
	return netsh("interface", "tcp", "show", "global")
}

func (windowsHost) TCPEnableTimestamps() error {
	_, err := netsh("interface", "tcp", "set", "global", "timestamps=enabled")
	return err
}

// engineExes are the processes that KillEngines stops.
var engineExes = []string{"winws.exe", "winws2.exe"}

// killWait is the time in milliseconds that KillEngines waits for each
// process to stop.
const killWait = 5000

// EngineProcesses gives the winws.exe and winws2.exe processes of this
// computer. It does not need administrator rights.
func EngineProcesses() ([]EngineProcess, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a close of a snapshot.
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var list []EngineProcess
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		exe := windows.UTF16ToString(e.ExeFile[:])
		for _, n := range engineExes {
			if strings.EqualFold(exe, n) {
				list = append(list, EngineProcess{Exe: n, Pid: e.ProcessID})
			}
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return list, nil
}

func (windowsHost) KillEngines() (int, error) {
	procs, err := EngineProcesses()
	if err != nil {
		return 0, err
	}
	var errs []error
	killed := 0
	for _, p := range procs {
		pid := p.Pid
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
			killed++
			_, _ = windows.WaitForSingleObject(h, killWait)
		}
		_ = windows.CloseHandle(h)
	}
	return killed, errors.Join(errs...)
}
