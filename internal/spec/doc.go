// Package spec reads a strategy JSON file (strategies/*.json) and builds the
// argv lines of winws.exe (zapret) and winws2.exe (zapret2).
//
// The v0.1 format ("specVersion": "0.1.0") is the format of
// src/ZapretSpec/ZapretSpec.psm1. Its types and functions have the V01 prefix
// or live in v01*.go, so that a later format can sit beside it.
//
// The golden files in testdata/golden hold the argv of each strategy. The
// first line is the version of the strategy and of its games
// ("# version: flowseal 1.10.4 r2"). The Go builder owns them: after a
// change of the output, increase rev or version of each changed strategy or
// game, run go test ./internal/spec -run "Golden|Edge" -update and examine
// the diff. When the argv changes and the version line does not change,
// TestGolden fails. The PowerShell module can give a different text.
package spec
