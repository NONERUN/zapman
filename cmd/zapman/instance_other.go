//go:build !windows

package main

import (
	"path/filepath"
	"time"
)

// exeID gives the absolute path: the program runs only on Windows.
func exeID(path string) (string, error) { return filepath.Abs(path) }

func waitPID(_ int, _ time.Duration) {}
