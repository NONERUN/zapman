// Package spec reads a strategy JSON file (strategies/*.json) and builds the
// argv lines of winws.exe (zapret) and winws2.exe (zapret2).
//
// The v0.1 format ("specVersion": "0.1.0") is the format of
// src/ZapretSpec/ZapretSpec.psm1. Its types and functions have the V01 prefix
// or live in v01*.go, so that a later format can sit beside it.
//
// The output must be the same text as the PowerShell module. The golden files
// in testdata/golden hold that text. Do not change the output here without a
// change of the golden files.
package spec
