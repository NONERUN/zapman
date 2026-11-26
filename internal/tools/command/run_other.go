//go:build !windows

package command

import (
	"context"
	"errors"
)

// ErrNotWindows is the error of Run on other systems.
var ErrNotWindows = errors.New("the tools work only on Windows")

// Run gives ErrNotWindows: the tools run the programs of Windows.
func Run(context.Context, string, ...string) (Result, error) {
	return Result{}, ErrNotWindows
}
