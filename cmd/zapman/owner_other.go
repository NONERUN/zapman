//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

const dialogExStyle = 0

func setOwner(_, _ *application.WebviewWindow) {}
