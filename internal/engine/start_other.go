//go:build !windows

package engine

import (
	"errors"
	"io"
)

// StartProcess gives an error: the engines run only on Windows.
func StartProcess(_, _, _ string, _ io.Writer) (Process, error) {
	return nil, errors.New("the engines run only on Windows")
}
