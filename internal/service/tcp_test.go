package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// The output of "netsh interface tcp show global" on Windows 11 (English).
const netshEnglish = "Querying active state...\r\n\r\nTCP Global Parameters\r\n" +
	"----------------------------------------------\r\n" +
	"Receive-Side Scaling State          : enabled \r\n" +
	"Receive Window Auto-Tuning Level    : normal \r\n" +
	"ECN Capability                      : disabled \r\n" +
	"RFC 1323 Timestamps                 : %s \r\n" +
	"Initial RTO                         : 1000 \r\n" +
	"Fast Open                           : enabled \r\n"

// netshRussian is the Russian output in code page 866 ("Временные метки
// RFC 1323"): only the ASCII parts are in the clear.
func netshRussian(v string) []byte {
	line := append([]byte{0x82, 0xE0, 0xA5, 0xAC, 0xA5, 0xAD, 0xAD, 0xEB, 0xA5, ' ', 0xAC, 0xA5, 0xE2, 0xAA, 0xA8},
		[]byte(" RFC 1323             : "+v+" \r\n")...)
	out := []byte{0x8F, 0xA0, 0xE0, 0xA0, 0xAC, 0xA5, 0xE2, 0xE0, 0xEB, ' ', 'T', 'C', 'P', '\r', '\n'}
	out = append(out, []byte{0x91, 0xAE, 0xE1, 0xE2, ' ', 'E', 'C', 'N', ' ', ' ', ':', ' '}...)
	out = append(out, []byte("disabled\r\n")...)
	return append(out, line...)
}

func TestParseTCPTimestamps(t *testing.T) {
	for _, c := range []struct {
		name string
		out  []byte
		want bool
	}{
		{"en enabled", []byte(strings.Replace(netshEnglish, "%s", "enabled", 1)), true},
		{"en disabled", []byte(strings.Replace(netshEnglish, "%s", "disabled", 1)), false},
		{"ru enabled", netshRussian("enabled"), true},
		{"ru disabled", netshRussian("disabled"), false},
		{"lf only", []byte("RFC 1323 Timestamps : enabled\n"), true},
		{"old name", []byte("Timestamps : Disabled\n"), false},
	} {
		got, err := ParseTCPTimestamps(c.out)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v", c.name, got, err)
		}
	}
	for name, out := range map[string]string{
		"no line": "ECN Capability : disabled\r\n",
		"value":   "RFC 1323 Timestamps : allowed\r\n",
		"empty":   "",
	} {
		if _, err := ParseTCPTimestamps([]byte(out)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTCPTimestamps(t *testing.T) {
	h := newFake()
	h.tcp = "disabled"
	h.elevated = false
	m := newManager(h)
	// The read does not need administrator rights.
	if on, err := m.TCPTimestamps(); err != nil || on {
		t.Fatalf("got %v, %v", on, err)
	}
	if err := m.EnableTCPTimestamps(); !errors.Is(err, ErrNotElevated) {
		t.Fatalf("got %v", err)
	}
	h.elevated = true
	if err := m.EnableTCPTimestamps(); err != nil || h.tcp != "enabled" {
		t.Fatalf("got %v, %s", err, h.tcp)
	}
	// On: no change.
	h.log = nil
	if err := m.EnableTCPTimestamps(); err != nil || len(h.log) != 0 {
		t.Fatalf("got %v, %v", err, h.log)
	}
	// Not known: set on.
	h.tcp = "?"
	if err := m.EnableTCPTimestamps(); err != nil || !reflect.DeepEqual(h.log, []string{"tcp on"}) {
		t.Fatalf("got %v, %v", err, h.log)
	}
}

func TestInstallTCPTimestamps(t *testing.T) {
	h := newFake()
	h.tcp = "disabled"
	m := newManager(h)
	if err := m.Install(testPlan()); err != nil {
		t.Fatal(err)
	}
	if len(h.log) == 0 || h.log[0] != "tcp on" || h.tcp != "enabled" {
		t.Errorf("log: %v", h.log)
	}

	// The set fails: no change of the old service.
	h = newFake()
	h.tcp = "disabled"
	h.tcpFails = true
	h.add(Name, Running, oldRecord())
	m = newManager(h)
	if err := m.Install(testPlan()); err == nil || !strings.Contains(err.Error(), "TCP timestamps") {
		t.Fatalf("got %v", err)
	}
	if !reflect.DeepEqual(h.log, []string{"tcp on"}) || h.services[Name].status.State != Running {
		t.Errorf("log: %v", h.log)
	}

	// Start sets them on too.
	h = newFake()
	h.tcp = "disabled"
	h.add(Name, Stopped, oldRecord())
	m = newManager(h)
	if err := m.Start(); err != nil || h.tcp != "enabled" {
		t.Fatalf("start: %v, %s", err, h.tcp)
	}
}
