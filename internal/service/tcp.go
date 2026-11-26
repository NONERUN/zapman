package service

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
)

// The TCP timestamps (RFC 1323). The fooling=ts of 12 stock strategies
// needs them. Windows has them off by default. Windows has no documented
// API or registry value for the active state, thus the Host reads the
// output of "netsh interface tcp show global".

// tcpTimestampsLine finds the line of the English ("RFC 1323 Timestamps")
// and the Russian ("... RFC 1323") netsh output.
var tcpTimestampsLine = regexp.MustCompile(`(?i)timestamps|rfc\s*1323`)

// ParseTCPTimestamps gives the timestamps state from the output of
// "netsh interface tcp show global". The output can be in an OEM code
// page: only the ASCII parts are examined. A missing line or an unknown
// value gives an error.
func ParseTCPTimestamps(out []byte) (bool, error) {
	for _, line := range bytes.Split(out, []byte("\n")) {
		if !tcpTimestampsLine.Match(line) {
			continue
		}
		i := bytes.LastIndexByte(line, ':')
		if i < 0 {
			continue
		}
		switch v := string(bytes.ToLower(bytes.TrimSpace(line[i+1:]))); v {
		case "enabled":
			return true, nil
		case "disabled":
			return false, nil
		default:
			return false, fmt.Errorf("the TCP timestamps value %q of netsh is not known", v)
		}
	}
	return false, errors.New("the output of netsh has no TCP timestamps line")
}

// TCPTimestamps tells if the TCP timestamps are on. It does not need
// administrator rights.
func (m *Manager) TCPTimestamps() (bool, error) {
	out, err := m.Host.TCPShowGlobal()
	if err != nil {
		return false, fmt.Errorf("read the TCP settings: %w", err)
	}
	return ParseTCPTimestamps(out)
}

// EnableTCPTimestamps sets the TCP timestamps on when they are off. When
// the state is not known, it sets them on. Install and Start call it; the
// caller calls it before a foreground run.
func (m *Manager) EnableTCPTimestamps() error {
	if !m.Host.Elevated() {
		return ErrNotElevated
	}
	if on, err := m.TCPTimestamps(); err == nil && on {
		return nil
	}
	if err := m.Host.TCPEnableTimestamps(); err != nil {
		return fmt.Errorf("set the TCP timestamps on: %w", err)
	}
	return nil
}
