# Console probe for the host. The GUI and CLI call Test-ZapretHostReady in the same process.
# Usage: cli.bat env

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')

exit (Show-ZapretHostReadyReport)
