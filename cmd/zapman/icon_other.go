//go:build !windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/NONERUN/zapman/internal/gui"
)

func setWindowIcon(_ *application.WebviewWindow, _ gui.Icon) {}
