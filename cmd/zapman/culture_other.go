//go:build !windows

package main

// uiCulture gives "": the program is for Windows only.
func uiCulture() string { return "" }
