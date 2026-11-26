//go:build !windows

package stratest

import (
	"context"
	"errors"
	"time"
)

// Ping gives an error: the ping uses IcmpSendEcho of Windows.
func Ping(context.Context, string, time.Duration) (time.Duration, error) {
	return 0, errors.New("the ping works only on Windows")
}
