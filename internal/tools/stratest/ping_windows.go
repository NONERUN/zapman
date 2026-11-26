package stratest

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreateFile  = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpCloseHandle = modIphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho    = modIphlpapi.NewProc("IcmpSendEcho")
)

// pingData is the data of the echo: 32 bytes, as the Ping class of .NET.
var pingData = []byte("abcdefghijklmnopqrstuvwabcdefghi")

// Ping sends one ICMP echo with IcmpSendEcho. It uses the first IPv4
// address of host.
func Ping(ctx context.Context, host string, timeout time.Duration) (time.Duration, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return 0, err
	}
	if len(ips) == 0 {
		return 0, fmt.Errorf("%s has no IPv4 address", host)
	}
	ip4 := ips[0].To4()
	if ip4 == nil {
		return 0, fmt.Errorf("%s has no IPv4 address", host)
	}
	if err := procIcmpSendEcho.Find(); err != nil {
		return 0, err
	}
	h, _, err := procIcmpCreateFile.Call()
	if windows.Handle(h) == windows.InvalidHandle {
		return 0, fmt.Errorf("IcmpCreateFile: %w", err)
	}
	defer func() { _, _, _ = procIcmpCloseHandle.Call(h) }()
	// ICMP_ECHO_REPLY is 40 bytes on 64-bit Windows; the reply also has the
	// data and 8 bytes for an ICMP error.
	reply := make([]byte, 64+len(pingData)+8)
	// IPAddr is the address in network byte order: the bytes of ip4 as
	// they are in memory.
	addr := binary.LittleEndian.Uint32(ip4)
	ms := max(timeout.Milliseconds(), 1)
	n, _, err := procIcmpSendEcho.Call(h, uintptr(addr),
		uintptr(unsafe.Pointer(&pingData[0])), uintptr(len(pingData)), 0, //nolint:gosec // G103: IcmpSendEcho reads the data buffer.
		uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), uintptr(ms)) //nolint:gosec // G103,G115: IcmpSendEcho writes the reply buffer; ms is from 1 to the timeout.
	if n == 0 {
		if errors.Is(err, windows.Errno(0)) {
			err = errors.New("no reply")
		}
		return 0, fmt.Errorf("ping %s: %w", host, err)
	}
	status := binary.LittleEndian.Uint32(reply[4:8])
	if status != 0 {
		return 0, fmt.Errorf("ping %s: ICMP status %d", host, status)
	}
	return time.Duration(binary.LittleEndian.Uint32(reply[8:12])) * time.Millisecond, nil
}
