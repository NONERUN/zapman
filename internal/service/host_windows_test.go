package service

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// TestReadConfigSystem reads the configuration of system services with
// ReadConfig and with mgr.Service.Config, and compares them. It only reads:
// it opens the services for a query.
func TestReadConfigSystem(t *testing.T) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		t.Skipf("no service manager: %v", err)
	}
	defer windows.CloseServiceHandle(scm) //nolint:errcheck // a close of a read handle.
	checked := 0
	for _, name := range []string{"Dnscache", "EventLog", "Schedule", "W32Time"} {
		n, err := windows.UTF16PtrFromString(name)
		if err != nil {
			t.Fatal(err)
		}
		h, err := windows.OpenService(scm, n, windows.SERVICE_QUERY_CONFIG)
		if err != nil {
			continue
		}
		s := &mgr.Service{Name: name, Handle: h}
		want, err := s.Config()
		if err != nil {
			_ = s.Close()
			continue
		}
		// The long path keeps the display name of the key as it is, thus
		// an indirect name is not the same as the name of the SCM.
		long, err := readLongConfig(h, name)
		_ = s.Close()
		if err != nil {
			t.Fatalf("readLongConfig(%s): %v", name, err)
		}
		if long.DisplayName == "" || long.Description != want.Description ||
			long.StartType != want.StartType || long.DelayedAutoStart != want.DelayedAutoStart {
			t.Errorf("readLongConfig(%s) = %+v, mgr: %q %d %v", name, long,
				want.Description, want.StartType, want.DelayedAutoStart)
		}
		got, err := windowsHost{}.ReadConfig(name)
		if err != nil {
			t.Fatalf("ReadConfig(%s): %v", name, err)
		}
		if got.DisplayName != want.DisplayName || got.Description != want.Description ||
			got.StartType != want.StartType || got.DelayedAutoStart != want.DelayedAutoStart {
			t.Errorf("ReadConfig(%s) = %+v, mgr: %q %q %d %v", name, got,
				want.DisplayName, want.Description, want.StartType, want.DelayedAutoStart)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no system service to compare")
	}
}

// TestReadConfigMissing checks that a service that is not there gives
// ErrNotInstalled.
func TestReadConfigMissing(t *testing.T) {
	_, err := windowsHost{}.ReadConfig("zapman-test-no-such-service")
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("err = %v, want ErrNotInstalled", err)
	}
}

// TestReadConfigZapret reads the zapret service when this computer has it.
// Its ImagePath is frequently longer than the limit of QueryServiceConfig.
func TestReadConfigZapret(t *testing.T) {
	c, err := windowsHost{}.ReadConfig(Name)
	if errors.Is(err, ErrNotInstalled) {
		t.Skip("no zapret service")
	}
	if err != nil {
		t.Fatalf("ReadConfig(zapret): %v", err)
	}
	if c.DisplayName == "" {
		t.Fatalf("ReadConfig(zapret) = %+v", c)
	}
}
