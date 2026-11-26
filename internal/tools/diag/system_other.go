//go:build !windows

package diag

import (
	"errors"

	"github.com/NONERUN/zapman/internal/service"
)

var errNotWindows = errors.New("the diagnostics work only on Windows")

// NewSystem gives a System that gives errors: the checks read Windows.
func NewSystem() System { return otherSystem{} }

type otherSystem struct{}

func (otherSystem) Services() ([]Service, error) { return nil, errNotWindows }
func (otherSystem) ServiceState(string) (service.State, error) {
	return 0, errNotWindows
}
func (otherSystem) ProcessRunning(...string) (bool, error) { return false, errNotWindows }
func (otherSystem) KillProcesses(string) error             { return errNotWindows }
func (otherSystem) Proxy() (bool, string, error)           { return false, "", errNotWindows }
func (otherSystem) SecureDNS() (bool, error)               { return false, errNotWindows }
func (otherSystem) TCPTimestamps() (bool, error)           { return false, errNotWindows }
