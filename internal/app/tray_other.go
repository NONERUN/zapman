//go:build !windows

package app

import (
	"context"
	"errors"
)

var errNotWindows = errors.New("the tray task needs Windows")

// Schtasks is the TrayTask of Windows. On other systems each method gives
// an error.
type Schtasks struct{}

var _ TrayTask = Schtasks{}

// Register implements TrayTask.
func (Schtasks) Register(context.Context, string) error { return errNotWindows }

// Unregister implements TrayTask.
func (Schtasks) Unregister(context.Context) error { return errNotWindows }
